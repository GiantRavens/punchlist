package cmd

import (
	"strings"
	"testing"
)

const notesBody = `# t

## Notes

- 2026-01-01T00:00:00Z: first
- 2026-01-02T00:00:00Z: second
- 2026-01-03T00:00:00Z: third

## Log

- 2026-01-01T00:00:00Z: Created task`

func TestParseNotes(t *testing.T) {
	notes := parseNotes(notesBody)
	if len(notes) != 3 {
		t.Fatalf("parseNotes = %d items, want 3", len(notes))
	}
	if !strings.HasSuffix(notes[1], "second") {
		t.Errorf("note 2 = %q, want ...second", notes[1])
	}
}

func TestRemoveNoteItemMiddle(t *testing.T) {
	body, removed, err := removeNoteItem(notesBody, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(removed, "second") {
		t.Errorf("removed = %q, want ...second", removed)
	}
	notes := parseNotes(body)
	if len(notes) != 2 || strings.Contains(body, "second") {
		t.Errorf("second not removed:\n%s", body)
	}
	// The ## Log section must survive untouched.
	if !strings.Contains(body, "## Log") || !strings.Contains(body, "Created task") {
		t.Errorf("Log section damaged:\n%s", body)
	}
}

func TestRemoveNoteItemOutOfRange(t *testing.T) {
	if _, _, err := removeNoteItem(notesBody, 9); err == nil {
		t.Error("expected error for out-of-range index")
	}
}

func TestRemoveNoteItemNoSection(t *testing.T) {
	if _, _, err := removeNoteItem("# t\n\nno notes here", 1); err == nil {
		t.Error("expected error when no ## Notes section")
	}
}
