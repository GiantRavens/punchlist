package cmd

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"punchlist/task"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// handleHelpFlag short-circuits state commands that have DisableFlagParsing set,
// so users can still run `pin todo --help` etc. Returns true if help was shown
// and the caller should return.
func handleHelpFlag(cmd *cobra.Command, args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			_ = cmd.Help()
			return true
		}
	}
	return false
}

// locate a task file by id prefix. Walks tasks/ recursively so by-id
// resolution agrees with pin ls, which lists tasks in subfolders too —
// a top-level-only ReadDir here made every by-id command fail on tasks
// that ls had just shown (futhark bridge punchlist deck, 2026-07-15).
func findTaskFile(id int) (string, error) {
	tasksPath, err := tasksDir()
	if err != nil {
		return "", err
	}

	var matches []string
	err = filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		prefix := strings.SplitN(d.Name(), "-", 2)[0]
		parsedID, convErr := strconv.Atoi(prefix)
		if convErr != nil || parsedID != id {
			return nil
		}
		matches = append(matches, path)
		return nil
	})
	if err != nil {
		return "", err
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("task with ID %d not found", id)
	case 1:
		return matches[0], nil
	default:
		// nothing enforces id uniqueness across subfolders; refuse to guess
		return "", fmt.Errorf("task ID %d is ambiguous (%d files): %s", id, len(matches), strings.Join(matches, ", "))
	}
}

// create the todo command
func newTodoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "todo [id]",
		Aliases: []string{"TODO"},
		Short:   "Move a task back to TODO, or create one (use --ticket for a Problem/Approach/Acceptance scaffold)",
		Args: cobra.MinimumNArgs(1),
		// Allow titles that begin with `--` (e.g. `pin todo "--rerun the orca cycle"`)
		// to pass through cobra's flag parser as plain args. State-creation commands
		// take only IDs and titles; no command-specific flags. `pin <verb> --help`
		// still works because cobra's help command runs at the parent level.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if handleHelpFlag(cmd, args) {
				return
			}
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateTodo)}, args...)); err != nil {
						failf("Failed to create todo task: %v\n", err)
					}
					return
				}
				failf("Invalid task IDs: %v\n", err)
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
		Args: cobra.MinimumNArgs(1),
		// Allow titles that begin with `--` (e.g. `pin todo "--rerun the orca cycle"`)
		// to pass through cobra's flag parser as plain args. State-creation commands
		// take only IDs and titles; no command-specific flags. `pin <verb> --help`
		// still works because cobra's help command runs at the parent level.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			if handleHelpFlag(cmd, args) {
				return
			}
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateBegun)}, args...)); err != nil {
						failf("Failed to create begun task: %v\n", err)
					}
					return
				}
				failf("Invalid task IDs: %v\n", err)
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
		Args: cobra.MinimumNArgs(1),
		// Allow titles that begin with `--` (e.g. `pin todo "--rerun the orca cycle"`)
		// to pass through cobra's flag parser as plain args. State-creation commands
		// take only IDs and titles; no command-specific flags. `pin <verb> --help`
		// still works because cobra's help command runs at the parent level.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			if handleHelpFlag(cmd, args) {
				return
			}
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateDone)}, args...)); err != nil {
						failf("Failed to create done task: %v\n", err)
					}
					return
				}
				failf("Invalid task IDs: %v\n", err)
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
		Args: cobra.MinimumNArgs(1),
		// Allow titles that begin with `--` (e.g. `pin todo "--rerun the orca cycle"`)
		// to pass through cobra's flag parser as plain args. State-creation commands
		// take only IDs and titles; no command-specific flags. `pin <verb> --help`
		// still works because cobra's help command runs at the parent level.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			if handleHelpFlag(cmd, args) {
				return
			}
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateNotDo)}, args...)); err != nil {
						failf("Failed to create notdo task: %v\n", err)
					}
					return
				}
				failf("Invalid task IDs: %v\n", err)
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
		Args: cobra.MinimumNArgs(1),
		// Allow titles that begin with `--` (e.g. `pin todo "--rerun the orca cycle"`)
		// to pass through cobra's flag parser as plain args. State-creation commands
		// take only IDs and titles; no command-specific flags. `pin <verb> --help`
		// still works because cobra's help command runs at the parent level.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			if handleHelpFlag(cmd, args) {
				return
			}
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateBlock)}, args...)); err != nil {
						failf("Failed to create block task: %v\n", err)
					}
					return
				}
				failf("Invalid task IDs: %v\n", err)
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
		Args: cobra.MinimumNArgs(1),
		// Allow titles that begin with `--` (e.g. `pin todo "--rerun the orca cycle"`)
		// to pass through cobra's flag parser as plain args. State-creation commands
		// take only IDs and titles; no command-specific flags. `pin <verb> --help`
		// still works because cobra's help command runs at the parent level.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			// parse one or many ids
			if handleHelpFlag(cmd, args) {
				return
			}
			ids, err := parseTaskIDs(args)
			if err != nil {
				if shouldCreateStateFromArgs(args) {
					if err := createTaskFromArgs(append([]string{string(task.StateConfirm)}, args...)); err != nil {
						failf("Failed to create confirm task: %v\n", err)
					}
					return
				}
				failf("Invalid task IDs: %v\n", err)
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
			failf("Error updating task %d: %v\n", id, err)
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

	oldState := t.State
	if oldState != newState {
		changeState(t, newState)
		addLog(t, fmt.Sprintf("State changed from %s to %s", oldState, newState))
	}

	if err := t.Write(taskPath); err != nil {
		return err
	}

	fmt.Printf("Task %d moved to %s\n", id, newState)
	return nil
}
