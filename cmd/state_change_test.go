package cmd

import (
	"path/filepath"
	"punchlist/task"
	"strings"
	"testing"
	"time"
)

func TestChangeState(t *testing.T) {
	t.Run("todo to begun sets started_at", func(t *testing.T) {
		tsk := &task.Task{
			ID: 1, Title: "Test", State: task.StateTodo,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		changeState(tsk, task.StateBegun)

		if tsk.State != task.StateBegun {
			t.Errorf("expected state BEGUN, got %s", tsk.State)
		}
		if tsk.StartedAt == nil {
			t.Error("expected StartedAt to be set")
		}
		if tsk.CompletedAt != nil {
			t.Error("expected CompletedAt to be nil")
		}
	})

	t.Run("todo to done sets completed_at", func(t *testing.T) {
		tsk := &task.Task{
			ID: 1, Title: "Test", State: task.StateTodo,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		changeState(tsk, task.StateDone)

		if tsk.State != task.StateDone {
			t.Errorf("expected state DONE, got %s", tsk.State)
		}
		if tsk.CompletedAt == nil {
			t.Error("expected CompletedAt to be set")
		}
	})

	t.Run("transition to todo does not set timestamps", func(t *testing.T) {
		tsk := &task.Task{
			ID: 1, Title: "Test", State: task.StateBegun,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		changeState(tsk, task.StateTodo)

		if tsk.State != task.StateTodo {
			t.Errorf("expected state TODO, got %s", tsk.State)
		}
		if tsk.StartedAt != nil {
			t.Error("expected StartedAt to remain nil")
		}
		if tsk.CompletedAt != nil {
			t.Error("expected CompletedAt to remain nil")
		}
	})

	t.Run("updates UpdatedAt", func(t *testing.T) {
		before := time.Now().Add(-time.Hour)
		tsk := &task.Task{
			ID: 1, Title: "Test", State: task.StateTodo,
			CreatedAt: before, UpdatedAt: before,
		}
		changeState(tsk, task.StateBlock)

		if !tsk.UpdatedAt.After(before) {
			t.Error("expected UpdatedAt to be updated")
		}
	})
}

func TestUpdateTaskStateIntegration(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	executeCommand("init")

	// create a task
	executeCommand("todo", "Integration state test")
	tasksPath, _ := tasksDir()

	// change its state via updateTaskStateSingle
	err := updateTaskStateSingle(1, task.StateDone)
	if err != nil {
		t.Fatalf("updateTaskStateSingle failed: %v", err)
	}

	// re-parse and verify
	files, _ := filepath.Glob(filepath.Join(tasksPath, "*.md"))
	if len(files) == 0 {
		t.Fatal("no task files found")
	}
	tsk, err := task.Parse(files[0])
	if err != nil {
		t.Fatalf("failed to parse task: %v", err)
	}

	if tsk.State != task.StateDone {
		t.Errorf("expected state DONE, got %s", tsk.State)
	}
	if tsk.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if !strings.Contains(tsk.Body, "State changed from TODO to DONE") {
		t.Error("expected state change log entry in body")
	}
}
