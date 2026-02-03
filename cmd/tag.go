package cmd

import (
	"fmt"
	"os"
	"punchlist/task"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// create the tag command
func newTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag [ids] [tags]",
		Short: "Add tags to one or more tasks",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ids, tagArgs, err := splitIDsAndTagArgs(args)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid tag command: %v\n", err)
				return
			}

			tags, err := parseTagArgs(tagArgs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid tags: %v\n", err)
				return
			}

			updateTaskTags(ids, tags)
		},
	}
}

// split args into id args and tag args
func splitIDsAndTagArgs(args []string) ([]int, []string, error) {
	if len(args) < 2 {
		return nil, nil, fmt.Errorf("missing tag values")
	}

	if selector, endIndex, ok, err := extractBracketSelectorPrefix(args); err != nil {
		return nil, nil, err
	} else if ok {
		ids, err := parseBracketSelector(selector)
		if err != nil {
			return nil, nil, err
		}
		if endIndex == len(args)-1 {
			return nil, nil, fmt.Errorf("missing tag values")
		}
		return ids, args[endIndex+1:], nil
	}

	idArgs := []string{}
	for i, arg := range args {
		token := strings.TrimSpace(arg)
		if token == "" {
			continue
		}
		if isValidIDToken(token) {
			idArgs = append(idArgs, token)
			continue
		}

		if len(idArgs) == 0 {
			return nil, nil, fmt.Errorf("missing task IDs")
		}
		ids, err := parseTaskIDs(idArgs)
		if err != nil {
			return nil, nil, err
		}
		return ids, args[i:], nil
	}

	return nil, nil, fmt.Errorf("missing tag values")
}

// parse tag args into a list of tags
func parseTagArgs(args []string) ([]string, error) {
	input := strings.TrimSpace(strings.Join(args, " "))
	if input == "" {
		return nil, fmt.Errorf("missing tag values")
	}

	trimmed := strings.TrimSpace(input)
	trimmed = strings.TrimPrefix(trimmed, "{")
	trimmed = strings.TrimSuffix(trimmed, "}")
	if trimmed == "" {
		return nil, fmt.Errorf("missing tag values")
	}

	parts := strings.Split(trimmed, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		for _, field := range fields {
			tag := strings.TrimSpace(field)
			if tag == "" {
				continue
			}
			tags = append(tags, tag)
		}
	}

	tags = dedupeTags(tags)
	if len(tags) == 0 {
		return nil, fmt.Errorf("missing tag values")
	}
	return tags, nil
}

// update multiple tasks with new tags
func updateTaskTags(ids []int, tags []string) {
	for _, id := range ids {
		if err := updateTaskTagsSingle(id, tags); err != nil {
			if printNotPunchlistError(err) {
				return
			}
			fmt.Fprintf(os.Stderr, "Error updating task %d: %v\n", id, err)
		}
	}
}

// update a single task's tags
func updateTaskTagsSingle(id int, tags []string) error {
	taskPath, err := findTaskFile(id)
	if err != nil {
		return err
	}

	t, err := task.Parse(taskPath)
	if err != nil {
		return err
	}

	t.Tags = mergeTags(t.Tags, tags)
	t.UpdatedAt = time.Now()

	if err := t.Write(taskPath); err != nil {
		return err
	}

	fmt.Printf("Updated tags for task %d\n", id)
	return nil
}

// merge tags while preserving order
func mergeTags(existing []string, incoming []string) []string {
	merged := make([]string, 0, len(existing)+len(incoming))
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, tag := range existing {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		merged = append(merged, tag)
	}
	for _, tag := range incoming {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		merged = append(merged, tag)
	}
	return merged
}

// dedupe tags while preserving order
func dedupeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	unique := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		unique = append(unique, tag)
	}
	return unique
}
