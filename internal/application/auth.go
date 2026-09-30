package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"try_golang/internal/domain/account"
)

type Auth struct {
	users     Users
	sessions  Sessions
	passwords Passwords
	generate  Generator
	now       Clock
	ttl       time.Duration
	dummyHash string
}
type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

func NewAuth(users Users, sessions Sessions, passwords Passwords, generate Generator, now Clock, ttl time.Duration) (*Auth, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("session TTL must be positive")
	}
	dummy, err := passwords.Hash("dummy-password-for-timing")
	if err != nil {
		return nil, fmt.Errorf("prepare password verifier: %w", err)
	}
	return &Auth{users: users, sessions: sessions, passwords: passwords, generate: generate, now: now, ttl: ttl, dummyHash: dummy}, nil
}

func (a *Auth) Register(ctx context.Context, rawEmail, password string) (account.User, error) {
	email, err := account.NewEmail(rawEmail)
	if err != nil {
		return account.User{}, err
	}
	if err = account.ValidatePassword(password); err != nil {
		return account.User{}, err
	}
	hash, err := a.passwords.Hash(password)
	if err != nil {
		return account.User{}, fmt.Errorf("hash password: %w", err)
	}
	id, err := a.generate()
	if err != nil {
		return account.User{}, fmt.Errorf("generate user ID: %w", err)
	}
	user, err := account.NewUser(id, email, hash, a.now())
	if err != nil {
		return account.User{}, fmt.Errorf("create user: %w", err)
	}
	if err = a.users.InsertUser(ctx, user); err != nil {
		return account.User{}, fmt.Errorf("register user: %w", err)
	}
	return user, nil
}

func (a *Auth) Login(ctx context.Context, rawEmail, password string) (Token, error) {
	email, err := account.NewEmail(rawEmail)
	if err != nil || account.ValidatePassword(password) != nil {
		return Token{}, ErrUnauthenticated
	}
	user, err := a.users.UserByEmail(ctx, email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Token{}, fmt.Errorf("find user: %w", err)
	}
	hash := a.dummyHash
	if err == nil {
		hash = user.PasswordHash()
	}
	verified := a.passwords.Verify(hash, password)
	if !verified || err != nil {
		return Token{}, ErrUnauthenticated
	}
	rawToken, err := a.generate()
	if err != nil {
		return Token{}, fmt.Errorf("generate token: %w", err)
	}
	expiresAt := a.now().Add(a.ttl)
	session := Session{TokenHash: tokenHash(rawToken), UserID: user.ID(), ExpiresAt: expiresAt}
	if err = a.sessions.SaveSession(ctx, session); err != nil {
		return Token{}, fmt.Errorf("save session: %w", err)
	}
	return Token{AccessToken: rawToken, ExpiresAt: expiresAt}, nil
}

func (a *Auth) Authenticate(ctx context.Context, rawToken string) (account.User, error) {
	if rawToken == "" || len(rawToken) > 1024 {
		return account.User{}, ErrUnauthenticated
	}
	session, err := a.sessions.FindSession(ctx, tokenHash(rawToken))
	if errors.Is(err, ErrNotFound) {
		return account.User{}, ErrUnauthenticated
	}
	if err != nil {
		return account.User{}, fmt.Errorf("find session: %w", err)
	}
	if !a.now().Before(session.ExpiresAt) {
		if err = a.sessions.DeleteSession(ctx, session.TokenHash); err != nil {
			return account.User{}, fmt.Errorf("delete expired session: %w", err)
		}
		return account.User{}, ErrUnauthenticated
	}
	user, err := a.users.UserByID(ctx, session.UserID)
	if errors.Is(err, ErrNotFound) {
		return account.User{}, ErrUnauthenticated
	}
	if err != nil {
		return account.User{}, fmt.Errorf("find session user: %w", err)
	}
	return user, nil
}

func (a *Auth) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrUnauthenticated
	}
	if err := a.sessions.DeleteSession(ctx, tokenHash(rawToken)); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
