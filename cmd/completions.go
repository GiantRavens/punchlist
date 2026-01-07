package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"punchlist/task"
)

// canonical state tokens for shell completion
var stateCompletionCandidates = []string{
	"TODO",
	"BEGUN",
	"BLOCK",
	"CONFIRM",
	"DONE",
	"NOTDO",
}

// complete a single state argument for commands like ls
func stateArgCompletion(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return stateCompletions(toComplete), cobra.ShellCompDirectiveNoFileComp
}

// complete root args, with dynamic ids after certain states
func rootArgCompletion(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return stateCompletions(toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	state, ok := task.ParseState(args[0])
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	switch state {
	case task.StateTodo:
		return nil, cobra.ShellCompDirectiveNoFileComp
	case task.StateBegun:
		return taskIDCompletions([]task.State{task.StateTodo}, toComplete), cobra.ShellCompDirectiveNoFileComp
	case task.StateDone:
		return taskIDCompletions(
			[]task.State{task.StateBegun, task.StateConfirm, task.StateTodo},
			toComplete,
		), cobra.ShellCompDirectiveNoFileComp
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// filter state completions by prefix
func stateCompletions(toComplete string) []cobra.Completion {
	if toComplete == "" {
		return stringsToCompletions(stateCompletionCandidates)
	}

	upper := strings.ToUpper(toComplete)
	filtered := make([]string, 0, len(stateCompletionCandidates))
	for _, candidate := range stateCompletionCandidates {
		if strings.HasPrefix(candidate, upper) {
			filtered = append(filtered, candidate)
		}
	}
	return stringsToCompletions(filtered)
}

// convert string slices to cobra completions
func stringsToCompletions(values []string) []cobra.Completion {
	completions := make([]cobra.Completion, 0, len(values))
	for _, value := range values {
		completions = append(completions, cobra.Completion(value))
	}
	return completions
}

// return id completions for tasks in the given state order
func taskIDCompletions(stateOrder []task.State, toComplete string) []cobra.Completion {
	tasksByState := loadTasksByState()
	if len(tasksByState) == 0 {
		return nil
	}

	completions := []cobra.Completion{}
	for _, state := range stateOrder {
		tasks := tasksByState[state]
		if len(tasks) == 0 {
			continue
		}

		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].ID < tasks[j].ID
		})

		for _, t := range tasks {
			idStr := strconv.Itoa(t.ID)
			if toComplete != "" && !strings.HasPrefix(idStr, toComplete) {
				continue
			}
			completions = append(completions, cobra.CompletionWithDesc(idStr, t.Title))
		}
	}

	return completions
}

// load tasks grouped by state from the tasks directory
func loadTasksByState() map[task.State][]*task.Task {
	tasksDir := "tasks"
	info, err := os.Stat(tasksDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	tasksByState := make(map[task.State][]*task.Task)
	_ = filepath.WalkDir(tasksDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		t, err := task.Parse(path)
		if err != nil {
			return nil
		}

		tasksByState[t.State] = append(tasksByState[t.State], t)
		return nil
	})

	return tasksByState
}
