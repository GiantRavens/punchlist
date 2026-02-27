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

// create the show command
func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [id]",
		Short: "Show a task in detail",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// parse id and load task
			id, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid task ID: %v\n", err)
				return
			}

			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error finding task: %v\n", err)
				return
			}

			t, err := task.Parse(taskPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing task: %v\n", err)
				return
			}

			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput {
				acceptance := parseAcceptance(t.Body)
				if err := marshalTaskShow(t, acceptance); err != nil {
					fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
				}
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
			if len(t.DependsOn) > 0 {
				depStrs := make([]string, len(t.DependsOn))
				for i, d := range t.DependsOn {
					depStrs[i] = strconv.Itoa(d)
				}
				fmt.Printf("Depends on: %s\n", strings.Join(depStrs, ", "))
			}
			if len(t.Meta) > 0 {
				fmt.Printf("Meta:\n")
				for k, v := range t.Meta {
					fmt.Printf("  %s: %v\n", k, v)
				}
			}
			fmt.Printf("Path: %s\n", filepath.Clean(taskPath))

			if t.Body != "" {
				fmt.Printf("\n-------\n%s\n", t.Body)
			}
		},
	}

	cmd.Flags().Bool("json", false, "Output as JSON")

	return cmd
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
