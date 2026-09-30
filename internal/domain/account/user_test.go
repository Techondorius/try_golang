package account_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"try_golang/internal/domain/account"
)

func TestEmail(t *testing.T) {
	email, err := account.NewEmail("  Alice@Example.COM ")
	if err != nil || email.String() != "alice@example.com" {
		t.Fatalf("email = %v, err = %v", email, err)
	}
	for _, raw := range []string{"", "alice", "Alice <alice@example.com>", "a@", "a b@example.com", strings.Repeat("a", 255) + "@example.com"} {
		if _, err := account.NewEmail(raw); !errors.Is(err, account.ErrInvalidEmail) {
			t.Errorf("accepted %q: %v", raw, err)
		}
	}
}

func TestUserRequiresIdentityEmailAndHash(t *testing.T) {
	email, _ := account.NewEmail("alice@example.com")
	now := time.Now()
	user, err := account.NewUser("user", email, "password-hash", now)
	if err != nil || user.ID() != "user" || user.Email() != email || user.PasswordHash() != "password-hash" || !user.CreatedAt().Equal(now) {
		t.Fatalf("user: %#v %v", user, err)
	}
	for _, input := range []struct {
		id    string
		email account.Email
		hash  string
	}{{"", email, "hash"}, {"id", account.Email{}, "hash"}, {"id", email, ""}} {
		if _, err := account.NewUser(input.id, input.email, input.hash, now); err == nil {
			t.Fatal("invalid user aggregate accepted")
		}
	}
}

func TestPasswordPolicy(t *testing.T) {
	for _, raw := range []string{"", "short", strings.Repeat("a", 73)} {
		if err := account.ValidatePassword(raw); !errors.Is(err, account.ErrInvalidPassword) {
			t.Errorf("accepted invalid password: %v", err)
		}
	}
	for _, raw := range []string{"long-password", strings.Repeat("a", 72)} {
		if err := account.ValidatePassword(raw); err != nil {
			t.Fatal(err)
		}
	}
}
