// Package memory は、application層の保存ポートをmapで実装するメモリアダプターです。
//
// Repositoryの契約と保存方式を切り離す例です。ユースケースがこのパッケージを
// importする必要はなく、main.goからinterfaceを通して実装を渡します。
// データはこのプロセスだけに存在し、再起動で消えます。永続DBに差し替える際は、
// 重複防止・集約のコピー・原子的な更新など、ポートに定義された契約を維持してください。
package memory

import (
	"context"
	"sort"
	"sync"

	"try_golang/internal/application"
	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

// Store は、Users・Sessions・TaskRepositoryの三つのポートを満たす保存アダプターです。
//
// 一つの具体型が複数のinterfaceを実装していても、ユースケースは必要なポートだけを使います。
// mapはそのままでは同時読み書きできないため、RWMutexで読み取りと書き込みを保護します。
// UserとTaskは値として保存・取得し、取得したコピーの変更だけでは保存状態を変えません。
// 使用する前にNewでmapとClockを初期化し、生成後のStore自体はコピーしないでください。
type Store struct {
	mu       sync.RWMutex                   // 全mapを保護します。参照はRLock、更新はLockを使います。
	users    map[string]account.User        // User IDをキーにした集約の保存先です。
	emailIDs map[string]string              // 正規化したメールからUser IDを引くための索引です。
	sessions map[string]application.Session // 生のトークンではなくハッシュをキーにします。
	tasks    map[string]task.Task           // Task IDをキーにした集約の保存先です。
	now      application.Clock              // 期限切れセッションを掃除する時刻を取得します。
}

// 次の代入は実行時に使う値ではなく、interfaceを満たすかのコンパイル時チェックです。
// ポートのメソッドが変更されたとき、アダプターの実装漏れをここで検出できます。
var (
	_ application.Users          = (*Store)(nil)
	_ application.Sessions       = (*Store)(nil)
	_ application.TaskRepository = (*Store)(nil)
)

// New は、空の保存領域とセッション期限の確認に使うClockを持つStoreを生成します。
// 本番ではtime.Now、テストでは固定時刻を返す関数を渡します。
func New(now application.Clock) *Store {
	return &Store{users: map[string]account.User{}, emailIDs: map[string]string{}, sessions: map[string]application.Session{}, tasks: map[string]task.Task{}, now: now}
}

// InsertUser は、ID・メールの重複を確認してUserを保存します。
// 確認から二つのmapへの登録まで同じ書き込みロック内で行うため、
// 同時に同じメールを登録しても一件しか成功しません。重複時はapplication.ErrConflictです。
func (s *Store) InsertUser(ctx context.Context, user account.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.emailIDs[user.Email().String()]; ok {
		return application.ErrConflict
	}
	if _, ok := s.users[user.ID()]; ok {
		return application.ErrConflict
	}
	s.users[user.ID()] = user
	s.emailIDs[user.Email().String()] = user.ID()
	return nil
}

// UserByEmail は、メールの索引を使ってUserのコピーを返します。
// 全Userを順番に調べずIDを引けるように、InsertUserが索引も同時に更新します。
// 未登録ならapplication.ErrNotFoundを返します。
func (s *Store) UserByEmail(ctx context.Context, email account.Email) (account.User, error) {
	if err := ctx.Err(); err != nil {
		return account.User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.emailIDs[email.String()]
	if !ok {
		return account.User{}, application.ErrNotFound
	}
	return s.users[id], nil
}

// UserByID は、IDに対応するUserのコピーを返します。未存在ならapplication.ErrNotFoundです。
// 読み取り専用のRLockを使うため、他の読み取りとは並行して実行できます。
func (s *Store) UserByID(ctx context.Context, id string) (account.User, error) {
	if err := ctx.Err(); err != nil {
		return account.User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return account.User{}, application.ErrNotFound
	}
	return user, nil
}

// SaveSession は、期限切れの記録を掃除してから新しいSessionを保存します。
// 同じTokenHashの記録が残っていればapplication.ErrConflictです。
// 定期実行する掃除処理は持たず、新規ログインの保存時に不要な記録を取り除きます。
func (s *Store) SaveSession(ctx context.Context, session application.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, item := range s.sessions {
		if !now.Before(item.ExpiresAt) {
			delete(s.sessions, key)
		}
	}
	if _, ok := s.sessions[session.TokenHash]; ok {
		return application.ErrConflict
	}
	s.sessions[session.TokenHash] = session
	return nil
}

// FindSession は、トークンのハッシュに対応するSessionを返します。
// ここでは有効期限を判定せず、未存在ならapplication.ErrNotFoundを返します。
// 取得できたSessionが利用可能かどうかはAuth.Authenticateが確認します。
func (s *Store) FindSession(ctx context.Context, hash string) (application.Session, error) {
	if err := ctx.Err(); err != nil {
		return application.Session{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[hash]
	if !ok {
		return application.Session{}, application.ErrNotFound
	}
	return session, nil
}

// DeleteSession は、指定ハッシュの記録を削除します。既に未存在でも成功します。
// 同じ削除を繰り返しても結果が変わらない性質を、冪等性と呼びます。
// この冪等性は保存ポートの性質であり、HTTPの認証済みログアウトの応答とは区別します。
func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, hash)
	return nil
}

// InsertTask は、ドメインで生成したTaskをID単位で新規保存します。
// 同じIDがあればapplication.ErrConflictを返し、既存の集約を上書きしません。
func (s *Store) InsertTask(ctx context.Context, item task.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[item.ID()]; ok {
		return application.ErrConflict
	}
	s.tasks[item.ID()] = item
	return nil
}

// FindTask は、Taskのコピーを返します。未存在ならapplication.ErrNotFoundです。
// コピーのRenameやCompleteだけでは保存済みTaskは変わらず、UpdateTaskによる保存が必要です。
// ここでは所有者を判定せず、取得後の認可はTasksユースケースが担当します。
func (s *Store) FindTask(ctx context.Context, id string) (task.Task, error) {
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.tasks[id]
	if !ok {
		return task.Task{}, application.ErrNotFound
	}
	return item, nil
}

// ListTasks は、指定所有者のTaskのコピーを作成日時・IDの順で返します。
// mapの走査順は一定ではないため、並べ替えて応答の順番を安定させます。
// 未存在の場合もnilではなく空のスライスを返します。
func (s *Store) ListTasks(ctx context.Context, owner string) ([]task.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := []task.Task{}
	for _, item := range s.tasks {
		if item.OwnerID() == owner {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt().Equal(items[j].CreatedAt()) {
			return items[i].ID() < items[j].ID()
		}
		return items[i].CreatedAt().Before(items[j].CreatedAt())
	})
	return items, nil
}

// UpdateTask は、集約の取得・changeの実行・保存を一つの書き込みロック内で行います。
//
// まずコピーにchangeを適用し、成功時だけmapへ戻すことで、失敗時に変更が残るのを防ぎます。
// 認可やドメイン操作も同じ更新単位で実行され、同時完了でも古い状態を上書きしません。
// changeは短い集約操作に限定し、同じStoreのメソッドや外部I/Oを呼ばないでください。
// ロックを保持したまま呼ぶため、ロックの再取得や長い待ち時間が問題になります。
func (s *Store) UpdateTask(ctx context.Context, id string, change func(*task.Task) error) (task.Task, error) {
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tasks[id]
	if !ok {
		return task.Task{}, application.ErrNotFound
	}
	if err := change(&item); err != nil {
		return task.Task{}, err
	}
	// changeが成功した場合にだけ保存する位置が、この実装での更新確定の境界です。
	s.tasks[id] = item
	return item, nil
}

// DeleteTask は、所有者確認と削除を同じ書き込みロック内で行います。
// 未存在と所有者不一致はapplication.ErrNotFoundです。完了状態は削除条件に含めません。
func (s *Store) DeleteTask(ctx context.Context, id, owner string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tasks[id]
	if !ok || item.OwnerID() != owner {
		return application.ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}
