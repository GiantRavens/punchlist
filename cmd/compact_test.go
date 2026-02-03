package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"strings"
	"testing"
	"time"
)

func TestCompactTasks(t *testing.T) {
	t.Run("renumbers tasks with gaps", func(t *testing.T) {
		teardown := setupTest(t)
		defer teardown()

		executeCommand("init")
		tasksPath, err := tasksDir()
		if err != nil {
			t.Fatalf("Failed to resolve tasks dir: %v", err)
		}

		// create tasks with gaps: 1, 3, 5
		now := time.Now()
		for _, entry := range []struct {
			id    int
			title string
		}{
			{1, "First task"},
			{3, "Third task"},
			{5, "Fifth task"},
		} {
			tsk := &task.Task{
				ID: entry.id, Title: entry.title, State: task.StateTodo,
				CreatedAt: now, UpdatedAt: now,
			}
			filename := filepath.Join(tasksPath, paddedFilename(entry.id, entry.title))
			tsk.Write(filename)
		}

		// set next_id to 6 so compact has something to update
		cfg, _ := config.LoadConfig()
		cfg.NextID = 6
		config.SaveConfig(cfg)

		output, err := executeCommand("compact")
		if err != nil {
			t.Fatalf("compact failed: %v", err)
		}
		if !strings.Contains(output, "Compacted") {
			t.Errorf("expected compacted message, got: %s", output)
		}

		// verify tasks are now 1, 2, 3
		files, _ := os.ReadDir(tasksPath)
		mdFiles := []string{}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".md") {
				mdFiles = append(mdFiles, f.Name())
			}
		}
		if len(mdFiles) != 3 {
			t.Fatalf("expected 3 task files, got %d", len(mdFiles))
		}

		// verify IDs in frontmatter
		for _, f := range mdFiles {
			tsk, err := task.Parse(filepath.Join(tasksPath, f))
			if err != nil {
				t.Fatalf("failed to parse %s: %v", f, err)
			}
			if tsk.ID < 1 || tsk.ID > 3 {
				t.Errorf("expected ID between 1-3, got %d in %s", tsk.ID, f)
			}
		}

		// verify config next_id updated
		cfg, _ = config.LoadConfig()
		if cfg.NextID != 4 {
			t.Errorf("expected next_id=4 after compact, got %d", cfg.NextID)
		}
	})

	t.Run("already contiguous is no-op", func(t *testing.T) {
		teardown := setupTest(t)
		defer teardown()

		executeCommand("init")
		executeCommand("todo", "Task one")
		executeCommand("todo", "Task two")
		executeCommand("todo", "Task three")

		output, err := executeCommand("compact")
		if err != nil {
			t.Fatalf("compact failed: %v", err)
		}
		if !strings.Contains(output, "Nothing to compact") {
			t.Errorf("expected 'Nothing to compact', got: %s", output)
		}
	})

	t.Run("compact adds log entries", func(t *testing.T) {
		teardown := setupTest(t)
		defer teardown()

		executeCommand("init")
		tasksPath, _ := tasksDir()

		now := time.Now()
		// create task with ID 3 (gap from 1)
		tsk := &task.Task{
			ID: 3, Title: "Gapped task", State: task.StateTodo,
			CreatedAt: now, UpdatedAt: now,
		}
		tsk.Write(filepath.Join(tasksPath, "003-gapped-task.md"))

		cfg, _ := config.LoadConfig()
		cfg.NextID = 4
		config.SaveConfig(cfg)

		executeCommand("compact")

		// find the compacted task and verify log entry
		files, _ := os.ReadDir(tasksPath)
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".md") {
				tsk, _ := task.Parse(filepath.Join(tasksPath, f.Name()))
				if strings.Contains(tsk.Body, "compacted id from 3 to 1") {
					return // found the log entry
				}
			}
		}
		t.Error("expected compact log entry in task body")
	})
}

func paddedFilename(id int, title string) string {
	return fmt.Sprintf("%03d-%s.md", id, slugify(title))
}
