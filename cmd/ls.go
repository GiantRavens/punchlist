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
			// read filter and sort flags
			lsPriority, _ := cmd.Flags().GetInt("pri")
			lsTags, _ := cmd.Flags().GetStringSlice("tag")
			lsOrder, _ := cmd.Flags().GetString("order")
			lsReverse, _ := cmd.Flags().GetBool("reverse")

			targetPath, remainingArgs := extractTargetPath(args)

			// scan tasks directory
			var tasksPath string
			var err error
			if targetPath != "" {
				root, err := punchlistRootFromPath(targetPath)
				if err != nil {
					if printNotPunchlistError(err) {
						return
					}
					fmt.Printf("Error locating tasks: %v\n", err)
					return
				}
				tasksPath = filepath.Join(root, "tasks")
			} else {
				tasksPath, err = tasksDir()
				if err != nil {
					if printNotPunchlistError(err) {
						return
					}
					fmt.Printf("Error locating tasks: %v\n", err)
					return
				}
			}
			if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
				fmt.Println("No tasks found.")
				return
			}

			// parse optional state filter
			var filterState task.State
			if len(remainingArgs) > 0 {
				if parsed, ok := task.ParseState(remainingArgs[0]); ok {
					filterState = parsed
				} else {
					filterState = task.State(remainingArgs[0])
				}
			}

			// load tasks and apply filters
			var tasks []*task.Task

			err = filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
					t, err := task.Parse(path)
					if err != nil {
						fmt.Printf("Error parsing task file %s: %v\n", path, err)
						return nil // continue walking
					}

					if filterState != "" && t.State != filterState {
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
				fmt.Printf("Error listing tasks: %v\n", err)
				return
			}

			// order results
			sortTasks(tasks, lsOrder, lsReverse)

			// print aligned ids
			idWidth := maxIDWidth(tasks)
			configWidth := loadIDWidth()
			if configWidth > idWidth {
				idWidth = configWidth
			}
			shouldGroupByState := filterState == "" &&
				lsPriority == 0 &&
				len(lsTags) == 0 &&
				strings.ToLower(strings.TrimSpace(lsOrder)) != "id"
			var lastState task.State
			for _, t := range tasks {
				if shouldGroupByState && lastState != "" && t.State != lastState {
					fmt.Println(stateSeparatorLine)
				}
				tagSuffix := ""
				if len(t.Tags) > 0 {
					tagSuffix = fmt.Sprintf(" {%s}", strings.Join(t.Tags, ","))
				}
				fmt.Printf("%*d %s %s pri:%d due:%s%s\n",
					idWidth,
					t.ID,
					t.State,
					t.Title,
					t.Priority,
					formatDueDate(t.Due),
					tagSuffix,
				)
				lastState = t.State
			}
		},
	}

	cmd.Flags().Int("pri", 0, "Filter by priority")
	cmd.Flags().StringSlice("tag", []string{}, "Filter by tag (can be used multiple times)")
	cmd.Flags().String("order", "state", "Order by state or id")
	cmd.Flags().Bool("reverse", false, "Reverse sort order")

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
func sortTasks(tasks []*task.Task, order string, reverse bool) {
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
		stateOrder := loadStateOrder()
		orderIndex := buildStateOrderIndex(stateOrder)
		sort.Slice(tasks, func(i, j int) bool {
			ai := orderIndex[stateOrderKey(tasks[i].State)]
			aj := orderIndex[stateOrderKey(tasks[j].State)]
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

// read state ordering from config
func loadStateOrder() []string {
	cfg, err := config.LoadConfig()
	if err != nil || len(cfg.LsStateOrder) == 0 {
		return config.DefaultLsStateOrder()
	}
	return cfg.LsStateOrder
}

// read id width from config
func loadIDWidth() int {
	cfg, err := config.LoadConfig()
	if err != nil || cfg.IDWidth <= 0 {
		return config.DefaultIDWidth()
	}
	return cfg.IDWidth
}

// build a lookup index from a state order list
func buildStateOrderIndex(order []string) map[string]int {
	index := make(map[string]int, len(order))
	for i, label := range order {
		if state, ok := normalizeOrderLabel(label); ok {
			index[state] = i
		}
	}
	defaultIndex := len(order)
	for _, state := range []task.State{
		task.StateBegun,
		task.StateBlock,
		task.StateTodo,
		task.StateConfirm,
		task.StateDone,
		task.StateNotDo,
	} {
		key := stateOrderKey(state)
		if _, ok := index[key]; !ok {
			index[key] = defaultIndex
			defaultIndex++
		}
	}
	return index
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

// normalize for state ordering map keys
func stateOrderKey(state task.State) string {
	return strings.ToUpper(string(state))
}

// normalize state labels from config for ordering
func normalizeOrderLabel(label string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(label)) {
	case "TODO":
		return stateOrderKey(task.StateTodo), true
	case "BEGUN", "DOING", "INPROGRESS", "IN-PROGRESS":
		return stateOrderKey(task.StateBegun), true
	case "BLOCK", "BLOCKED":
		return stateOrderKey(task.StateBlock), true
	case "CONFIRM", "FOLLOWUP", "FOLLOW-UP", "CHASE":
		return stateOrderKey(task.StateConfirm), true
	case "DONE":
		return stateOrderKey(task.StateDone), true
	case "NOTDO", "DEFER", "DEFERRED":
		return stateOrderKey(task.StateNotDo), true
	default:
		return "", false
	}
}
