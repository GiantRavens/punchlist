package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"punchlist/task"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func findTaskFile(id int) (string, error) {
	tasksDir := "tasks"
	files, err := os.ReadDir(tasksDir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), fmt.Sprintf("%06d", id)) {
			return filepath.Join(tasksDir, file.Name()), nil
		}
	}

	return "", fmt.Errorf("task with ID %d not found", id)
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start [id]",
		Aliases: []string{"begun", "BEGUN", "START"},
		Short:   "Start a task",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			updateTaskState(args[0], task.StateBegun)
		},
	}
}

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "done [id]",
		Aliases: []string{"DONE"},
		Short:   "Complete a task",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			updateTaskState(args[0], task.StateDone)
		},
	}
}

func newDeferCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "notdo [id]",
		Aliases: []string{"defer", "NOTDO"},
		Short:   "Mark a task as not to do",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			updateTaskState(args[0], task.StateNotDo)
		},
	}
}

func newBlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "block [id]",
		Aliases: []string{"BLOCK"},
		Short:   "Block a task",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			updateTaskState(args[0], task.StateBlock)
		},
	}
}

func newConfirmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "confirm [id]",
		Aliases: []string{"CONFIRM"},
		Short:   "Mark a task as needing confirmation",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			updateTaskState(args[0], task.StateConfirm)
		},
	}
}

func updateTaskState(idStr string, newState task.State) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Printf("Invalid task ID: %v\n", err)
		return
	}

	taskPath, err := findTaskFile(id)
	if err != nil {
		fmt.Printf("Error finding task: %v\n", err)
		return
	}

	t, err := task.Parse(taskPath)
	if err != nil {
		fmt.Printf("Error parsing task: %v\n", err)
		return
	}

	t.State = newState
	t.UpdatedAt = time.Now()
	if newState == task.StateBegun {
		now := time.Now()
		t.StartedAt = &now
	} else if newState == task.StateDone {
		now := time.Now()
		t.CompletedAt = &now
	}

	if err := t.Write(taskPath); err != nil {
		fmt.Printf("Error updating task: %v\n", err)
		return
	}

	fmt.Printf("Task %d moved to %s\n", id, newState)
}
