package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// create the delete command
func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "del [id]",
		Aliases: []string{"rm", "delete"},
		Short:   "Move a task to the trash",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			deleteTasks(ids)
		},
	}
}

// delete multiple tasks, reporting errors per id
func deleteTasks(ids []int) {
	for _, id := range ids {
		if err := deleteTaskSingle(id); err != nil {
			fmt.Printf("Error deleting task %d: %v\n", id, err)
		}
	}
}

// delete a single task by moving it to trash
func deleteTaskSingle(id int) error {
	taskPath, err := findTaskFile(id)
	if err != nil {
		return err
	}

	trashDir := ".trash"
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	destPath := filepath.Join(trashDir, filepath.Base(taskPath))
	if _, err := os.Stat(destPath); err == nil {
		destPath = uniqueTrashPath(destPath)
	}

	if err := os.Rename(taskPath, destPath); err != nil {
		return fmt.Errorf("failed to move task to trash: %w", err)
	}

	fmt.Printf("Moved task %d to %s\n", id, destPath)
	return nil
}

// avoid name collisions in the trash directory
func uniqueTrashPath(destPath string) string {
	ext := filepath.Ext(destPath)
	base := strings.TrimSuffix(filepath.Base(destPath), ext)
	dir := filepath.Dir(destPath)
	stamp := time.Now().Unix()
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, stamp, ext))
}
