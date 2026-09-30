// Package application coordinates domain objects through ports owned by the
// use cases. Infrastructure implements these interfaces; dependencies point in.
package application

import (
	"context"
	"errors"
	"time"

	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

var (
	ErrNotFound        = errors.New("resource not found")
	ErrConflict        = errors.New("resource already exists")
	ErrUnauthenticated = errors.New("authentication required")
)

type Clock func() time.Time
type Generator func() (string, error)

type Users interface {
	InsertUser(context.Context, account.User) error
	UserByEmail(context.Context, account.Email) (account.User, error)
	UserByID(context.Context, string) (account.User, error)
}
type Passwords interface {
	Hash(string) (string, error)
	Verify(hash, password string) bool
}
type Session struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
}
type Sessions interface {
	SaveSession(context.Context, Session) error
	FindSession(context.Context, string) (Session, error)
	DeleteSession(context.Context, string) error
}
type TaskRepository interface {
	InsertTask(context.Context, task.Task) error
	FindTask(context.Context, string) (task.Task, error)
	ListTasks(context.Context, string) ([]task.Task, error)
	// UpdateTask applies a domain operation atomically. On error nothing is saved.
	UpdateTask(context.Context, string, func(*task.Task) error) (task.Task, error)
	// DeleteTask atomically checks ownership and deletes, returning ErrNotFound
	// for both missing tasks and another user's tasks.
	DeleteTask(context.Context, string, string) error
}
