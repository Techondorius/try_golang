package task_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"try_golang/internal/domain/task"
)

func TestTaskLifecycle(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	item, err := task.New("task-1", "user-1", "  Write tests  ", now)
	if err != nil {
		t.Fatal(err)
	}
	if item.Title() != "Write tests" || item.Status() != task.Pending || item.OwnerID() != "user-1" || item.ID() != "task-1" || !item.CreatedAt().Equal(now) {
		t.Fatalf("unexpected task: %#v", item)
	}
	if err := item.Rename("Refactor", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if item.Title() != "Refactor" || !item.UpdatedAt().Equal(now.Add(time.Minute)) {
		t.Fatal("rename did not update task")
	}
	if err := item.Complete(now.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if item.Status() != task.Completed {
		t.Fatal("task not completed")
	}
	if err := item.Complete(now); !errors.Is(err, task.ErrCompleted) {
		t.Fatalf("duplicate completion: %v", err)
	}
	if err := item.Rename("Changed", now); !errors.Is(err, task.ErrCompleted) {
		t.Fatalf("renamed completed task: %v", err)
	}
}

func TestInvalidTaskDoesNotMutate(t *testing.T) {
	now := time.Now()
	for _, title := range []string{"", "   ", strings.Repeat("あ", 201)} {
		if _, err := task.New("id", "owner", title, now); !errors.Is(err, task.ErrInvalidTitle) {
			t.Errorf("accepted title: %v", err)
		}
	}
	for _, ids := range [][2]string{{"", "owner"}, {"id", ""}} {
		if _, err := task.New(ids[0], ids[1], "title", now); !errors.Is(err, task.ErrInvalidIdentity) {
			t.Errorf("accepted identity: %v", err)
		}
	}
	item, _ := task.New("id", "owner", "original", now)
	if err := item.Rename(" ", now.Add(time.Hour)); !errors.Is(err, task.ErrInvalidTitle) {
		t.Fatal(err)
	}
	if item.Title() != "original" || !item.UpdatedAt().Equal(now) {
		t.Fatal("invalid rename mutated aggregate")
	}
}
