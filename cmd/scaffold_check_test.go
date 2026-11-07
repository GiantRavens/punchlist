package cmd

import (
	"strings"
	"testing"
)

func TestScaffoldUnfilledDetectsFreshScaffold(t *testing.T) {
	body := "# t\n\n" + ticketScaffold
	unfilled := scaffoldUnfilledSections(body)
	if len(unfilled) != 3 {
		t.Fatalf("fresh scaffold: got %v, want all three", unfilled)
	}
}

func TestScaffoldUnfilledClearsPerSection(t *testing.T) {
	body := "# t\n\n" + ticketScaffold
	filled, err := setSectionBody(body, "## Problem", "real diagnosis", false)
	if err != nil {
		t.Fatal(err)
	}
	unfilled := scaffoldUnfilledSections(filled)
	for _, s := range unfilled {
		if s == "Problem" {
			t.Errorf("Problem should be filled, still flagged: %v", unfilled)
		}
	}
	if len(unfilled) != 2 {
		t.Errorf("got %v, want Approach+Acceptance", unfilled)
	}
}

func TestScaffoldUnfilledIgnoresNonTicket(t *testing.T) {
	body := "# t\n\nsome prose that mentions fill me in casually? no section."
	if got := scaffoldUnfilledSections(body); len(got) != 0 {
		t.Errorf("non-ticket task should report nothing, got %v", got)
	}
}

func TestSetAcceptanceItemTextPreservesChecked(t *testing.T) {
	body := "# t\n\n## Acceptance\n\n- [x] done thing\n- [ ] todo thing"
	out, err := setAcceptanceItemText(body, 1, "renamed thing")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- [x] renamed thing") {
		t.Errorf("checked marker not preserved:\n%s", out)
	}
	if !strings.Contains(out, "- [ ] todo thing") {
		t.Errorf("other item changed:\n%s", out)
	}
}

func TestSetAcceptanceItemTextOutOfRange(t *testing.T) {
	body := "# t\n\n## Acceptance\n\n- [ ] only one"
	if _, err := setAcceptanceItemText(body, 5, "x"); err == nil {
		t.Error("expected error for out-of-range index")
	}
}
