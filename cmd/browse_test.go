package cmd

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"punchlist/task"

	tea "github.com/charmbracelet/bubbletea"
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

func TestBrowseViewStartsLongTaskAtTopAndKeepsFooter(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	bodyLines := []string{"# Long task"}
	for i := 1; i <= 30; i++ {
		bodyLines = append(bodyLines, "line "+strconv.Itoa(i))
	}
	tsk := &task.Task{
		ID:        6,
		Title:     "Long task",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Body:      strings.Join(bodyLines, "\n"),
	}

	m := model{
		tasks:  []*task.Task{tsk},
		cursor: 0,
		width:  100,
		height: 8,
		margin: 0,
		mode:   modeBrowse,
	}

	view := m.View()
	if !strings.Contains(view, "Long task") {
		t.Fatalf("expected initial view to include task title, got:\n%s", view)
	}
	if !strings.Contains(view, "line 1") {
		t.Fatalf("expected initial view to start at top of body, got:\n%s", view)
	}
	if strings.Contains(view, "line 30") {
		t.Fatalf("expected initial view to hide bottom of long body, got:\n%s", view)
	}
	if !strings.Contains(view, "q:quit") {
		t.Fatalf("expected fixed footer to remain visible, got:\n%s", view)
	}
}

func TestBrowseViewScrollsLongTaskBody(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	bodyLines := []string{"# Long task"}
	for i := 1; i <= 30; i++ {
		bodyLines = append(bodyLines, "line "+strconv.Itoa(i))
	}
	tsk := &task.Task{
		ID:        6,
		Title:     "Long task",
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Body:      strings.Join(bodyLines, "\n"),
	}

	m := model{
		tasks:  []*task.Task{tsk},
		cursor: 0,
		width:  100,
		height: 8,
		margin: 0,
		mode:   modeBrowse,
	}

	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	updated, ok := updatedModel.(model)
	if !ok {
		t.Fatalf("expected model, got %T", updatedModel)
	}
	view := updated.View()
	if strings.Contains(view, "line 1") {
		t.Fatalf("expected scrolled view to hide top body line, got:\n%s", view)
	}
	if !strings.Contains(view, "line 30") {
		t.Fatalf("expected scrolled view to show bottom body line, got:\n%s", view)
	}
	if !strings.Contains(view, "q:quit") {
		t.Fatalf("expected fixed footer to remain visible after scroll, got:\n%s", view)
	}
}

func makeBrowseModelWithTasks(n int) model {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	tasks := make([]*task.Task, n)
	for i := 0; i < n; i++ {
		tasks[i] = &task.Task{
			ID:        i + 1,
			Title:     "task " + strconv.Itoa(i+1),
			State:     task.StateTodo,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return model{
		tasks:  tasks,
		cursor: 0,
		width:  100,
		height: 24,
		margin: 0,
		mode:   modeBrowse,
	}
}

func sendRune(m model, r rune) model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(model)
}

func TestBrowseGCapitalJumpsToLastTask(t *testing.T) {
	m := makeBrowseModelWithTasks(5)
	m.cursor = 1
	m.contentScroll = 7

	updated := sendRune(m, 'G')

	if updated.cursor != 4 {
		t.Fatalf("expected cursor at last task (4), got %d", updated.cursor)
	}
	if updated.contentScroll != 0 {
		t.Fatalf("expected contentScroll reset to 0, got %d", updated.contentScroll)
	}
}

func TestBrowseDoubleGJumpsToFirstTask(t *testing.T) {
	m := makeBrowseModelWithTasks(5)
	m.cursor = 3
	m.contentScroll = 4

	afterFirstG := sendRune(m, 'g')
	if !afterFirstG.pendingG {
		t.Fatalf("expected pendingG=true after single g, got false")
	}
	if afterFirstG.cursor != 3 {
		t.Fatalf("expected cursor unchanged on first g, got %d", afterFirstG.cursor)
	}

	afterSecondG := sendRune(afterFirstG, 'g')
	if afterSecondG.pendingG {
		t.Fatalf("expected pendingG=false after gg, got true")
	}
	if afterSecondG.cursor != 0 {
		t.Fatalf("expected cursor at first task (0), got %d", afterSecondG.cursor)
	}
	if afterSecondG.contentScroll != 0 {
		t.Fatalf("expected contentScroll reset to 0, got %d", afterSecondG.contentScroll)
	}
}

func TestBrowsePendingGCanceledByOtherKey(t *testing.T) {
	m := makeBrowseModelWithTasks(5)
	m.cursor = 2

	afterG := sendRune(m, 'g')
	if !afterG.pendingG {
		t.Fatalf("expected pendingG=true, got false")
	}

	afterRight, _ := afterG.Update(tea.KeyMsg{Type: tea.KeyRight})
	afterRightModel := afterRight.(model)
	if afterRightModel.pendingG {
		t.Fatalf("expected non-g key to cancel pendingG, still true")
	}
	if afterRightModel.cursor != 3 {
		t.Fatalf("expected right-arrow to advance cursor to 3, got %d", afterRightModel.cursor)
	}

	afterSecondG := sendRune(afterRightModel, 'g')
	if !afterSecondG.pendingG {
		t.Fatalf("expected fresh pendingG=true, got false")
	}
	if afterSecondG.cursor != 3 {
		t.Fatalf("expected single-g not to jump (canceled gg sequence), got cursor=%d", afterSecondG.cursor)
	}
}
