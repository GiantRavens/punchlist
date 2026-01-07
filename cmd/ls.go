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

func newLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls [state]",
		Short: "List tasks",
		Long:  `List tasks, optionally filtering by state, priority, and tags.`,
		Run: func(cmd *cobra.Command, args []string) {
			lsPriority, _ := cmd.Flags().GetInt("pri")
			lsTags, _ := cmd.Flags().GetStringSlice("tag")
			lsOrder, _ := cmd.Flags().GetString("order")
			lsReverse, _ := cmd.Flags().GetBool("reverse")

			tasksDir := "tasks"
			if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
				fmt.Println("No tasks found.")
				return
			}

			var filterState task.State
			if len(args) > 0 {
				if parsed, ok := task.ParseState(args[0]); ok {
					filterState = parsed
				} else {
					filterState = task.State(args[0])
				}
			}

			var tasks []*task.Task

			err := filepath.WalkDir(tasksDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
					t, err := task.Parse(path)
					if err != nil {
						fmt.Printf("Error parsing task file %s: %v\n", path, err)
						return nil // Continue walking
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

					tagSuffix := ""
					if len(t.Tags) > 0 {
						tagSuffix = fmt.Sprintf("  {%s}", strings.Join(t.Tags, ","))
					}
					tasks = append(tasks, t)
				}
				return nil
			})

			if err != nil {
				fmt.Printf("Error listing tasks: %v\n", err)
				return
			}

			sortTasks(tasks, lsOrder, lsReverse)

			for _, t := range tasks {
				tagSuffix := ""
				if len(t.Tags) > 0 {
					tagSuffix = fmt.Sprintf(" {%s}", strings.Join(t.Tags, ","))
				}
				fmt.Printf("%d %s %s pri:%d due:%s%s\n",
					t.ID,
					t.State,
					t.Title,
					t.Priority,
					formatDueDate(t.Due),
					tagSuffix,
				)
			}
		},
	}

	cmd.Flags().Int("pri", 0, "Filter by priority")
	cmd.Flags().StringSlice("tag", []string{}, "Filter by tag (can be used multiple times)")
	cmd.Flags().String("order", "state", "Order by state or id")
	cmd.Flags().Bool("reverse", false, "Reverse sort order")

	return cmd
}

func formatDueDate(t *time.Time) string {
	if t == nil {
		return "n/a"
	}
	return t.Format("2006-01-02")
}

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

func loadStateOrder() []string {
	cfg, err := config.LoadConfig()
	if err != nil || len(cfg.LsStateOrder) == 0 {
		return config.DefaultLsStateOrder()
	}
	return cfg.LsStateOrder
}

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

func stateOrderKey(state task.State) string {
	return strings.ToUpper(string(state))
}

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
