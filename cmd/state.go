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

// locate a task file by id prefix
func findTaskFile(id int) (string, error) {
	tasksPath, err := tasksDir()
	if err != nil {
		return "", err
	}
	files, err := os.ReadDir(tasksPath)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		name := file.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		prefix := strings.SplitN(name, "-", 2)[0]
		if prefix == "" {
			continue
		}
		parsedID, err := strconv.Atoi(prefix)
		if err != nil {
			continue
		}
		if parsedID == id {
			return filepath.Join(tasksPath, name), nil
		}
	}

	return "", fmt.Errorf("task with ID %d not found", id)
}

// create the start command
func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start [id]",
		Aliases: []string{"begun", "BEGUN", "START"},
		Short:   "Start a task",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			updateTaskState(ids, task.StateBegun)
		},
	}
}

// create the done command
func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "done [id]",
		Aliases: []string{"DONE"},
		Short:   "Complete a task",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			updateTaskState(ids, task.StateDone)
		},
	}
}

// create the notdo/defer command
func newDeferCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "notdo [id]",
		Aliases: []string{"defer", "NOTDO"},
		Short:   "Mark a task as not to do",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			updateTaskState(ids, task.StateNotDo)
		},
	}
}

// create the block command
func newBlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "block [id]",
		Aliases: []string{"BLOCK"},
		Short:   "Change a task's status to BLOCKED",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			updateTaskState(ids, task.StateBlock)
		},
	}
}

// create the confirm command
func newConfirmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "confirm [id]",
		Aliases: []string{"CONFIRM"},
		Short:   "Mark a task as needing confirmation",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			updateTaskState(ids, task.StateConfirm)
		},
	}
}

// update multiple tasks to a new state
func updateTaskState(ids []int, newState task.State) {
	for _, id := range ids {
		if err := updateTaskStateSingle(id, newState); err != nil {
			if printNotPunchlistError(err) {
				return
			}
			fmt.Printf("Error updating task %d: %v\n", id, err)
		}
	}
}

// update a single task's state and timestamps
func updateTaskStateSingle(id int, newState task.State) error {
	taskPath, err := findTaskFile(id)
	if err != nil {
		return err
	}

	t, err := task.Parse(taskPath)
	if err != nil {
		return err
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
		return err
	}

	fmt.Printf("Task %d moved to %s\n", id, newState)
	return nil
}
