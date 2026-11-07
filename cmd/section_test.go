package cmd

import (
	"strings"
	"testing"
)

const sectionTicketBody = `# sample

## Problem

(what's wrong + root cause + evidence — fill me in)

## Approach

(the fix — fill me in)

## Acceptance

- [ ] (testable post-condition that proves the fix durably landed — fill me in)`

func TestNormalizeHeading(t *testing.T) {
	for _, in := range []string{"Problem", "## Problem", "  ##  Problem  ", "#Problem"} {
		if got := normalizeHeading(in); got != "## Problem" {
			t.Errorf("normalizeHeading(%q) = %q, want %q", in, got, "## Problem")
		}
	}
}

func TestSetSectionBodyReplace(t *testing.T) {
	out, err := setSectionBody(sectionTicketBody, "## Problem", "real diagnosis", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "## Problem\n\nreal diagnosis") {
		t.Errorf("Problem not replaced:\n%s", out)
	}
	if !strings.Contains(out, "## Approach\n\n(the fix — fill me in)") {
		t.Errorf("Approach should be untouched:\n%s", out)
	}
	// Acceptance checkbox must survive so parseAcceptance still works.
	if items := parseAcceptance(out); len(items) != 1 {
		t.Errorf("acceptance items = %d, want 1", len(items))
	}
}

func TestSetSectionBodyAppendPreservesExisting(t *testing.T) {
	once, _ := setSectionBody(sectionTicketBody, "## Approach", "first para", false)
	twice, err := setSectionBody(once, "## Approach", "second para", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(twice, "first para") || !strings.Contains(twice, "second para") {
		t.Errorf("append lost a paragraph:\n%s", twice)
	}
}

func TestSetSectionBodyCreatesAboveAcceptance(t *testing.T) {
	out, err := setSectionBody(sectionTicketBody, "## Risks", "a risk", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ri := strings.Index(out, "## Risks")
	ai := strings.Index(out, "## Acceptance")
	if ri == -1 || ai == -1 || ri > ai {
		t.Errorf("new section should land before Acceptance (risks=%d acceptance=%d):\n%s", ri, ai, out)
	}
}

func TestSetSectionBodyCreatesWhenNoAnchors(t *testing.T) {
	out, err := setSectionBody("# title\n\nsome text", "## Notes-ish", "body", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "## Notes-ish\n\nbody") {
		t.Errorf("section not appended:\n%s", out)
	}
}

func TestReservedSectionsMapCoversManaged(t *testing.T) {
	for _, h := range []string{"## Acceptance", "## Notes", "## Log"} {
		if _, ok := reservedSections[h]; !ok {
			t.Errorf("%s should be reserved", h)
		}
	}
}
