package cmd

import (
	"fmt"
	"punchlist/task"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// create the tag command. Bare `pin tag` ADDS (merges) tags; the `set` and
// `rm` subcommands replace and remove. The two subcommands exist so the MCP
// shim (which supports replace/remove) can route every tag mutation through
// `pin` rather than writing task files itself — see mcp/server.py.
func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag [ids] [tags]",
		Short: "Add tags to one or more tasks (or `tag set`/`tag rm`)",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ids, tagArgs, err := splitIDsAndTagArgs(args)
			if err != nil {
				failf("Invalid tag command: %v\n", err)
				return
			}

			tags, err := parseTagArgs(tagArgs)
			if err != nil {
				failf("Invalid tags: %v\n", err)
				return
			}

			updateTaskTags(ids, tags)
		},
	}
	cmd.AddCommand(newTagSetCmd())
	cmd.AddCommand(newTagRmCmd())
	return cmd
}

// newTagSetCmd REPLACES a task's tag set with exactly the tags given.
func newTagSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [ids] [tags]",
		Short: "Replace a task's tags with exactly the tags given",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ids, tagArgs, err := splitIDsAndTagArgs(args)
			if err != nil {
				failf("Invalid tag command: %v\n", err)
				return
			}
			tags, err := parseTagArgs(tagArgs)
			if err != nil {
				failf("Invalid tags: %v\n", err)
				return
			}
			for _, id := range ids {
				if err := setTaskTagsSingle(id, tags); err != nil {
					if printNotPunchlistError(err) {
						return
					}
					failf("Error updating task %d: %v\n", id, err)
				}
			}
		},
	}
}

// newTagRmCmd REMOVES the given tags from one or more tasks. Tags not present
// are silently ignored (removal is idempotent).
func newTagRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm [ids] [tags]",
		Aliases: []string{"remove"},
		Short:   "Remove the given tags from one or more tasks",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ids, tagArgs, err := splitIDsAndTagArgs(args)
			if err != nil {
				failf("Invalid tag command: %v\n", err)
				return
			}
			tags, err := parseTagArgs(tagArgs)
			if err != nil {
				failf("Invalid tags: %v\n", err)
				return
			}
			for _, id := range ids {
				if err := removeTaskTagsSingle(id, tags); err != nil {
					if printNotPunchlistError(err) {
						return
					}
					failf("Error updating task %d: %v\n", id, err)
				}
			}
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
			failf("Error updating task %d: %v\n", id, err)
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

// replace a single task's tags with exactly the given set (order preserved,
// deduped). An empty set clears all tags.
func setTaskTagsSingle(id int, tags []string) error {
	taskPath, err := findTaskFile(id)
	if err != nil {
		return err
	}
	t, err := task.Parse(taskPath)
	if err != nil {
		return err
	}
	t.Tags = dedupeTags(tags)
	t.UpdatedAt = time.Now()
	if err := t.Write(taskPath); err != nil {
		return err
	}
	fmt.Printf("Set tags for task %d\n", id)
	return nil
}

// remove the given tags from a single task. Tags absent from the task are
// ignored, so removal is idempotent.
func removeTaskTagsSingle(id int, tags []string) error {
	taskPath, err := findTaskFile(id)
	if err != nil {
		return err
	}
	t, err := task.Parse(taskPath)
	if err != nil {
		return err
	}
	t.Tags = removeTags(t.Tags, tags)
	t.UpdatedAt = time.Now()
	if err := t.Write(taskPath); err != nil {
		return err
	}
	fmt.Printf("Removed tags from task %d\n", id)
	return nil
}

// removeTags returns existing with every tag in toRemove dropped (matched on
// trimmed value), preserving the order of the tags that remain. Tags absent
// from existing are ignored, so removal is idempotent.
func removeTags(existing []string, toRemove []string) []string {
	drop := make(map[string]struct{}, len(toRemove))
	for _, tag := range toRemove {
		if tag = strings.TrimSpace(tag); tag != "" {
			drop[tag] = struct{}{}
		}
	}
	kept := make([]string, 0, len(existing))
	for _, tag := range existing {
		if _, remove := drop[strings.TrimSpace(tag)]; remove {
			continue
		}
		kept = append(kept, tag)
	}
	return kept
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
