package cmd

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Filling a ticket at creation (pin #21).
//
// `pin todo --ticket` seeds Problem/Approach/Acceptance with placeholders, and
// the CLI-only rule forbids writing the task file directly, so an agent that
// cannot drive $EDITOR had no legal way to author the sections it was told to
// fill. These flags fill them in the same command that creates the task:
//
//	pin todo --ticket "title" --problem "..." --approach "..." \
//	    --acceptance "first criterion" --acceptance "second criterion"
//	pin todo "title" --body-file ticket.md      # or --body-file - for stdin
//
// Everything here is opt-in. A create command with none of these flags takes
// exactly the pre-1.3.9 path and writes a byte-identical file, so pin used as
// a plain todo list is unaffected.

// createFlagsHelp is shown by `pin todo --help` (state commands disable
// cobra's flag parsing, so cobra cannot list these itself).
const createFlagsHelp = `Creating a task:
  pin todo "title" [pri:N] [tags:{a,b}] [by:DATE] [depends:1,2]

Tickets (all optional; a plain title stays a one-line todo):
  --ticket               seed ## Problem, ## Approach and ## Acceptance
  --problem TEXT         fill ## Problem (implies --ticket)
  --approach TEXT        fill ## Approach (implies --ticket)
  --acceptance TEXT      add a criterion; repeat for more (implies --ticket)
  --body-file PATH|-     body as markdown from a file or stdin: prose, then
                         ## Problem, ## Approach, ## Acceptance (list items)

Example:
  pin todo --ticket "Fix the sweep" pri:1 --problem "..." --approach "..." \
    --acceptance "a failing drop exits non-zero"`

// createFlags are the creation-time options parsed out of the free-form
// argument list (state-creation commands disable cobra's flag parsing so that
// titles may begin with "--"; see newTodoCmd).
type createFlags struct {
	ticket     bool
	problem    string
	approach   string
	acceptance []string
	bodyFile   string
	bodyText   string // contents of bodyFile, read before the write lock is taken
}

// fills reports whether any section content was supplied.
func (f createFlags) fills() bool {
	return f.problem != "" || f.approach != "" || len(f.acceptance) > 0 || f.bodyText != ""
}

// stdinReader is where `-` reads from; tests replace it.
var stdinReader io.Reader = os.Stdin

// preloaded holds sources read before the write lock was taken, by path
// ("-" for stdin), so the command body never blocks on I/O under the lock.
var preloaded = map[string]string{}

// preloadTextSources reads the --body-file of a create, or the --file of a
// section command, ahead of the lock. Errors surface here, before any lock.
func preloadTextSources(cmd *cobra.Command, args []string) error {
	var path string
	if f := cmd.Flags().Lookup("file"); f != nil {
		path = f.Value.String()
	} else if _, flags, err := parseCreateFlags(args); err == nil {
		path = flags.bodyFile
	}
	if path == "" {
		return nil
	}
	if _, done := preloaded[path]; done {
		return nil
	}
	text, err := readSource(path)
	if err != nil {
		return err
	}
	preloaded[path] = text
	return nil
}

// readTextSource returns a preloaded source, or reads the file (or stdin for
// "-") now when no hook preloaded it (tests call commands directly).
func readTextSource(path string) (string, error) {
	if text, ok := preloaded[path]; ok {
		delete(preloaded, path)
		return text, nil
	}
	return readSource(path)
}

func readSource(path string) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(stdinReader)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseCreateFlags pulls --ticket, --problem, --approach, --acceptance
// (repeatable) and --body-file out of a free-form create argument list. Value
// flags accept "--flag value" or "--flag=value". Only exact flag tokens are
// consumed, so an ordinary title is never mistaken for a flag. Any of
// --problem, --approach or --acceptance implies --ticket; --body-file alone
// does not (a body may be plain prose).
func parseCreateFlags(args []string) ([]string, createFlags, error) {
	var f createFlags
	out := args[:0:0]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--ticket" {
			f.ticket = true
			continue
		}
		name, val, hasEq := strings.Cut(a, "=")
		switch name {
		case "--problem", "--approach", "--acceptance", "--body-file":
		default:
			out = append(out, a)
			continue
		}
		if !hasEq {
			if i+1 >= len(args) {
				return nil, f, fmt.Errorf("%s needs a value", name)
			}
			i++
			val = args[i]
		}
		if strings.TrimSpace(val) == "" {
			return nil, f, fmt.Errorf("%s must not be empty", name)
		}
		switch name {
		case "--problem":
			f.problem = val
		case "--approach":
			f.approach = val
		case "--acceptance":
			if err := checkCriterion(val); err != nil {
				return nil, f, err
			}
			f.acceptance = append(f.acceptance, strings.TrimSpace(val))
		case "--body-file":
			f.bodyFile = val
		}
	}
	if f.problem != "" || f.approach != "" || len(f.acceptance) > 0 {
		f.ticket = true
	}
	return out, f, nil
}

// bodySection is one "## Heading" block of a --body-file.
type bodySection struct {
	heading string
	text    string
}

// canonicalHeadings fixes the case of the headings pin itself manages, so a
// body that writes "## problem" fills the scaffold's ## Problem instead of
// adding a second one.
var canonicalHeadings = map[string]string{
	"## problem": "## Problem", "## approach": "## Approach", "## acceptance": "## Acceptance",
	"## notes": "## Notes", "## log": "## Log",
}

func canonicalHeading(line string) string {
	h := normalizeHeading(line)
	if c, ok := canonicalHeadings[strings.ToLower(h)]; ok {
		return c
	}
	return h
}

// parseBodyText splits a markdown body into the prose before its first
// "## " heading and its "## " sections. A "## " or "# " line counts as a
// heading even inside a code fence, because pin's section reader (splitSection)
// does not track fences either: accepting it here would corrupt the task when
// it is written. Indent such a line in code samples. A "# " title line is
// refused (the title comes from the command), as are ## Notes and ## Log
// (append-only journals pin writes itself), a heading given twice, and an
// empty section.
func parseBodyText(text string) (string, []bodySection, error) {
	var (
		preamble []string
		sections []bodySection
		cur      *bodySection
		lines    []string
		seen     = map[string]bool{}
	)
	flush := func() {
		if cur != nil {
			cur.text = strings.TrimSpace(strings.Join(lines, "\n"))
			sections = append(sections, *cur)
		} else {
			preamble = lines
		}
		lines = nil
	}
	text = strings.TrimPrefix(text, "\ufeff")
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "# ") {
			return "", nil, fmt.Errorf("the body must not contain a # title line (%q); the title comes from the command", strings.TrimSpace(line))
		}
		if strings.HasPrefix(line, "## ") {
			heading := canonicalHeading(line)
			if heading == "## Notes" || heading == "## Log" {
				return "", nil, fmt.Errorf("the body must not contain %s; it is append-only (use `pin note` after creating the task)", heading)
			}
			if seen[heading] {
				return "", nil, fmt.Errorf("the body gives %s twice", heading)
			}
			seen[heading] = true
			flush()
			cur = &bodySection{heading: heading}
			continue
		}
		lines = append(lines, line)
	}
	flush()
	for _, s := range sections {
		if s.text == "" {
			return "", nil, fmt.Errorf("%s in the body is empty", s.heading)
		}
	}
	return strings.TrimSpace(strings.Join(preamble, "\n")), sections, nil
}

// acceptanceListItem matches a markdown list item, with or without a checkbox.
var acceptanceListItem = regexp.MustCompile(`^[-*] (?:\[([ xX])\] )?(.+)$`)

// acceptanceLines turns the body of an ## Acceptance section into checkbox
// lines. Plain list items become unchecked criteria and a checked box stays
// checked; any other non-blank line is an error, because a criterion has to
// be a checkbox for `pin acceptance` and `pin check` to see it.
func acceptanceLines(text string) ([]string, error) {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if line != strings.TrimLeft(line, " \t") {
			return nil, fmt.Errorf("## Acceptance takes a flat list; nested or indented item %q is not supported (each criterion is one checkbox)", trimmed)
		}
		m := acceptanceListItem.FindStringSubmatch(trimmed)
		if m == nil {
			return nil, fmt.Errorf("## Acceptance must be a list of \"- \" items; got %q", trimmed)
		}
		mark := " "
		if m[1] == "x" || m[1] == "X" {
			mark = "x"
		}
		out = append(out, fmt.Sprintf("- [%s] %s", mark, strings.TrimSpace(m[2])))
	}
	return out, nil
}

// checkSectionProse refuses section text that would break the task's
// structure: a "#" or "## " line would start a new section in the middle of
// this one. Fences do not protect it, because the section reader does not
// track them; indent the line inside a code sample instead.
func checkSectionProse(heading, text string) error {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			return fmt.Errorf("text for %s must not contain a heading line (%q); use `pin section set` for another section, or indent the line in a code sample", heading, strings.TrimSpace(line))
		}
	}
	return nil
}

// checkCriterion refuses an acceptance criterion that is not one line: a
// criterion is one checkbox, and a newline would let it write headings or
// list items of its own into the task.
func checkCriterion(text string) error {
	if strings.ContainsAny(text, "\r\n") {
		return fmt.Errorf("an acceptance criterion must be one line; got %q", text)
	}
	return nil
}

// setAcceptanceItems replaces the ## Acceptance section with the given
// checkbox lines, or creates it above ## Notes / ## Log.
func setAcceptanceItems(body string, items []string) string {
	section := "## Acceptance\n\n" + strings.Join(items, "\n")
	before, _, after, found := splitSection(body, "## Acceptance")
	if found {
		return joinBlocks(before, section, after)
	}
	return insertSectionBefore(body, section, []string{"## Notes", "## Log"})
}

// composeTaskBody builds a new task's body. With no fill flags it is exactly
// the body pin has always written: the h1, plus the scaffold for --ticket.
func composeTaskBody(fullTitle string, f createFlags) (string, error) {
	if !f.fills() {
		if f.ticket {
			return fmt.Sprintf("# %s\n\n%s\n", fullTitle, ticketScaffold), nil
		}
		return fmt.Sprintf("# %s\n", fullTitle), nil
	}

	preamble, sections, err := parseBodyText(f.bodyText)
	if err != nil {
		return "", err
	}
	blocks := []string{"# " + fullTitle, preamble}
	if f.ticket {
		blocks = append(blocks, ticketScaffold)
	}
	body := joinBlocks(blocks...)

	var criteria []string
	for _, s := range sections {
		if s.heading == "## Acceptance" {
			lines, err := acceptanceLines(s.text)
			if err != nil {
				return "", err
			}
			criteria = append(criteria, lines...)
			continue
		}
		if body, err = setSectionBody(body, s.heading, s.text, false); err != nil {
			return "", err
		}
	}
	for _, s := range []struct{ heading, text string }{{"## Problem", f.problem}, {"## Approach", f.approach}} {
		if s.text == "" {
			continue
		}
		text := strings.TrimSpace(s.text)
		if err := checkSectionProse(s.heading, text); err != nil {
			return "", err
		}
		if body, err = setSectionBody(body, s.heading, text, false); err != nil {
			return "", err
		}
	}
	for _, c := range f.acceptance {
		criteria = append(criteria, "- [ ] "+c)
	}
	if len(criteria) > 0 {
		body = setAcceptanceItems(body, criteria)
	}
	return strings.TrimRight(body, "\n") + "\n", nil
}

// scaffoldPlaceholders are the exact seeded --ticket hints, in the current
// wording and the pre-1.3.6 one. Replacement matches them whole, never by
// substring, so authored text that mentions "fill me in" is never deleted.
var scaffoldPlaceholders = func() map[string]bool {
	set := map[string]bool{
		"(what's wrong + root cause + evidence — fill me in)":                       true,
		"(the fix — fill me in)":                                                    true,
		"(testable post-condition that proves the fix durably landed — fill me in)": true,
	}
	for _, line := range strings.Split(ticketScaffold, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- [ ]"))
		if strings.HasPrefix(line, "(") && strings.HasSuffix(line, ")") {
			set[line] = true
		}
	}
	return set
}()

// isScaffoldPlaceholder reports whether text is exactly a seeded --ticket
// hint rather than authored content.
func isScaffoldPlaceholder(text string) bool {
	return scaffoldPlaceholders[strings.TrimSpace(text)]
}
