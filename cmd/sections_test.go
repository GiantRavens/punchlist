package cmd

import (
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
