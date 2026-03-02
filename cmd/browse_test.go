package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"punchlist/task"
)

func TestRenderBrowseContentShowsTitle(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	tsk := &task.Task{
		ID:        6,
		Title:     "Ship a browse view",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Body:      "Details go here.",
	}

	content := renderBrowseContent(tsk, 80, nil)
	if strings.TrimSpace(content) == "" {
		t.Fatal("expected non-empty browse content")
	}
	if !strings.Contains(content, tsk.Title) {
		t.Fatalf("expected browse content to include title %q", tsk.Title)
	}
}

func TestBrowseViewRendersContent(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	tsk := &task.Task{
		ID:        6,
		Title:     "Browse renders",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Body:      "Some notes.",
	}

	m := model{
		tasks:  []*task.Task{tsk},
		cursor: 0,
		width:  100,
		height: 30,
		margin: 0,
		mode:   modeBrowse,
	}

	view := m.View()
	if strings.TrimSpace(view) == "" {
		t.Fatal("expected non-empty browse view")
	}
	if !strings.Contains(view, tsk.Title) {
		t.Fatalf("expected browse view to include title %q", tsk.Title)
	}
}

func TestApplyStateChangeAdvancesCursor(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	taskPath1 := filepath.Join(t.TempDir(), "001-first.md")
	taskPath2 := filepath.Join(t.TempDir(), "002-second.md")
	first := &task.Task{
		ID:        1,
		Title:     "First",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Path:      taskPath1,
	}
	second := &task.Task{
		ID:        2,
		Title:     "Second",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Path:      taskPath2,
	}
	if err := first.Write(first.Path); err != nil {
		t.Fatalf("write first task: %v", err)
	}
	if err := second.Write(second.Path); err != nil {
		t.Fatalf("write second task: %v", err)
	}

	m := model{
		tasks:  []*task.Task{first, second},
		cursor: 0,
	}
	updated, _ := applyStateChange(m, task.StateDone)

	if updated.cursor != 1 {
		t.Fatalf("expected cursor to advance to 1, got %d", updated.cursor)
	}
	if updated.tasks[0].State != task.StateDone {
		t.Fatalf("expected first task state DONE, got %s", updated.tasks[0].State)
	}
}

func TestApplyStateChangeDoesNotAdvancePastLastTask(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	taskPath := filepath.Join(t.TempDir(), "001-last.md")
	last := &task.Task{
		ID:        1,
		Title:     "Last",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Path:      taskPath,
	}
	if err := last.Write(last.Path); err != nil {
		t.Fatalf("write task: %v", err)
	}

	m := model{
		tasks:  []*task.Task{last},
		cursor: 0,
	}
	updated, _ := applyStateChange(m, task.StateDone)

	if updated.cursor != 0 {
		t.Fatalf("expected cursor to remain at 0, got %d", updated.cursor)
	}
}
