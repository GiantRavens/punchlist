package cmd

import (
	"fmt"
	"os"
	"punchlist/task"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var acceptanceCheckboxRe = regexp.MustCompile(`^- \[([ xX])\] (.+)$`)

// parseAcceptance extracts checkbox items from the ## Acceptance section of a task body
func parseAcceptance(body string) []AcceptanceItem {
	_, section, _, found := splitSection(body, "## Acceptance")
	if !found {
		return nil
	}

	lines := strings.Split(section, "\n")
	var items []AcceptanceItem
	index := 1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		match := acceptanceCheckboxRe.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}
		checked := match[1] == "x" || match[1] == "X"
		items = append(items, AcceptanceItem{
			Text:    match[2],
			Checked: checked,
			Index:   index,
		})
		index++
	}
	return items
}

// create the acceptance command
func newAcceptanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "acceptance [id]",
		Aliases: []string{"checks"},
		Short:   "List acceptance criteria for a task",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
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

			items := parseAcceptance(t.Body)
			if len(items) == 0 {
				fmt.Println("No acceptance criteria found.")
				return
			}

			for _, item := range items {
				marker := "[ ]"
				if item.Checked {
					marker = "[x]"
				}
				fmt.Printf("%d. %s %s\n", item.Index, marker, item.Text)
			}
		},
	}
}

// create the check command
func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [id] [index]",
		Short: "Toggle an acceptance criterion checkbox",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid task ID: %v\n", err)
				return
			}
			index, err := strconv.Atoi(args[1])
			if err != nil || index < 1 {
				fmt.Fprintf(os.Stderr, "Invalid check index: %s (must be 1 or greater)\n", args[1])
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

			newBody, nowChecked, err := toggleAcceptanceCheck(t.Body, index)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}

			t.Body = newBody
			t.UpdatedAt = time.Now()
			if err := t.Write(taskPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating task: %v\n", err)
				return
			}

			state := "unchecked"
			if nowChecked {
				state = "checked"
			}
			fmt.Printf("Toggled check %d to %s on task %d\n", index, state, id)
		},
	}
}

// toggleAcceptanceCheck toggles the nth checkbox in ## Acceptance.
// Returns the new body, whether the item is now checked, and any error.
func toggleAcceptanceCheck(body string, targetIndex int) (string, bool, error) {
	before, section, after, found := splitSection(body, "## Acceptance")
	if !found {
		return "", false, fmt.Errorf("no ## Acceptance section found")
	}

	lines := strings.Split(section, "\n")
	checkIndex := 0
	toggled := false
	nowChecked := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		match := acceptanceCheckboxRe.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}
		checkIndex++
		if checkIndex != targetIndex {
			continue
		}

		if match[1] == " " {
			lines[i] = strings.Replace(line, "[ ]", "[x]", 1)
			nowChecked = true
		} else {
			lines[i] = strings.Replace(line, "[x]", "[ ]", 1)
			lines[i] = strings.Replace(lines[i], "[X]", "[ ]", 1)
		}
		toggled = true
		break
	}

	if !toggled {
		return "", false, fmt.Errorf("check index %d not found (found %d items)", targetIndex, checkIndex)
	}

	newSection := strings.Join(lines, "\n")
	newBody := joinBlocks(before, newSection, after)
	return newBody, nowChecked, nil
}
