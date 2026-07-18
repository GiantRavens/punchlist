package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func doctorSetupScope(t *testing.T) func() {
	t.Helper()
	teardown := setupTest(t)
	exitCode = 0 // doctor signals findings via the failf exit-status var
	if _, err := executeCommand("init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	return func() {
		exitCode = 0
		teardown()
	}
}

func doctorFindTask(t *testing.T, want string) string {
	t.Helper()
	entries, err := os.ReadDir("tasks")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), want) {
			return filepath.Join("tasks", e.Name())
		}
	}
	t.Fatalf("no task file matching %q", want)
	return ""
}

func TestDoctorCleanScope(t *testing.T) {
	teardown := doctorSetupScope(t)
	defer teardown()
	if _, err := executeCommand("todo", "healthy task"); err != nil {
		t.Fatal(err)
	}
	out, err := executeCommand("doctor")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if !strings.Contains(out, "0 findings") || !strings.Contains(out, "all clean") {
		t.Fatalf("expected clean report, got:\n%s", out)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit status 0 for a clean scope, got %d", exitCode)
	}
}

func TestDoctorDetectsSemanticProblems(t *testing.T) {
	teardown := doctorSetupScope(t)
	defer teardown()
	if _, err := executeCommand("todo", "victim task"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand("todo", "dependent task", "depends:99"); err != nil {
		t.Fatal(err)
	}

	// corrupt the first task: unknown state, duplicated Log, checkbox glyph
	path := doctorFindTask(t, "victim")
	raw, _ := os.ReadFile(path)
	text := string(raw)
	text = strings.Replace(text, "state: TODO", "state: PRI", 1)
	text += "\n- [?] broken checkbox\n\n## Log\n\n- 2026-01-02T00:00:00Z: second log section entry\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v", err)
	}
	var report doctorReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	checks := map[string]bool{}
	for _, f := range report.Findings {
		checks[f.Check] = true
	}
	for _, want := range []string{"unknown_state", "duplicate_section", "bad_checkbox", "dangling_dependency"} {
		if !checks[want] {
			t.Errorf("expected finding %q, got %v", want, checks)
		}
	}
	if exitCode != 1 {
		t.Fatalf("expected exit status 1 when findings remain, got %d", exitCode)
	}
}

func TestDoctorFixMergesDuplicateLogSections(t *testing.T) {
	teardown := doctorSetupScope(t)
	defer teardown()
	if _, err := executeCommand("todo", "double log task"); err != nil {
		t.Fatal(err)
	}
	path := doctorFindTask(t, "double-log")
	raw, _ := os.ReadFile(path)
	frontmatterBefore := strings.SplitAfter(string(raw), "\n---\n")[0]
	text := string(raw) + "\n## Log\n\n- 2026-01-02T00:00:00Z: entry from second section\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("doctor", "--fix")
	if err != nil {
		t.Fatalf("doctor --fix: %v", err)
	}
	if !strings.Contains(out, "1 fixed") {
		t.Fatalf("expected 1 fixed, got:\n%s", out)
	}

	repaired, _ := os.ReadFile(path)
	body := string(repaired)
	if got := strings.Count(body, "\n## Log\n"); got != 1 {
		t.Fatalf("expected exactly one Log section after fix, got %d:\n%s", got, body)
	}
	if !strings.Contains(body, "Created task") || !strings.Contains(body, "entry from second section") {
		t.Fatalf("expected every log entry preserved:\n%s", body)
	}
	if !strings.HasPrefix(body, frontmatterBefore) {
		t.Fatal("expected frontmatter preserved byte-for-byte")
	}

	// idempotent: a second run reports clean
	out, err = executeCommand("doctor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "0 findings") {
		t.Fatalf("expected clean report after fix, got:\n%s", out)
	}
	// and the repaired file still round-trips through pin's own mutations
	if _, err := executeCommand("note", "1", "post-repair note"); err != nil {
		t.Fatalf("note after repair: %v", err)
	}
}

func TestDoctorFixMergesDuplicateNotesBeforeLog(t *testing.T) {
	teardown := doctorSetupScope(t)
	defer teardown()
	if _, err := executeCommand("todo", "double notes task"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand("note", "1", "first note"); err != nil {
		t.Fatal(err)
	}
	path := doctorFindTask(t, "double-notes")
	raw, _ := os.ReadFile(path)
	text := string(raw) + "\n## Notes\n\n- 2026-01-02T00:00:00Z: note from second section\n"
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand("doctor", "--fix"); err != nil {
		t.Fatalf("doctor --fix: %v", err)
	}
	repaired, _ := os.ReadFile(path)
	body := string(repaired)
	if got := strings.Count(body, "\n## Notes\n"); got != 1 {
		t.Fatalf("expected one Notes section, got %d:\n%s", got, body)
	}
	if !strings.Contains(body, "first note") || !strings.Contains(body, "note from second section") {
		t.Fatalf("expected all notes preserved:\n%s", body)
	}
	if strings.Index(body, "\n## Notes\n") > strings.Index(body, "\n## Log\n") {
		t.Fatalf("expected merged Notes to stay before Log:\n%s", body)
	}
}

func TestDoctorNextIDDrift(t *testing.T) {
	teardown := doctorSetupScope(t)
	defer teardown()
	if _, err := executeCommand("todo", "task one"); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(".punchlist", "config.yaml")
	raw, _ := os.ReadFile(cfgPath)
	text := strings.Replace(string(raw), "next_id: 2", "next_id: 1", 1)
	if err := os.WriteFile(cfgPath, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := executeCommand("doctor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "next_id_drift") {
		t.Fatalf("expected next_id_drift finding, got:\n%s", out)
	}
}
