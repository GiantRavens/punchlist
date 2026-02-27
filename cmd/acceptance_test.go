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
