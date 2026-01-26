package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"punchlist/task"
	"strings"

	"github.com/spf13/cobra"
)

// create the search command
func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [path] <query>",
		Short: "Search tasks by text",
		Long:  "Search tasks by text across frontmatter, title, and body/notes (excluding logs).",
		Run: func(cmd *cobra.Command, args []string) {
			// read filter and sort flags
			lsPriority, _ := cmd.Flags().GetInt("pri")
			lsTags, _ := cmd.Flags().GetStringSlice("tag")
			lsOrder, _ := cmd.Flags().GetString("order")
			lsReverse, _ := cmd.Flags().GetBool("reverse")
			lsState, _ := cmd.Flags().GetString("state")
			lsStatus, _ := cmd.Flags().GetString("status")

			targetPath, remainingArgs := extractTargetPath(args)
			if len(remainingArgs) == 0 {
				fmt.Println("Error: search query required")
				return
			}
			query := strings.Join(remainingArgs, " ")
			query = strings.TrimSpace(query)
			if query == "" {
				fmt.Println("Error: search query required")
				return
			}
			queryLower := strings.ToLower(query)

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

			// parse optional state filter (flag only for search)
			var filterState task.State
			stateToken := lsState
			if stateToken == "" {
				stateToken = lsStatus
			} else if lsStatus != "" && !strings.EqualFold(lsState, lsStatus) {
				fmt.Println("Error: state provided twice (use either --state or --status)")
				return
			}
			if stateToken != "" {
				if parsed, ok := task.ParseState(stateToken); ok {
					filterState = parsed
				} else {
					filterState = task.State(stateToken)
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

					content, err := os.ReadFile(path)
					if err != nil {
						fmt.Printf("Error reading task file %s: %v\n", path, err)
						return nil
					}
					frontmatter, body := splitFrontmatterAndBody(string(content))
					body = removeLogSection(body)
					searchable := strings.Join([]string{frontmatter, t.Title, body}, "\n")

					if !strings.Contains(strings.ToLower(searchable), queryLower) {
						return nil
					}

					tasks = append(tasks, t)
				}
				return nil
			})

			if err != nil {
				fmt.Printf("Error searching tasks: %v\n", err)
				return
			}

			if len(tasks) == 0 {
				fmt.Println("No matches found.")
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
			titleMaxLen := loadLsTitleMaxLen()
			shouldGroupByState := filterState == "" &&
				strings.ToLower(strings.TrimSpace(lsOrder)) != "id"
			var lastState task.State
			for _, t := range tasks {
				if shouldGroupByState && lastState != "" && t.State != lastState {
					fmt.Println(stateSeparatorLine)
				}
				displayTitle := truncateWithEllipsis(t.Title, titleMaxLen)
				lineParts := []string{
					fmt.Sprintf("%*d", idWidth, t.ID),
					string(t.State),
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
				lastState = t.State
			}
		},
	}

	cmd.Flags().Int("pri", 0, "Filter by priority")
	cmd.Flags().StringSlice("tag", []string{}, "Filter by tag (can be used multiple times)")
	cmd.Flags().String("state", "", "Filter by state")
	cmd.Flags().String("status", "", "Alias for --state")
	cmd.Flags().String("order", "state", "Order by state or id")
	cmd.Flags().Bool("reverse", false, "Reverse sort order")

	return cmd
}

const searchFrontmatterSeparator = "---"

func splitFrontmatterAndBody(content string) (string, string) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", ""
	}
	if strings.TrimSpace(lines[0]) != searchFrontmatterSeparator {
		return "", content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == searchFrontmatterSeparator {
			frontmatter := strings.Join(lines[1:i], "\n")
			body := strings.Join(lines[i+1:], "\n")
			return frontmatter, body
		}
	}
	return "", content
}

func removeLogSection(body string) string {
	before, _, after, found := splitSection(body, "## Log")
	if !found {
		return body
	}
	return joinBlocks(before, after)
}
