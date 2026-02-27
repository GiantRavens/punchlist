package cmd

import (
	"encoding/json"
	"testing"
)

func TestLsJSON(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "JSON test task", "pri:2")

	output, err := executeCommand("ls", "--json")
	if err != nil {
		t.Fatalf("ls --json failed: %v", err)
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		t.Fatalf("invalid JSON from ls --json: %v\nOutput: %s", err, output)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0]["title"] != "JSON test task" {
		t.Errorf("unexpected title: %v", tasks[0]["title"])
	}
	if _, hasBody := tasks[0]["body"]; hasBody {
		t.Error("ls --json should not include body field")
	}
	if _, hasPath := tasks[0]["path"]; !hasPath {
		t.Error("ls --json should include path field")
	}
}

func TestLsJSONMultipleTasks(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "First task", "pri:1")
	executeCommand("todo", "Second task", "pri:2", "tags:{ai,launch}")

	output, err := executeCommand("ls", "--json")
	if err != nil {
		t.Fatalf("ls --json failed: %v", err)
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	// verify tags appear in JSON
	second := tasks[1]
	tags, ok := second["tags"].([]interface{})
	if !ok {
		t.Fatalf("expected tags array, got %T", second["tags"])
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestShowJSON(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Show JSON task")

	output, err := executeCommand("show", "--json", "1")
	if err != nil {
		t.Fatalf("show --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON from show --json: %v\nOutput: %s", err, output)
	}
	if result["title"] != "Show JSON task" {
		t.Errorf("unexpected title: %v", result["title"])
	}
	if _, hasBody := result["body"]; !hasBody {
		t.Error("show --json should include body")
	}
}

func TestSearchJSON(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Searchable thing")

	output, err := executeCommand("search", "--json", "searchable")
	if err != nil {
		t.Fatalf("search --json failed: %v", err)
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		t.Fatalf("invalid JSON from search --json: %v\nOutput: %s", err, output)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0]["title"] != "Searchable thing" {
		t.Errorf("unexpected title: %v", tasks[0]["title"])
	}
}

func TestLsJSONFilteredByState(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "Open task")
	executeCommand("done", "1")
	executeCommand("todo", "Active task")

	output, err := executeCommand("ls", "--json", "todo")
	if err != nil {
		t.Fatalf("ls --json todo failed: %v", err)
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 TODO task, got %d", len(tasks))
	}
	if tasks[0]["title"] != "Active task" {
		t.Errorf("unexpected title: %v", tasks[0]["title"])
	}
}
