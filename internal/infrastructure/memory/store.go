// Package memory implements use-case ports with process-local, concurrency-safe
// storage. Replace this adapter to introduce durable persistence.
package memory

import (
	"context"
	"sort"
	"sync"

	"try_golang/internal/application"
	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

type Store struct {
	mu       sync.RWMutex
	users    map[string]account.User
	emailIDs map[string]string
	sessions map[string]application.Session
	tasks    map[string]task.Task
	now      application.Clock
}

var (
	_ application.Users          = (*Store)(nil)
	_ application.Sessions       = (*Store)(nil)
	_ application.TaskRepository = (*Store)(nil)
)

func New(now application.Clock) *Store {
	return &Store{users: map[string]account.User{}, emailIDs: map[string]string{}, sessions: map[string]application.Session{}, tasks: map[string]task.Task{}, now: now}
}
func (s *Store) InsertUser(ctx context.Context, user account.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.emailIDs[user.Email().String()]; ok {
		return application.ErrConflict
	}
	if _, ok := s.users[user.ID()]; ok {
		return application.ErrConflict
	}
	s.users[user.ID()] = user
	s.emailIDs[user.Email().String()] = user.ID()
	return nil
}
func (s *Store) UserByEmail(ctx context.Context, email account.Email) (account.User, error) {
	if err := ctx.Err(); err != nil {
		return account.User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.emailIDs[email.String()]
	if !ok {
		return account.User{}, application.ErrNotFound
	}
	return s.users[id], nil
}
func (s *Store) UserByID(ctx context.Context, id string) (account.User, error) {
	if err := ctx.Err(); err != nil {
		return account.User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return account.User{}, application.ErrNotFound
	}
	return user, nil
}
func (s *Store) SaveSession(ctx context.Context, session application.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, item := range s.sessions {
		if !now.Before(item.ExpiresAt) {
			delete(s.sessions, key)
		}
	}
	if _, ok := s.sessions[session.TokenHash]; ok {
		return application.ErrConflict
	}
	s.sessions[session.TokenHash] = session
	return nil
}
func (s *Store) FindSession(ctx context.Context, hash string) (application.Session, error) {
	if err := ctx.Err(); err != nil {
		return application.Session{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[hash]
	if !ok {
		return application.Session{}, application.ErrNotFound
	}
	return session, nil
}
func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, hash)
	return nil
}
func (s *Store) InsertTask(ctx context.Context, item task.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[item.ID()]; ok {
		return application.ErrConflict
	}
	s.tasks[item.ID()] = item
	return nil
}
func (s *Store) FindTask(ctx context.Context, id string) (task.Task, error) {
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.tasks[id]
	if !ok {
		return task.Task{}, application.ErrNotFound
	}
	return item, nil
}
func (s *Store) ListTasks(ctx context.Context, owner string) ([]task.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := []task.Task{}
	for _, item := range s.tasks {
		if item.OwnerID() == owner {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt().Equal(items[j].CreatedAt()) {
			return items[i].ID() < items[j].ID()
		}
		return items[i].CreatedAt().Before(items[j].CreatedAt())
	})
	return items, nil
}
func (s *Store) UpdateTask(ctx context.Context, id string, change func(*task.Task) error) (task.Task, error) {
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tasks[id]
	if !ok {
		return task.Task{}, application.ErrNotFound
	}
	if err := change(&item); err != nil {
		return task.Task{}, err
	}
	s.tasks[id] = item
	return item, nil
}
func (s *Store) DeleteTask(ctx context.Context, id, owner string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tasks[id]
	if !ok || item.OwnerID() != owner {
		return application.ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}
