package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"try_golang/internal/application"
)

type brokenSessions struct {
	*store
	err error
}

func (s brokenSessions) SaveSession(context.Context, application.Session) error { return s.err }
func (s brokenSessions) FindSession(context.Context, string) (application.Session, error) {
	return application.Session{}, s.err
}
func (s brokenSessions) DeleteSession(context.Context, string) error { return s.err }

type brokenPasswords struct {
	passwords
	err error
}

func (p *brokenPasswords) Hash(raw string) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.passwords.Hash(raw)
}

func TestAuthFailsClosedWhenDependenciesFail(t *testing.T) {
	ctx := context.Background()
	auth, s, p, now, ids := fixture(t)
	auth.Register(ctx, "alice@example.com", "long-password")
	outage := errors.New("storage unavailable")
	down, err := application.NewAuth(s, brokenSessions{s, outage}, p, ids, func() time.Time { return *now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := down.Login(ctx, "alice@example.com", "long-password")
	if !errors.Is(err, outage) || token.AccessToken != "" {
		t.Fatalf("issued token while storage down: %v", err)
	}
	if _, err := down.Authenticate(ctx, "opaque-token"); !errors.Is(err, outage) {
		t.Fatalf("masked outage as bad credentials: %v", err)
	}
	if err := down.Logout(ctx, "opaque-token"); !errors.Is(err, outage) {
		t.Fatalf("reported successful logout during outage: %v", err)
	}
	if err := auth.Logout(ctx, ""); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatal(err)
	}
	if _, err := application.NewAuth(s, s, p, ids, func() time.Time { return *now }, 0); err == nil {
		t.Fatal("accepted nonpositive TTL")
	}
}

func TestRegistrationDoesNotSaveWhenHashOrIDGenerationFails(t *testing.T) {
	ctx := context.Background()
	_, s, _, now, ids := fixture(t)
	failure := errors.New("crypto unavailable")
	hasher := &brokenPasswords{err: failure}
	if _, err := application.NewAuth(s, s, hasher, ids, func() time.Time { return *now }, time.Hour); !errors.Is(err, failure) {
		t.Fatalf("ignored dummy hash failure: %v", err)
	}
	hasher.err = nil
	auth, err := application.NewAuth(s, s, hasher, ids, func() time.Time { return *now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	hasher.err = failure
	if _, err = auth.Register(ctx, "alice@example.com", "long-password"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	hasher.err = nil
	badIDs := func() (string, error) { return "", failure }
	auth, err = application.NewAuth(s, s, hasher, badIDs, func() time.Time { return *now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = auth.Register(ctx, "alice@example.com", "long-password"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if len(s.users) != 0 {
		t.Fatal("partial registration saved")
	}
	tasks := application.NewTasks(s, badIDs, func() time.Time { return *now })
	if _, err = tasks.Create(ctx, "alice", "title"); !errors.Is(err, failure) || len(s.tasks) != 0 {
		t.Fatalf("partial task saved: %v", err)
	}
}

func TestLogoutRevokesOnlyCurrentSessionAndDeletedUserCannotAuthenticate(t *testing.T) {
	ctx := context.Background()
	auth, s, _, _, _ := fixture(t)
	user, err := auth.Register(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	first, err := auth.Login(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := auth.Login(ctx, "alice@example.com", "long-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.Logout(ctx, first.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.Authenticate(ctx, first.AccessToken); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatal(err)
	}
	if _, err = auth.Authenticate(ctx, second.AccessToken); err != nil {
		t.Fatalf("revoked another session: %v", err)
	}
	delete(s.users, user.ID())
	if _, err = auth.Authenticate(ctx, second.AccessToken); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("authenticated deleted user: %v", err)
	}
}
