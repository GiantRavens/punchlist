package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitSection(t *testing.T) {
	t.Run("heading at start", func(t *testing.T) {
		body := "## Notes\n\n- first note\n\n## Log\n\n- log entry"
		before, section, after, found := splitSection(body, "## Notes")
		if !found {
			t.Fatal("expected found=true")
		}
		if before != "" {
			t.Errorf("expected empty before, got %q", before)
		}
		if !strings.HasPrefix(section, "## Notes") {
			t.Errorf("expected section to start with heading, got %q", section)
		}
		if !strings.HasPrefix(after, "\n## Log") {
			t.Errorf("expected after to start with next heading, got %q", after)
		}
	})

	t.Run("heading in middle", func(t *testing.T) {
		body := "# Title\n\nSome text.\n\n## Notes\n\n- a note\n\n## Log\n\n- a log"
		before, section, after, found := splitSection(body, "## Notes")
		if !found {
			t.Fatal("expected found=true")
		}
		if !strings.Contains(before, "Some text.") {
			t.Errorf("expected before to contain body text, got %q", before)
		}
		if !strings.HasPrefix(section, "## Notes") {
			t.Errorf("expected section to start with heading, got %q", section)
		}
		if !strings.Contains(section, "a note") {
			t.Errorf("expected section to contain note content, got %q", section)
		}
		if !strings.Contains(after, "Log") {
			t.Errorf("expected after to contain log heading, got %q", after)
		}
	})

	t.Run("heading at end with no next section", func(t *testing.T) {
		body := "# Title\n\n## Log\n\n- entry one"
		before, section, after, found := splitSection(body, "## Log")
		if !found {
			t.Fatal("expected found=true")
		}
		if !strings.Contains(before, "Title") {
			t.Errorf("expected before to contain title, got %q", before)
		}
		if !strings.Contains(section, "entry one") {
			t.Errorf("expected section to contain entry, got %q", section)
		}
		if after != "" {
			t.Errorf("expected empty after, got %q", after)
		}
	})

	t.Run("heading not found", func(t *testing.T) {
		body := "# Title\n\nJust a body."
		before, section, after, found := splitSection(body, "## Notes")
		if found {
			t.Fatal("expected found=false")
		}
		if before != body {
			t.Errorf("expected before to be entire body, got %q", before)
		}
		if section != "" {
			t.Errorf("expected empty section, got %q", section)
		}
		if after != "" {
			t.Errorf("expected empty after, got %q", after)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		before, section, after, found := splitSection("", "## Notes")
		if found {
			t.Fatal("expected found=false")
		}
		if before != "" || section != "" || after != "" {
			t.Errorf("expected all empty, got before=%q section=%q after=%q", before, section, after)
		}
	})

	// pin #28: prose that merely MENTIONS a heading must not split there.
	// Specimen: tasks/021 — an acceptance criterion naming "## Notes or
	// ## Log" mid-sentence was carved into fake sections by a later note.
	t.Run("heading text mid-line is not a section boundary", func(t *testing.T) {
		body := "# Title\n\n## Acceptance\n\n- [ ] must not touch ## Notes or ## Log sections\n\n## Log\n\n- 2026-01-01T00:00:00Z: Created task"
		before, section, after, found := splitSection(body, "## Notes")
		if found {
			t.Fatalf("expected mid-line mention not to match, got section=%q", section)
		}
		if before != body || section != "" || after != "" {
			t.Error("expected body returned untouched when heading only appears mid-line")
		}

		_, section, _, found = splitSection(body, "## Log")
		if !found {
			t.Fatal("expected the real ## Log line to be found")
		}
		if !strings.Contains(section, "Created task") {
			t.Errorf("expected the real Log section, got %q", section)
		}
	})

	t.Run("heading line with trailing text is not a boundary", func(t *testing.T) {
		body := "## Notes or otherwise\n\ncontent\n\n## Notes\n\n- real note"
		_, section, _, found := splitSection(body, "## Notes")
		if !found {
			t.Fatal("expected the exact heading line to be found")
		}
		if !strings.Contains(section, "real note") {
			t.Errorf("expected match on the exact heading line only, got %q", section)
		}
	})
}

// End-to-end regression for pin #28: the exact command sequence that
// produced specimen tasks/021 must leave section structure intact.
func TestAcceptanceCriterionNamingSectionsSurvivesNote(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := executeCommand("todo", "--ticket", "ticket under test"); err != nil {
		t.Fatalf("create: %v", err)
	}
	criterion := "pin section replaces one section without touching ## Notes or ## Log (append-only)"
	if _, err := executeCommand("acceptance", "add", "1", criterion); err != nil {
		t.Fatalf("acceptance add: %v", err)
	}
	if _, err := executeCommand("note", "1", "a note after the heading-naming criterion"); err != nil {
		t.Fatalf("note: %v", err)
	}

	tasksDir := "tasks"
	name := firstMarkdown(t, tasksDir)
	raw, err := os.ReadFile(filepath.Join(tasksDir, name))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, heading := range []string{"\n## Log", "\n## Notes", "\n## Acceptance"} {
		if got := strings.Count(body, heading+"\n"); got != 1 {
			t.Errorf("expected exactly one %q section, got %d\n%s", strings.TrimSpace(heading), got, body)
		}
	}
	if !strings.Contains(body, criterion) {
		t.Error("expected the criterion text preserved verbatim")
	}
}

func TestAppendEntry(t *testing.T) {
	t.Run("empty section", func(t *testing.T) {
		result := appendEntry("", "- new entry")
		if !strings.Contains(result, "- new entry") {
			t.Errorf("expected entry in result, got %q", result)
		}
	})

	t.Run("section with existing content", func(t *testing.T) {
		result := appendEntry("## Notes\n\n- existing", "- new entry")
		if !strings.Contains(result, "- existing") {
			t.Errorf("expected existing content preserved, got %q", result)
		}
		if !strings.Contains(result, "- new entry") {
			t.Errorf("expected new entry in result, got %q", result)
		}
	})

	t.Run("section with trailing newlines", func(t *testing.T) {
		result := appendEntry("## Notes\n\n- existing\n\n\n", "- new entry")
		if strings.Contains(result, "\n\n\n\n") {
			t.Errorf("expected trailing newlines to be trimmed, got %q", result)
		}
		if !strings.Contains(result, "- new entry") {
			t.Errorf("expected new entry in result, got %q", result)
		}
	})

	// 1.3.2 tight-list contract: consecutive entries share consecutive
	// lines; only the heading is separated by a blank line.
	t.Run("appends tight after an existing list item", func(t *testing.T) {
		result := appendEntry("## Log\n\n- first", "- second")
		want := "## Log\n\n- first\n- second\n\n"
		if result != want {
			t.Errorf("expected tight list emission %q, got %q", want, result)
		}
	})

	t.Run("first item after bare heading keeps blank separator", func(t *testing.T) {
		result := appendEntry("## Log", "- first")
		want := "## Log\n\n- first\n\n"
		if result != want {
			t.Errorf("expected blank line after heading %q, got %q", want, result)
		}
	})

	t.Run("appends tight to a legacy loose list", func(t *testing.T) {
		result := appendEntry("## Log\n\n- first\n\n- second", "- third")
		if !strings.HasSuffix(strings.TrimRight(result, "\n"), "- second\n- third") {
			t.Errorf("expected new entry tight against the last loose item, got %q", result)
		}
	})
}

func TestJoinBlocks(t *testing.T) {
	t.Run("no blocks", func(t *testing.T) {
		result := joinBlocks()
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("single block", func(t *testing.T) {
		result := joinBlocks("hello")
		if result != "hello" {
			t.Errorf("expected 'hello', got %q", result)
		}
	})

	t.Run("multiple blocks", func(t *testing.T) {
		result := joinBlocks("first", "second", "third")
		if result != "first\n\nsecond\n\nthird" {
			t.Errorf("expected blocks joined with blank lines, got %q", result)
		}
	})

	t.Run("empty blocks filtered", func(t *testing.T) {
		result := joinBlocks("first", "", "\n\n", "second")
		if result != "first\n\nsecond" {
			t.Errorf("expected empty blocks filtered, got %q", result)
		}
	})

	t.Run("blocks with internal newlines preserved", func(t *testing.T) {
		result := joinBlocks("line1\nline2", "line3\nline4")
		if !strings.Contains(result, "line1\nline2") {
			t.Errorf("expected internal newlines preserved, got %q", result)
		}
		if !strings.Contains(result, "\n\n") {
			t.Errorf("expected blank line separator, got %q", result)
		}
	})
}
