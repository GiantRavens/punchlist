package task

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// test task parse and write behavior
func TestParseAndWriteTask(t *testing.T) {
	sandboxDir, err := filepath.Abs("sandbox")
	if err != nil {
		t.Fatalf("Failed to get absolute path for sandbox: %v", err)
	}
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		t.Fatalf("Failed to create sandbox dir: %v", err)
	}
	defer os.RemoveAll(sandboxDir)

	t.Run("writes and parses a full task", func(t *testing.T) {
		due := time.Date(2025, 2, 1, 9, 0, 0, 0, time.UTC)
		task := &Task{
			ID:        1,
			Title:     "Full Task",
			State:     StateTodo,
			Priority:  1,
			Due:       &due,
			Tags:      []string{"hot", "hugeco"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Body:      "This is the body of the task.",
		}

		filePath := filepath.Join(sandboxDir, "full_task.md")
		if err := task.Write(filePath); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}

		parsedTask, err := Parse(filePath)
		if err != nil {
			t.Fatalf("Parse() failed: %v", err)
		}

		if parsedTask.ID != task.ID {
			t.Errorf("Expected ID %d, got %d", task.ID, parsedTask.ID)
		}
		if parsedTask.Title != task.Title {
			t.Errorf("Expected Title '%s', got '%s'", task.Title, parsedTask.Title)
		}
		if parsedTask.State != task.State {
			t.Errorf("Expected State '%s', got '%s'", task.State, parsedTask.State)
		}
		if parsedTask.Body != task.Body {
			t.Errorf("Expected Body '%s', got '%s'", task.Body, parsedTask.Body)
		}
		if !parsedTask.Due.Equal(*task.Due) {
			t.Errorf("Expected Due '%s', got '%s'", task.Due, parsedTask.Due)
		}
		if !reflect.DeepEqual(parsedTask.Tags, task.Tags) {
			t.Errorf("Expected Tags %v, got %v", task.Tags, parsedTask.Tags)
		}
	})

	t.Run("handles task with minimal frontmatter", func(t *testing.T) {
		task := &Task{
			ID:        2,
			Title:     "Minimal Task",
			State:     StateBegun,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		filePath := filepath.Join(sandboxDir, "minimal_task.md")
		if err := task.Write(filePath); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}

		parsedTask, err := Parse(filePath)
		if err != nil {
			t.Fatalf("Parse() failed: %v", err)
		}

		if parsedTask.ID != task.ID {
			t.Errorf("Expected ID %d, got %d", task.ID, parsedTask.ID)
		}
		if parsedTask.Title != task.Title {
			t.Errorf("Expected Title '%s', got '%s'", task.Title, parsedTask.Title)
		}
	})

	t.Run("parses title with unescaped colon via lenient fallback", func(t *testing.T) {
		frontmatter := strings.Join([]string{
			"id: 7",
			"title: Fix encapsulation breach: GameHUD directly accesses game_system.manifest",
			"state: DONE",
			"created_at: 2026-01-26T12:55:46.547687-06:00",
			"updated_at: 2026-01-26T13:56:52.216953-06:00",
		}, "\n")

		payload := strings.Join([]string{
			"---",
			frontmatter,
			"---",
			"",
			"Body line.",
		}, "\n")

		filePath := filepath.Join(sandboxDir, "colon_title_task.md")
		if err := os.WriteFile(filePath, []byte(payload), 0644); err != nil {
			t.Fatalf("Failed to write task file: %v", err)
		}

		parsedTask, err := Parse(filePath)
		if err != nil {
			t.Fatalf("Parse() failed: %v", err)
		}

		if parsedTask.Title != "Fix encapsulation breach: GameHUD directly accesses game_system.manifest" {
			t.Errorf("Unexpected Title '%s'", parsedTask.Title)
		}
		if parsedTask.State != StateDone {
			t.Errorf("Expected State '%s', got '%s'", StateDone, parsedTask.State)
		}
	})

	t.Run("round-trips meta field", func(t *testing.T) {
		task := &Task{
			ID:        10,
			Title:     "Meta Task",
			State:     StateTodo,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Meta: map[string]interface{}{
				"source": "standup-2026-02-27",
				"from":   "alice",
				"to":     "bob",
			},
		}

		filePath := filepath.Join(sandboxDir, "meta_task.md")
		if err := task.Write(filePath); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}

		parsed, err := Parse(filePath)
		if err != nil {
			t.Fatalf("Parse() failed: %v", err)
		}

		if parsed.Meta["source"] != "standup-2026-02-27" {
			t.Errorf("Expected meta source='standup-2026-02-27', got %v", parsed.Meta["source"])
		}
		if parsed.Meta["from"] != "alice" {
			t.Errorf("Expected meta from='alice', got %v", parsed.Meta["from"])
		}
		if parsed.Meta["to"] != "bob" {
			t.Errorf("Expected meta to='bob', got %v", parsed.Meta["to"])
		}
	})

	t.Run("round-trips depends_on field", func(t *testing.T) {
		task := &Task{
			ID:        11,
			Title:     "Deps Task",
			State:     StateTodo,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DependsOn: []int{3, 7},
		}

		filePath := filepath.Join(sandboxDir, "deps_task.md")
		if err := task.Write(filePath); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}

		parsed, err := Parse(filePath)
		if err != nil {
			t.Fatalf("Parse() failed: %v", err)
		}

		if !reflect.DeepEqual(parsed.DependsOn, []int{3, 7}) {
			t.Errorf("Expected DependsOn [3, 7], got %v", parsed.DependsOn)
		}
	})

	t.Run("old tasks without new fields parse fine", func(t *testing.T) {
		task := &Task{
			ID:        12,
			Title:     "Legacy Task",
			State:     StateTodo,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		filePath := filepath.Join(sandboxDir, "legacy_task.md")
		if err := task.Write(filePath); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}

		parsed, err := Parse(filePath)
		if err != nil {
			t.Fatalf("Parse() failed: %v", err)
		}

		if parsed.Meta != nil {
			t.Errorf("Expected nil Meta, got %v", parsed.Meta)
		}
		if parsed.DependsOn != nil {
			t.Errorf("Expected nil DependsOn, got %v", parsed.DependsOn)
		}
	})

	t.Run("lenient parser handles depends_on", func(t *testing.T) {
		frontmatter := strings.Join([]string{
			"id: 20",
			"title: Lenient deps: needs colon in title",
			"state: TODO",
			"depends_on: [3, 7]",
			"created_at: 2026-01-26T12:55:46.547687-06:00",
			"updated_at: 2026-01-26T13:56:52.216953-06:00",
		}, "\n")

		payload := strings.Join([]string{
			"---",
			frontmatter,
			"---",
			"",
			"Body.",
		}, "\n")

		filePath := filepath.Join(sandboxDir, "lenient_deps.md")
		if err := os.WriteFile(filePath, []byte(payload), 0644); err != nil {
			t.Fatalf("Failed to write task file: %v", err)
		}

		parsed, err := Parse(filePath)
		if err != nil {
			t.Fatalf("Parse() failed: %v", err)
		}

		if !reflect.DeepEqual(parsed.DependsOn, []int{3, 7}) {
			t.Errorf("Expected DependsOn [3, 7], got %v", parsed.DependsOn)
		}
	})
}
