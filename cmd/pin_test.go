package cmd

import (
	"os"
	"path/filepath"
	"punchlist/config"
	"strings"
	"testing"
	"time"
)

func TestParseCreateModifiers(t *testing.T) {
	t.Run("valid priority", func(t *testing.T) {
		opts, err := parseCreateModifiers([]string{"pri:3"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.priority != 3 {
			t.Errorf("expected priority 3, got %d", opts.priority)
		}
	})

	t.Run("invalid priority non-numeric", func(t *testing.T) {
		_, err := parseCreateModifiers([]string{"pri:abc"})
		if err == nil {
			t.Fatal("expected error for non-numeric priority")
		}
	})

	t.Run("priority out of range high", func(t *testing.T) {
		_, err := parseCreateModifiers([]string{"pri:999"})
		if err == nil {
			t.Fatal("expected error for priority > 10")
		}
		if !strings.Contains(err.Error(), "between 0 and 10") {
			t.Errorf("expected bounds error message, got: %v", err)
		}
	})

	t.Run("priority out of range negative", func(t *testing.T) {
		_, err := parseCreateModifiers([]string{"pri:-1"})
		if err == nil {
			t.Fatal("expected error for negative priority")
		}
	})

	t.Run("priority at boundary 10", func(t *testing.T) {
		opts, err := parseCreateModifiers([]string{"pri:10"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.priority != 10 {
			t.Errorf("expected priority 10, got %d", opts.priority)
		}
	})

	t.Run("priority at boundary 0", func(t *testing.T) {
		opts, err := parseCreateModifiers([]string{"pri:0"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.priority != 0 {
			t.Errorf("expected priority 0, got %d", opts.priority)
		}
	})

	t.Run("valid due date", func(t *testing.T) {
		opts, err := parseCreateModifiers([]string{"due:2026-06-15"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.due == nil {
			t.Fatal("expected due date to be set")
		}
		if opts.due.Year() != 2026 || opts.due.Month() != 6 || opts.due.Day() != 15 {
			t.Errorf("unexpected due date: %v", opts.due)
		}
	})

	t.Run("valid tags", func(t *testing.T) {
		opts, err := parseCreateModifiers([]string{"tags:{alpha,beta}"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(opts.tags) != 2 || opts.tags[0] != "alpha" || opts.tags[1] != "beta" {
			t.Errorf("unexpected tags: %v", opts.tags)
		}
	})

	t.Run("state modifier", func(t *testing.T) {
		opts, err := parseCreateModifiers([]string{"state:BEGUN"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.state != "BEGUN" {
			t.Errorf("expected state BEGUN, got %s", opts.state)
		}
	})

	t.Run("unknown modifier key", func(t *testing.T) {
		_, err := parseCreateModifiers([]string{"foo:bar"})
		if err == nil {
			t.Fatal("expected error for unknown modifier")
		}
	})
}

func TestParseDueNatural(t *testing.T) {
	// use a fixed reference time: Wednesday 2026-02-04 at 10:00 AM
	ref := time.Date(2026, 2, 4, 10, 0, 0, 0, time.Local)

	t.Run("today", func(t *testing.T) {
		result, ok := parseDueNatural("today", ref)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if result.Day() != ref.Day() || result.Month() != ref.Month() {
			t.Errorf("expected today's date, got %v", result)
		}
		if result.Hour() != 12 {
			t.Errorf("expected noon, got hour %d", result.Hour())
		}
	})

	t.Run("tomorrow", func(t *testing.T) {
		result, ok := parseDueNatural("tomorrow", ref)
		if !ok {
			t.Fatal("expected ok=true")
		}
		expected := ref.AddDate(0, 0, 1)
		if result.Day() != expected.Day() {
			t.Errorf("expected tomorrow, got %v", result)
		}
	})

	t.Run("weekday name", func(t *testing.T) {
		result, ok := parseDueNatural("friday", ref)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if result.Weekday() != time.Friday {
			t.Errorf("expected Friday, got %v", result.Weekday())
		}
		if result.Before(ref) {
			t.Errorf("expected date to be in the future, got %v", result)
		}
	})

	t.Run("next weekday", func(t *testing.T) {
		result, ok := parseDueNatural("next friday", ref)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if result.Weekday() != time.Friday {
			t.Errorf("expected Friday, got %v", result.Weekday())
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		_, ok := parseDueNatural("never", ref)
		if ok {
			t.Fatal("expected ok=false for invalid input")
		}
	})
}

func TestParseDue(t *testing.T) {
	t.Run("ISO date", func(t *testing.T) {
		result, err := parseDue("2026-03-15")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Year() != 2026 || result.Month() != 3 || result.Day() != 15 {
			t.Errorf("unexpected date: %v", result)
		}
	})

	t.Run("ISO datetime", func(t *testing.T) {
		result, err := parseDue("2026-03-15T14:30")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Hour() != 14 || result.Minute() != 30 {
			t.Errorf("unexpected time: %v", result)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		_, err := parseDue("not-a-date")
		if err == nil {
			t.Fatal("expected error for invalid date")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := parseDue("")
		if err == nil {
			t.Fatal("expected error for empty string")
		}
	})
}

func TestSlugify(t *testing.T) {
	t.Run("normal string", func(t *testing.T) {
		result := slugify("My Test Task")
		if result != "my-test-task" {
			t.Errorf("expected 'my-test-task', got %q", result)
		}
	})

	t.Run("special characters", func(t *testing.T) {
		result := slugify("hello@world! #123")
		if result != "hello-world-123" {
			t.Errorf("expected 'hello-world-123', got %q", result)
		}
	})

	t.Run("leading trailing hyphens trimmed", func(t *testing.T) {
		result := slugify("--test--")
		if result != "test" {
			t.Errorf("expected 'test', got %q", result)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		result := slugify("")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("consecutive special chars collapse", func(t *testing.T) {
		result := slugify("a   b___c")
		if result != "a-b-c" {
			t.Errorf("expected 'a-b-c', got %q", result)
		}
	})
}

func TestSplitTitleAndModifiers(t *testing.T) {
	t.Run("title only", func(t *testing.T) {
		title, mods, err := splitTitleAndModifiers([]string{"my task"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "my task" {
			t.Errorf("expected 'my task', got %q", title)
		}
		if len(mods) != 0 {
			t.Errorf("expected no modifiers, got %v", mods)
		}
	})

	t.Run("title with inline modifier", func(t *testing.T) {
		title, mods, err := splitTitleAndModifiers([]string{"my task", "pri:1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "my task" {
			t.Errorf("expected 'my task', got %q", title)
		}
		if len(mods) != 1 || mods[0] != "pri:1" {
			t.Errorf("expected [pri:1], got %v", mods)
		}
	})

	t.Run("missing title", func(t *testing.T) {
		_, _, err := splitTitleAndModifiers([]string{})
		if err == nil {
			t.Fatal("expected error for missing title")
		}
	})

	t.Run("title with tags modifier", func(t *testing.T) {
		title, mods, err := splitTitleAndModifiers([]string{"do thing", "tags:{a,b}"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if title != "do thing" {
			t.Errorf("expected 'do thing', got %q", title)
		}
		if len(mods) != 1 || !strings.Contains(mods[0], "tags:") {
			t.Errorf("expected tags modifier, got %v", mods)
		}
	})
}

// test that pin todo walks past a scope marked accepting: false (pin #12)
func TestCreateWalksPastClosedScope(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	// parent scope (accepting by default)
	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("parent init failed: %v", err)
	}
	parentDir, _ := os.Getwd()

	// closed child scope
	childDir := filepath.Join(parentDir, "child")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	if err := os.Chdir(childDir); err != nil {
		t.Fatalf("chdir child: %v", err)
	}
	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("child init failed: %v", err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load child config: %v", err)
	}
	closed := false
	cfg.Accepting = &closed
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("save child config: %v", err)
	}

	// a task created in the closed child scope lands in the parent
	output, err := executeCommand("todo", "walked task")
	if err != nil {
		t.Fatalf("todo in closed scope failed: %v", err)
	}
	if !strings.Contains(output, "closed to new tasks") {
		t.Errorf("expected closed-scope notice, got: %s", output)
	}
	if !strings.Contains(output, filepath.Join(parentDir, "tasks")) {
		t.Errorf("expected task created under parent scope, got: %s", output)
	}
	entries, _ := os.ReadDir(filepath.Join(childDir, "tasks"))
	if len(entries) != 0 {
		t.Errorf("expected no tasks in closed child scope, found %d", len(entries))
	}
	parentEntries, _ := os.ReadDir(filepath.Join(parentDir, "tasks"))
	if len(parentEntries) != 1 {
		t.Errorf("expected 1 task in parent scope, found %d", len(parentEntries))
	}

	// ls / note / done still serve the closed scope: seed a task by
	// temporarily reopening, then close again and operate on it
	open := true
	cfg.Accepting = &open
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("reopen child config: %v", err)
	}
	if _, err := executeCommand("todo", "local task"); err != nil {
		t.Fatalf("todo in reopened scope failed: %v", err)
	}
	cfg.Accepting = &closed
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("re-close child config: %v", err)
	}

	lsOut, err := executeCommand("ls")
	if err != nil {
		t.Fatalf("ls in closed scope failed: %v", err)
	}
	if !strings.Contains(lsOut, "local task") {
		t.Errorf("ls should still serve a closed scope, got: %s", lsOut)
	}
	if _, err := executeCommand("note", "1", "still serviceable"); err != nil {
		t.Fatalf("note in closed scope failed: %v", err)
	}
	if _, err := executeCommand("done", "1"); err != nil {
		t.Fatalf("done in closed scope failed: %v", err)
	}
}

// test that a closed scope with no accepting ancestor fails loudly
func TestCreateClosedScopeNoAcceptingAncestor(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	closed := false
	cfg.Accepting = &closed
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	err = createTaskFromArgs([]string{"should not land"})
	if err == nil {
		t.Fatal("expected error creating in closed scope with no accepting ancestor")
	}
	if !strings.Contains(err.Error(), "closed to new tasks") {
		t.Errorf("unexpected error: %v", err)
	}
	entries, _ := os.ReadDir("tasks")
	if len(entries) != 0 {
		t.Errorf("expected no task files, found %d", len(entries))
	}
}
