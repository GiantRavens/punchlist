package cmd

import (
	"fmt"
	"punchlist/task"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// changePriority mutates the task's priority field and updated_at, returning
// a log-entry message and whether anything actually changed. Pure helper so
// the state-transition logic is testable without touching disk.
func changePriority(tsk *task.Task, newPri int, now time.Time) (msg string, changed bool) {
	prev := tsk.Priority
	if prev == newPri {
		return "", false
	}
	tsk.Priority = newPri
	tsk.UpdatedAt = now
	switch {
	case prev == 0 && newPri != 0:
		return fmt.Sprintf("priority set to %d", newPri), true
	case prev != 0 && newPri == 0:
		return fmt.Sprintf("priority cleared (was %d)", prev), true
	default:
		return fmt.Sprintf("priority changed from %d to %d", prev, newPri), true
	}
}

// newPriCmd sets or clears the priority on existing tasks.
//
// Grammar: pin pri <id> [<id> ...] <priority>
//
// Priority is parsed as an int; pass 0 to clear (priority is yaml:omitempty).
// Mirrors the due-command shape: load task, mutate field, append log entry,
// write back. Lets bulk priority resets stay inside pin discipline instead
// of touching task .md files directly.
func newPriCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "pri [id ...] [priority]",
		Aliases: []string{"PRI", "priority"},
		Short:   "Set or clear the priority on one or more tasks (0 clears)",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			priArg := args[len(args)-1]
			newPri, err := strconv.Atoi(priArg)
			if err != nil {
				failf("Invalid priority %q: must be an integer (0 clears)\n", priArg)
				return
			}

			idArgs := args[:len(args)-1]
			ids := make([]int, 0, len(idArgs))
			for _, a := range idArgs {
				id, err := strconv.Atoi(a)
				if err != nil {
					failf("Invalid task ID %q: %v\n", a, err)
					return
				}
				ids = append(ids, id)
			}

			for _, id := range ids {
				taskPath, err := findTaskFile(id)
				if err != nil {
					if printNotPunchlistError(err) {
						return
					}
					failf("Error finding task %d: %v\n", id, err)
					continue
				}

				t, err := task.Parse(taskPath)
				if err != nil {
					failf("Error parsing task %d: %v\n", id, err)
					continue
				}

				now := time.Now()
				msg, changed := changePriority(t, newPri, now)
				if !changed {
					fmt.Printf("Task %d priority unchanged (already %d)\n", id, newPri)
					continue
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
					failf("Error updating task %d: %v\n", id, err)
					continue
				}

				fmt.Printf("Task %d: %s\n", id, msg)
			}
		},
	}
}
