package cmd

import (
	"fmt"
	"os"
	"punchlist/task"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// create the meta command
func newMetaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "meta [id] [key=value ...]",
		Short: "View or set metadata on a task",
		Long:  "With no key=value args, display current meta. With key=value pairs, set metadata values.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid task ID: %v\n", err)
				return
			}

			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error finding task: %v\n", err)
				return
			}

			t, err := task.Parse(taskPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing task: %v\n", err)
				return
			}

			// no key=value args: display current meta
			if len(args) == 1 {
				displayMeta(t)
				return
			}

			// parse and set key=value pairs
			if t.Meta == nil {
				t.Meta = make(map[string]interface{})
			}
			for _, arg := range args[1:] {
				key, value, ok := parseMetaKV(arg)
				if !ok {
					fmt.Fprintf(os.Stderr, "Invalid meta argument (expected key=value): %s\n", arg)
					return
				}
				if value == "" {
					delete(t.Meta, key)
				} else {
					t.Meta[key] = value
				}
			}
			if len(t.Meta) == 0 {
				t.Meta = nil
			}

			t.UpdatedAt = time.Now()
			if err := t.Write(taskPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating task: %v\n", err)
				return
			}
			fmt.Printf("Updated meta for task %d\n", id)
		},
	}
}

// display meta values for a task
func displayMeta(t *task.Task) {
	if len(t.Meta) == 0 {
		fmt.Println("No meta values set.")
		return
	}
	for key, value := range t.Meta {
		fmt.Printf("%s: %v\n", key, value)
	}
}

// parse a key=value argument
func parseMetaKV(arg string) (string, string, bool) {
	idx := strings.Index(arg, "=")
	if idx == -1 || idx == 0 {
		return "", "", false
	}
	key := strings.TrimSpace(arg[:idx])
	value := strings.TrimSpace(arg[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}
