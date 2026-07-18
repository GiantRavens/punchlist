package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tasks may live in subfolders of tasks/ (pin ls walks recursively), so by-id
// resolution must agree. Regression tests for the futhark bridge punchlist
// deck failure (2026-07-15): ls listed subfolder tasks that show/note/pri
// could not find.
func TestFindTaskFileInSubfolder(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "top-level task")

	orig, err := findTaskFile(1)
	if err != nil {
		t.Fatalf("expected task 1 at top level: %v", err)
	}

	// relocate the task into a subfolder, as dedup/recovery stashes do
	sub := filepath.Join("tasks", "_recovery")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(sub, filepath.Base(orig))
	if err := os.Rename(orig, moved); err != nil {
		t.Fatal(err)
	}

	got, err := findTaskFile(1)
	if err != nil {
		t.Fatalf("findTaskFile should resolve tasks in subfolders: %v", err)
	}
	wantAbs, _ := filepath.Abs(moved)
	if got != wantAbs {
		t.Errorf("expected %s, got %s", wantAbs, got)
	}
}

func TestFindTaskFileAmbiguousID(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")
	executeCommand("todo", "duplicated task")

	orig, err := findTaskFile(1)
	if err != nil {
		t.Fatalf("expected task 1 at top level: %v", err)
	}
	content, err := os.ReadFile(orig)
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join("tasks", "dup")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, filepath.Base(orig)), content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = findTaskFile(1)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error when two files share an id, got %v", err)
	}
}

func TestFindTaskFileNotFound(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")

	_, err := findTaskFile(999)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// failf must record a non-zero exit status (applied by Execute after the
// write-lock post-run) instead of the old print-to-stderr-and-exit-0, which
// made callers like the bridge unable to tell success from failure.
func TestShowMissingTaskRecordsFailure(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()
	executeCommand("init")

	exitCode = 0
	defer func() { exitCode = 0 }()
	executeCommand("show", "999")
	if exitCode != 1 {
		t.Errorf("expected exitCode 1 after show of a missing task, got %d", exitCode)
	}
}
