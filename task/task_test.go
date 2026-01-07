package task

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

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
}
