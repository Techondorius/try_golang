// Package application は、ドメインの操作を組み合わせて利用者の目的を実現するユースケース層です。
//
// ドメインが「タスクを完了できる条件」を持つのに対し、この層は「誰が操作するか」を確認し、
// 対象を取得してドメインの操作を呼び、結果を保存します。Ginや具体的な保存方式には依存しません。
//
// 外部に頼みたい処理は、この層が所有するinterface（ポート）として定義します。
// infrastructure層の実装（アダプター）がポートを満たし、main.goで組み立てられます。
// これが依存性逆転です。テストでは同じポートをFakeで満たし、実時間やDBに頼らず検証できます。
package application

import (
	"context"
	"errors"
	"time"

	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

var (
	// ErrNotFound は、対象が存在しない場合のエラーです。
	// タスクでは、他ユーザーの対象を見つけた場合もこのエラーにして存在を漏らしません。
	ErrNotFound = errors.New("resource not found")
	// ErrConflict は、メールやIDの重複など、保存時の競合を表します。
	ErrConflict = errors.New("resource already exists")
	// ErrUnauthenticated は、認証情報がない・不正・期限切れなどで利用者を確認できないことを表します。
	ErrUnauthenticated = errors.New("authentication required")
)

// Clock は、現在時刻を取得するための関数型のポートです。
// 本番ではtime.Now、テストでは固定・変更可能な時刻を返す関数を渡します。
// 期限切れをテストするために実際の一時間を待つ必要がなくなります。
type Clock func() time.Time

// Generator は、IDや認証トークンを生成するための関数型のポートです。
// 本番では暗号学的な乱数、テストでは順番に値を返すFakeなどを使います。
// 同じポートがトークン生成にも使われるため、本番の値には十分な推測困難性が必要です。
type Generator func() (string, error)

// Users は、User集約を保存・取得するためのポートです。
//
// ユースケースに必要な操作を列挙し、SQLやmapといった実装の詳細を隠しています。
// 異なるUser間のメールの一意性は、単一集約だけでは判定できないため、このポートの契約にします。
type Users interface {
	// InsertUser はUserを新規保存します。IDまたはメールの重複はErrConflictを返します。
	// 重複の確認と保存は一体として行い、同時登録でも一件だけ成功させます。
	InsertUser(ctx context.Context, user account.User) error
	// UserByEmail は、正規化済みのEmailでUserを取得します。未登録ならErrNotFoundです。
	UserByEmail(ctx context.Context, email account.Email) (account.User, error)
	// UserByID は、集約のIDでUserを取得します。未登録ならErrNotFoundです。
	UserByID(ctx context.Context, id string) (account.User, error)
}

// Passwords は、パスワードのハッシュ化と照合を抽象化するポートです。
// ハッシュの方式をユースケースから切り離し、本番のbcryptをテストでFakeに置き換えられます。
// 平文を保存する実装はこのポートの意図を満たしません。
type Passwords interface {
	// Hash は、平文から保存用のハッシュを生成します。暗号処理に失敗したらエラーを返します。
	Hash(password string) (string, error)
	// Verify は、保存済みハッシュと入力パスワードを照合します。
	// 不一致や不正なハッシュにはfalseを返し、認証失敗として扱います。
	Verify(hash, password string) bool
}

// Session は、ログインによって作成されるサーバー側の認証記録です。
//
// UserやTaskの業務上の集約とは別に、application層の認証処理を支えるデータです。
// トークンの平文を保存せず、ハッシュを検索キーとして利用します。
// 期限切れの判定はAuthが行い、Sessionsポートは保存・取得・削除を担当します。
type Session struct {
	// TokenHash は、BearerトークンのSHA-256ハッシュです。平文のトークンではありません。
	TokenHash string
	// UserID は、このセッションで認証されるUser集約のIDです。
	UserID string
	// ExpiresAt は有効期限です。この時刻と同じ時刻になった時点で失効します。
	ExpiresAt time.Time
}

// Sessions は、認証記録を保存・検索・削除するポートです。
// サーバー側に記録を持つ方式なので、記録の削除で個別のログアウトを実現できます。
type Sessions interface {
	// SaveSession は認証記録を新規保存します。同じTokenHashの重複はErrConflictです。
	SaveSession(ctx context.Context, session Session) error
	// FindSession はハッシュで認証記録を取得します。未存在ならErrNotFoundです。
	// 取得できても有効期限内とは限らないため、呼び出し側が期限を確認します。
	FindSession(ctx context.Context, tokenHash string) (Session, error)
	// DeleteSession は指定ハッシュの記録を失効させます。既に未存在なら成功として扱います。
	DeleteSession(ctx context.Context, tokenHash string) error
}

// TaskRepository は、Task集約を単位として保存・取得するRepositoryのポートです。
//
// Repositoryは、集約を取り出して操作し、再び保存するための窓口です。
// 個々のカラムを直接操作するAPIにせず、Taskのルールを通して変更する形にしています。
// 取得した値の変更だけでは保存済み状態は変わらず、UpdateTaskを通した成功時だけ保存します。
type TaskRepository interface {
	// InsertTask は新規Taskを保存します。同じIDが存在する場合はErrConflictです。
	InsertTask(ctx context.Context, item task.Task) error
	// FindTask はIDでTaskのスナップショットを取得します。未存在ならErrNotFoundです。
	// 所有者の認可はこの取得操作では行わず、Tasks.GetUseCaseが確認します。
	FindTask(ctx context.Context, id string) (task.Task, error)
	// ListTasks は指定所有者のTaskだけを、作成日時・IDの順で返します。
	// 対象がなければ空の一覧を返します。
	ListTasks(ctx context.Context, ownerID string) ([]task.Task, error)
	// UpdateTask は取得・changeの実行・保存を原子的に行います。
	// changeは所有者の確認やドメイン操作を行い、エラーなら変更を保存しません。
	// 同時更新で古い状態を上書きしないことが契約です。SQL実装ではトランザクションや
	// バージョン番号などで、このメモリ実装と同じ性質を保証する必要があります。
	UpdateTask(ctx context.Context, id string, change func(*task.Task) error) (task.Task, error)
	// DeleteTask は所有者の確認と削除を原子的に行います。
	// 未存在と所有者不一致はどちらもErrNotFoundにし、他人のTaskの存在を漏らしません。
	DeleteTask(ctx context.Context, id, ownerID string) error
}
