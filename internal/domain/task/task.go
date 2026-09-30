// Package task owns task lifecycle rules. It has no HTTP or storage dependencies.
package task

import (
	"errors"
	"time"
)

var (
	ErrInvalidTitle    = errors.New("title must contain 1 to 200 characters")
	ErrInvalidIdentity = errors.New("task and owner IDs are required")
	ErrCompleted       = errors.New("completed task cannot be changed")
)

type Status string

const (
	Pending   Status = "pending"
	Completed Status = "completed"
)

type Task struct {
	id        string
	ownerID   string
	title     Title
	status    Status
	createdAt time.Time
	updatedAt time.Time
}

func New(id, ownerID, rawTitle string, now time.Time) (Task, error) {
	if id == "" || ownerID == "" {
		return Task{}, ErrInvalidIdentity
	}
	title, err := NewTitle(rawTitle)
	if err != nil {
		return Task{}, err
	}
	return Task{id: id, ownerID: ownerID, title: title, status: Pending, createdAt: now, updatedAt: now}, nil
}
func (t *Task) Rename(raw string, now time.Time) error {
	if t.status == Completed {
		return ErrCompleted
	}
	title, err := NewTitle(raw)
	if err != nil {
		return err
	}
	t.title, t.updatedAt = title, now
	return nil
}
func (t *Task) Complete(now time.Time) error {
	if t.status == Completed {
		return ErrCompleted
	}
	t.status, t.updatedAt = Completed, now
	return nil
}
func (t Task) ID() string           { return t.id }
func (t Task) OwnerID() string      { return t.ownerID }
func (t Task) Title() string        { return t.title.String() }
func (t Task) Status() Status       { return t.status }
func (t Task) CreatedAt() time.Time { return t.createdAt }
func (t Task) UpdatedAt() time.Time { return t.updatedAt }
