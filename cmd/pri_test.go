package cmd

import (
	"punchlist/task"
	"strings"
	"testing"
	"time"
)

func TestChangePriority(t *testing.T) {
	t.Run("set priority on task with none", func(t *testing.T) {
		tsk := &task.Task{ID: 1, Title: "x", Priority: 0, CreatedAt: time.Now()}
		before := tsk.UpdatedAt
		now := time.Now().Add(time.Hour)
		msg, changed := changePriority(tsk, 2, now)

		if !changed {
			t.Fatal("expected changed=true")
		}
		if tsk.Priority != 2 {
			t.Errorf("expected priority 2, got %d", tsk.Priority)
		}
		if !strings.Contains(msg, "set to 2") {
			t.Errorf("expected 'set to 2' in message, got %q", msg)
		}
		if !tsk.UpdatedAt.After(before) {
			t.Error("expected UpdatedAt to advance")
		}
	})

	t.Run("clear priority", func(t *testing.T) {
		tsk := &task.Task{ID: 1, Title: "x", Priority: 3, CreatedAt: time.Now()}
		msg, changed := changePriority(tsk, 0, time.Now())

		if !changed {
			t.Fatal("expected changed=true")
		}
		if tsk.Priority != 0 {
			t.Errorf("expected priority 0, got %d", tsk.Priority)
		}
		if !strings.Contains(msg, "cleared (was 3)") {
			t.Errorf("expected 'cleared (was 3)' in message, got %q", msg)
		}
	})

	t.Run("change priority between two non-zero values", func(t *testing.T) {
		tsk := &task.Task{ID: 1, Title: "x", Priority: 1, CreatedAt: time.Now()}
		msg, changed := changePriority(tsk, 3, time.Now())

		if !changed {
			t.Fatal("expected changed=true")
		}
		if tsk.Priority != 3 {
			t.Errorf("expected priority 3, got %d", tsk.Priority)
		}
		if !strings.Contains(msg, "from 1 to 3") {
			t.Errorf("expected 'from 1 to 3' in message, got %q", msg)
		}
	})

	t.Run("no-op when priority unchanged", func(t *testing.T) {
		tsk := &task.Task{ID: 1, Title: "x", Priority: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		before := tsk.UpdatedAt
		msg, changed := changePriority(tsk, 2, time.Now().Add(time.Hour))

		if changed {
			t.Error("expected changed=false for no-op")
		}
		if msg != "" {
			t.Errorf("expected empty message for no-op, got %q", msg)
		}
		if !tsk.UpdatedAt.Equal(before) {
			t.Error("expected UpdatedAt not to advance on no-op")
		}
		if tsk.Priority != 2 {
			t.Errorf("expected priority unchanged at 2, got %d", tsk.Priority)
		}
	})

	t.Run("clear when already cleared is a no-op", func(t *testing.T) {
		tsk := &task.Task{ID: 1, Title: "x", Priority: 0, CreatedAt: time.Now()}
		_, changed := changePriority(tsk, 0, time.Now())
		if changed {
			t.Error("expected changed=false when clearing already-zero priority")
		}
	})
}
