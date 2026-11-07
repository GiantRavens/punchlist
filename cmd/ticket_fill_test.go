package cmd

import (
	"os"
	"path/filepath"
	"punchlist/task"
	"strings"
	"testing"
)

// The tests below are the acceptance criteria of pin #21, one per test where
// the criterion is a behavior.

func readOnlyTask(t *testing.T) *task.Task {
	t.Helper()
	tasksPath, err := tasksDir()
	if err != nil {
		t.Fatalf("tasks dir: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(tasksPath, "*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected exactly one task file, found %d: %v", len(matches), matches)
	}
	tk, err := task.Parse(matches[0])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tk
}

func initStore(t *testing.T) func() {
	t.Helper()
	teardown := setupTest(t)
	if _, err := executeCommand("init"); err != nil {
		teardown()
		t.Fatalf("init failed: %v", err)
	}
	exitCode = 0
	return func() { exitCode = 0; teardown() }
}

// criterion: --problem, --approach and two --acceptance flags fill all three
// sections, leaving no placeholder for doctor's scaffold_unfilled to find.
func TestTodoTicketFillFlags(t *testing.T) {
	defer initStore(t)()
	_, err := executeCommand("todo", "--ticket", "Fix the sensor", "pri:2", "tags:{bug}",
		"--problem", "The sensor reads stale data.",
		"--approach=Refresh the cache before each read.",
		"--acceptance", "the sensor reports fresh values",
		"--acceptance", "a regression test covers the cache")
	if err != nil || exitCode != 0 {
		t.Fatalf("create failed: err=%v exit=%d", err, exitCode)
	}
	tk := readOnlyTask(t)
	if tk.Title != "Fix the sensor" || tk.Priority != 2 || !hasTag(tk.Tags, "bug") {
		t.Errorf("modifiers lost: title=%q pri=%d tags=%v", tk.Title, tk.Priority, tk.Tags)
	}
	for _, want := range []string{"## Problem\n\nThe sensor reads stale data.", "## Approach\n\nRefresh the cache before each read."} {
		if !strings.Contains(tk.Body, want) {
			t.Errorf("body missing %q:\n%s", want, tk.Body)
		}
	}
	items := parseAcceptance(tk.Body)
	if len(items) != 2 || items[0].Text != "the sensor reports fresh values" || items[1].Text != "a regression test covers the cache" {
		t.Errorf("acceptance = %+v", items)
	}
	if u := scaffoldUnfilledSections(tk.Body); len(u) != 0 {
		t.Errorf("placeholders left in %v:\n%s", u, tk.Body)
	}
	if !strings.Contains(tk.Body, "## Log") {
		t.Errorf("Log missing:\n%s", tk.Body)
	}
}

// a fill flag implies --ticket
func TestFillFlagImpliesTicket(t *testing.T) {
	defer initStore(t)()
	if _, err := executeCommand("todo", "Only a problem", "--problem", "It breaks."); err != nil {
		t.Fatal(err)
	}
	tk := readOnlyTask(t)
	for _, h := range []string{"## Problem\n\nIt breaks.", "## Approach", "## Acceptance"} {
		if !strings.Contains(tk.Body, h) {
			t.Errorf("missing %q:\n%s", h, tk.Body)
		}
	}
	if u := scaffoldUnfilledSections(tk.Body); len(u) != 2 {
		t.Errorf("Approach and Acceptance should still be placeholders, got %v", u)
	}
}

// criterion: --body-file - reads stdin, and shell-hostile prose lands verbatim.
func TestTodoBodyFileFromStdin(t *testing.T) {
	defer initStore(t)()
	body := "Context before the sections.\n\n" +
		"## Problem\n\nIt prints \"$HOME\" and `rm -rf` and 'quotes' verbatim.\n\n" +
		"A sample, indented:\n\n    ## not a heading when indented\n\n" +
		"## Approach\n\nTwo paragraphs.\n\nSecond one.\n\n" +
		"## Acceptance\n\n- first\n- [x] already done\n* third\n"
	old := stdinReader
	stdinReader = strings.NewReader(body)
	defer func() { stdinReader = old }()

	if _, err := executeCommand("todo", "Body from stdin", "--ticket", "--body-file", "-"); err != nil || exitCode != 0 {
		t.Fatalf("create failed: err=%v exit=%d", err, exitCode)
	}
	tk := readOnlyTask(t)
	for _, want := range []string{
		"# Body from stdin\n\nContext before the sections.\n\n## Problem",
		"It prints \"$HOME\" and `rm -rf` and 'quotes' verbatim.",
		"    ## not a heading when indented",
		"## Approach\n\nTwo paragraphs.\n\nSecond one.",
	} {
		if !strings.Contains(tk.Body, want) {
			t.Errorf("body missing %q:\n%s", want, tk.Body)
		}
	}
	items := parseAcceptance(tk.Body)
	if len(items) != 3 || items[0].Checked || !items[1].Checked || items[2].Text != "third" {
		t.Errorf("acceptance = %+v", items)
	}
	if u := scaffoldUnfilledSections(tk.Body); len(u) != 0 {
		t.Errorf("placeholders left: %v\n%s", u, tk.Body)
	}
}

// --body-file from a path, without --ticket: plain prose, no scaffold.
func TestTodoBodyFilePlainProse(t *testing.T) {
	defer initStore(t)()
	f := filepath.Join(t.TempDir(), "b.md")
	os.WriteFile(f, []byte("Just a description.\n"), 0o644)
	if _, err := executeCommand("todo", "Prose only", "--body-file", f); err != nil {
		t.Fatal(err)
	}
	tk := readOnlyTask(t)
	if !strings.Contains(tk.Body, "# Prose only\n\nJust a description.") || strings.Contains(tk.Body, "## Problem") {
		t.Errorf("unexpected body:\n%s", tk.Body)
	}
}

// criterion: a body holding ## Notes or ## Log is refused and no task is made.
func TestTodoBodyFileRefusesJournals(t *testing.T) {
	for _, bad := range []string{
		"## Problem\n\nx\n\n## Notes\n\n- a note\n",
		"## Log\n\n- forged\n",
		"# A second title\n",
		"## Problem\n\nx\n\n## Problem\n\ny\n",
		"## Acceptance\n\nnot a list item\n",
	} {
		func() {
			defer initStore(t)()
			f := filepath.Join(t.TempDir(), "b.md")
			os.WriteFile(f, []byte(bad), 0o644)
			executeCommand("todo", "--ticket", "Refused", "--body-file", f)
			if exitCode != 1 {
				t.Errorf("expected failure for body %q, exit=%d", bad, exitCode)
			}
			tasksPath, _ := tasksDir()
			if m, _ := filepath.Glob(filepath.Join(tasksPath, "*.md")); len(m) != 0 {
				t.Errorf("a refused body still created %v", m)
			}
		}()
	}
}

// a flag with no value, or an empty value, is an error and creates nothing
func TestParseCreateFlags(t *testing.T) {
	rest, f, err := parseCreateFlags([]string{"todo", "a title with --problem inside", "--problem=p", "pri:1", "--acceptance", "c1", "--acceptance=c2"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rest, "|") != "todo|a title with --problem inside|pri:1" {
		t.Errorf("rest = %v", rest)
	}
	if !f.ticket || f.problem != "p" || len(f.acceptance) != 2 {
		t.Errorf("flags = %+v", f)
	}
	for _, bad := range [][]string{{"t", "--problem"}, {"t", "--acceptance", "  "}, {"t", "--body-file="}} {
		if _, _, err := parseCreateFlags(bad); err == nil {
			t.Errorf("expected an error for %v", bad)
		}
	}
	// --body-file alone does not imply --ticket
	if _, f, _ := parseCreateFlags([]string{"t", "--body-file", "x.md"}); f.ticket {
		t.Errorf("--body-file must not imply --ticket")
	}
}

// criterion: pin problem --file reads a file, and --file - reads stdin.
func TestSectionFromFile(t *testing.T) {
	defer initStore(t)()
	if _, err := executeCommand("todo", "--ticket", "File sections"); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "p.md")
	os.WriteFile(f, []byte("Problem from a file, with \"quotes\".\n\nSecond paragraph.\n"), 0o644)
	if _, err := executeCommand("problem", "1", "--file", f); err != nil || exitCode != 0 {
		t.Fatalf("problem --file failed: err=%v exit=%d", err, exitCode)
	}
	old := stdinReader
	stdinReader = strings.NewReader("Approach from stdin.\n")
	defer func() { stdinReader = old }()
	if _, err := executeCommand("approach", "1", "--file", "-"); err != nil || exitCode != 0 {
		t.Fatalf("approach --file - failed: err=%v exit=%d", err, exitCode)
	}
	if _, err := executeCommand("section", "set", "1", "Risks", "--file", f); err != nil || exitCode != 0 {
		t.Fatalf("section set --file failed: err=%v exit=%d", err, exitCode)
	}
	tk := readOnlyTask(t)
	for _, want := range []string{
		"## Problem\n\nProblem from a file, with \"quotes\".\n\nSecond paragraph.",
		"## Approach\n\nApproach from stdin.",
		"## Risks\n\nProblem from a file",
	} {
		if !strings.Contains(tk.Body, want) {
			t.Errorf("body missing %q:\n%s", want, tk.Body)
		}
	}
	// the text argument and --file together, or neither, is a usage error
	if _, err := executeCommand("problem", "1", "text", "--file", f); err == nil {
		t.Errorf("expected an error for both text and --file")
	}
}

// section text may not smuggle in a heading that would split the section
func TestSectionTextRefusesHeadings(t *testing.T) {
	defer initStore(t)()
	executeCommand("todo", "--ticket", "Guarded")
	executeCommand("problem", "1", "fine\n\n## Approach\n\nhijack")
	if exitCode != 1 {
		t.Errorf("expected a heading inside section text to be refused")
	}
	tk := readOnlyTask(t)
	if strings.Count(tk.Body, "## Approach") != 1 {
		t.Errorf("structure changed:\n%s", tk.Body)
	}
}

// criterion: acceptance add on a fresh scaffold leaves exactly the new criterion.
func TestAcceptanceAddReplacesPlaceholder(t *testing.T) {
	defer initStore(t)()
	executeCommand("todo", "--ticket", "Placeholder")
	if _, err := executeCommand("acceptance", "add", "1", "the real criterion"); err != nil {
		t.Fatal(err)
	}
	items := parseAcceptance(readOnlyTask(t).Body)
	if len(items) != 1 || items[0].Text != "the real criterion" {
		t.Errorf("acceptance = %+v", items)
	}
	executeCommand("acceptance", "add", "1", "a second one")
	if items := parseAcceptance(readOnlyTask(t).Body); len(items) != 2 {
		t.Errorf("second add should append, got %+v", items)
	}
}

// section add on a placeholder replaces it rather than appending after it
func TestSectionAddReplacesPlaceholder(t *testing.T) {
	defer initStore(t)()
	executeCommand("todo", "--ticket", "Append")
	executeCommand("section", "add", "1", "Problem", "authored text")
	body := readOnlyTask(t).Body
	if strings.Contains(body, "fill with") && strings.Contains(strings.SplitN(body, "## Approach", 2)[0], "fill with") {
		t.Errorf("placeholder kept in Problem:\n%s", body)
	}
}

// criterion: a plain create writes the same body as before (lightweight use
// unchanged): the h1 and the Log, nothing else.
func TestPlainTodoUnchanged(t *testing.T) {
	defer initStore(t)()
	if _, err := executeCommand("todo", "Buy milk", "pri:2"); err != nil {
		t.Fatal(err)
	}
	tk := readOnlyTask(t)
	headings := []string{}
	for _, line := range strings.Split(tk.Body, "\n") {
		if strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}
	if strings.Join(headings, "|") != "# Buy milk|## Log" {
		t.Errorf("plain task headings = %v\n%s", headings, tk.Body)
	}
	if !strings.HasPrefix(tk.Body, "# Buy milk\n\n## Log\n") {
		t.Errorf("plain task body changed:\n%q", tk.Body)
	}
	// and composeTaskBody with no flags is byte-identical to the old formats
	if b, _ := composeTaskBody("T", createFlags{}); b != "# T\n" {
		t.Errorf("plain body = %q", b)
	}
	if b, _ := composeTaskBody("T", createFlags{ticket: true}); b != "# T\n\n"+ticketScaffold+"\n" {
		t.Errorf("ticket body = %q", b)
	}
}

// flag values that look like ids (-, a bare number) must not turn a create
// into a state change, and a malformed flag reports the flag, not "invalid id"
func TestCreateRoutingIgnoresFlagValues(t *testing.T) {
	defer initStore(t)()
	if _, err := executeCommand("todo", "Numbered criterion", "--acceptance", "5"); err != nil || exitCode != 0 {
		t.Fatalf("create failed: err=%v exit=%d", err, exitCode)
	}
	if items := parseAcceptance(readOnlyTask(t).Body); len(items) != 1 || items[0].Text != "5" {
		t.Errorf("acceptance = %+v", items)
	}
	if !shouldCreateStateFromArgs([]string{"A title", "--problem"}) {
		t.Errorf("a malformed creation flag should route to creation")
	}
	if shouldCreateStateFromArgs([]string{"12", "14"}) {
		t.Errorf("plain ids must still route to a state change")
	}
}

// --- regressions from the 2026-11-07 skeptic review ---

// authored text that merely mentions the placeholder words is never deleted
func TestPlaceholderMatchIsExact(t *testing.T) {
	defer initStore(t)()
	executeCommand("todo", "a", "--problem", `The form says "fill me in" when empty`,
		"--acceptance", "docs say add with `pin acceptance add` works")
	executeCommand("section", "add", "1", "Problem", "second paragraph")
	executeCommand("acceptance", "add", "1", "second criterion")
	tk := readOnlyTask(t)
	if !strings.Contains(tk.Body, `The form says "fill me in" when empty`) || !strings.Contains(tk.Body, "second paragraph") {
		t.Errorf("authored Problem text lost:\n%s", tk.Body)
	}
	if items := parseAcceptance(tk.Body); len(items) != 2 {
		t.Errorf("authored criterion lost: %+v", items)
	}
	// both placeholder wordings are still recognized whole
	for _, ph := range []string{"(the fix — fill me in)", "(the fix — fill with `pin approach <id> \"...\"`)"} {
		if !isScaffoldPlaceholder(ph) {
			t.Errorf("placeholder not recognized: %q", ph)
		}
	}
}

// a criterion is one line; a newline could write headings into the task
func TestCriterionMustBeOneLine(t *testing.T) {
	defer initStore(t)()
	executeCommand("todo", "inj", "--acceptance", "ok\n## Notes\nfake note")
	if exitCode != 1 {
		t.Errorf("a multi-line --acceptance should be refused")
	}
	tasksPath, _ := tasksDir()
	if m, _ := filepath.Glob(filepath.Join(tasksPath, "*.md")); len(m) != 0 {
		t.Fatalf("a refused criterion still created %v", m)
	}
	exitCode = 0
	executeCommand("todo", "--ticket", "later")
	executeCommand("acceptance", "add", "1", "ok\n# H1")
	if exitCode != 1 {
		t.Errorf("a multi-line acceptance add should be refused")
	}
}

// fences do not protect a heading line, because the section reader ignores them
func TestFencedHeadingsRefused(t *testing.T) {
	for _, args := range [][]string{
		{"todo", "f1", "--problem", "see:\n```\n## Acceptance\n- [ ] fake\n```"},
		{"todo", "f2", "--problem", "```\n## Log\n```"},
	} {
		func() {
			defer initStore(t)()
			executeCommand(args...)
			if exitCode != 1 {
				t.Errorf("expected refusal for %q", args[3])
			}
		}()
	}
	defer initStore(t)()
	f := filepath.Join(t.TempDir(), "b.md")
	os.WriteFile(f, []byte("```\n## Problem\n```\n\n## Problem\n\nreal\n"), 0o644)
	executeCommand("todo", "--ticket", "f3", "--body-file", f)
	if exitCode != 1 {
		t.Errorf("a fenced heading in a body file should be refused")
	}
}

// nested items, lower-case headings and empty sections
func TestBodyEdgeCases(t *testing.T) {
	refuse := []string{
		"## Acceptance\n\n- parent\n  - child\n",
		"## Problem\n\n## Approach\n\nx\n",
	}
	for _, body := range refuse {
		func() {
			defer initStore(t)()
			f := filepath.Join(t.TempDir(), "b.md")
			os.WriteFile(f, []byte(body), 0o644)
			executeCommand("todo", "--ticket", "edge", "--body-file", f)
			if exitCode != 1 {
				t.Errorf("expected refusal for %q", body)
			}
		}()
	}
	defer initStore(t)()
	f := filepath.Join(t.TempDir(), "b.md")
	os.WriteFile(f, []byte("\ufeff## problem\r\n\r\nlower case, BOM and CRLF\r\n"), 0o644)
	executeCommand("todo", "--ticket", "case", "--body-file", f)
	tk := readOnlyTask(t)
	if strings.Count(tk.Body, "## Problem") != 1 || !strings.Contains(tk.Body, "lower case, BOM and CRLF") {
		t.Errorf("lower-case heading not merged into ## Problem:\n%s", tk.Body)
	}
}

// help is not a task and takes no lock; -h as a flag value is text
func TestHelpAndFlagValues(t *testing.T) {
	if !wantsHelp([]string{"--help"}) || wantsHelp([]string{"x", "--problem", "-h"}) {
		t.Errorf("wantsHelp misread its arguments")
	}
	defer initStore(t)()
	executeCommand("todo", "dash value", "--problem", "-h")
	if p := parseAcceptance(readOnlyTask(t).Body); len(p) != 1 { // scaffold placeholder only
		t.Errorf("unexpected acceptance %+v", p)
	}
	if !strings.Contains(readOnlyTask(t).Body, "## Problem\n\n-h") {
		t.Errorf("-h was not kept as Problem text:\n%s", readOnlyTask(t).Body)
	}
}

// a source read before the lock is consumed once by the command
func TestPreloadedSourceIsUsed(t *testing.T) {
	defer initStore(t)()
	old := stdinReader
	stdinReader = strings.NewReader("## Problem\n\nread before the lock\n")
	defer func() { stdinReader = old }()
	if _, err := executeCommand("todo", "--ticket", "pre", "--body-file", "-"); err != nil || exitCode != 0 {
		t.Fatalf("create failed: %v %d", err, exitCode)
	}
	if !strings.Contains(readOnlyTask(t).Body, "read before the lock") {
		t.Errorf("preloaded body not used")
	}
	if len(preloaded) != 0 {
		t.Errorf("preloaded source not consumed: %v", preloaded)
	}
}
