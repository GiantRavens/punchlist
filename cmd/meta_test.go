package cmd

import (
	"punchlist/task"
	"strings"
	"testing"
)

func TestMetaCmd(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Meta test task")

	t.Run("display empty meta", func(t *testing.T) {
		output, err := executeCommand("meta", "1")
		if err != nil {
			t.Fatalf("meta command failed: %v", err)
		}
		if !strings.Contains(output, "No meta values set") {
			t.Errorf("expected empty meta message, got: %s", output)
		}
	})

	t.Run("set meta values", func(t *testing.T) {
		output, err := executeCommand("meta", "1", "source=standup-2026-02-27", "from=alice")
		if err != nil {
			t.Fatalf("meta set failed: %v", err)
		}
		if !strings.Contains(output, "Updated meta for task 1") {
			t.Errorf("expected success message, got: %s", output)
		}

		// verify round-trip
		taskPath, _ := findTaskFile(1)
		parsed, _ := task.Parse(taskPath)
		if parsed.Meta["source"] != "standup-2026-02-27" {
			t.Errorf("expected source=standup-2026-02-27, got %v", parsed.Meta["source"])
		}
		if parsed.Meta["from"] != "alice" {
			t.Errorf("expected from=alice, got %v", parsed.Meta["from"])
		}
	})

	t.Run("display populated meta", func(t *testing.T) {
		output, err := executeCommand("meta", "1")
		if err != nil {
			t.Fatalf("meta display failed: %v", err)
		}
		if !strings.Contains(output, "source: standup-2026-02-27") {
			t.Errorf("expected source in output, got: %s", output)
		}
		if !strings.Contains(output, "from: alice") {
			t.Errorf("expected from in output, got: %s", output)
		}
	})

	t.Run("delete meta key with empty value", func(t *testing.T) {
		_, err := executeCommand("meta", "1", "from=")
		if err != nil {
			t.Fatalf("meta delete failed: %v", err)
		}

		taskPath, _ := findTaskFile(1)
		parsed, _ := task.Parse(taskPath)
		if _, exists := parsed.Meta["from"]; exists {
			t.Errorf("expected from key to be deleted")
		}
		if parsed.Meta["source"] != "standup-2026-02-27" {
			t.Errorf("expected source to survive deletion of other key")
		}
	})

	t.Run("meta survives state change", func(t *testing.T) {
		executeCommand("start", "1")
		taskPath, _ := findTaskFile(1)
		parsed, _ := task.Parse(taskPath)
		if parsed.Meta["source"] != "standup-2026-02-27" {
			t.Errorf("meta lost after state change, got %v", parsed.Meta)
		}
	})

	t.Run("meta appears in show json", func(t *testing.T) {
		output, err := executeCommand("show", "--json", "1")
		if err != nil {
			t.Fatalf("show --json failed: %v", err)
		}
		if !strings.Contains(output, "standup-2026-02-27") {
			t.Errorf("expected meta in JSON output, got: %s", output)
		}
	})
}
