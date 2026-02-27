package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"punchlist/task"
	"time"
)

// AcceptanceItem represents a parsed checkbox from ## Acceptance
type AcceptanceItem struct {
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
	Index   int    `json:"index"`
}

// taskListView is the compact JSON representation for ls and search output
type taskListView struct {
	ID           int                    `json:"id"`
	Title        string                 `json:"title"`
	State        task.State             `json:"state"`
	Priority     int                    `json:"priority,omitempty"`
	Due          *time.Time             `json:"due,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	ExternalRefs []string               `json:"external_refs,omitempty"`
	DependsOn    []int                  `json:"depends_on,omitempty"`
	Meta         map[string]interface{} `json:"meta,omitempty"`
	Path         string                 `json:"path"`
}

// taskShowView is the enriched JSON representation for show output
type taskShowView struct {
	task.Task
	Acceptance []AcceptanceItem `json:"acceptance,omitempty"`
}

// marshalTaskList outputs a JSON array of tasks in compact format (no body)
func marshalTaskList(tasks []*task.Task) error {
	views := make([]taskListView, len(tasks))
	for i, t := range tasks {
		views[i] = taskListView{
			ID:           t.ID,
			Title:        t.Title,
			State:        t.State,
			Priority:     t.Priority,
			Due:          t.Due,
			Tags:         t.Tags,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
			StartedAt:    t.StartedAt,
			CompletedAt:  t.CompletedAt,
			ExternalRefs: t.ExternalRefs,
			DependsOn:    t.DependsOn,
			Meta:         t.Meta,
			Path:         t.Path,
		}
	}
	return writeJSON(views)
}

// marshalTaskShow outputs a single task with body and acceptance criteria
func marshalTaskShow(t *task.Task, acceptance []AcceptanceItem) error {
	view := taskShowView{
		Task:       *t,
		Acceptance: acceptance,
	}
	return writeJSON(view)
}

// writeJSON marshals v to stdout with indentation
func writeJSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}
