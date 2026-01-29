package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"strings"
	"testing"
)

// setupTest creates a temporary directory for a test, changes into it, and returns a teardown function.
func setupTest(t *testing.T) func() {
	t.Helper()
	testDir := t.TempDir()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("Failed to change directory to %s: %v", testDir, err)
	}

	// teardown function
	return func() {
		os.Chdir(originalWd)
	}
}

// executeCommand executes a cobra command and captures its output.
func executeCommand(args ...string) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := executeWithArgs(args)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	return buf.String(), err
}

// executeWithArgs runs cobra using provided args
func executeWithArgs(args []string) error {
	root := NewRootCmd()
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && !isSubcommand(root, args[0]) {
		return createTaskFromArgs(args)
	}
	root.SetArgs(args)
	return root.Execute()
}

// test init command behavior
func TestInitCmd(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	output, err := executeCommand("init")
	if err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	if !strings.Contains(output, "Punchlist project initialized successfully.") {
		t.Errorf("Expected success message, but got: %s", output)
	}

	// check that .punchlist/config.yaml was created
	if _, err := os.Stat(filepath.Join(config.PunchlistDir, "config.yaml")); os.IsNotExist(err) {
		t.Errorf(".punchlist/config.yaml was not created")
	}

	// check that tasks directory was created
	if info, err := os.Stat("tasks"); os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		t.Errorf("tasks directory was not created")
	}
}

// test task creation via implicit pin
func TestPinCmd(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	// init project first
	_, err := executeCommand("init")
	if err != nil {
		t.Fatalf("init command failed: %v", err)
	}
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("Failed to resolve tasks dir: %v", err)
	}

	t.Run("creates a task with all modifiers", func(t *testing.T) {
		// run pin command
		output, err := executeCommand("todo", "My test task", "pri:1", "due:2025-01-01", "tags:{test,important}")
		if err != nil {
			t.Fatalf("pin command failed: %v", err)
		}

		if !strings.Contains(output, "Created task 1:") {
			t.Errorf("Expected success message for task 1, but got: %s", output)
		}

		// check that task file was created
		taskFile := filepath.Join(tasksPath, "001-my-test-task.md")
		content, err := os.ReadFile(taskFile)
		if err != nil {
			t.Fatalf("Failed to read task file: %v", err)
		}

		// check for new data in the file content
		if !strings.Contains(string(content), "state: TODO") {
			t.Errorf("state was not set correctly in the task file")
		}
		if !strings.Contains(string(content), "priority: 1") {
			t.Errorf("priority was not set correctly in the task file")
		}
		if !strings.Contains(string(content), "due:") || !strings.Contains(string(content), "2025-01-01") {
			t.Errorf("due date was not set correctly in the task file")
		}
		if !strings.Contains(string(content), "tags:") || !strings.Contains(string(content), "- test") || !strings.Contains(string(content), "- important") {
			t.Errorf("tags were not set correctly in the task file")
		}

		// check that next_id was updated
		cfg, err := config.LoadConfig()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}
		if cfg.NextID != 2 {
			t.Errorf("Expected NextID to be 2, but got %d", cfg.NextID)
		}
	})

	t.Run("defaults to TODO when state is omitted", func(t *testing.T) {
		output, err := executeCommand("Default state task")
		if err != nil {
			t.Fatalf("pin command failed: %v", err)
		}

		if !strings.Contains(output, "Created task 2:") {
			t.Errorf("Expected success message for task 2, but got: %s", output)
		}

		taskFile := filepath.Join(tasksPath, "002-default-state-task.md")
		content, err := os.ReadFile(taskFile)
		if err != nil {
			t.Fatalf("Failed to read task file: %v", err)
		}

		if !strings.Contains(string(content), "state: TODO") {
			t.Errorf("default state was not set correctly in the task file")
		}
	})

	t.Run("creates a DONE task via done subcommand", func(t *testing.T) {
		output, err := executeCommand("done", "Quick done task")
		if err != nil {
			t.Fatalf("done command failed: %v", err)
		}
		if !strings.Contains(output, "Created task 3:") {
			t.Errorf("Expected success message for task 3, but got: %s", output)
		}

		taskFile := filepath.Join(tasksPath, "003-quick-done-task.md")
		content, err := os.ReadFile(taskFile)
		if err != nil {
			t.Fatalf("Failed to read task file: %v", err)
		}
		if !strings.Contains(string(content), "state: DONE") {
			t.Errorf("done state was not set correctly in the task file")
		}
	})

	t.Run("creates a BEGUN task via begun subcommand", func(t *testing.T) {
		output, err := executeCommand("begun", "Quick begun task")
		if err != nil {
			t.Fatalf("begun command failed: %v", err)
		}
		if !strings.Contains(output, "Created task 4:") {
			t.Errorf("Expected success message for task 4, but got: %s", output)
		}

		taskFile := filepath.Join(tasksPath, "004-quick-begun-task.md")
		content, err := os.ReadFile(taskFile)
		if err != nil {
			t.Fatalf("Failed to read task file: %v", err)
		}
		if !strings.Contains(string(content), "state: BEGUN") {
			t.Errorf("begun state was not set correctly in the task file")
		}
	})

	t.Run("creates a BLOCK task via block subcommand", func(t *testing.T) {
		output, err := executeCommand("block", "Quick block task")
		if err != nil {
			t.Fatalf("block command failed: %v", err)
		}
		if !strings.Contains(output, "Created task 5:") {
			t.Errorf("Expected success message for task 5, but got: %s", output)
		}

		taskFile := filepath.Join(tasksPath, "005-quick-block-task.md")
		content, err := os.ReadFile(taskFile)
		if err != nil {
			t.Fatalf("Failed to read task file: %v", err)
		}
		if !strings.Contains(string(content), "state: BLOCK") {
			t.Errorf("block state was not set correctly in the task file")
		}
	})

	t.Run("creates a task in another directory by path", func(t *testing.T) {
		otherDir := t.TempDir()

		if err := withWorkingDir(otherDir, func() error {
			_, err := executeCommand("init")
			return err
		}); err != nil {
			t.Fatalf("init command failed in other dir: %v", err)
		}

		output, err := executeCommand("todo", otherDir, "Path task")
		if err != nil {
			t.Fatalf("pin command failed: %v", err)
		}
		if !strings.Contains(output, "Created task 1:") {
			t.Errorf("Expected success message for path task, but got: %s", output)
		}

		taskFile := filepath.Join(otherDir, "tasks", "001-path-task.md")
		if _, err := os.Stat(taskFile); err != nil {
			t.Fatalf("Failed to stat task file: %v", err)
		}
	})
}

// test state changes and ls filters
func TestStateAndLsCmds(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	// init and create tasks manually
	executeCommand("init")
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("Failed to resolve tasks dir: %v", err)
	}
	os.MkdirAll(tasksPath, 0755)

	task1 := &task.Task{ID: 1, Title: "Task 1", State: task.StateTodo, Priority: 1, Tags: []string{"team-a", "urgent"}}
	task1.Write(filepath.Join(tasksPath, "001-task-1.md"))

	task2 := &task.Task{ID: 2, Title: "Task 2", State: task.StateBegun, Priority: 2, Tags: []string{"team-b"}}
	task2.Write(filepath.Join(tasksPath, "002-task-2.md"))

	task3 := &task.Task{ID: 3, Title: "Task 3", State: task.StateTodo, Priority: 1, Tags: []string{"team-a"}}
	task3.Write(filepath.Join(tasksPath, "003-task-3.md"))

	task4 := &task.Task{ID: 4, Title: "Task 4", State: task.StateBlock, Priority: 3, Tags: []string{"team-c"}}
	task4.Write(filepath.Join(tasksPath, "004-task-4.md"))

	output, err := executeCommand("ls")
	if err != nil {
		t.Fatalf("ls command failed: %v", err)
	}
	t.Logf("ls output:\n%s", output)
	if strings.Count(output, stateSeparatorLine) != 2 {
		t.Errorf("ls output should include 2 state separators, got %d", strings.Count(output, stateSeparatorLine))
	}

	output, err = executeCommand("ls", "todo")
	if err != nil {
		t.Fatalf("ls todo command failed: %v", err)
	}
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 3") {
		t.Errorf("ls todo should include Task 1 and Task 3. Got: %s", output)
	}
	if strings.Contains(output, "Task 2") || strings.Contains(output, "Task 4") {
		t.Errorf("ls todo should not include Task 2 or Task 4. Got: %s", output)
	}

	output, err = executeCommand("ls", "--state", "todo")
	if err != nil {
		t.Fatalf("ls --state todo command failed: %v", err)
	}
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 3") {
		t.Errorf("ls --state todo should include Task 1 and Task 3. Got: %s", output)
	}
	if strings.Contains(output, "Task 2") || strings.Contains(output, "Task 4") {
		t.Errorf("ls --state todo should not include Task 2 or Task 4. Got: %s", output)
	}

	output, err = executeCommand("ls", "--status", "todo")
	if err != nil {
		t.Fatalf("ls --status todo command failed: %v", err)
	}
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 3") {
		t.Errorf("ls --status todo should include Task 1 and Task 3. Got: %s", output)
	}
	if strings.Contains(output, "Task 2") || strings.Contains(output, "Task 4") {
		t.Errorf("ls --status todo should not include Task 2 or Task 4. Got: %s", output)
	}

	// test ls with priority filter
	output, err = executeCommand("ls", "--pri", "1")
	if err != nil {
		t.Fatalf("ls --pri command failed: %v", err)
	}
	if !strings.Contains(output, "Task 1") {
		t.Errorf("ls --pri=1 should contain Task 1. Got: %s", output)
	}
	if strings.Contains(output, "Task 2") {
		t.Errorf("ls --pri=1 should not contain Task 2. Got: %s", output)
	}
	if !strings.Contains(output, "Task 3") {
		t.Errorf("ls --pri=1 should contain Task 3. Got: %s", output)
	}

	// test ls with tag filter
	output, err = executeCommand("ls", "--tag", "team-b")
	if err != nil {
		t.Fatalf("ls --tag command failed: %v", err)
	}
	if strings.Contains(output, "Task 1") {
		t.Errorf("ls --tag=team-b should not contain Task 1. Got: %s", output)
	}
	if !strings.Contains(output, "Task 2") {
		t.Errorf("ls --tag=team-b should contain Task 2. Got: %s", output)
	}

	// test ls with multiple filters
	output, err = executeCommand("ls", "--state", "todo", "--pri", "1", "--tag", "urgent")
	if err != nil {
		t.Fatalf("ls with multiple filters failed: %v", err)
	}
	if !strings.Contains(output, "Task 1") {
		t.Errorf("ls with multiple filters should contain Task 1. Got: %s", output)
	}
	if strings.Contains(output, "Task 2") {
		t.Errorf("ls with multiple filters should not contain Task 2. Got: %s", output)
	}
	if strings.Contains(output, "Task 3") {
		t.Errorf("ls with multiple filters should not contain Task 3. Got: %s", output)
	}

	// test ls with block state
	output, err = executeCommand("ls", "block")
	if err != nil {
		t.Fatalf("ls block failed: %v", err)
	}
	if !strings.Contains(output, "Task 4") {
		t.Errorf("ls block should contain Task 4. Got: %s", output)
	}
}

func TestSearchCmd(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	executeCommand("init")
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("Failed to resolve tasks dir: %v", err)
	}
	os.MkdirAll(tasksPath, 0755)

	task1 := &task.Task{
		ID:    1,
		Title: "ExaGrid setup",
		State: task.StateTodo,
		Body:  "## Notes\n\n- called ExaGrid support\n\n## Log\n\n- 2026-01-01: moved to TODO\n",
	}
	task1.Write(filepath.Join(tasksPath, "001-exagrid-setup.md"))

	task2 := &task.Task{
		ID:    2,
		Title: "Follow-up",
		State: task.StateDone,
		Body:  "Investigate exagrid appliance options.\n",
	}
	task2.Write(filepath.Join(tasksPath, "002-follow-up.md"))

	task3 := &task.Task{
		ID:    3,
		Title: "Log only",
		State: task.StateTodo,
		Body:  "## Log\n\n- mention exagrid in log only\n",
	}
	task3.Write(filepath.Join(tasksPath, "003-log-only.md"))

	output, err := executeCommand("search", "exagrid")
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}
	if !strings.Contains(output, "ExaGrid setup") || !strings.Contains(output, "Follow-up") {
		t.Errorf("search should include Task 1 and Task 2. Got: %s", output)
	}
	if strings.Contains(output, "Log only") {
		t.Errorf("search should not include Task 3 (log-only). Got: %s", output)
	}

	output, err = executeCommand("search", "--state", "done", "exagrid")
	if err != nil {
		t.Fatalf("search --state done command failed: %v", err)
	}
	if !strings.Contains(output, "Follow-up") || strings.Contains(output, "ExaGrid setup") {
		t.Errorf("search --state done should include only Task 2. Got: %s", output)
	}

	output, err = executeCommand("search", "--status", "done", "exagrid")
	if err != nil {
		t.Fatalf("search --status done command failed: %v", err)
	}
	if !strings.Contains(output, "Follow-up") || strings.Contains(output, "ExaGrid setup") {
		t.Errorf("search --status done should include only Task 2. Got: %s", output)
	}
}

func TestTagCmd(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	executeCommand("init")
	executeCommand("todo", "Task 1", "tags:{alpha}")
	executeCommand("todo", "Task 2")

	output, err := executeCommand("tag", "1", "2", "today, urgent")
	if err != nil {
		t.Fatalf("tag command failed: %v", err)
	}
	if !strings.Contains(output, "Updated tags for task 1") || !strings.Contains(output, "Updated tags for task 2") {
		t.Errorf("Expected success messages, got: %s", output)
	}

	task1, err := task.Parse("tasks/001-task-1.md")
	if err != nil {
		t.Fatalf("Failed to parse task 1: %v", err)
	}
	task2, err := task.Parse("tasks/002-task-2.md")
	if err != nil {
		t.Fatalf("Failed to parse task 2: %v", err)
	}

	if !hasTag(task1.Tags, "alpha") || !hasTag(task1.Tags, "today") || !hasTag(task1.Tags, "urgent") {
		t.Errorf("Task 1 tags not updated as expected: %v", task1.Tags)
	}
	if !hasTag(task2.Tags, "today") || !hasTag(task2.Tags, "urgent") {
		t.Errorf("Task 2 tags not updated as expected: %v", task2.Tags)
	}
}

// test delete command behavior
func TestDeleteCmd(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	executeCommand("init")
	executeCommand("todo", "My delete task")
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("Failed to resolve tasks dir: %v", err)
	}
	trashPath, err := trashDir()
	if err != nil {
		t.Fatalf("Failed to resolve trash dir: %v", err)
	}

	output, err := executeCommand("del", "1")
	if err != nil {
		t.Fatalf("del command failed: %v", err)
	}
	if !strings.Contains(output, "Moved task 1 to") {
		t.Errorf("Expected success message, but got: %s", output)
	}

	if _, err := os.Stat(filepath.Join(tasksPath, "001-my-delete-task.md")); !os.IsNotExist(err) {
		t.Errorf("Expected task file to be removed, but it still exists")
	}

	if _, err := os.Stat(filepath.Join(trashPath, "001-my-delete-task.md")); err != nil {
		t.Errorf("Expected task file to be in trash, but got error: %v", err)
	}
}

func TestLsPathTarget(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	otherDir := t.TempDir()

	if err := withWorkingDir(otherDir, func() error {
		_, err := executeCommand("init")
		if err != nil {
			return err
		}
		_, err = executeCommand("todo", "Other task")
		return err
	}); err != nil {
		t.Fatalf("Failed to setup other dir tasks: %v", err)
	}

	output, err := executeCommand("ls", otherDir)
	if err != nil {
		t.Fatalf("ls with path failed: %v", err)
	}
	if !strings.Contains(output, "Other task") {
		t.Errorf("ls with path should include task from target dir. Got: %s", output)
	}
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
