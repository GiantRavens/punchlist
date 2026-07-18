package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"punchlist/config"
	"punchlist/task"

	tea "github.com/charmbracelet/bubbletea"
)

func sendKey(m model, keyType tea.KeyType) model {
	updated, _ := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(model)
}

func TestBrowseVimNavKeys(t *testing.T) {
	m := makeBrowseModelWithTasks(5)
	m.cursor = 2

	m = sendRune(m, 'l')
	if m.cursor != 3 {
		t.Fatalf("expected l to advance to next task (3), got %d", m.cursor)
	}
	m = sendRune(m, 'h')
	if m.cursor != 2 {
		t.Fatalf("expected h to go back to previous task (2), got %d", m.cursor)
	}

	long := makeOversizeBrowseModel(60, 15, 2)
	long = sendRune(long, 'j')
	if long.contentScroll != 1 {
		t.Fatalf("expected j to scroll down one line, got contentScroll=%d", long.contentScroll)
	}
	long = sendRune(long, 'k')
	if long.contentScroll != 0 {
		t.Fatalf("expected k to scroll back up, got contentScroll=%d", long.contentScroll)
	}
}

func makeGroupedBrowseModel() model {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	states := []task.State{task.StateTodo, task.StateTodo, task.StateBegun, task.StateBegun, task.StateDone}
	tasks := make([]*task.Task, len(states))
	for i, st := range states {
		tasks[i] = &task.Task{
			ID:        i + 1,
			Title:     "task " + string(rune('a'+i)),
			State:     st,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	m := initialModel(tasks, nil, "")
	m.width = 100
	m.height = 24
	m.margin = 0
	return m
}

func TestBrowseTabJumpsBetweenStateGroups(t *testing.T) {
	m := makeGroupedBrowseModel() // TODO TODO BEGUN BEGUN DONE

	m = sendKey(m, tea.KeyTab)
	if m.cursor != 2 {
		t.Fatalf("expected tab to jump to first BEGUN task (2), got %d", m.cursor)
	}
	m = sendKey(m, tea.KeyTab)
	if m.cursor != 4 {
		t.Fatalf("expected tab to jump to first DONE task (4), got %d", m.cursor)
	}
	m = sendKey(m, tea.KeyTab)
	if m.cursor != 0 {
		t.Fatalf("expected tab to wrap to first task (0), got %d", m.cursor)
	}

	m.cursor = 3
	m = sendKey(m, tea.KeyShiftTab)
	if m.cursor != 2 {
		t.Fatalf("expected shift+tab to jump to head of current group (2), got %d", m.cursor)
	}
	m = sendKey(m, tea.KeyShiftTab)
	if m.cursor != 0 {
		t.Fatalf("expected shift+tab at group head to jump to previous group head (0), got %d", m.cursor)
	}
	m = sendKey(m, tea.KeyShiftTab)
	if m.cursor != 4 {
		t.Fatalf("expected shift+tab at first task to wrap to last group head (4), got %d", m.cursor)
	}
}

func TestBrowseFilterAppliesAndClears(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	tasks := []*task.Task{
		{ID: 1, Title: "alpha one", State: task.StateTodo, CreatedAt: now, UpdatedAt: now},
		{ID: 2, Title: "beta two", State: task.StateTodo, CreatedAt: now, UpdatedAt: now},
	}
	m := initialModel(tasks, nil, "")
	m.width = 100
	m.height = 24
	m.margin = 0

	m = sendRune(m, '/')
	if m.mode != modeFilter {
		t.Fatalf("expected / to enter filter mode, got mode %d", m.mode)
	}
	m.textinput.SetValue("beta")
	m = sendKey(m, tea.KeyEnter)
	if m.mode != modeBrowse {
		t.Fatalf("expected enter to return to browse mode, got %d", m.mode)
	}
	if len(m.tasks) != 1 || m.tasks[0].ID != 2 {
		t.Fatalf("expected filter to show only task 2, got %d tasks", len(m.tasks))
	}
	if !strings.Contains(m.View(), `filter:"beta"`) {
		t.Fatal("expected footer to show the active filter")
	}

	m = sendKey(m, tea.KeyEscape)
	if m.filter != "" || len(m.tasks) != 2 {
		t.Fatalf("expected esc to clear filter and restore 2 tasks, got filter=%q tasks=%d", m.filter, len(m.tasks))
	}
}

func TestBrowseFilterNoMatchesShowsHint(t *testing.T) {
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	tasks := []*task.Task{
		{ID: 1, Title: "alpha", State: task.StateTodo, CreatedAt: now, UpdatedAt: now},
	}
	m := initialModel(tasks, nil, "")
	m.width = 100
	m.height = 24
	m.margin = 0
	m = sendRune(m, '/')
	m.textinput.SetValue("zzz-nothing")
	m = sendKey(m, tea.KeyEnter)
	if len(m.tasks) != 0 {
		t.Fatalf("expected zero matches, got %d", len(m.tasks))
	}
	if !strings.Contains(m.View(), "esc to clear") {
		t.Fatalf("expected no-match view to hint at esc, got:\n%s", m.View())
	}
	m = sendKey(m, tea.KeyEscape)
	if len(m.tasks) != 1 {
		t.Fatalf("expected esc to restore tasks, got %d", len(m.tasks))
	}
}

func writeBrowseTask(t *testing.T, dir string, id int, title string) *task.Task {
	t.Helper()
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	tsk := &task.Task{
		ID:        id,
		Title:     title,
		State:     task.StateTodo,
		CreatedAt: now,
		UpdatedAt: now,
		Path:      filepath.Join(dir, "tasks", "001-task.md"),
	}
	if err := os.MkdirAll(filepath.Dir(tsk.Path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := tsk.Write(tsk.Path); err != nil {
		t.Fatal(err)
	}
	return tsk
}

func TestBrowseStateHotkeyDefaultsAndCaseFold(t *testing.T) {
	dir := t.TempDir()
	tsk := writeBrowseTask(t, dir, 1, "hotkey target")
	catalog := config.DefaultStateCatalog()
	m := initialModel([]*task.Task{tsk}, catalog, "")

	// b -> BLOCKED under the new defaults
	updated := sendRune(m, 'b')
	if updated.tasks[0].State != task.State("BLOCKED") {
		t.Fatalf("expected b to set BLOCKED, got %s", updated.tasks[0].State)
	}

	// F case-folds to f (FOLLOWUP)
	updated = sendRune(updated, 'F')
	if updated.tasks[0].State != task.State("FOLLOWUP") {
		t.Fatalf("expected F to case-fold to followup, got %s", updated.tasks[0].State)
	}

	// s -> BEGUN (moved off b)
	updated = sendRune(updated, 's')
	if updated.tasks[0].State != task.StateBegun {
		t.Fatalf("expected s to set BEGUN, got %s", updated.tasks[0].State)
	}
}

func TestBrowseLegacyConfigHotkeyShadowedByNav(t *testing.T) {
	// legacy configs carry l:defer — nav must win, and the dead hotkey
	// must vanish from the map and help line rather than lying
	legacy := &config.Config{States: []config.StateConfig{
		{Name: "TODO", Aliases: []string{"todo"}, TuiHotkey: "t"},
		{Name: "DEFER", Aliases: []string{"defer"}, TuiHotkey: "l"},
	}}
	catalog, err := config.BuildStateCatalog(legacy)
	if err != nil {
		t.Fatalf("legacy config must keep loading: %v", err)
	}

	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	tasks := []*task.Task{
		{ID: 1, Title: "one", State: task.StateTodo, CreatedAt: now, UpdatedAt: now},
		{ID: 2, Title: "two", State: task.StateTodo, CreatedAt: now, UpdatedAt: now},
	}
	m := initialModel(tasks, catalog, "")
	m.width = 100
	m.height = 24

	if _, ok := m.stateHotkeys["l"]; ok {
		t.Fatal("expected nav-colliding hotkey l to be dropped from the state hotkey map")
	}
	if strings.Contains(m.stateHelpLine, "l:defer") {
		t.Fatalf("expected shadowed hotkey to be omitted from help, got %q", m.stateHelpLine)
	}
	updated := sendRune(m, 'l')
	if updated.cursor != 1 {
		t.Fatalf("expected l to navigate to next task, got cursor %d", updated.cursor)
	}
	if updated.tasks[0].State != task.StateTodo {
		t.Fatalf("expected state untouched by shadowed hotkey, got %s", updated.tasks[0].State)
	}
}

func TestBrowseNewTaskCreation(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, err := executeCommand("todo", "seed task"); err != nil {
		t.Fatalf("seed create failed: %v", err)
	}
	cwd, _ := os.Getwd()

	seedPath := filepath.Join(cwd, "tasks")
	seed, err := task.Parse(filepath.Join(seedPath, firstMarkdown(t, seedPath)))
	if err != nil {
		t.Fatalf("parse seed: %v", err)
	}

	catalog := config.DefaultStateCatalog()
	m := initialModel([]*task.Task{seed}, catalog, cwd)
	m.width = 100
	m.height = 24

	m = sendRune(m, 'n')
	if m.mode != modeNew {
		t.Fatalf("expected n to enter new-task mode, got %d", m.mode)
	}
	m.textinput.SetValue("browse created task pri:2")
	m = sendKey(m, tea.KeyEnter)
	if m.err != nil {
		t.Fatalf("browse creation failed: %v", m.err)
	}
	if len(m.tasks) != 2 {
		t.Fatalf("expected 2 tasks after creation, got %d", len(m.tasks))
	}
	created := m.tasks[m.cursor]
	if created.Title != "browse created task" || created.Priority != 2 {
		t.Fatalf("expected cursor on created task with pri 2, got %q pri %d", created.Title, created.Priority)
	}
	if _, err := os.Stat(created.Path); err != nil {
		t.Fatalf("expected created task file on disk: %v", err)
	}
}

func firstMarkdown(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			return e.Name()
		}
	}
	t.Fatal("no markdown file found")
	return ""
}
