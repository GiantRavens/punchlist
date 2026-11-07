package cmd

import (
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"strings"
	"testing"
	"time"
)

// seedCollision writes the pin #49 shape: two files claiming id 7, the open ticket created first.
func seedCollision(t *testing.T) (tasksPath, older, newer string) {
	t.Helper()
	executeCommand("init")
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("tasks dir: %v", err)
	}
	base := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	write := func(id int, name, title string, state task.State, created time.Time) string {
		p := filepath.Join(tasksPath, name)
		tk := &task.Task{ID: id, Title: title, State: state, CreatedAt: created, UpdatedAt: created,
			Body: "# " + title + "\n"}
		if err := tk.Write(p); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		return p
	}
	older = write(7, "007-gate-ollama.md", "Gate ollama", task.StateTodo, base)
	newer = write(7, "007-restored-syncthing.md", "Restored syncthing", task.StateDone, base.Add(48*time.Hour))
	write(9, "009-other.md", "Other", task.StateTodo, base) // the floor: fresh ids start at 10
	return tasksPath, older, newer
}

func TestRenumberCollisionsKeepsOldest(t *testing.T) {
	defer setupTest(t)()
	tasksPath, older, newer := seedCollision(t)

	if _, err := findTaskFile(7); err == nil || !strings.Contains(err.Error(), "pin renumber --collisions") {
		t.Fatalf("ambiguous lookup should point at the repair, got %v", err)
	}

	out, err := executeCommand("renumber", "--collisions")
	if err != nil {
		t.Fatalf("renumber: %v (%s)", err, out)
	}
	if !strings.Contains(out, "Renumbered 7 -> 10") {
		t.Fatalf("unexpected output: %q", out)
	}
	if _, err := os.Stat(older); err != nil {
		t.Fatalf("the earliest-created task must keep its file and id: %v", err)
	}
	if _, err := os.Stat(newer); !os.IsNotExist(err) {
		t.Fatalf("the newer file should have moved")
	}
	moved, err := task.Parse(filepath.Join(tasksPath, "010-restored-syncthing.md"))
	if err != nil {
		t.Fatalf("moved file: %v", err)
	}
	if moved.ID != 10 || moved.State != task.StateDone || moved.Title != "Restored syncthing" {
		t.Fatalf("moved task wrong: id=%d state=%s title=%q", moved.ID, moved.State, moved.Title)
	}
	if !strings.Contains(moved.Body, "renumbered id from 7 to 10 (id collision; 007-gate-ollama.md keeps it)") {
		t.Fatalf("log entry missing: %q", moved.Body)
	}
	if p, err := findTaskFile(7); err != nil || p != older {
		t.Fatalf("id 7 should now resolve to the open ticket, got %q %v", p, err)
	}
	cfg, _ := config.LoadConfig()
	if cfg.NextID != 11 {
		t.Fatalf("next_id should advance past the used id, got %d", cfg.NextID)
	}
	if report, err := runDoctor(false); err != nil {
		t.Fatalf("doctor: %v", err)
	} else {
		for _, f := range report.Findings {
			if f.Check == "duplicate_id" || f.Check == "id_mismatch" {
				t.Fatalf("doctor still reports %s: %s", f.Check, f.Detail)
			}
		}
	}

	out, err = executeCommand("renumber", "--collisions")
	if err != nil || !strings.Contains(out, "No duplicate ids.") {
		t.Fatalf("second run should be a no-op, got %q %v", out, err)
	}
}

func TestRenumberDryRunChangesNothing(t *testing.T) {
	defer setupTest(t)()
	_, older, newer := seedCollision(t)
	cfgBefore, _ := config.LoadConfig()

	out, err := executeCommand("renumber", "--collisions", "--dry-run")
	if err != nil || !strings.Contains(out, "Would renumber 7 -> 10") {
		t.Fatalf("dry run: %q %v", out, err)
	}
	for _, p := range []string{older, newer} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("dry run moved %s", p)
		}
	}
	if cfgAfter, _ := config.LoadConfig(); cfgAfter.NextID != cfgBefore.NextID {
		t.Fatalf("dry run changed next_id")
	}
}

func TestRenumberNamedFile(t *testing.T) {
	defer setupTest(t)()
	tasksPath, older, _ := seedCollision(t)

	// a bare filename resolves inside tasks/; renumbering the OLDER one is allowed when asked
	out, err := executeCommand("renumber", filepath.Base(older))
	if err != nil || !strings.Contains(out, "Renumbered 7 -> 10") {
		t.Fatalf("named renumber: %q %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(tasksPath, "010-gate-ollama.md")); err != nil {
		t.Fatalf("expected 010-gate-ollama.md: %v", err)
	}
}

func TestRenumberRefusesBadArgs(t *testing.T) {
	defer setupTest(t)()
	seedCollision(t)
	outside := filepath.Join(t.TempDir(), "007-x.md")
	os.WriteFile(outside, []byte("---\nid: 7\n---\n"), 0644)

	for _, args := range [][]string{
		{"renumber"}, // neither file nor --collisions
		{"renumber", "--collisions", "007-gate-ollama.md"}, // both
		{"renumber", outside},                              // outside this scope
		{"renumber", "no-such-file.md"},
	} {
		if _, err := executeCommand(args...); err == nil {
			t.Errorf("%v should be refused", args)
		}
	}
}
