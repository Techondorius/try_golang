package security

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"

	"try_golang/internal/application"
)

type Bcrypt struct{ Cost int }

var _ application.Passwords = Bcrypt{}

func (b Bcrypt) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), b.Cost)
	return string(hash), err
}
func (b Bcrypt) Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// RandomID creates 256-bit IDs and opaque bearer tokens using the OS CSPRNG.
func RandomID() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
