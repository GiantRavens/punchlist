package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type compactEntry struct {
	task     *task.Task
	oldID    int
	newID    int
	oldPath  string
	tempPath string
	suffix   string
}

// create the compact command
func newCompactCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compact",
		Short: "Compact task IDs into a contiguous sequence",
		Run: func(cmd *cobra.Command, args []string) {
			if err := compactTasks(); err != nil {
				if printNotPunchlistError(err) {
					return
				}
				fmt.Printf("Error compacting tasks: %v\n", err)
			}
		},
	}
}

// compact tasks to contiguous ids, updating filenames and frontmatter
func compactTasks() error {
	tasksPath, err := tasksDir()
	if err != nil {
		return err
	}
	entries, err := loadCompactEntries(tasksPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	idWidth := idWidthFromConfig(cfg)

	changed := false
	for i := range entries {
		entries[i].newID = i + 1
		if entries[i].oldID != entries[i].newID {
			changed = true
		}
	}
	if !changed {
		fmt.Println("Nothing to compact.")
		return nil
	}

	// rename to temp files to avoid collisions
	for i := range entries {
		tempPath := compactTempPath(entries[i].oldPath)
		if err := os.Rename(entries[i].oldPath, tempPath); err != nil {
			return fmt.Errorf("failed to stage %s: %w", entries[i].oldPath, err)
		}
		entries[i].tempPath = tempPath
	}

	// write updated tasks to final paths
	now := time.Now()
	for i := range entries {
		entry := &entries[i]
		if entry.oldID == entry.newID {
			// keep file as-is but move back to original path
			if err := os.Rename(entry.tempPath, entry.oldPath); err != nil {
				return fmt.Errorf("failed to restore %s: %w", entry.oldPath, err)
			}
			continue
		}

		entry.task.ID = entry.newID
		entry.task.UpdatedAt = now
		entry.task.Body = appendCompactLog(entry.task.Body, entry.oldID, entry.newID, now)

		newPath := compactTargetPath(tasksPath, entry.task, entry.suffix, entry.newID, idWidth)
		if err := entry.task.Write(newPath); err != nil {
			return fmt.Errorf("failed to write %s: %w", newPath, err)
		}
		if err := os.Remove(entry.tempPath); err != nil {
			return fmt.Errorf("failed to remove temp file %s: %w", entry.tempPath, err)
		}
	}

	// update next id in config
	cfg.NextID = len(entries) + 1
	cfg.IDWidth = idWidth
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Compacted %d tasks.\n", len(entries))
	return nil
}

// load all tasks and prep compact entries
func loadCompactEntries(tasksPath string) ([]compactEntry, error) {
	info, err := os.Stat(tasksPath)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	entries := []compactEntry{}
	err = filepath.WalkDir(tasksPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		t, err := task.Parse(path)
		if err != nil {
			return nil
		}

		suffix := compactSuffix(d.Name(), t.Title)
		entries = append(entries, compactEntry{
			task:    t,
			oldID:   t.ID,
			oldPath: path,
			suffix:  suffix,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].oldID == entries[j].oldID {
			return entries[i].oldPath < entries[j].oldPath
		}
		return entries[i].oldID < entries[j].oldID
	})

	return entries, nil
}

// build a new path using configured width and existing suffix
func compactTargetPath(tasksPath string, t *task.Task, suffix string, newID int, idWidth int) string {
	if suffix == "" {
		suffix = slugify(t.Title)
	}
	filename := fmt.Sprintf("%0*d-%s.md", idWidth, newID, suffix)
	return filepath.Join(tasksPath, filename)
}

// derive a filename suffix from the existing filename or title
func compactSuffix(filename string, title string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.SplitN(base, "-", 2)
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return slugify(title)
}

// build a temp path for staging renames
func compactTempPath(oldPath string) string {
	dir := filepath.Dir(oldPath)
	base := filepath.Base(oldPath)
	stamp := time.Now().UnixNano()
	return filepath.Join(dir, fmt.Sprintf(".compact-%d-%s", stamp, base))
}

// append a log entry describing the id change
func appendCompactLog(body string, oldID int, newID int, now time.Time) string {
	entry := fmt.Sprintf("- %s: compacted id from %d to %d", now.Format(time.RFC3339), oldID, newID)

	pre, logSection, afterLog, found := splitSection(body, "## Log")
	if found {
		pre += afterLog
	} else {
		logSection = "## Log"
	}

	logSection = appendEntry(logSection, entry)
	return joinBlocks(pre, logSection)
}
