package cmd

import (
	"os"
	"path/filepath"
	"punchlist/task"
	"strings"
	"testing"
)

// test that `pin todo --ticket` seeds the Problem/Approach/Acceptance scaffold
// and that the seeded ## Acceptance stub is parseAcceptance-compatible (one
// unchecked criterion), while composing with the existing create modifiers.
func TestTodoTicketScaffold(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init command failed: %v", err)
	}
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("Failed to resolve tasks dir: %v", err)
	}

	output, err := executeCommand("todo", "--ticket", "Fix the flaky sensor", "pri:1", "tags:{bug}")
	if err != nil {
		t.Fatalf("todo --ticket failed: %v", err)
	}
	if !strings.Contains(output, "Created task 1:") {
		t.Fatalf("expected creation message, got: %s", output)
	}

	taskFile := filepath.Join(tasksPath, "001-fix-the-flaky-sensor.md")
	t1, err := task.Parse(taskFile)
	if err != nil {
		t.Fatalf("failed to parse created task: %v", err)
	}

	// the scaffold sections are present, in order, after the title
	for _, heading := range []string{"## Problem", "## Approach", "## Acceptance"} {
		if !strings.Contains(t1.Body, heading) {
			t.Errorf("ticket body missing %q. Body:\n%s", heading, t1.Body)
		}
	}
	pIdx := strings.Index(t1.Body, "## Problem")
	aIdx := strings.Index(t1.Body, "## Approach")
	accIdx := strings.Index(t1.Body, "## Acceptance")
	if !(pIdx < aIdx && aIdx < accIdx) {
		t.Errorf("scaffold sections out of order: Problem=%d Approach=%d Acceptance=%d", pIdx, aIdx, accIdx)
	}

	// the normal ## Log section is still appended by the create path
	if !strings.Contains(t1.Body, "## Log") {
		t.Errorf("expected ## Log section to be present. Body:\n%s", t1.Body)
	}

	// parseAcceptance sees exactly one unchecked criterion
	items := parseAcceptance(t1.Body)
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 acceptance item, got %d: %+v", len(items), items)
	}
	if items[0].Checked {
		t.Errorf("expected the seeded acceptance item to be unchecked, got checked")
	}

	// --ticket composes with existing modifiers
	if t1.Priority != 1 {
		t.Errorf("expected priority 1, got %d", t1.Priority)
	}
	if !hasTag(t1.Tags, "bug") {
		t.Errorf("expected tag 'bug', got %v", t1.Tags)
	}
	if t1.State != task.StateTodo {
		t.Errorf("expected TODO state, got %s", t1.State)
	}

	// `pin check 1` ticks the seeded criterion (the substrate stays operable)
	if _, err := executeCommand("check", "1", "1"); err != nil {
		t.Fatalf("check command failed on ticket task: %v", err)
	}
	t1, _ = task.Parse(taskFile)
	if items := parseAcceptance(t1.Body); len(items) != 1 || !items[0].Checked {
		t.Errorf("expected the criterion to be checked after `pin check`, got %+v", items)
	}
}

// test that a normal (non-ticket) task keeps the lean default body so the
// scaffold is strictly opt-in.
func TestTodoWithoutTicketHasNoScaffold(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init command failed: %v", err)
	}
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("Failed to resolve tasks dir: %v", err)
	}

	if _, err := executeCommand("todo", "Quick one-liner"); err != nil {
		t.Fatalf("todo failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tasksPath, "001-quick-one-liner.md"))
	if err != nil {
		t.Fatalf("failed to read task file: %v", err)
	}
	body := string(content)
	if strings.Contains(body, "## Problem") || strings.Contains(body, "## Acceptance") {
		t.Errorf("non-ticket task should not carry the scaffold. Body:\n%s", body)
	}
}
