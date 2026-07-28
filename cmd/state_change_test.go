package cmd

import (
	"path/filepath"
	"punchlist/config"
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

// TestUnknownStateRefusal covers pin#42: a bare word that is NOT a known state
// token must be refused (never minted into an arbitrary state and applied).
func TestUnknownStateRefusal(t *testing.T) {
	catalog := config.DefaultStateCatalog()

	t.Run("known tokens resolve; the dispatch gate would apply them", func(t *testing.T) {
		for _, tok := range []string{"done", "DONE", "begun", "started", "todo", "block"} {
			if _, ok := catalog.Resolve(tok); !ok {
				t.Errorf("expected %q to resolve as a known state token", tok)
			}
		}
	})

	t.Run("unknown tokens do NOT resolve (the dispatch gate refuses)", func(t *testing.T) {
		// `get` is the reported specimen: `pin get 178` minted a GET state. It is a
		// read verb, never a state, and must not resolve.
		for _, tok := range []string{"get", "show", "view", "help", "banana", "gett"} {
			if _, ok := catalog.Resolve(tok); ok {
				t.Errorf("expected %q to be UNKNOWN (must not resolve to a state)", tok)
			}
		}
	})

	t.Run("refusal message names the token, suggests show for read verbs, lists states", func(t *testing.T) {
		msg := unknownStateMessage("get", []string{"178"}, catalog)
		if !strings.Contains(msg, `"get"`) {
			t.Errorf("message should name the offending token: %q", msg)
		}
		if !strings.Contains(msg, "pin show 178") {
			t.Errorf("read verb 'get' should suggest 'pin show 178': %q", msg)
		}
		if !strings.Contains(msg, "done") || !strings.Contains(msg, "todo") {
			t.Errorf("message should list valid states: %q", msg)
		}
		if !strings.Contains(msg, "pin todo") {
			t.Errorf("message should offer explicit task creation as the alternative: %q", msg)
		}
	})

	t.Run("non-read unknown token still refuses but omits the show suggestion", func(t *testing.T) {
		msg := unknownStateMessage("banana", []string{"1", "2"}, catalog)
		if strings.Contains(msg, "Did you mean:  pin show") {
			t.Errorf("non-read token should not suggest 'pin show': %q", msg)
		}
		if !strings.Contains(msg, `"banana"`) {
			t.Errorf("message should still name the token: %q", msg)
		}
	})
}
