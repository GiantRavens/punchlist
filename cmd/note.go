package cmd

import (
	"fmt"
	"punchlist/task"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// a note list item: "- <timestamp>: <message>" under ## Notes. The capture is
// everything after the leading "- " so the index/display matches the file.
var noteItemRe = regexp.MustCompile(`^- (.+)$`)

// create the note command (add on the bare form; ls/rm as subcommands)
func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note [id] [message]",
		Short: "Add a note to a task (or `note ls`/`note rm`)",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				failf("Invalid task ID: %v\n", err)
				return
			}

			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				failf("Error finding task: %v\n", err)
				return
			}

			t, err := task.Parse(taskPath)
			if err != nil {
				failf("Error parsing task: %v\n", err)
				return
			}

			addNote(t, args[1])

			if err := t.Write(taskPath); err != nil {
				failf("Error updating task: %v\n", err)
				return
			}

			fmt.Printf("Added note to task %d\n", id)
		},
	}
	cmd.AddCommand(newNoteLsCmd())
	cmd.AddCommand(newNoteRmCmd())
	return cmd
}

// parseNotes returns the ## Notes list items (text after "- "), in file order.
func parseNotes(body string) []string {
	_, section, _, found := splitSection(body, "## Notes")
	if !found {
		return nil
	}
	var items []string
	for _, line := range strings.Split(section, "\n") {
		if m := noteItemRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			items = append(items, m[1])
		}
	}
	return items
}

func newNoteLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls [id]",
		Aliases: []string{"list"},
		Short:   "List a task's notes with indices",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				failf("Invalid task ID: %v\n", err)
				return
			}
			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				failf("Error finding task: %v\n", err)
				return
			}
			t, err := task.Parse(taskPath)
			if err != nil {
				failf("Error parsing task: %v\n", err)
				return
			}
			notes := parseNotes(t.Body)
			if len(notes) == 0 {
				fmt.Println("No notes found.")
				return
			}
			for i, n := range notes {
				fmt.Printf("%d. %s\n", i+1, n)
			}
		},
	}
}

func newNoteRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm [id] [index]",
		Aliases: []string{"remove"},
		Short:   "Remove a note by index (see `pin note ls <id>`)",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				failf("Invalid task ID: %v\n", err)
				return
			}
			index, err := strconv.Atoi(args[1])
			if err != nil || index < 1 {
				failf("Invalid note index: %s (must be 1 or greater)\n", args[1])
				return
			}
			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				failf("Error finding task: %v\n", err)
				return
			}
			t, err := task.Parse(taskPath)
			if err != nil {
				failf("Error parsing task: %v\n", err)
				return
			}
			newBody, removed, err := removeNoteItem(t.Body, index)
			if err != nil {
				failf("Error: %v\n", err)
				return
			}
			t.Body = newBody
			t.UpdatedAt = time.Now()
			if err := t.Write(taskPath); err != nil {
				failf("Error updating task: %v\n", err)
				return
			}
			fmt.Printf("Removed note %d from task %d: %s\n", index, id, removed)
		},
	}
}

// removeNoteItem drops the nth "- " item from ## Notes. It also drops any
// indented continuation lines belonging to that item (a wrapped multi-line
// note), stopping at the next "- " item, a blank line, or a new heading.
// Returns the new body, the removed item's text, and any error.
func removeNoteItem(body string, targetIndex int) (string, string, error) {
	before, section, after, found := splitSection(body, "## Notes")
	if !found {
		return "", "", fmt.Errorf("no ## Notes section found")
	}

	lines := strings.Split(section, "\n")
	itemIndex := 0
	removedText := ""
	start, end := -1, -1

	for i, line := range lines {
		if m := noteItemRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			itemIndex++
			if itemIndex == targetIndex {
				removedText = m[1]
				start = i
				// consume continuation lines (indented, non-item, non-heading)
				end = i + 1
				for end < len(lines) {
					t := lines[end]
					trimmed := strings.TrimSpace(t)
					if trimmed == "" || strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "## ") {
						break
					}
					if t != strings.TrimLeft(t, " \t") { // indented continuation
						end++
						continue
					}
					break
				}
				break
			}
		}
	}

	if start == -1 {
		return "", "", fmt.Errorf("note index %d not found (found %d notes)", targetIndex, itemIndex)
	}

	lines = append(lines[:start], lines[end:]...)
	newSection := strings.Join(lines, "\n")
	newBody := joinBlocks(before, newSection, after)
	return newBody, removedText, nil
}
