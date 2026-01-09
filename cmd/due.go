package cmd

import (
	"fmt"
	"punchlist/task"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// create the due command
func newDueCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "due [id] [date]",
		Aliases: []string{"DUE"},
		Short:   "Set or change a task due date",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// parse inputs
			id, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Printf("Invalid task ID: %v\n", err)
				return
			}

			// support multi-word dates like "next tuesday"
			dueInput := strings.Join(args[1:], " ")
			dueTime, err := parseDue(dueInput)
			if err != nil {
				fmt.Printf("Invalid due date: %v\n", err)
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

			// update task and append a log entry
			now := time.Now()
			prevDue := t.Due
			t.Due = dueTime
			t.UpdatedAt = now

			dueText := dueTime.Format(time.RFC3339)
			var msg string
			if prevDue == nil {
				msg = fmt.Sprintf("added due date: %s", dueText)
			} else {
				msg = fmt.Sprintf("due date changed to: %s", dueText)
			}
			logEntry := fmt.Sprintf("- %s: %s", now.Format(time.RFC3339), msg)

			pre, logSection, afterLog, found := splitSection(t.Body, "## Log")
			if found {
				pre += afterLog
			} else {
				logSection = "## Log"
			}
			logSection = appendEntry(logSection, logEntry)
			t.Body = joinBlocks(pre, logSection)

			if err := t.Write(taskPath); err != nil {
				fmt.Printf("Error updating task: %v\n", err)
				return
			}

			fmt.Printf("Updated due date for task %d\n", id)
		},
	}
}
