package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"try_golang/internal/application"
)

// brokenSessions は、保存ポートの呼び出しを意図的に失敗させるテスト用アダプターです。
// 保存障害を認証成功やログアウト成功として扱わないことを、実際の障害なしに検証します。
type brokenSessions struct {
	*store
	err error
}

func (s brokenSessions) SaveSession(context.Context, application.Session) error { return s.err }
func (s brokenSessions) FindSession(context.Context, string) (application.Session, error) {
	return application.Session{}, s.err
}
func (s brokenSessions) DeleteSession(context.Context, string) error { return s.err }

// brokenPasswords は、Passwordsの基本Fakeを埋め込み、Hashだけを失敗させる型です。
// 埋め込みでVerifyを再利用し、errを切り替えて初期化時と登録時の障害を個別に検証します。
type brokenPasswords struct {
	passwords
	err error
}

// Hash は、errが設定されていれば失敗し、なければ基本Fakeへ委譲します。
func (p *brokenPasswords) Hash(raw string) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.passwords.Hash(raw)
}

func TestAuthFailsClosedWhenDependenciesFail(t *testing.T) {
	ctx := context.Background()
	auth, s, p, now, ids := fixture(t)
	auth.RegisterUseCase(ctx, "alice@example.com", "long-password")
	outage := errors.New("storage unavailable")
	down, err := application.NewAuthUseCase(s, brokenSessions{s, outage}, p, ids, func() time.Time { return *now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := down.LoginUseCase(ctx, "alice@example.com", "long-password")
	if !errors.Is(err, outage) || token.AccessToken != "" {
		t.Fatalf("issued token while storage down: %v", err)
	}
	if _, err := down.AuthenticateUseCase(ctx, "opaque-token"); !errors.Is(err, outage) {
		t.Fatalf("masked outage as bad credentials: %v", err)
	}
	if err := down.LogoutUseCase(ctx, "opaque-token"); !errors.Is(err, outage) {
		t.Fatalf("reported successful logout during outage: %v", err)
	}
	if err := auth.LogoutUseCase(ctx, ""); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatal(err)
	}
	if _, err := application.NewAuthUseCase(s, s, p, ids, func() time.Time { return *now }, 0); err == nil {
		t.Fatal("accepted nonpositive TTL")
	}
}

func TestRegistrationDoesNotSaveWhenHashOrIDGenerationFails(t *testing.T) {
	ctx := context.Background()
	_, s, _, now, ids := fixture(t)
	failure := errors.New("crypto unavailable")
	hasher := &brokenPasswords{err: failure}
	if _, err := application.NewAuthUseCase(s, s, hasher, ids, func() time.Time { return *now }, time.Hour); !errors.Is(err, failure) {
		t.Fatalf("ignored dummy hash failure: %v", err)
	}
	hasher.err = nil
	auth, err := application.NewAuthUseCase(s, s, hasher, ids, func() time.Time { return *now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	hasher.err = failure
	if _, err = auth.RegisterUseCase(ctx, "alice@example.com", "long-password"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	hasher.err = nil
	badIDs := func() (string, error) { return "", failure }
	auth, err = application.NewAuthUseCase(s, s, hasher, badIDs, func() time.Time { return *now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = auth.RegisterUseCase(ctx, "alice@example.com", "long-password"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if len(s.users) != 0 {
		t.Fatal("partial registration saved")
	}
	tasks := application.NewTasksUseCase(s, badIDs, func() time.Time { return *now })
	if _, err = tasks.CreateUseCase(ctx, "alice", "title"); !errors.Is(err, failure) || len(s.tasks) != 0 {
		t.Fatalf("partial task saved: %v", err)
	}
}

func TestLogoutRevokesOnlyCurrentSessionAndDeletedUserCannotAuthenticate(t *testing.T) {
	ctx := context.Background()
	auth, s, _, _, _ := fixture(t)
	user, err := auth.RegisterUseCase(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	first, err := auth.LoginUseCase(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := auth.LoginUseCase(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.LogoutUseCase(ctx, first.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.AuthenticateUseCase(ctx, first.AccessToken); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatal(err)
	}
	if _, err = auth.AuthenticateUseCase(ctx, second.AccessToken); err != nil {
		t.Fatalf("revoked another session: %v", err)
	}
	delete(s.users, user.ID())
	if _, err = auth.AuthenticateUseCase(ctx, second.AccessToken); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("authenticated deleted user: %v", err)
	}
}
