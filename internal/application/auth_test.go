package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"try_golang/internal/application"
	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

// store は、ユースケースのテストで保存ポートを満たすFakeです。
// Fakeは、本物より単純な実装で依存先の役割を代行するテスト用の部品です。
// このFakeはテストに必要な保存・検索だけを持ち、並行実行や永続保存は検証しません。
// 本番Storeのロックや整合性は、infrastructure/memoryのテストで別途確認します。
type store struct {
	users    map[string]account.User
	sessions map[string]application.Session
	tasks    map[string]task.Task
}

// newStore は、テストごとに独立した保存状態を作り、他のテストとの状態共有を避けます。
func newStore() *store {
	return &store{users: map[string]account.User{}, sessions: map[string]application.Session{}, tasks: map[string]task.Task{}}
}
func (s *store) InsertUser(_ context.Context, u account.User) error {
	for _, existing := range s.users {
		if existing.Email() == u.Email() {
			return application.ErrConflict
		}
	}
	s.users[u.ID()] = u
	return nil
}
func (s *store) UserByEmail(_ context.Context, e account.Email) (account.User, error) {
	for _, u := range s.users {
		if u.Email() == e {
			return u, nil
		}
	}
	return account.User{}, application.ErrNotFound
}
func (s *store) UserByID(_ context.Context, id string) (account.User, error) {
	u, ok := s.users[id]
	if !ok {
		return u, application.ErrNotFound
	}
	return u, nil
}
func (s *store) SaveSession(_ context.Context, session application.Session) error {
	s.sessions[session.TokenHash] = session
	return nil
}
func (s *store) FindSession(_ context.Context, hash string) (application.Session, error) {
	v, ok := s.sessions[hash]
	if !ok {
		return v, application.ErrNotFound
	}
	return v, nil
}
func (s *store) DeleteSession(_ context.Context, hash string) error {
	delete(s.sessions, hash)
	return nil
}
func (s *store) InsertTask(_ context.Context, v task.Task) error { s.tasks[v.ID()] = v; return nil }
func (s *store) FindTask(_ context.Context, id string) (task.Task, error) {
	v, ok := s.tasks[id]
	if !ok {
		return v, application.ErrNotFound
	}
	return v, nil
}
func (s *store) ListTasks(_ context.Context, owner string) ([]task.Task, error) {
	out := []task.Task{}
	for _, v := range s.tasks {
		if v.OwnerID() == owner {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *store) UpdateTask(ctx context.Context, id string, change func(*task.Task) error) (task.Task, error) {
	v, err := s.FindTask(ctx, id)
	if err != nil {
		return v, err
	}
	if err = change(&v); err != nil {
		return task.Task{}, err
	}
	s.tasks[id] = v
	return v, nil
}
func (s *store) DeleteTask(ctx context.Context, id, owner string) error {
	v, err := s.FindTask(ctx, id)
	if err != nil {
		return err
	}
	if v.OwnerID() != owner {
		return application.ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}

// passwords は、照合処理が呼ばれた回数も数える、テスト専用のPasswords実装です。
// 文字列に平文を含む単純な照合は、ユースケースの分岐を高速に検証するためだけに使います。
// 暗号学的な安全性はありません。本番にはsecurity.Bcryptを注入します。
type passwords struct{ verifications int }

// Hash は、FakeのVerifyと対応する照合用文字列を作ります。安全なハッシュ化ではありません。
func (*passwords) Hash(raw string) (string, error) { return "hash:" + raw, nil }

// Verify は、照合回数を記録して入力を比較します。未登録ユーザーでも呼ばれるかを確認できます。
func (p *passwords) Verify(hash, raw string) bool { p.verifications++; return hash == "hash:"+raw }

// fixture は、AuthへFakeの保存・照合・ID生成・時計をDIしたテスト環境を組み立てます。
// 返した時刻の値を進めると、Authが読むClockも変わるため、期限切れを待たずに検証できます。
func fixture(t *testing.T) (*application.Auth, *store, *passwords, *time.Time, application.Generator) {
	t.Helper()
	s := newStore()
	p := &passwords{}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	sequence := 0
	ids := func() (string, error) { sequence++; return fmt.Sprintf("opaque-%d", sequence), nil }
	auth, err := application.NewAuthUseCase(s, s, p, ids, func() time.Time { return now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return auth, s, p, &now, ids
}

func TestRegistrationLoginExpiryAndLogout(t *testing.T) {
	ctx := context.Background()
	auth, s, _, now, _ := fixture(t)
	user, err := auth.RegisterUseCase(ctx, "Alice@Example.com", "long-password")
	if err != nil || user.Email().String() != "alice@example.com" {
		t.Fatalf("register: %v", err)
	}
	if user.PasswordHash() == "long-password" {
		t.Fatal("stored plaintext password")
	}
	if _, err = auth.RegisterUseCase(ctx, "alice@example.com", "long-password"); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	token, err := auth.LoginUseCase(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	if !token.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatal("incorrect expiration")
	}
	for hash, session := range s.sessions {
		if hash == token.AccessToken || session.TokenHash == token.AccessToken {
			t.Fatal("stored raw bearer token")
		}
	}
	got, err := auth.AuthenticateUseCase(ctx, token.AccessToken)
	if err != nil || got.ID() != user.ID() {
		t.Fatalf("authenticate: %v", err)
	}
	*now = now.Add(time.Hour)
	if _, err = auth.AuthenticateUseCase(ctx, token.AccessToken); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("expired token: %v", err)
	}
	token, err = auth.LoginUseCase(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.LogoutUseCase(ctx, token.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.AuthenticateUseCase(ctx, token.AccessToken); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("revoked token: %v", err)
	}
}

func TestBadCredentialsAndUnknownToken(t *testing.T) {
	auth, _, p, _, _ := fixture(t)
	ctx := context.Background()
	auth.RegisterUseCase(ctx, "alice@example.com", "long-password")
	for _, email := range []string{"alice@example.com", "unknown@example.com"} {
		before := p.verifications
		if _, err := auth.LoginUseCase(ctx, email, "wrong-password"); !errors.Is(err, application.ErrUnauthenticated) {
			t.Fatalf("login %s: %v", email, err)
		}
		if p.verifications != before+1 {
			t.Fatal("unknown user skipped password verification")
		}
	}
	for _, token := range []string{"", "unknown"} {
		if _, err := auth.AuthenticateUseCase(ctx, token); !errors.Is(err, application.ErrUnauthenticated) {
			t.Fatalf("token: %v", err)
		}
	}
	for _, credentials := range [][2]string{{"bad", "long-password"}, {"a@example.com", "short"}} {
		if _, err := auth.RegisterUseCase(ctx, credentials[0], credentials[1]); err == nil {
			t.Fatal("invalid registration accepted")
		}
	}
}

func TestTaskOwnershipAndLifecycle(t *testing.T) {
	_, s, _, now, ids := fixture(t)
	ctx := context.Background()
	service := application.NewTasksUseCase(s, ids, func() time.Time { return *now })
	v, err := service.CreateUseCase(ctx, "alice", "First task")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.GetUseCase(ctx, "bob", v.ID()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
	if _, err = service.RenameUseCase(ctx, "bob", v.ID(), "stolen"); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("foreign rename: %v", err)
	}
	if _, err = service.CompleteUseCase(ctx, "bob", v.ID()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("foreign complete: %v", err)
	}
	if err = service.DeleteUseCase(ctx, "bob", v.ID()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	list, err := service.ListUseCase(ctx, "bob")
	if err != nil || len(list) != 0 {
		t.Fatalf("foreign list: %v %v", list, err)
	}
	v, err = service.RenameUseCase(ctx, "alice", v.ID(), "Renamed")
	if err != nil || v.Title() != "Renamed" {
		t.Fatalf("rename: %v", err)
	}
	v, err = service.CompleteUseCase(ctx, "alice", v.ID())
	if err != nil || v.Status() != task.Completed {
		t.Fatalf("complete: %v", err)
	}
	if _, err = service.CompleteUseCase(ctx, "alice", v.ID()); !errors.Is(err, task.ErrCompleted) {
		t.Fatalf("repeat: %v", err)
	}
	if err = service.DeleteUseCase(ctx, "alice", v.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.GetUseCase(ctx, "alice", v.ID()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if _, err = service.CreateUseCase(ctx, "", "title"); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("anonymous create: %v", err)
	}
}
