package cmd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"punchlist/task"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// makeOversizeBrowseTask builds a task whose assembled browse block exceeds
// 20KB — the regime that previously skipped word-wrapping entirely — with
// long lines, an unbroken URL-like token, and tab characters.
func makeOversizeBrowseTask() *task.Task {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	longLine := strings.Repeat("wrapcheck ", 30) + "https://example.com/" + strings.Repeat("abcdefghij", 30)

	details := []string{"TOPMARKER start of details"}
	for i := 0; i < 15; i++ {
		details = append(details, longLine)
	}
	details = append(details, "col1\tcol2\tcol3\ttabbed line")

	notes := []string{"## Notes"}
	for i := 0; i < 20; i++ {
		notes = append(notes, fmt.Sprintf("- 2026-01-%02dT00:00:00Z: %s", i+1, longLine))
	}

	logs := []string{"## Log"}
	for i := 0; i < 20; i++ {
		logs = append(logs, fmt.Sprintf("- 2026-01-%02dT00:00:00Z: %s", i+1, longLine))
	}

	return &task.Task{
		ID:        7,
		Title:     "Oversize task with long lines",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Body:      strings.Join(details, "\n") + "\n\n" + strings.Join(notes, "\n") + "\n\n" + strings.Join(logs, "\n"),
	}
}

func makeOversizeBrowseModel(width, height, margin int) model {
	return model{
		tasks:  []*task.Task{makeOversizeBrowseTask()},
		cursor: 0,
		width:  width,
		height: height,
		margin: margin,
		mode:   modeBrowse,
	}
}

// The load-bearing invariant: every View line must fit the terminal width and
// the View must never emit more lines than the terminal height. Violating
// either makes bubbletea's renderer drop lines from the top of the screen,
// which presents as "long tasks don't start at the top / can't scroll".
func TestBrowseViewNeverExceedsTerminalBounds(t *testing.T) {
	geometries := []struct {
		width, height, margin int
	}{
		{100, 30, 10}, // full-size terminal, default-ish margin
		{60, 15, 2},   // tmux split pane
		{34, 9, 0},    // pathologically narrow pane
	}
	for _, g := range geometries {
		t.Run(fmt.Sprintf("%dx%d-m%d", g.width, g.height, g.margin), func(t *testing.T) {
			m := makeOversizeBrowseModel(g.width, g.height, g.margin)
			view := m.View()
			lines := strings.Split(view, "\n")
			if len(lines) > g.height {
				t.Fatalf("view emits %d lines for terminal height %d — renderer will chop the top", len(lines), g.height)
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > g.width {
					t.Fatalf("view line %d has visual width %d > terminal width %d — it will wrap and overflow the screen:\n%q", i, w, g.width, line)
				}
			}
		})
	}
}

func TestBrowseOversizeTaskStartsAtTop(t *testing.T) {
	m := makeOversizeBrowseModel(60, 15, 2)
	view := m.View()
	if !strings.Contains(view, "TOPMARKER") {
		t.Fatalf("expected initial view of an oversize task to start at the top, got:\n%s", view)
	}
	if !strings.Contains(view, "q:quit") {
		t.Fatalf("expected footer to remain visible, got:\n%s", view)
	}
}

func TestBrowseOversizeTaskScrolls(t *testing.T) {
	m := makeOversizeBrowseModel(60, 15, 2)
	if m.maxContentScroll() == 0 {
		t.Fatal("expected an oversize task to be scrollable (maxContentScroll > 0)")
	}

	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	updated := updatedModel.(model)
	view := updated.View()
	if strings.Contains(view, "TOPMARKER") {
		t.Fatalf("expected end-scrolled view to hide the top of the task, got:\n%s", view)
	}
	if !strings.Contains(view, "q:quit") {
		t.Fatalf("expected footer to remain visible after scroll, got:\n%s", view)
	}
}

func TestBrowseMouseWheelScrolls(t *testing.T) {
	m := makeOversizeBrowseModel(60, 15, 2)

	down := tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	updatedModel, _ := m.Update(down)
	updated := updatedModel.(model)
	if updated.contentScroll != browseWheelStep {
		t.Fatalf("expected wheel-down to scroll %d lines, got %d", browseWheelStep, updated.contentScroll)
	}

	up := tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}
	updatedModel, _ = updated.Update(up)
	updated = updatedModel.(model)
	if updated.contentScroll != 0 {
		t.Fatalf("expected wheel-up to scroll back to 0, got %d", updated.contentScroll)
	}
}
