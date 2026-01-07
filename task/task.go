package task

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
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
	default:
		return "", false
	}
}

// task is the canonical in-memory representation
type Task struct {
	ID           int        `yaml:"id"`
	Title        string     `yaml:"title"`
	State        State      `yaml:"state"`
	Priority     int        `yaml:"priority,omitempty"`
	Due          *time.Time `yaml:"due,omitempty"`
	Tags         []string   `yaml:"tags,omitempty"`
	CreatedAt    time.Time  `yaml:"created_at"`
	UpdatedAt    time.Time  `yaml:"updated_at"`
	StartedAt    *time.Time `yaml:"started_at,omitempty"`
	CompletedAt  *time.Time `yaml:"completed_at,omitempty"`
	ExternalRefs []string   `yaml:"external_refs,omitempty"`
	Body         string     `yaml:"-"`
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
		return nil, fmt.Errorf("failed to unmarshal frontmatter: %w", err)
	}

	task.Body = strings.TrimSpace(bodyContent.String())

	return &task, nil
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

	return os.WriteFile(filePath, buf.Bytes(), 0644)
}
