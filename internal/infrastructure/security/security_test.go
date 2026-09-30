package security_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"try_golang/internal/infrastructure/security"
)

func TestBcrypt(t *testing.T) {
	p := security.Bcrypt{Cost: bcrypt.MinCost}
	hash, err := p.Hash("long-password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "long-password" || !p.Verify(hash, "long-password") || p.Verify(hash, "wrong-password") || p.Verify("broken", "long-password") {
		t.Fatal("password verification failed")
	}
	other, err := p.Hash("long-password")
	if err != nil || hash == other {
		t.Fatal("missing random salt")
	}
	if _, err := p.Hash(strings.Repeat("a", 73)); err == nil {
		t.Fatal("accepted oversized password")
	}
}

func TestRandomIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := security.RandomID()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.RawURLEncoding.DecodeString(id)
		if err != nil || len(raw) != 32 || seen[id] {
			t.Fatalf("invalid or duplicate ID: %s", id)
		}
		seen[id] = true
	}
}
