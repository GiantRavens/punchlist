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
	cmd := &cobra.Command{
		Use:     "acceptance [id]",
		Aliases: []string{"checks"},
		Short:   "List acceptance criteria for a task",
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
	cmd.AddCommand(newAcceptanceAddCmd())
	cmd.AddCommand(newAcceptanceRmCmd())
	return cmd
}

// create the acceptance add subcommand
func newAcceptanceAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [id] [criterion]",
		Short: "Add an acceptance criterion to a task",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				failf("Invalid task ID: %v\n", err)
				return
			}
			text := strings.TrimSpace(args[1])
			if text == "" {
				failf("Acceptance criterion text must not be empty\n")
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

			t.Body = addAcceptanceItem(t.Body, text)
			t.UpdatedAt = time.Now()
			if err := t.Write(taskPath); err != nil {
				failf("Error updating task: %v\n", err)
				return
			}

			index := len(parseAcceptance(t.Body))
			fmt.Printf("Added acceptance criterion %d to task %d\n", index, id)
		},
	}
}

// create the acceptance rm subcommand
func newAcceptanceRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm [id] [index]",
		Aliases: []string{"remove"},
		Short:   "Remove an acceptance criterion from a task by index",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				failf("Invalid task ID: %v\n", err)
				return
			}
			index, err := strconv.Atoi(args[1])
			if err != nil || index < 1 {
				failf("Invalid check index: %s (must be 1 or greater)\n", args[1])
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

			newBody, removedText, err := removeAcceptanceItem(t.Body, index)
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

			fmt.Printf("Removed acceptance criterion %d from task %d: %s\n", index, id, removedText)
		},
	}
}

// append a checkbox to ## Acceptance, creating the section if needed.
// A new section lands before ## Notes and ## Log so criteria stay with
// the task description, mirroring addNote's placement logic.
func addAcceptanceItem(body, text string) string {
	entry := fmt.Sprintf("- [ ] %s", text)

	before, section, after, found := splitSection(body, "## Acceptance")
	if found {
		section = strings.TrimRight(section, "\n") + "\n" + entry
		return joinBlocks(before, section, after)
	}

	newSection := "## Acceptance\n\n" + entry

	preBody, logSection, afterLog, logFound := splitSection(body, "## Log")
	if logFound {
		preBody = joinBlocks(preBody, afterLog)
	}

	beforeNotes, notesSection, afterNotes, notesFound := splitSection(preBody, "## Notes")
	var newPreBody string
	if notesFound {
		newPreBody = joinBlocks(beforeNotes, newSection, notesSection, afterNotes)
	} else {
		newPreBody = joinBlocks(preBody, newSection)
	}
	if logFound {
		return joinBlocks(newPreBody, logSection)
	}
	return newPreBody
}

// remove the nth checkbox from ## Acceptance.
// Returns the new body, the removed criterion text, and any error.
func removeAcceptanceItem(body string, targetIndex int) (string, string, error) {
	before, section, after, found := splitSection(body, "## Acceptance")
	if !found {
		return "", "", fmt.Errorf("no ## Acceptance section found")
	}

	lines := strings.Split(section, "\n")
	checkIndex := 0
	removedText := ""
	removed := false

	for i, line := range lines {
		match := acceptanceCheckboxRe.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		checkIndex++
		if checkIndex != targetIndex {
			continue
		}
		removedText = match[2]
		lines = append(lines[:i], lines[i+1:]...)
		removed = true
		break
	}

	if !removed {
		return "", "", fmt.Errorf("check index %d not found (found %d items)", targetIndex, checkIndex)
	}

	newSection := strings.Join(lines, "\n")
	newBody := joinBlocks(before, newSection, after)
	return newBody, removedText, nil
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
				failf("Invalid task ID: %v\n", err)
				return
			}
			index, err := strconv.Atoi(args[1])
			if err != nil || index < 1 {
				failf("Invalid check index: %s (must be 1 or greater)\n", args[1])
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

			newBody, nowChecked, err := toggleAcceptanceCheck(t.Body, index)
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
