package task

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// state represents the canonical task state values
type State string

const (
	StateTodo    State = "TODO"
	StateBegun   State = "BEGUN"
	StateNotDo   State = "NOTDO"
	StateDone    State = "DONE"
	StateBlock   State = "BLOCK"
	StateConfirm State = "CONFIRM"
)

// parse a state token into a canonical state
func ParseState(input string) (State, bool) {
	switch strings.ToUpper(strings.TrimSpace(input)) {
	case string(StateTodo):
		return StateTodo, true
	case string(StateBegun):
		return StateBegun, true
	case string(StateNotDo):
		return StateNotDo, true
	case string(StateDone):
		return StateDone, true
	case string(StateBlock):
		return StateBlock, true
	case string(StateConfirm):
		return StateConfirm, true
	case "FOLLOWUP", "FOLLOW-UP", "REVIEW":
		return StateConfirm, true
	case "WAITING":
		return StateBlock, true
	default:
		return "", false
	}
}

// task is the canonical in-memory representation
type Task struct {
	ID           int                    `yaml:"id" json:"id"`
	Title        string                 `yaml:"title" json:"title"`
	State        State                  `yaml:"state" json:"state"`
	Priority     int                    `yaml:"priority,omitempty" json:"priority,omitempty"`
	Due          *time.Time             `yaml:"due,omitempty" json:"due,omitempty"`
	Tags         []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
	CreatedAt    time.Time              `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time              `yaml:"updated_at" json:"updated_at"`
	StartedAt    *time.Time             `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt  *time.Time             `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
	ExternalRefs []string               `yaml:"external_refs,omitempty" json:"external_refs,omitempty"`
	DependsOn    []int                  `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Meta         map[string]interface{} `yaml:"meta,omitempty" json:"meta,omitempty"`
	Path         string                 `yaml:"-" json:"path"`
	Body         string                 `yaml:"-" json:"body,omitempty"`
}

// frontmatterSeparator defines yaml delimiters
const frontmatterSeparator = "---"

// parse reads a task file into a Task struct
func Parse(filePath string) (*Task, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open task file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var yamlContent, bodyContent strings.Builder
	inFrontmatter := false
	frontmatterClosed := false

	// check for initial separator
	if scanner.Scan() && scanner.Text() == frontmatterSeparator {
		inFrontmatter = true
	} else {
		// no frontmatter, treat entire file as body
		bodyContent.WriteString(scanner.Text() + "\n")
	}

	for scanner.Scan() {
		line := scanner.Text()
		if inFrontmatter && line == frontmatterSeparator {
			inFrontmatter = false
			frontmatterClosed = true
			continue
		}

		if inFrontmatter {
			yamlContent.WriteString(line + "\n")
		} else if frontmatterClosed {
			bodyContent.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var task Task
	if err := yaml.Unmarshal([]byte(yamlContent.String()), &task); err != nil {
		recovered := false
		if normalized, changed := normalizeFrontmatterTimestamps(yamlContent.String()); changed {
			var normalizedTask Task
			if retryErr := yaml.Unmarshal([]byte(normalized), &normalizedTask); retryErr == nil {
				task = normalizedTask
				recovered = true
			}
		}
		if !recovered {
			fallbackTask, fallbackErr := parseFrontmatterLenient(yamlContent.String())
			if fallbackErr != nil {
				return nil, fmt.Errorf("failed to unmarshal frontmatter: %w", err)
			}
			task = *fallbackTask
		}
	}

	task.Path = filePath
	task.Body = strings.TrimSpace(bodyContent.String())

	return &task, nil
}

func parseFrontmatterLenient(frontmatter string) (*Task, error) {
	var task Task
	var hasID, hasTitle, hasState, hasCreated, hasUpdated bool

	lines := strings.Split(frontmatter, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if value == "" {
			continue
		}

		value = trimOuterQuotes(value)
		switch key {
		case "id":
			id, err := strconv.Atoi(value)
			if err == nil {
				task.ID = id
				hasID = true
			}
		case "title":
			task.Title = value
			hasTitle = true
		case "state":
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				task.State = State(trimmed)
				hasState = true
			}
		case "priority":
			if priority, err := strconv.Atoi(value); err == nil {
				task.Priority = priority
			}
		case "due":
			if parsed, ok := parseTimestamp(value); ok {
				task.Due = &parsed
			}
		case "tags":
			task.Tags = parseInlineList(value)
		case "created_at":
			if parsed, ok := parseTimestamp(value); ok {
				task.CreatedAt = parsed
				hasCreated = true
			}
		case "updated_at":
			if parsed, ok := parseTimestamp(value); ok {
				task.UpdatedAt = parsed
				hasUpdated = true
			}
		case "started_at":
			if parsed, ok := parseTimestamp(value); ok {
				task.StartedAt = &parsed
			}
		case "completed_at":
			if parsed, ok := parseTimestamp(value); ok {
				task.CompletedAt = &parsed
			}
		case "external_refs":
			task.ExternalRefs = parseInlineList(value)
		case "depends_on":
			task.DependsOn = parseInlineIntList(value)
		}
	}

	if !hasID || !hasTitle || !hasState || !hasCreated || !hasUpdated {
		return nil, fmt.Errorf("missing required frontmatter fields")
	}

	return &task, nil
}

func trimOuterQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	first := value[0]
	last := value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func parseInlineList(value string) []string {
	clean := strings.TrimSpace(value)
	if clean == "" || clean == "-" || clean == "[]" {
		return nil
	}

	if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
		inner := strings.TrimSpace(clean[1 : len(clean)-1])
		if inner == "" {
			return nil
		}
		parts := strings.Split(inner, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			item = trimOuterQuotes(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	}

	clean = trimOuterQuotes(clean)
	if clean == "" || clean == "-" {
		return nil
	}

	return []string{clean}
}

func parseInlineIntList(value string) []int {
	clean := strings.TrimSpace(value)
	if clean == "" || clean == "[]" {
		return nil
	}
	if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
		clean = strings.TrimSpace(clean[1 : len(clean)-1])
	}
	if clean == "" {
		return nil
	}
	parts := strings.Split(clean, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err == nil {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// tolerant fallback layouts for timestamps written by non-pin writers
// (e.g. the 2026-03-era space-separated formats). Layouts marked local
// carry no zone information and are parsed in local time.
var tolerantTimestampLayouts = []struct {
	layout string
	local  bool
}{
	{"2006-01-02 15:04:05.999999999 -0700 MST", false}, // Go time.Time.String()
	{"2006-01-02 15:04:05.999999999 -0700", false},
	{"2006-01-02 15:04:05.999999999 -07:00", false},
	{"2006-01-02 15:04:05.999999999Z07:00", false}, // Python str(datetime) and friends
	{"2006-01-02 15:04:05.999999999", true},        // naive, assume local
	{"2006-01-02T15:04:05.999999999", true},        // ISO-T without offset
	{"2006-01-02", true},                           // date only
}

func parseTimestamp(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed, true
	}
	parsed, err = time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed, true
	}
	for _, fallback := range tolerantTimestampLayouts {
		if fallback.local {
			parsed, err = time.ParseInLocation(fallback.layout, value, time.Local)
		} else {
			parsed, err = time.Parse(fallback.layout, value)
		}
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// timestamp frontmatter keys eligible for tolerant normalization
var timestampFrontmatterKeys = map[string]bool{
	"due":          true,
	"created_at":   true,
	"updated_at":   true,
	"started_at":   true,
	"completed_at": true,
}

// rewrite tolerated non-RFC3339 timestamp values to RFC3339 so the strict
// yaml path (which preserves every other field, including block-style lists)
// can parse the file. Values are normalized to ISO-T on the next write.
func normalizeFrontmatterTimestamps(frontmatter string) (string, bool) {
	lines := strings.Split(frontmatter, "\n")
	changed := false
	for i, line := range lines {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' {
			continue // timestamp keys only appear at the top level
		}
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if !timestampFrontmatterKeys[key] {
			continue
		}
		value := trimOuterQuotes(strings.TrimSpace(line[idx+1:]))
		if value == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
			continue // already strict; leave untouched
		}
		if parsed, ok := parseTimestamp(value); ok {
			lines[i] = key + ": " + parsed.Format(time.RFC3339Nano)
			changed = true
		}
	}
	if !changed {
		return frontmatter, false
	}
	return strings.Join(lines, "\n"), true
}

// write serializes a Task back to disk
func (t *Task) Write(filePath string) error {
	var buf bytes.Buffer

	// write frontmatter
	buf.WriteString(frontmatterSeparator + "\n")
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(t); err != nil {
		return fmt.Errorf("failed to encode frontmatter: %w", err)
	}
	buf.WriteString(frontmatterSeparator + "\n")

	// write body
	if t.Body != "" {
		buf.WriteString("\n")
		buf.WriteString(t.Body)
		if !strings.HasSuffix(t.Body, "\n") {
			buf.WriteString("\n")
		}
	}

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	return os.Rename(tmpPath, filePath)
}
