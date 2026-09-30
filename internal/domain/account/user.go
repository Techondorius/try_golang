// Package account contains the user aggregate and its value objects.
package account

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("password must be 12 to 72 bytes")
	ErrInvalidUser     = errors.New("user identity, email and password hash are required")
)

type Email struct{ value string }

func NewEmail(raw string) (Email, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 || !strings.Contains(value, "@") {
		return Email{}, ErrInvalidEmail
	}
	return Email{value}, nil
}
func (e Email) String() string { return e.value }

func ValidatePassword(raw string) error {
	if len(raw) < 12 || len(raw) > 72 {
		return ErrInvalidPassword
	}
	return nil
}

type User struct {
	id           string
	email        Email
	passwordHash string
	createdAt    time.Time
}

func NewUser(id string, email Email, passwordHash string, now time.Time) (User, error) {
	if id == "" || email.value == "" || passwordHash == "" {
		return User{}, ErrInvalidUser
	}
	return User{id: id, email: email, passwordHash: passwordHash, createdAt: now}, nil
}
func (u User) ID() string           { return u.id }
func (u User) Email() Email         { return u.email }
func (u User) PasswordHash() string { return u.passwordHash }
func (u User) CreatedAt() time.Time { return u.createdAt }
