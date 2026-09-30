package application

import (
	"context"
	"fmt"

	"try_golang/internal/domain/task"
)

type Tasks struct {
	repo     TaskRepository
	generate Generator
	now      Clock
}

func NewTasks(repo TaskRepository, generate Generator, now Clock) *Tasks {
	return &Tasks{repo, generate, now}
}

func (s *Tasks) Create(ctx context.Context, owner, title string) (task.Task, error) {
	if owner == "" {
		return task.Task{}, ErrUnauthenticated
	}
	id, err := s.generate()
	if err != nil {
		return task.Task{}, fmt.Errorf("generate task ID: %w", err)
	}
	item, err := task.New(id, owner, title, s.now())
	if err != nil {
		return task.Task{}, err
	}
	if err = s.repo.InsertTask(ctx, item); err != nil {
		return task.Task{}, fmt.Errorf("create task: %w", err)
	}
	return item, nil
}
func (s *Tasks) Get(ctx context.Context, owner, id string) (task.Task, error) {
	if owner == "" {
		return task.Task{}, ErrUnauthenticated
	}
	item, err := s.repo.FindTask(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	if item.OwnerID() != owner {
		return task.Task{}, ErrNotFound
	}
	return item, nil
}
func (s *Tasks) List(ctx context.Context, owner string) ([]task.Task, error) {
	if owner == "" {
		return nil, ErrUnauthenticated
	}
	return s.repo.ListTasks(ctx, owner)
}
func (s *Tasks) Rename(ctx context.Context, owner, id, title string) (task.Task, error) {
	return s.change(ctx, owner, id, func(item *task.Task) error { return item.Rename(title, s.now()) })
}
func (s *Tasks) Complete(ctx context.Context, owner, id string) (task.Task, error) {
	return s.change(ctx, owner, id, func(item *task.Task) error { return item.Complete(s.now()) })
}
func (s *Tasks) change(ctx context.Context, owner, id string, operation func(*task.Task) error) (task.Task, error) {
	if owner == "" {
		return task.Task{}, ErrUnauthenticated
	}
	return s.repo.UpdateTask(ctx, id, func(item *task.Task) error {
		if item.OwnerID() != owner {
			return ErrNotFound
		}
		return operation(item)
	})
}
func (s *Tasks) Delete(ctx context.Context, owner, id string) error {
	if owner == "" {
		return ErrUnauthenticated
	}
	return s.repo.DeleteTask(ctx, id, owner)
}
