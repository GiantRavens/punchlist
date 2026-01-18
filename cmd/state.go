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

// create the todo command
func newTodoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "todo [id]",
		Aliases: []string{"TODO"},
		Short:   "Move a task back to TODO",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateTodo)}, args...)); err != nil {
						fmt.Printf("Failed to create todo task: %v\n", err)
					}
					return
				}
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			updateTaskState(ids, task.StateTodo)
		},
	}
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
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateBegun)}, args...)); err != nil {
						fmt.Printf("Failed to create begun task: %v\n", err)
					}
					return
				}
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
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateDone)}, args...)); err != nil {
						fmt.Printf("Failed to create done task: %v\n", err)
					}
					return
				}
				fmt.Printf("Invalid task IDs: %v\n", err)
				return
			}
			updateTaskState(ids, task.StateDone)
		},
	}
}

func shouldCreateStateFromArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}

	first := strings.TrimSpace(args[0])
	if strings.HasPrefix(first, "[") {
		return false
	}

	hasValidID := false
	hasNonID := false
	for _, arg := range args {
		token := strings.TrimSpace(arg)
		if token == "" {
			continue
		}
		if isValidIDToken(token) {
			hasValidID = true
			continue
		}
		if isPotentialIDToken(token) {
			return false
		}
		hasNonID = true
	}

	return hasNonID && !hasValidID
}

func isValidIDToken(token string) bool {
	if token == "" {
		return false
	}
	if strings.Count(token, "-") == 1 {
		parts := strings.SplitN(token, "-", 2)
		if parts[0] == "" || parts[1] == "" {
			return false
		}
		return isAllDigits(parts[0]) && isAllDigits(parts[1])
	}
	if strings.Contains(token, "-") {
		return false
	}
	return isAllDigits(token)
}

func isPotentialIDToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r == '-' {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isAllDigits(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// create the notdo/defer command
func newDeferCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "notdo [id]",
		Aliases: []string{"defer", "DEFER", "NOTDO"},
		Short:   "Mark a task as not to do",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateNotDo)}, args...)); err != nil {
						fmt.Printf("Failed to create notdo task: %v\n", err)
					}
					return
				}
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
		Aliases: []string{"BLOCK", "waiting", "WAITING"},
		Short:   "Change a task's status to BLOCKED",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateBlock)}, args...)); err != nil {
						fmt.Printf("Failed to create block task: %v\n", err)
					}
					return
				}
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
		Aliases: []string{"CONFIRM", "followup", "FOLLOWUP", "review", "REVIEW"},
		Short:   "Mark a task as needing confirmation",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateConfirm)}, args...)); err != nil {
						fmt.Printf("Failed to create confirm task: %v\n", err)
					}
					return
				}
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
