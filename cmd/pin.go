package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// options extracted from creation modifiers
type createOptions struct {
	priority int
	due      *time.Time
	tags     []string
	state    string
	depends  []int
}

// ticketScaffold seeds the body of a --ticket task with the three
// sections a substantive pin needs: what's wrong (Problem), how it gets
// fixed (Approach), and a testable post-condition proving it durably
// landed (Acceptance). The single unchecked criterion is parseAcceptance-
// compatible so `pin acceptance`/`pin check` work immediately. ## Log is
// appended by the normal addLog path, and `pin note` creates ## Notes on
// demand — both land after these sections. (pin #17, decision-0100.)
const ticketScaffold = `## Problem

(what's wrong + root cause + evidence — fill me in)

## Approach

(the fix — fill me in)

## Acceptance

- [ ] (testable post-condition that proves the fix durably landed — fill me in)`

// stripTicketFlag pulls the --ticket boolean out of a free-form arg list,
// returning the remaining args and whether the flag was present. State-
// creation commands set DisableFlagParsing, so the flag arrives as a plain
// token rather than being parsed by cobra; handling it here means --ticket
// composes with every create path (todo/begun/done and implicit creation)
// and with the existing pri/tags/due/state modifiers.
func stripTicketFlag(args []string) ([]string, bool) {
	ticket := false
	out := args[:0:0]
	for _, a := range args {
		if a == "--ticket" {
			ticket = true
			continue
		}
		out = append(out, a)
	}
	return out, ticket
}

// create a task from free-form args
func createTaskFromArgs(args []string) error {
	args, ticket := stripTicketFlag(args)
	targetPath, remaining := extractTargetPath(args)
	if len(remaining) == 0 {
		return fmt.Errorf("missing title")
	}

	startDir := ""
	if targetPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get current working directory: %w", err)
		}
		startDir = cwd
	} else {
		root, err := punchlistRootFromPath(targetPath)
		if err != nil {
			return err
		}
		startDir = root
	}

	// scopes marked accepting: false are closed to new tasks; walk up to
	// the nearest accepting ancestor scope instead (pin #12)
	root, skipped, err := config.FindAcceptingRootFrom(startDir)
	if err != nil {
		return err
	}
	for _, closed := range skipped {
		fmt.Printf("Scope %s is closed to new tasks (accepting: false); walking up.\n", closed)
	}

	return withProjectWriteLock(root, func() error {
		if targetPath == "" && len(skipped) == 0 {
			return createTaskFromArgsInDir(remaining, ticket)
		}
		return withWorkingDir(root, func() error {
			return createTaskFromArgsInDir(remaining, ticket)
		})
	})
}

func createTaskFromArgsInDir(args []string, ticket bool) error {
	newTask, err := createTaskInCwd(args, ticket)
	if err != nil {
		return err
	}
	fmt.Printf("Created task %d: %s\n", newTask.ID, newTask.Path)
	return nil
}

// createTaskInCwd is the print-free creation core shared by the CLI and the
// browse TUI (which cannot write to stdout mid-render). Callers must already
// hold the project write lock.
func createTaskInCwd(args []string, ticket bool) (*task.Task, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing title")
	}

	var (
		state task.State
		mods  []string
	)

	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}
	stateCatalog, err := config.BuildStateCatalog(cfg)
	if err != nil {
		return nil, fmt.Errorf("error loading state config: %w", err)
	}

	// parse optional leading state
	if canonical, ok := stateCatalog.Resolve(args[0]); ok {
		state = task.State(canonical)
		args = args[1:]
	} else if isUpperStateToken(args[0]) {
		state = task.State(stateCatalog.Canonicalize(args[0]))
		args = args[1:]
	} else {
		state = task.StateTodo
	}

	// split title from modifiers
	title, mods, err := splitTitleAndModifiers(args)
	if err != nil {
		return nil, err
	}

	// parse modifiers into options
	opts, err := parseCreateModifiers(mods)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(opts.state) != "" {
		state = task.State(stateCatalog.Canonicalize(opts.state))
	}

	fullTitle := title
	title = truncateWithEllipsis(fullTitle, titleMaxLenFromConfig(cfg))

	id := cfg.NextID
	// build file path
	slug := slugify(title)
	idWidth := idWidthFromConfig(cfg)
	filename := fmt.Sprintf("%0*d-%s.md", idWidth, id, slug)

	tasksPath, err := tasksDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tasksPath, 0755); err != nil {
		return nil, fmt.Errorf("error creating tasks directory: %w", err)
	}
	filePath := filepath.Join(tasksPath, filename)

	// assemble the task object
	newTask := &task.Task{
		ID:        id,
		Title:     title,
		State:     state,
		Priority:  opts.priority,
		Tags:      opts.tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Due:       opts.due,
		DependsOn: opts.depends,
		Path:      filePath,
	}
	// use a default h1 body; --ticket seeds the Problem/Approach/Acceptance scaffold
	if ticket {
		newTask.Body = fmt.Sprintf("# %s\n\n%s\n", fullTitle, ticketScaffold)
	} else {
		newTask.Body = fmt.Sprintf("# %s\n", fullTitle)
	}
	addLog(newTask, "Created task")

	if err := newTask.Write(filePath); err != nil {
		return nil, fmt.Errorf("error writing task file: %w", err)
	}

	// increment and save the next id
	cfg.NextID++
	if err := config.SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("error saving config: %w", err)
	}

	return newTask, nil
}

// choose a safe id width from config or defaults
func idWidthFromConfig(cfg *config.Config) int {
	if cfg == nil || cfg.IDWidth <= 0 {
		return config.DefaultIDWidth()
	}
	return cfg.IDWidth
}

// parse modifier tokens into options
func parseCreateModifiers(args []string) (createOptions, error) {
	opts := createOptions{}
	for _, arg := range args {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			return opts, fmt.Errorf("invalid modifier: %s", arg)
		}
		key := strings.TrimSpace(parts[0])
		normKey, ok := normalizeModifierKey(key)
		if !ok {
			return opts, fmt.Errorf("unknown modifier: %s", key)
		}
		value := strings.TrimSpace(parts[1])

		switch normKey {
		case "pri":
			priority, err := strconv.Atoi(value)
			if err != nil {
				return opts, fmt.Errorf("invalid priority: %s", value)
			}
			if priority < 0 || priority > 10 {
				return opts, fmt.Errorf("priority must be between 0 and 10, got %d", priority)
			}
			opts.priority = priority
		case "due":
			parsed, err := parseDue(value)
			if err != nil {
				return opts, err
			}
			opts.due = parsed
		case "tags":
			opts.tags = parseTags(value)
		case "state":
			opts.state = value
		case "depends":
			opts.depends = parseDependsList(value)
		default:
			return opts, fmt.Errorf("unknown modifier: %s", key)
		}
	}

	return opts, nil
}

// split args into a title and key:value modifiers
func splitTitleAndModifiers(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing title")
	}

	var titleParts []string
	var mods []string
	foundTitle := false

	for i := 0; i < len(args); i++ {
		key, value, ok, inline := parseModifierToken(args[i])
		if ok {
			if inline && value != "" {
				mods = append(mods, fmt.Sprintf("%s:%s", key, value))
				continue
			}

			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing value for %s", key)
			}

			j := i + 1
			valueParts := []string{}
			for j < len(args) {
				if k, v, ok, inline := parseModifierToken(args[j]); ok {
					if inline && v == "" {
						break
					}
					if k != "" {
						break
					}
				}
				valueParts = append(valueParts, args[j])
				j++
			}

			if len(valueParts) == 0 {
				return "", nil, fmt.Errorf("missing value for %s", key)
			}
			mods = append(mods, fmt.Sprintf("%s:%s", key, strings.Join(valueParts, " ")))
			i = j - 1
			continue
		}

		if foundTitle {
			return "", nil, fmt.Errorf("title must be quoted")
		}
		titleParts = append(titleParts, args[i])
		foundTitle = true
	}

	title := strings.TrimSpace(strings.Join(titleParts, " "))
	if title == "" {
		return "", nil, fmt.Errorf("missing title")
	}
	return title, mods, nil
}

// parse modifier tokens in key:value or key value form
func parseModifierToken(token string) (key, value string, ok bool, inline bool) {
	parts := strings.SplitN(token, ":", 2)
	if len(parts) == 2 {
		norm, ok := normalizeModifierKey(parts[0])
		if ok {
			return norm, strings.TrimSpace(parts[1]), true, true
		}
	}

	if norm, ok := normalizeModifierKey(token); ok {
		return norm, "", true, false
	}

	return "", "", false, false
}

// normalize modifier keys to canonical names
func normalizeModifierKey(key string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "pri", "priority":
		return "pri", true
	case "due", "by":
		return "due", true
	case "tags", "tag":
		return "tags", true
	case "state":
		return "state", true
	case "depends", "deps", "depends_on":
		return "depends", true
	default:
		return "", false
	}
}

// parse tag lists in {a,b,c} format
func parseTags(value string) []string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "{")
	trimmed = strings.TrimSuffix(trimmed, "}")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}
	return tags
}

func isUpperStateToken(token string) bool {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return false
	}
	hasLetter := false
	for _, r := range trimmed {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter && trimmed == strings.ToUpper(trimmed)
}

// parse due dates in natural and structured formats
func parseDue(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid due date: %s", value)
	}

	// try natural language shortcuts first
	now := time.Now()
	if parsed, ok := parseDueNatural(trimmed, now); ok {
		return &parsed, nil
	}

	// parse date-only formats at noon local time
	loc := now.Location()
	dateOnlyLayouts := []string{
		"2006-01-02",
	}
	for _, layout := range dateOnlyLayouts {
		parsed, err := time.ParseInLocation(layout, trimmed, loc)
		if err == nil {
			noon := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, loc)
			return &noon, nil
		}
	}

	// parse date-time formats with local time zone
	dateTimeLayouts := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
	}
	for _, layout := range dateTimeLayouts {
		parsed, err := time.ParseInLocation(layout, trimmed, loc)
		if err == nil {
			return &parsed, nil
		}
	}

	// fall back to rfc3339
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return &parsed, nil
	}

	return nil, fmt.Errorf("invalid due date: %s", value)
}

// parse a small set of natural date tokens
func parseDueNatural(input string, now time.Time) (time.Time, bool) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" {
		return time.Time{}, false
	}

	if normalized == "today" {
		return dateAtNoon(now, 0), true
	}
	if normalized == "tomorrow" {
		return dateAtNoon(now, 1), true
	}

	fields := strings.Fields(normalized)
	if len(fields) == 2 && fields[0] == "next" {
		if weekday, ok := parseWeekday(fields[1]); ok {
			return nextWeekdayAtNoon(now, weekday, true), true
		}
	}
	if len(fields) == 1 {
		if weekday, ok := parseWeekday(fields[0]); ok {
			return nextWeekdayAtNoon(now, weekday, false), true
		}
	}

	return time.Time{}, false
}

// map weekday strings to time.Weekday
func parseWeekday(input string) (time.Weekday, bool) {
	switch input {
	case "sun", "sunday":
		return time.Sunday, true
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tues", "tuesday":
		return time.Tuesday, true
	case "wed", "weds", "wednesday":
		return time.Wednesday, true
	case "thu", "thur", "thurs", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	default:
		return time.Sunday, false
	}
}

// compute the next matching weekday at noon
func nextWeekdayAtNoon(now time.Time, weekday time.Weekday, forceNextWeek bool) time.Time {
	daysAhead := (int(weekday) - int(now.Weekday()) + 7) % 7
	if forceNextWeek && daysAhead == 0 {
		daysAhead = 7
	}
	return dateAtNoon(now, daysAhead)
}

// normalize a date to noon local time
func dateAtNoon(now time.Time, addDays int) time.Time {
	target := now.AddDate(0, 0, addDays)
	return time.Date(target.Year(), target.Month(), target.Day(), 12, 0, 0, 0, target.Location())
}

// parse comma-separated int lists like "1,2,3" or "{1,2,3}"
func parseDependsList(value string) []int {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "{")
	trimmed = strings.TrimSuffix(trimmed, "}")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

var slugifyRegex = regexp.MustCompile("[^a-z0-9]+")

// generate a filename-safe slug
func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugifyRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
