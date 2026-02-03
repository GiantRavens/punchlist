package cmd

import (
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
