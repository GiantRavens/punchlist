package cmd

import (
	"encoding/json"
	"punchlist/task"
	"strings"
	"testing"
)

func TestDepsCmd(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Base task")
	executeCommand("todo", "Dependent task", "depends:1")

	t.Run("show forward dependencies", func(t *testing.T) {
		output, err := executeCommand("deps", "2")
		if err != nil {
			t.Fatalf("deps command failed: %v", err)
		}
		if !strings.Contains(output, "depends on") {
			t.Errorf("expected 'depends on' message, got: %s", output)
		}
		if !strings.Contains(output, "Base task") {
			t.Errorf("expected dependency title in output, got: %s", output)
		}
	})

	t.Run("show no dependencies", func(t *testing.T) {
		output, err := executeCommand("deps", "1")
		if err != nil {
			t.Fatalf("deps command failed: %v", err)
		}
		if !strings.Contains(output, "no dependencies") {
			t.Errorf("expected 'no dependencies' message, got: %s", output)
		}
	})

	t.Run("show reverse dependencies", func(t *testing.T) {
		output, err := executeCommand("deps", "1")
		if err != nil {
			t.Fatalf("deps command failed: %v", err)
		}
		if !strings.Contains(output, "Depended on by") {
			t.Errorf("expected reverse dependency section, got: %s", output)
		}
		if !strings.Contains(output, "Dependent task") {
			t.Errorf("expected dependent task title, got: %s", output)
		}
	})
}

func TestDependsCreationModifier(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "First task")
	executeCommand("todo", "Second task")
	executeCommand("todo", "Third task", "depends:1,2")

	taskPath, err := findTaskFile(3)
	if err != nil {
		t.Fatalf("failed to find task 3: %v", err)
	}
	parsed, err := task.Parse(taskPath)
	if err != nil {
		t.Fatalf("failed to parse task 3: %v", err)
	}
	if len(parsed.DependsOn) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(parsed.DependsOn))
	}
	if parsed.DependsOn[0] != 1 || parsed.DependsOn[1] != 2 {
		t.Errorf("expected [1, 2], got %v", parsed.DependsOn)
	}
}

func TestDependsCreationModifierBraces(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "First task")
	executeCommand("todo", "Second task", "depends:{1}")

	taskPath, _ := findTaskFile(2)
	parsed, _ := task.Parse(taskPath)
	if len(parsed.DependsOn) != 1 || parsed.DependsOn[0] != 1 {
		t.Errorf("expected [1], got %v", parsed.DependsOn)
	}
}

func TestReadyFlag(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Blocking task")
	executeCommand("todo", "Blocked task", "depends:1")
	executeCommand("todo", "Free task")

	t.Run("blocked task not in ready list", func(t *testing.T) {
		output, err := executeCommand("ls", "--ready")
		if err != nil {
			t.Fatalf("ls --ready failed: %v", err)
		}
		if strings.Contains(output, "Blocked task") {
			t.Errorf("blocked task should not appear in ready list, got: %s", output)
		}
		if !strings.Contains(output, "Blocking task") {
			t.Errorf("expected unblocked task in ready list, got: %s", output)
		}
		if !strings.Contains(output, "Free task") {
			t.Errorf("expected free task in ready list, got: %s", output)
		}
	})

	t.Run("task becomes ready after dependency done", func(t *testing.T) {
		executeCommand("done", "1")
		output, err := executeCommand("ls", "--ready")
		if err != nil {
			t.Fatalf("ls --ready failed: %v", err)
		}
		if !strings.Contains(output, "Blocked task") {
			t.Errorf("task should be ready after dependency done, got: %s", output)
		}
	})
}

func TestReadyFlagJSON(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Prereq task")
	executeCommand("todo", "Waiting task", "depends:1")

	output, err := executeCommand("ls", "--ready", "--json")
	if err != nil {
		t.Fatalf("ls --ready --json failed: %v", err)
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// only the prereq task (no deps) should appear
	if len(tasks) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(tasks))
	}
	if tasks[0]["title"] != "Prereq task" {
		t.Errorf("unexpected task: %v", tasks[0]["title"])
	}
}

func TestDepsInShowJSON(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Base")
	executeCommand("todo", "Child", "depends:1")

	output, err := executeCommand("show", "--json", "2")
	if err != nil {
		t.Fatalf("show --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	deps, ok := result["depends_on"].([]interface{})
	if !ok {
		t.Fatalf("expected depends_on array, got %T", result["depends_on"])
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
}

func TestParseDependsList(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"1,2,3", []int{1, 2, 3}},
		{"{1,2}", []int{1, 2}},
		{"1", []int{1}},
		{"{1}", []int{1}},
		{"", nil},
		{"{}", nil},
		{"abc", nil},
		{"1,abc,3", []int{1, 3}},
	}

	for _, tt := range tests {
		result := parseDependsList(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseDependsList(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("parseDependsList(%q)[%d] = %d, want %d", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}
