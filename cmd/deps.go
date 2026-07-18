package cmd

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// create the deps command
func newDepsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deps [id]",
		Short: "Show dependencies for a task",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				failf("Invalid task ID: %v\n", err)
				return
			}

			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				failf("Error finding task: %v\n", err)
				return
			}

			t, err := task.Parse(taskPath)
			if err != nil {
				failf("Error parsing task: %v\n", err)
				return
			}

			// show what this task depends on
			if len(t.DependsOn) > 0 {
				fmt.Printf("Task %d depends on: %s\n", id, formatIntList(t.DependsOn))
				for _, depID := range t.DependsOn {
					depPath, err := findTaskFile(depID)
					if err != nil {
						fmt.Printf("  %d: (not found)\n", depID)
						continue
					}
					dep, err := task.Parse(depPath)
					if err != nil {
						fmt.Printf("  %d: (parse error)\n", depID)
						continue
					}
					fmt.Printf("  %d: %s - %s\n", dep.ID, dep.State, dep.Title)
				}
			} else {
				fmt.Printf("Task %d has no dependencies.\n", id)
			}

			// reverse lookup: what depends on this task
			dependents := findDependents(id)
			if len(dependents) > 0 {
				fmt.Printf("\nDepended on by:\n")
				for _, dep := range dependents {
					fmt.Printf("  %d: %s - %s\n", dep.ID, dep.State, dep.Title)
				}
			}
		},
	}
}

// findDependents returns all tasks that have targetID in their DependsOn list
func findDependents(targetID int) []*task.Task {
	tasksPath, err := tasksDir()
	if err != nil {
		return nil
	}

	var dependents []*task.Task
	_ = filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		t, err := task.Parse(path)
		if err != nil {
			return nil
		}
		for _, depID := range t.DependsOn {
			if depID == targetID {
				dependents = append(dependents, t)
				break
			}
		}
		return nil
	})
	return dependents
}

// formatIntList renders an int slice as "1, 2, 3"
func formatIntList(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ", ")
}

// buildTaskMap loads all tasks from disk into an id-keyed map
func buildTaskMap(tasksPath string) map[int]*task.Task {
	m := make(map[int]*task.Task)
	_ = filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		t, err := task.Parse(path)
		if err != nil {
			return nil
		}
		m[t.ID] = t
		return nil
	})
	return m
}

// isTaskReady checks if all dependencies of t are in DONE state
func isTaskReady(t *task.Task, taskMap map[int]*task.Task, stateCatalog *config.StateCatalog) bool {
	if len(t.DependsOn) == 0 {
		return true
	}
	for _, depID := range t.DependsOn {
		dep, ok := taskMap[depID]
		if !ok {
			return false // unknown dependency = not ready
		}
		canonical := canonicalizeState(stateCatalog, dep.State)
		if canonical != "DONE" {
			return false
		}
	}
	return true
}
