package cmd

import (
	"fmt"
	"os"
	"punchlist/task"
	"strconv"

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

			addNote(t, message)

			if err := t.Write(taskPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating task: %v\n", err)
				return
			}

			fmt.Printf("Added note to task %d\n", id)
		},
	}
}
