package cmd

import (
	"fmt"
	"punchlist/task"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Sections with dedicated commands own their structure and must not be
// rewritten as free prose: Acceptance holds parseAcceptance checkboxes
// (pin acceptance add/rm, pin check), and Notes/Log are append-only
// timestamped journals (pin note / the addLog path). `pin section` edits
// prose sections only — Problem, Approach, or any custom heading.
var reservedSections = map[string]string{
	"## Acceptance": "use `pin acceptance add/rm` and `pin check`",
	"## Notes":      "use `pin note`",
	"## Log":        "use `pin note` (the Log is append-only)",
}

// normalizeHeading turns "Problem" or "## Problem" into the canonical
// "## Problem" form so callers can pass either.
func normalizeHeading(h string) string {
	h = strings.TrimSpace(h)
	h = strings.TrimLeft(h, "#")
	h = strings.TrimSpace(h)
	return "## " + h
}

// create the section command (with set/add subcommands)
func newSectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "section",
		Short: "Set or append prose in a task section (Problem, Approach, or any custom heading)",
		Long: "Edit the free-prose body of a task section without opening an editor.\n" +
			"Use for Problem/Approach or any custom section. Acceptance, Notes, and\n" +
			"Log have dedicated commands (pin acceptance, pin note) and are rejected here.",
	}
	cmd.AddCommand(newSectionSetCmd())
	cmd.AddCommand(newSectionAddCmd())
	return cmd
}

func newSectionSetCmd() *cobra.Command {
	return newSectionTextCmd("set [id] [heading] [text]",
		"Replace a section's body (creates the section if missing)", nil, false)
}

func newSectionAddCmd() *cobra.Command {
	return newSectionTextCmd("add [id] [heading] [text]",
		"Append a paragraph to a section's body (creates the section if missing)", nil, true)
}

// newSectionTextCmd builds a command whose last positional argument is the
// section text, or which reads that text from --file (a path, or - for
// stdin) so long prose needs no shell quoting. heading is fixed for the
// problem/approach shortcuts and nil for `pin section set/add`.
func newSectionTextCmd(use, short string, heading *string, appendMode bool) *cobra.Command {
	var file string
	fixed := 2 // id, heading
	if heading != nil {
		fixed = 1 // id
	}
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args: func(cmd *cobra.Command, args []string) error {
			if file != "" {
				return cobra.ExactArgs(fixed)(cmd, args)
			}
			return cobra.ExactArgs(fixed+1)(cmd, args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			var text string
			if file != "" {
				t, err := readTextSource(file)
				if err != nil {
					failf("--file: %v\n", err)
					return
				}
				text = t
			} else {
				text = args[fixed]
			}
			h := ""
			if heading != nil {
				h = *heading
			} else {
				h = args[1]
			}
			runSection([]string{args[0], h, text}, appendMode)
		},
	}
	c.Flags().StringVar(&file, "file", "", "read the section text from a file (- for stdin) instead of an argument")
	return c
}

// newProblemCmd / newApproachCmd are shortcuts for the two --ticket
// scaffold prose sections, mirroring how `pin note` fronts the Log.
func newProblemCmd() *cobra.Command {
	h := "Problem"
	return newSectionTextCmd("problem [id] [text]",
		"Set the ## Problem section (shortcut for `pin section set <id> Problem`)", &h, false)
}

func newApproachCmd() *cobra.Command {
	h := "Approach"
	return newSectionTextCmd("approach [id] [text]",
		"Set the ## Approach section (shortcut for `pin section set <id> Approach`)", &h, false)
}

func runSection(args []string, appendMode bool) {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		failf("Invalid task ID: %v\n", err)
		return
	}
	heading := normalizeHeading(args[1])
	text := strings.TrimSpace(args[2])
	if text == "" {
		failf("Section text must not be empty\n")
		return
	}
	if hint, reserved := reservedSections[heading]; reserved {
		failf("%s is managed; %s\n", heading, hint)
		return
	}
	if err := checkSectionProse(heading, text); err != nil {
		failf("%v\n", err)
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

	t.Body, err = setSectionBody(t.Body, heading, text, appendMode)
	if err != nil {
		failf("Error: %v\n", err)
		return
	}
	t.UpdatedAt = time.Now()
	if err := t.Write(taskPath); err != nil {
		failf("Error updating task: %v\n", err)
		return
	}

	verb := "Set"
	if appendMode {
		verb = "Appended to"
	}
	fmt.Printf("%s %s on task %d\n", verb, heading, id)
}

// setSectionBody replaces (or, in appendMode, extends) the prose body of
// `heading`. A missing section is created and placed above the structured
// trailing sections (Acceptance, Notes, Log) so Problem/Approach stay with
// the task description, matching the --ticket scaffold order.
func setSectionBody(body, heading, text string, appendMode bool) (string, error) {
	before, section, after, found := splitSection(body, heading)
	if found {
		var newSection string
		if appendMode {
			// strip the heading line, keep existing prose, append a paragraph
			existing := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(section), heading))
			if existing == "" || isScaffoldPlaceholder(existing) {
				// a --ticket placeholder is not authored text: appending replaces it
				newSection = heading + "\n\n" + text
			} else {
				newSection = heading + "\n\n" + existing + "\n\n" + text
			}
		} else {
			newSection = heading + "\n\n" + text
		}
		return joinBlocks(before, newSection, after), nil
	}

	newSection := heading + "\n\n" + text
	return insertSectionBefore(body, newSection, []string{"## Acceptance", "## Notes", "## Log"}), nil
}

// insertSectionBefore places newSection immediately before the earliest of
// the given anchor headings that exists in body; if none exist it appends.
func insertSectionBefore(body, newSection string, anchors []string) string {
	insertAt := -1
	for _, a := range anchors {
		if idx := indexOfHeadingLine(body, a); idx != -1 {
			if insertAt == -1 || idx < insertAt {
				insertAt = idx
			}
		}
	}
	if insertAt == -1 {
		return joinBlocks(body, newSection)
	}
	return joinBlocks(body[:insertAt], newSection, body[insertAt:])
}
