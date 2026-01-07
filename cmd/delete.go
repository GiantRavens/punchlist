package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "del [id]",
		Aliases: []string{"rm", "delete"},
		Short:   "Move a task to the trash",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := deleteTask(args[0]); err != nil {
				fmt.Printf("Error deleting task: %v\n", err)
				return
			}
		},
	}
}

func deleteTask(idStr string) error {
	id, err := parseTaskID(idStr)
	if err != nil {
		return err
	}

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

func parseTaskID(idStr string) (int, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("invalid task ID: %v", err)
	}
	return id, nil
}

func uniqueTrashPath(destPath string) string {
	ext := filepath.Ext(destPath)
	base := strings.TrimSuffix(filepath.Base(destPath), ext)
	dir := filepath.Dir(destPath)
	stamp := time.Now().Unix()
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, stamp, ext))
}
