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

type createOptions struct {
	priority int
	due      *time.Time
	tags     []string
}

func createTaskFromArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing title")
	}

	var (
		state task.State
		mods  []string
	)

	if parsed, ok := task.ParseState(args[0]); ok {
		state = parsed
		args = args[1:]
	} else {
		state = task.StateTodo
	}

	title, mods, err := splitTitleAndModifiers(args)
	if err != nil {
		return err
	}

	opts, err := parseCreateModifiers(mods)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	id := cfg.NextID
	slug := slugify(title)
	filename := fmt.Sprintf("%06d-%s.md", id, slug)

	// This is a simplification. We should eventually get the tasks dir from config
	tasksDir := "tasks"
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return fmt.Errorf("error creating tasks directory: %w", err)
	}
	filePath := filepath.Join(tasksDir, filename)

	newTask := &task.Task{
		ID:        id,
		Title:     title,
		State:     state,
		Priority:  opts.priority,
		Tags:      opts.tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Due:       opts.due,
	}
	newTask.Body = fmt.Sprintf("# %s\n", title)

	if err := newTask.Write(filePath); err != nil {
		return fmt.Errorf("error writing task file: %w", err)
	}

	fmt.Printf("Created task %d: %s\n", id, filePath)

	// Increment and save the next ID
	cfg.NextID++
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}

	return nil
}

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
			opts.priority = priority
		case "due":
			parsed, err := parseDue(value)
			if err != nil {
				return opts, err
			}
			opts.due = parsed
		case "tags":
			opts.tags = parseTags(value)
		default:
			return opts, fmt.Errorf("unknown modifier: %s", key)
		}
	}

	return opts, nil
}

func splitTitleAndModifiers(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing title")
	}

	var titleParts []string
	var mods []string

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

		titleParts = append(titleParts, args[i])
	}

	title := strings.TrimSpace(strings.Join(titleParts, " "))
	if title == "" {
		return "", nil, fmt.Errorf("missing title")
	}
	return title, mods, nil
}

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

func normalizeModifierKey(key string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "pri", "priority":
		return "pri", true
	case "due", "by":
		return "due", true
	case "tags", "tag":
		return "tags", true
	default:
		return "", false
	}
}

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

func parseDue(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid due date: %s", value)
	}

	now := time.Now()
	if parsed, ok := parseDueNatural(trimmed, now); ok {
		return &parsed, nil
	}

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

	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return &parsed, nil
	}

	return nil, fmt.Errorf("invalid due date: %s", value)
}

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

func nextWeekdayAtNoon(now time.Time, weekday time.Weekday, forceNextWeek bool) time.Time {
	daysAhead := (int(weekday) - int(now.Weekday()) + 7) % 7
	if forceNextWeek && daysAhead == 0 {
		daysAhead = 7
	}
	return dateAtNoon(now, daysAhead)
}

func dateAtNoon(now time.Time, addDays int) time.Time {
	target := now.AddDate(0, 0, addDays)
	return time.Date(target.Year(), target.Month(), target.Day(), 12, 0, 0, 0, target.Location())
}

func slugify(s string) string {
	s = strings.ToLower(s)
	re := regexp.MustCompile("[^a-z0-9]+")
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
