package cmd

import (
	"fmt"
	"punchlist/task"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// create the note command
func newNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "note [id] [message]",
		Short: "Add a note to a task",
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
				fmt.Printf("Error finding task: %v\n", err)
				return
			}

			t, err := task.Parse(taskPath)
			if err != nil {
				fmt.Printf("Error parsing task: %v\n", err)
				return
			}

			// add a timestamped entry
			noteEntry := fmt.Sprintf("- %s: %s", time.Now().Format(time.RFC3339), message)

			pre, logSection, afterLog, logFound := splitSection(t.Body, "## Log")
			if logFound {
				pre += afterLog
			}

			beforeNotes, notesSection, afterNotes, notesFound := splitSection(pre, "## Notes")
			if !notesFound {
				notesSection = "## Notes"
			}

			notesSection = appendEntry(notesSection, noteEntry)
			pre = joinBlocks(beforeNotes, notesSection, afterNotes)
			if logFound {
				t.Body = joinBlocks(pre, logSection)
			} else {
				t.Body = pre
			}
			t.UpdatedAt = time.Now()

			if err := t.Write(taskPath); err != nil {
				fmt.Printf("Error updating task: %v\n", err)
				return
			}

			fmt.Printf("Added note to task %d\n", id)
		},
	}
}
