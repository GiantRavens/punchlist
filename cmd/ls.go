package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const stateSeparatorLine = "----------------------------------------"

// create the ls command
func newLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "ls [state]",
		Short:             "List tasks",
		Long:              `List tasks, optionally filtering by state, priority, and tags.`,
		ValidArgsFunction: stateArgCompletion,
		Run: func(cmd *cobra.Command, args []string) {
			stateCatalog, err := loadStateCatalog()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading state config: %v\n", err)
				return
			}
			// read filter and sort flags
			lsPriority, _ := cmd.Flags().GetInt("pri")
			lsTags, _ := cmd.Flags().GetStringSlice("tag")
			lsOrder, _ := cmd.Flags().GetString("order")
			lsReverse, _ := cmd.Flags().GetBool("reverse")
			lsState, _ := cmd.Flags().GetString("state")
			lsStatus, _ := cmd.Flags().GetString("status")
			jsonOutput, _ := cmd.Flags().GetBool("json")
			lsReady, _ := cmd.Flags().GetBool("ready")

			targetPath, remainingArgs := extractTargetPath(args)

			// scan tasks directory
			var tasksPath string
			if targetPath != "" {
				root, err := punchlistRootFromPath(targetPath)
				if err != nil {
					if printNotPunchlistError(err) {
						return
					}
					fmt.Fprintf(os.Stderr, "Error locating tasks: %v\n", err)
					return
				}
				tasksPath = filepath.Join(root, "tasks")
			} else {
				tasksPath, err = tasksDir()
				if err != nil {
					if printNotPunchlistError(err) {
						return
					}
					fmt.Fprintf(os.Stderr, "Error locating tasks: %v\n", err)
					return
				}
			}
			if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
				fmt.Println("No tasks found.")
				return
			}

			// parse optional state filter
			stateToken := lsState
			if stateToken == "" {
				stateToken = lsStatus
			} else if lsStatus != "" && !strings.EqualFold(lsState, lsStatus) {
				fmt.Fprintln(os.Stderr, "Error: state provided twice (use either --state or --status)")
				return
			}
			if stateToken != "" && len(remainingArgs) > 0 {
				if !strings.EqualFold(stateToken, remainingArgs[0]) {
					fmt.Fprintln(os.Stderr, "Error: state provided twice (use either --state or positional state)")
					return
				}
			}
			if stateToken == "" && len(remainingArgs) > 0 {
				stateToken = remainingArgs[0]
			}
			filterToken := strings.TrimSpace(stateToken)

			// load tasks and apply filters
			var tasks []*task.Task

			err = filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
					t, err := task.Parse(path)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error parsing task file %s: %v\n", path, err)
						return nil // continue walking
					}

					if filterToken != "" && !compareState(stateCatalog, t.State, filterToken) {
						return nil
					}
					if lsPriority != 0 && t.Priority != lsPriority {
						return nil
					}

					if len(lsTags) > 0 {
						tagMatch := false
						for _, tag := range lsTags {
							for _, taskTag := range t.Tags {
								if tag == taskTag {
									tagMatch = true
									break
								}
							}
							if tagMatch {
								break
							}
						}
						if !tagMatch {
							return nil
						}
					}

					tasks = append(tasks, t)
				}
				return nil
			})

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing tasks: %v\n", err)
				return
			}

			// filter to tasks whose dependencies are all DONE
			if lsReady {
				taskMap := buildTaskMap(tasksPath)
				var ready []*task.Task
				for _, t := range tasks {
					if isTaskReady(t, taskMap, stateCatalog) {
						ready = append(ready, t)
					}
				}
				tasks = ready
			}

			if len(tasks) == 0 {
				fmt.Println("No matches found.")
				return
			}

			// order results
			sortTasks(tasks, lsOrder, lsReverse, stateCatalog)

			if jsonOutput {
				if err := marshalTaskList(tasks); err != nil {
					fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
				}
				return
			}

			// print aligned ids
			idWidth := maxIDWidth(tasks)
			configWidth := loadIDWidth()
			if configWidth > idWidth {
				idWidth = configWidth
			}
			titleMaxLen := loadLsTitleMaxLen()
			shouldGroupByState := filterToken == "" &&
				lsPriority == 0 &&
				len(lsTags) == 0 &&
				strings.ToLower(strings.TrimSpace(lsOrder)) != "id"
			var lastState string
			for _, t := range tasks {
				displayState := canonicalizeState(stateCatalog, t.State)
				if shouldGroupByState && lastState != "" && displayState != lastState {
					fmt.Println(stateSeparatorLine)
				}
				displayTitle := truncateWithEllipsis(t.Title, titleMaxLen)
				lineParts := []string{
					fmt.Sprintf("%*d", idWidth, t.ID),
					displayState,
					displayTitle,
				}
				if t.Priority > 0 {
					lineParts = append(lineParts, fmt.Sprintf("pri:%d", t.Priority))
				}
				if t.Due != nil {
					lineParts = append(lineParts, fmt.Sprintf("due:%s", formatDueDate(t.Due)))
				}
				if len(t.Tags) > 0 {
					lineParts = append(lineParts, fmt.Sprintf("{%s}", strings.Join(t.Tags, ",")))
				}
				fmt.Printf("%s\n", strings.Join(lineParts, " "))
				lastState = displayState
			}
		},
	}

	cmd.Flags().Int("pri", 0, "Filter by priority")
	cmd.Flags().StringSlice("tag", []string{}, "Filter by tag (can be used multiple times)")
	cmd.Flags().String("state", "", "Filter by state")
	cmd.Flags().String("status", "", "Alias for --state")
	cmd.Flags().String("order", "state", "Order by state or id")
	cmd.Flags().Bool("reverse", false, "Reverse sort order")
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().Bool("ready", false, "Show only tasks whose dependencies are all done")

	return cmd
}

// render due dates consistently
func formatDueDate(t *time.Time) string {
	if t == nil {
		return "n/a"
	}
	return t.Format("2006-01-02")
}

// sort tasks by the requested ordering
func sortTasks(tasks []*task.Task, order string, reverse bool, catalog *config.StateCatalog) {
	order = strings.ToLower(strings.TrimSpace(order))
	switch order {
	case "id":
		sort.Slice(tasks, func(i, j int) bool {
			if reverse {
				return tasks[i].ID > tasks[j].ID
			}
			return tasks[i].ID < tasks[j].ID
		})
	default:
		orderIndex := buildStateOrderIndex(catalog, tasks)
		sort.Slice(tasks, func(i, j int) bool {
			stateA := canonicalizeState(catalog, tasks[i].State)
			stateB := canonicalizeState(catalog, tasks[j].State)
			ai, _ := orderIndexForState(orderIndex, stateA)
			aj, _ := orderIndexForState(orderIndex, stateB)
			if ai == aj {
				if reverse {
					return tasks[i].ID > tasks[j].ID
				}
				return tasks[i].ID < tasks[j].ID
			}
			if reverse {
				return ai > aj
			}
			return ai < aj
		})
	}
}

// read id width from config
func loadIDWidth() int {
	cfg, err := config.LoadConfig()
	if err != nil || cfg.IDWidth <= 0 {
		return config.DefaultIDWidth()
	}
	return cfg.IDWidth
}

// calculate width for right-aligned id output
func maxIDWidth(tasks []*task.Task) int {
	width := 1
	for _, t := range tasks {
		digits := len(fmt.Sprintf("%d", t.ID))
		if digits > width {
			width = digits
		}
	}
	return width
}
