package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"punchlist/config"
	"punchlist/task"
)

// The failure this guards against was observed in the wild: a .punchlist shared
// between two machines conflicts on config.yaml, both hosts allocate from their
// own stale view of next_id, and two tasks end up claiming one id.
func TestNextIDTakesFloorFromDisk(t *testing.T) {
	t.Run("stale next_id does not reissue an id already on disk", func(t *testing.T) {
		teardown := setupTest(t)
		defer teardown()

		executeCommand("init")
		tasksPath, err := tasksDir()
		if err != nil {
			t.Fatalf("Failed to resolve tasks dir: %v", err)
		}

		// tasks 1..12 arrive from the other host
		now := time.Now()
		for _, id := range []int{1, 2, 12} {
			tsk := &task.Task{
				ID: id, Title: "Synced task", State: task.StateTodo,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tsk.Write(filepath.Join(tasksPath, paddedFilename(id, "Synced task"))); err != nil {
				t.Fatalf("Failed to write task %d: %v", id, err)
			}
		}

		// ...but our config.yaml is the losing side of the conflict
		cfg, _ := config.LoadConfig()
		cfg.NextID = 3
		config.SaveConfig(cfg)

		if _, err := executeCommand("todo", "Task landed locally"); err != nil {
			t.Fatalf("todo failed: %v", err)
		}

		// must be 13, not 3 — 3 would collide with nothing yet, but the store
		// would then hand out 4..12 straight into the synced ids
		if _, err := os.Stat(filepath.Join(tasksPath, paddedFilename(13, "Task landed locally"))); err != nil {
			names := []string{}
			entries, _ := os.ReadDir(tasksPath)
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Errorf("expected id 13 from the on-disk floor; tasks dir holds: %v", names)
		}

		// and the counter must advance past the id actually used
		cfg, _ = config.LoadConfig()
		if cfg.NextID != 14 {
			t.Errorf("expected next_id 14 after healing forward, got %d", cfg.NextID)
		}
	})

	t.Run("trashed ids are not reissued", func(t *testing.T) {
		teardown := setupTest(t)
		defer teardown()

		executeCommand("init")
		executeCommand("todo", "Doomed task")
		executeCommand("del", "1")

		// rewind the counter as a conflicted config would
		cfg, _ := config.LoadConfig()
		cfg.NextID = 1
		config.SaveConfig(cfg)

		if _, err := executeCommand("todo", "Replacement task"); err != nil {
			t.Fatalf("todo failed: %v", err)
		}

		tasksPath, _ := tasksDir()
		if _, err := os.Stat(filepath.Join(tasksPath, paddedFilename(1, "Replacement task"))); err == nil {
			t.Error("reissued id 1 while a trashed task still holds it")
		}
	})

	t.Run("uncontended store keeps allocating sequentially", func(t *testing.T) {
		teardown := setupTest(t)
		defer teardown()

		executeCommand("init")
		for i := 1; i <= 3; i++ {
			if _, err := executeCommand("todo", "Ordinary task"); err != nil {
				t.Fatalf("todo %d failed: %v", i, err)
			}
		}

		cfg, _ := config.LoadConfig()
		if cfg.NextID != 4 {
			t.Errorf("expected next_id 4 after 3 creates, got %d", cfg.NextID)
		}
	})
}

func TestLeadingTaskID(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"001-first-task.md", 1},
		{"042-answer.md", 42},
		{"1234-wide-id.md", 1234},
		// the mangled shape the collision produced: id 11, slug starting "010-"
		{"011-010-keep-media-title-payloads.md", 11},
		{"no-leading-id.md", 0},
		{"-leading-dash.md", 0},
		{"", 0},
		{"9999999999-absurd.md", 0},
	}
	for _, c := range cases {
		if got := leadingTaskID(c.name); got != c.want {
			t.Errorf("leadingTaskID(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestMaxOnDiskIDIgnoresNonTasks(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	executeCommand("init")
	tasksPath, _ := tasksDir()

	for _, name := range []string{"007-real-task.md", "README.txt", "099-notes.txt", "subdir"} {
		p := filepath.Join(tasksPath, name)
		if name == "subdir" {
			os.MkdirAll(p, 0755)
			continue
		}
		os.WriteFile(p, []byte("x"), 0644)
	}

	if got := maxOnDiskID(tasksPath); got != 7 {
		t.Errorf("maxOnDiskID = %d, want 7 (non-.md and dirs must not count)", got)
	}
}
