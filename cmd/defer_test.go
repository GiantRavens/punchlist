package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"punchlist/config"
	"punchlist/task"
)

// pin#45: `pin defer N` used to write NOTDO because the notdo command claimed
// "defer"/"DEFER" as cobra aliases, shadowing the catalog's DEFER token.
func TestDeferAndNotDoAreDistinctStates(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := executeCommand("todo", "park me"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := executeCommand("todo", "drop me"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := executeCommand("defer", "1"); err != nil {
		t.Fatalf("defer: %v", err)
	}
	if _, err := executeCommand("notdo", "2"); err != nil {
		t.Fatalf("notdo: %v", err)
	}

	assertState := func(id int, want task.State) {
		t.Helper()
		path, err := findTaskFile(id)
		if err != nil {
			t.Fatalf("find task %d: %v", id, err)
		}
		tsk, err := task.Parse(path)
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Base(path), err)
		}
		if tsk.State != want {
			t.Errorf("task %d: state = %s, want %s", id, tsk.State, want)
		}
	}
	assertState(1, task.StateDefer)
	assertState(2, task.StateNotDo)

	// creation from a title goes to DEFER too
	if _, err := executeCommand("defer", "someday: revisit the plan"); err != nil {
		t.Fatalf("defer create: %v", err)
	}
	assertState(3, task.StateDefer)
}

// Every state verb that claims a token the default catalog knows must be the
// verb designated for that state — otherwise the cobra alias shadows the
// generic `pin <STATE> <ids>` path and silently writes the wrong state
// (the pin#45 class).
func TestStateVerbsDoNotShadowCatalogTokens(t *testing.T) {
	designated := map[string]string{ // command name -> catalog state it writes
		"todo":    "TODO",
		"start":   "BEGUN",
		"done":    "DONE",
		"notdo":   "NOTDO",
		"defer":   "DEFER",
		"block":   "BLOCKED",
		"confirm": "FOLLOWUP",
	}
	catalog := config.DefaultStateCatalog()
	root := NewRootCmd()
	for _, cmd := range root.Commands() {
		want, isStateVerb := designated[cmd.Name()]
		tokens := append([]string{cmd.Name()}, cmd.Aliases...)
		for _, tok := range tokens {
			canonical, known := catalog.Resolve(tok)
			if !known {
				continue
			}
			if !isStateVerb {
				t.Errorf("command %q claims catalog token %q (state %s) but is not a state verb", cmd.Name(), tok, canonical)
				continue
			}
			if !strings.EqualFold(canonical, want) {
				t.Errorf("command %q claims token %q which the catalog resolves to %s, but the verb writes %s", cmd.Name(), tok, canonical, want)
			}
		}
	}
}
