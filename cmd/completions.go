package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"punchlist/config"
	"punchlist/task"
)

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

	stateCatalog, err := loadStateCatalog()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	stateName := strings.ToUpper(strings.TrimSpace(args[0]))
	if stateCatalog != nil {
		if canonical, ok := stateCatalog.Resolve(args[0]); ok {
			stateName = strings.ToUpper(canonical)
		}
	}

	switch stateName {
	case "TODO":
		return nil, cobra.ShellCompDirectiveNoFileComp
	case "BEGUN":
		return taskIDCompletions(stateCatalog, []string{"TODO"}, toComplete), cobra.ShellCompDirectiveNoFileComp
	case "DONE":
		return taskIDCompletions(
			stateCatalog,
			[]string{"BEGUN", "CONFIRM", "TODO"},
			toComplete,
		), cobra.ShellCompDirectiveNoFileComp
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// filter state completions by prefix
func stateCompletions(toComplete string) []cobra.Completion {
	candidates := stateCompletionCandidates()
	if len(candidates) == 0 {
		return nil
	}
	if toComplete == "" {
		return stringsToCompletions(candidates)
	}

	upper := strings.ToUpper(toComplete)
	filtered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
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
func taskIDCompletions(catalog *config.StateCatalog, stateOrder []string, toComplete string) []cobra.Completion {
	tasksByState := loadTasksByState(catalog)
	if len(tasksByState) == 0 {
		return nil
	}

	completions := []cobra.Completion{}
	for _, state := range stateOrder {
		stateKey := strings.ToUpper(strings.TrimSpace(state))
		tasks := tasksByState[stateKey]
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
func loadTasksByState(catalog *config.StateCatalog) map[string][]*task.Task {
	tasksPath, err := tasksDir()
	if err != nil {
		return nil
	}
	info, err := os.Stat(tasksPath)
	if err != nil || !info.IsDir() {
		return nil
	}

	tasksByState := make(map[string][]*task.Task)
	_ = filepath.WalkDir(tasksPath, func(path string, d os.DirEntry, err error) error {
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

		canonical := canonicalizeState(catalog, t.State)
		key := strings.ToUpper(strings.TrimSpace(canonical))
		if key != "" {
			tasksByState[key] = append(tasksByState[key], t)
		}
		return nil
	})

	return tasksByState
}

func stateCompletionCandidates() []string {
	stateCatalog, err := loadStateCatalog()
	if err != nil || stateCatalog == nil {
		return []string{}
	}
	seen := map[string]struct{}{}
	candidates := []string{}
	for _, name := range stateCatalog.Order {
		upper := strings.ToUpper(strings.TrimSpace(name))
		if upper == "" {
			continue
		}
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		candidates = append(candidates, upper)
	}
	return candidates
}
