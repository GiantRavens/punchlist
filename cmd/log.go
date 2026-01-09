package cmd

import (
	"fmt"
	"punchlist/task"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// create the log command
func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log [id] [message]",
		Short: "Add a log entry to a task",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// parse input
			idStr := args[0]
			message := args[1]

			id, err := strconv.Atoi(idStr)
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

			// add a timestamped entry
			logEntry := fmt.Sprintf("- %s: %s", time.Now().Format(time.RFC3339), message)

			pre, logSection, afterLog, found := splitSection(t.Body, "## Log")
			if found {
				pre += afterLog
			} else {
				logSection = "## Log"
			}

			logSection = appendEntry(logSection, logEntry)
			t.Body = joinBlocks(pre, logSection)
			t.UpdatedAt = time.Now()

			if err := t.Write(taskPath); err != nil {
				fmt.Printf("Error updating task: %v\n", err)
				return
			}

			fmt.Printf("Added log to task %d\n", id)
		},
	}
}
