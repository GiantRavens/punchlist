package cmd

import (
	"fmt"
	"path/filepath"
	"punchlist/task"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// create the show command
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [id]",
		Short: "Show a task in detail",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse id and load task
			id, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Printf("Invalid task ID: %v\n", err)
				return
			}

			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				fmt.Printf("Error finding task: %v\n", err)
				return
			}

			t, err := task.Parse(taskPath)
			if err != nil {
				fmt.Printf("Error parsing task: %v\n", err)
				return
			}

			// print task details
			fmt.Printf("ID: %d\n", t.ID)
			fmt.Printf("Title: %s\n", t.Title)
			fmt.Printf("State: %s\n", t.State)
			fmt.Printf("Priority: %d\n", t.Priority)
			fmt.Printf("Due: %s\n", formatOptionalTime(t.Due))
			fmt.Printf("Tags: %s\n", formatList(t.Tags))
			fmt.Printf("Created: %s\n", t.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Updated: %s\n", t.UpdatedAt.Format(time.RFC3339))
			fmt.Printf("Started: %s\n", formatOptionalTime(t.StartedAt))
			fmt.Printf("Completed: %s\n", formatOptionalTime(t.CompletedAt))
			fmt.Printf("External refs: %s\n", formatList(t.ExternalRefs))
			fmt.Printf("Path: %s\n", filepath.Clean(taskPath))

			if t.Body != "" {
				fmt.Printf("\n-------\n%s\n", t.Body)
			}
		},
	}
}

// render optional timestamps consistently
func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return "n/a"
	}
	return t.Format(time.RFC3339)
}

// format string slices for display
func formatList(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ",")
}
