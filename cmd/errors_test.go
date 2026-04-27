package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"punchlist/config"
)

func TestNotPunchlistHint_FromTasksDirOfRealPunchlist(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.PunchlistDir), 0755); err != nil {
		t.Fatalf("mkdir punchlist: %v", err)
	}
	tasks := filepath.Join(root, "tasks")
	if err := os.MkdirAll(tasks, 0755); err != nil {
		t.Fatalf("mkdir tasks: %v", err)
	}

	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	if err := os.Chdir(tasks); err != nil {
		t.Fatalf("chdir tasks: %v", err)
	}

	got := notPunchlistHint()
	if !strings.Contains(got, "tasks/ folder") || !strings.Contains(got, "cd ..") {
		t.Errorf("expected tasks/ hint with 'cd ..', got: %q", got)
	}
}

func TestNotPunchlistHint_FromUnrelatedDir(t *testing.T) {
	root := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	if got := notPunchlistHint(); got != notPunchlistMessage {
		t.Errorf("expected default message, got: %q", got)
	}
}

func TestNotPunchlistHint_FromTasksDirWithoutSibling(t *testing.T) {
	root := t.TempDir()
	tasks := filepath.Join(root, "tasks")
	if err := os.MkdirAll(tasks, 0755); err != nil {
		t.Fatalf("mkdir tasks: %v", err)
	}
	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	if err := os.Chdir(tasks); err != nil {
		t.Fatalf("chdir tasks: %v", err)
	}
	if got := notPunchlistHint(); got != notPunchlistMessage {
		t.Errorf("tasks/ without sibling .punchlist should fall back to default, got: %q", got)
	}
}
