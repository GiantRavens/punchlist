package cmd

import (
	"encoding/json"
	"punchlist/task"
	"strings"
	"testing"
)

func TestParseAcceptance(t *testing.T) {
	body := "# My Task\n\n## Acceptance\n\n- [ ] First criterion\n- [x] Second criterion\n- [ ] Third criterion\n\n## Notes\n\n- Some note\n"

	items := parseAcceptance(body)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Text != "First criterion" || items[0].Checked {
		t.Errorf("item 0 wrong: %+v", items[0])
	}
	if items[1].Text != "Second criterion" || !items[1].Checked {
		t.Errorf("item 1 wrong: %+v", items[1])
	}
	if items[2].Index != 3 {
		t.Errorf("expected index 3, got %d", items[2].Index)
	}
}

func TestParseAcceptanceNoSection(t *testing.T) {
	body := "# My Task\n\nJust a body."
	items := parseAcceptance(body)
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestToggleAcceptanceCheck(t *testing.T) {
	body := "# Task\n\n## Acceptance\n\n- [ ] Unchecked\n- [x] Checked\n\n## Notes\n"

	t.Run("check an unchecked item", func(t *testing.T) {
		newBody, nowChecked, err := toggleAcceptanceCheck(body, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !nowChecked {
			t.Error("expected nowChecked=true")
		}
		if !strings.Contains(newBody, "[x] Unchecked") {
			t.Errorf("expected toggled checkbox, got: %s", newBody)
		}
	})

	t.Run("uncheck a checked item", func(t *testing.T) {
		newBody, nowChecked, err := toggleAcceptanceCheck(body, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if nowChecked {
			t.Error("expected nowChecked=false")
		}
		if !strings.Contains(newBody, "[ ] Checked") {
			t.Errorf("expected toggled checkbox, got: %s", newBody)
		}
	})

	t.Run("index out of range", func(t *testing.T) {
		_, _, err := toggleAcceptanceCheck(body, 5)
		if err == nil {
			t.Error("expected error for out-of-range index")
		}
	})

	t.Run("no acceptance section", func(t *testing.T) {
		_, _, err := toggleAcceptanceCheck("# No acceptance here", 1)
		if err == nil {
			t.Error("expected error for missing section")
		}
	})
}

func TestAcceptanceCmdIntegration(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "AC task")

	// write acceptance section to the task body directly
	taskPath, _ := findTaskFile(1)
	tsk, _ := task.Parse(taskPath)
	tsk.Body = "# AC task\n\n## Acceptance\n\n- [ ] First check\n- [ ] Second check\n"
	tsk.Write(taskPath)

	t.Run("list acceptance criteria", func(t *testing.T) {
		output, err := executeCommand("acceptance", "1")
		if err != nil {
			t.Fatalf("acceptance command failed: %v", err)
		}
		if !strings.Contains(output, "1. [ ] First check") {
			t.Errorf("expected formatted acceptance items, got: %s", output)
		}
		if !strings.Contains(output, "2. [ ] Second check") {
			t.Errorf("expected second item, got: %s", output)
		}
	})

	t.Run("toggle a check", func(t *testing.T) {
		output, err := executeCommand("check", "1", "1")
		if err != nil {
			t.Fatalf("check command failed: %v", err)
		}
		if !strings.Contains(output, "checked") {
			t.Errorf("expected toggle confirmation, got: %s", output)
		}

		// verify persistence
		tsk, _ := task.Parse(taskPath)
		items := parseAcceptance(tsk.Body)
		if !items[0].Checked {
			t.Error("expected first item to be checked after toggle")
		}
		if items[1].Checked {
			t.Error("expected second item to remain unchecked")
		}
	})

	t.Run("acceptance in show json", func(t *testing.T) {
		output, err := executeCommand("show", "--json", "1")
		if err != nil {
			t.Fatalf("show --json failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		acceptance, ok := result["acceptance"].([]interface{})
		if !ok {
			t.Fatalf("expected acceptance array, got %T", result["acceptance"])
		}
		if len(acceptance) != 2 {
			t.Fatalf("expected 2 acceptance items, got %d", len(acceptance))
		}
		first := acceptance[0].(map[string]interface{})
		if first["checked"] != true {
			t.Errorf("expected first item checked=true, got %v", first["checked"])
		}
	})

	t.Run("checks alias works", func(t *testing.T) {
		output, err := executeCommand("checks", "1")
		if err != nil {
			t.Fatalf("checks alias failed: %v", err)
		}
		if !strings.Contains(output, "1. [x] First check") {
			t.Errorf("expected checked item via alias, got: %s", output)
		}
	})
}

func TestAddAcceptanceItem(t *testing.T) {
	t.Run("appends to existing section", func(t *testing.T) {
		body := "# Task\n\n## Acceptance\n\n- [ ] First\n\n## Notes\n\n- a note"
		newBody := addAcceptanceItem(body, "Second")
		items := parseAcceptance(newBody)
		if len(items) != 2 || items[1].Text != "Second" || items[1].Checked {
			t.Fatalf("expected appended unchecked item, got %+v", items)
		}
		if !strings.Contains(newBody, "## Notes") || !strings.Contains(newBody, "- a note") {
			t.Errorf("notes section damaged: %s", newBody)
		}
	})

	t.Run("creates section before notes and log", func(t *testing.T) {
		body := "# Task\n\n## Notes\n\n- a note\n\n## Log\n\n- created"
		newBody := addAcceptanceItem(body, "Only criterion")
		items := parseAcceptance(newBody)
		if len(items) != 1 || items[0].Text != "Only criterion" {
			t.Fatalf("expected one item, got %+v", items)
		}
		accIdx := strings.Index(newBody, "## Acceptance")
		notesIdx := strings.Index(newBody, "## Notes")
		logIdx := strings.Index(newBody, "## Log")
		if !(accIdx < notesIdx && notesIdx < logIdx) {
			t.Errorf("section order wrong (acc=%d notes=%d log=%d):\n%s", accIdx, notesIdx, logIdx, newBody)
		}
	})

	t.Run("creates section on bare body", func(t *testing.T) {
		newBody := addAcceptanceItem("# Task", "Criterion")
		items := parseAcceptance(newBody)
		if len(items) != 1 {
			t.Fatalf("expected one item, got %+v", items)
		}
	})
}

func TestRemoveAcceptanceItem(t *testing.T) {
	body := "# Task\n\n## Acceptance\n\n- [ ] First\n- [x] Second\n- [ ] Third\n\n## Notes\n"

	t.Run("removes by index", func(t *testing.T) {
		newBody, removedText, err := removeAcceptanceItem(body, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if removedText != "Second" {
			t.Errorf("expected removed text 'Second', got %q", removedText)
		}
		items := parseAcceptance(newBody)
		if len(items) != 2 || items[0].Text != "First" || items[1].Text != "Third" {
			t.Errorf("unexpected remaining items: %+v", items)
		}
	})

	t.Run("index out of range", func(t *testing.T) {
		if _, _, err := removeAcceptanceItem(body, 9); err == nil {
			t.Error("expected error for out-of-range index")
		}
	})

	t.Run("no section", func(t *testing.T) {
		if _, _, err := removeAcceptanceItem("# Bare", 1); err == nil {
			t.Error("expected error for missing section")
		}
	})
}

func TestAcceptanceAddRmCmdIntegration(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "AC add task")

	output, err := executeCommand("acceptance", "add", "1", "First criterion")
	if err != nil {
		t.Fatalf("acceptance add failed: %v", err)
	}
	if !strings.Contains(output, "Added acceptance criterion 1 to task 1") {
		t.Errorf("unexpected add output: %s", output)
	}
	executeCommand("acceptance", "add", "1", "Second criterion")

	taskPath, _ := findTaskFile(1)
	tsk, _ := task.Parse(taskPath)
	items := parseAcceptance(tsk.Body)
	if len(items) != 2 || items[0].Text != "First criterion" || items[1].Text != "Second criterion" {
		t.Fatalf("expected 2 persisted criteria, got %+v", items)
	}

	// check interoperates with added criteria
	if _, err := executeCommand("check", "1", "2"); err != nil {
		t.Fatalf("check failed on added criterion: %v", err)
	}
	tsk, _ = task.Parse(taskPath)
	if items := parseAcceptance(tsk.Body); !items[1].Checked {
		t.Error("expected second criterion checked")
	}

	output, err = executeCommand("acceptance", "rm", "1", "1")
	if err != nil {
		t.Fatalf("acceptance rm failed: %v", err)
	}
	if !strings.Contains(output, "Removed acceptance criterion 1 from task 1: First criterion") {
		t.Errorf("unexpected rm output: %s", output)
	}
	tsk, _ = task.Parse(taskPath)
	items = parseAcceptance(tsk.Body)
	if len(items) != 1 || items[0].Text != "Second criterion" {
		t.Errorf("expected only second criterion to remain, got %+v", items)
	}

	// bare listing still works with subcommands present
	output, err = executeCommand("acceptance", "1")
	if err != nil {
		t.Fatalf("acceptance list failed: %v", err)
	}
	if !strings.Contains(output, "1. [x] Second criterion") {
		t.Errorf("unexpected list output: %s", output)
	}
}
