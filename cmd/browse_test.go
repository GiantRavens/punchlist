package cmd

import (
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

	content := renderBrowseContent(tsk, 80)
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
