package memory_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"try_golang/internal/application"
	"try_golang/internal/domain"
	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
	"try_golang/internal/infrastructure/memory"
)

func TestConcurrentRegistrationAndCompletion(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	s := memory.New(func() time.Time { return now })
	email, _ := account.NewEmail("alice@example.com")
	var inserted, completed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			user, err := account.NewUser("user", email, "hash", now)
			if err != nil {
				t.Error(err)
				return
			}
			err = s.InsertUser(ctx, user)
			if err == nil {
				inserted.Add(1)
			} else if !errors.Is(err, domain.ErrConflict) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if inserted.Load() != 1 {
		t.Fatalf("inserted %d duplicate users", inserted.Load())
	}
	item, _ := task.New("task", "user", "title", now)
	if err := s.InsertTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			_, err := s.UpdateTask(ctx, "task", func(item *task.Task) error { return item.Complete(now) })
			if err == nil {
				completed.Add(1)
			} else if !errors.Is(err, task.ErrCompleted) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if completed.Load() != 1 {
		t.Fatalf("completed %d times", completed.Load())
	}
}

func TestSnapshotsRollbackAndOwnership(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	s := memory.New(func() time.Time { return now })
	item, _ := task.New("task", "owner", "original", now)
	s.InsertTask(ctx, item)
	fetched, _ := s.FindTask(ctx, "task")
	fetched.Rename("outside", now)
	saved, _ := s.FindTask(ctx, "task")
	if saved.Title() != "original" {
		t.Fatal("aggregate escaped store")
	}
	sentinel := errors.New("rollback")
	_, err := s.UpdateTask(ctx, "task", func(v *task.Task) error { v.Rename("inside", now); return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	saved, _ = s.FindTask(ctx, "task")
	if saved.Title() != "original" {
		t.Fatal("failed update persisted")
	}
	if err := s.DeleteTask(ctx, "task", "stranger"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.InsertTask(ctx, item); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate task: %v", err)
	}
	if _, err := s.UpdateTask(ctx, "missing", func(*task.Task) error { t.Fatal("called missing task callback"); return nil }); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestListOrderAndSessionCleanup(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	s := memory.New(func() time.Time { return now })
	for _, id := range []string{"b", "a"} {
		v, _ := task.New(id, "owner", "title", now)
		s.InsertTask(ctx, v)
	}
	foreign, _ := task.New("c", "other", "title", now)
	s.InsertTask(ctx, foreign)
	items, err := s.ListTasks(ctx, "owner")
	if err != nil || len(items) != 2 || items[0].ID() != "a" {
		t.Fatalf("list: %v %v", items, err)
	}
	s.SaveSession(ctx, application.Session{TokenHash: "old", UserID: "u", ExpiresAt: now.Add(time.Minute)})
	now = now.Add(time.Minute)
	s.SaveSession(ctx, application.Session{TokenHash: "new", UserID: "u", ExpiresAt: now.Add(time.Minute)})
	if _, err := s.FindSession(ctx, "old"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired session retained: %v", err)
	}
	if _, err := s.FindSession(ctx, "new"); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.ListTasks(canceled, "owner"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
