package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// doctorFinding is one problem in one task file. Check names are stable —
// sensors key off them.
type doctorFinding struct {
	File     string `json:"file"`
	ID       int    `json:"id,omitempty"`
	Check    string `json:"check"`
	Severity string `json:"severity"` // error | warning
	Detail   string `json:"detail"`
	Fixable  bool   `json:"fixable"`
	Fixed    bool   `json:"fixed,omitempty"`
}

type doctorReport struct {
	Files    int             `json:"files"`
	Findings []doctorFinding `json:"findings"`
	Fixed    int             `json:"fixed"`
}

var doctorSectionHeadings = []string{"## Log", "## Notes", "## Acceptance"}

var (
	doctorFilenameID = regexp.MustCompile(`^(\d+)-`)
	doctorCheckbox   = regexp.MustCompile(`^- \[(.)\] `)
)

func newDoctorCmd() *cobra.Command {
	var jsonOut bool
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check task files for semantic problems (and repair safe ones with --fix)",
		Long: "Audit every task file in the scope for problems that parse cleanly but are wrong: " +
			"unknown states, duplicated sections, duplicate or mismatched ids, dangling dependencies, " +
			"malformed checkboxes, stray lines in the log, title/H1 divergence, and next_id drift. " +
			"--fix applies only mechanically safe repairs (currently: merging duplicated Log and Notes " +
			"sections, preserving every entry). Judgment-shaped findings are always report-only. " +
			"Exits 0 when the scope is clean (or every finding was fixed), 1 when findings remain; " +
			"consume --json for automation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := runDoctor(fix)
			if err != nil {
				if printNotPunchlistError(err) {
					return nil
				}
				return err
			}
			if jsonOut {
				out, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
			} else {
				printDoctorReport(report, fix)
			}
			// lint-style exit status: remaining findings mean the substrate
			// is not healthy. The findings on stdout are the message, so no
			// extra stderr line — just the status for scripts and sensors.
			if len(report.Findings) > report.Fixed {
				exitCode = 1
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit findings as JSON")
	cmd.Flags().BoolVar(&fix, "fix", false, "apply mechanically safe repairs")
	return cmd
}

func runDoctor(fix bool) (*doctorReport, error) {
	tasksPath, err := tasksDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Dir(tasksPath)

	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}
	catalog, err := config.BuildStateCatalog(cfg)
	if err != nil {
		return nil, fmt.Errorf("error loading state config: %w", err)
	}

	report := &doctorReport{}
	idsSeen := map[int]string{}
	allIDs := map[int]struct{}{}
	maxID := 0

	type checkedTask struct {
		path string
		rel  string
		tsk  *task.Task
	}
	var tasks []checkedTask

	err = filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		report.Files++
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		t, parseErr := task.Parse(path)
		if parseErr != nil {
			report.Findings = append(report.Findings, doctorFinding{
				File: rel, Check: "parse_error", Severity: "error", Detail: parseErr.Error(),
			})
			return nil
		}
		tasks = append(tasks, checkedTask{path: path, rel: rel, tsk: t})
		allIDs[t.ID] = struct{}{}
		if t.ID > maxID {
			maxID = t.ID
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking tasks: %w", err)
	}

	for _, ct := range tasks {
		t := ct.tsk
		flag := func(check, severity, detail string, fixable bool) {
			report.Findings = append(report.Findings, doctorFinding{
				File: ct.rel, ID: t.ID, Check: check, Severity: severity, Detail: detail, Fixable: fixable,
			})
		}

		if m := doctorFilenameID.FindStringSubmatch(filepath.Base(ct.path)); m != nil {
			var fnID int
			fmt.Sscanf(m[1], "%d", &fnID)
			if fnID != t.ID {
				flag("id_mismatch", "error", fmt.Sprintf("filename id %d != frontmatter id %d", fnID, t.ID), false)
			}
		}
		if prev, dup := idsSeen[t.ID]; dup {
			flag("duplicate_id", "error", fmt.Sprintf("id %d also used by %s", t.ID, prev), false)
		} else {
			idsSeen[t.ID] = ct.rel
		}

		if _, known := catalog.Resolve(string(t.State)); !known {
			flag("unknown_state", "warning", fmt.Sprintf("state %q not in config states or ls_state_order", t.State), false)
		}

		for _, dep := range t.DependsOn {
			if _, ok := allIDs[dep]; !ok {
				flag("dangling_dependency", "warning", fmt.Sprintf("depends_on %d not found in scope", dep), false)
			}
		}

		if t.CreatedAt.IsZero() {
			flag("zero_timestamp", "warning", "created_at missing or unparseable", false)
		}

		for _, heading := range doctorSectionHeadings {
			if n := countHeadingLines(t.Body, heading); n > 1 {
				// Log and Notes are append-only entry lists — merging
				// duplicates is mechanically safe. Acceptance is ordered
				// (pin check addresses by index), so it stays report-only.
				fixable := heading == "## Log" || heading == "## Notes"
				flag("duplicate_section", "error", fmt.Sprintf("%d %s sections", n, heading), fixable)
			}
		}

		if h1 := firstH1(t.Body); h1 != "" && t.Title != "" {
			titleNorm := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(t.Title), "..."), "…")
			if !strings.HasPrefix(h1, titleNorm) && !strings.HasPrefix(titleNorm, h1) {
				flag("title_divergence", "warning", fmt.Sprintf("H1 %q != frontmatter title %q", truncateWithEllipsis(h1, 50), truncateWithEllipsis(t.Title, 50)), false)
			}
		}

		if _, logBody, _, found := splitSection(t.Body, "## Log"); found {
			for _, line := range strings.Split(logBody, "\n")[1:] {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if !strings.HasPrefix(trimmed, "- ") {
					flag("log_noise", "warning", fmt.Sprintf("non-entry line in Log: %q", truncateWithEllipsis(trimmed, 60)), false)
					break
				}
			}
		}

		for _, line := range strings.Split(t.Body, "\n") {
			if m := doctorCheckbox.FindStringSubmatch(line); m != nil {
				if glyph := m[1]; glyph != " " && glyph != "x" && glyph != "X" {
					flag("bad_checkbox", "warning", fmt.Sprintf("checkbox glyph %q", glyph), false)
				}
			}
		}
	}

	if cfg.NextID <= maxID {
		report.Findings = append(report.Findings, doctorFinding{
			File: ".punchlist/config.yaml", Check: "next_id_drift", Severity: "error",
			Detail: fmt.Sprintf("next_id %d <= max task id %d — future id collision", cfg.NextID, maxID),
		})
	}

	if fix {
		if err := withProjectWriteLock(root, func() error {
			for i, f := range report.Findings {
				if !f.Fixable || f.Check != "duplicate_section" {
					continue
				}
				heading := "## Log"
				if strings.Contains(f.Detail, "## Notes") {
					heading = "## Notes"
				}
				path := filepath.Join(root, f.File)
				if repaired, err := mergeDuplicateSections(path, heading); err != nil {
					return fmt.Errorf("fixing %s: %w", f.File, err)
				} else if repaired {
					report.Findings[i].Fixed = true
					report.Fixed++
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity == "error"
		}
		return report.Findings[i].File < report.Findings[j].File
	})
	return report, nil
}

func countHeadingLines(body, heading string) int {
	count := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimRight(line, " \t\r") == heading {
			count++
		}
	}
	return count
}

func firstH1(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// mergeDuplicateSections rewrites a task file whose body contains more than
// one occurrence of the given heading line: every entry from every such
// section is preserved in encounter order under a single merged section at
// the position where the first occurrence stood (keeping Notes before Log
// per pin's layout). The frontmatter block is preserved byte-for-byte —
// this is raw text surgery, not a parse/serialize roundtrip, so nothing
// else in the file changes.
func mergeDuplicateSections(path, heading string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	text := string(raw)

	fmEnd := 0
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[4:], "\n---\n"); end != -1 {
			fmEnd = 4 + end + len("\n---\n")
		}
	}
	front, body := text[:fmEnd], text[fmEnd:]

	lines := strings.Split(body, "\n")
	inSection := false
	sectionCount := 0
	placeholder := -1
	var kept []string
	var entries []string
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == heading {
			inSection = true
			sectionCount++
			if placeholder == -1 {
				placeholder = len(kept)
				kept = append(kept, "") // merged section slots in here
			}
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "## ") {
			inSection = false
		}
		if inSection {
			if strings.TrimSpace(line) != "" {
				entries = append(entries, strings.TrimRight(line, " \t\r"))
			}
			continue
		}
		kept = append(kept, line)
	}
	if sectionCount < 2 {
		return false, nil
	}

	merged := heading + "\n\n" + strings.Join(entries, "\n\n")
	kept[placeholder] = merged
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	rebuilt := strings.Join(kept, "\n") + "\n"

	tmp := path + ".doctor-tmp"
	if err := os.WriteFile(tmp, []byte(front+rebuilt), 0644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return false, err
	}
	return true, nil
}

func printDoctorReport(report *doctorReport, fixed bool) {
	fixable := 0
	for _, f := range report.Findings {
		if f.Fixable {
			fixable++
		}
	}
	fmt.Printf("%d task files checked, %d findings", report.Files, len(report.Findings))
	if fixed {
		fmt.Printf(", %d fixed", report.Fixed)
	} else if fixable > 0 {
		fmt.Printf(" (%d fixable with --fix)", fixable)
	}
	fmt.Println()
	for _, f := range report.Findings {
		status := ""
		if f.Fixed {
			status = "  [fixed]"
		} else if f.Fixable && !fixed {
			status = "  [fixable]"
		}
		fmt.Printf("  %-9s %s  %s: %s%s\n", f.Severity, f.File, f.Check, f.Detail, status)
	}
	if len(report.Findings) == 0 {
		fmt.Println("  all clean")
	}
}
