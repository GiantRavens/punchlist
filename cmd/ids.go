package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

// parse ids from args, supporting brackets, commas, and ranges
func parseTaskIDs(args []string) ([]int, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing task IDs")
	}

	if selector, ok, err := extractBracketSelector(args); err != nil {
		return nil, err
	} else if ok {
		return parseBracketSelector(selector)
	}

	ids := make([]int, 0, len(args))
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "[") || strings.HasSuffix(arg, "]") {
			return nil, fmt.Errorf("bracket selector must be a single argument")
		}
		expanded, err := expandIDToken(arg)
		if err != nil {
			return nil, err
		}
		ids = append(ids, expanded...)
	}

	return dedupeIDs(ids), nil
}

// extract a bracket selector that may be split across tokens
func extractBracketSelector(args []string) (string, bool, error) {
	first := strings.TrimSpace(args[0])
	if !strings.HasPrefix(first, "[") {
		return "", false, nil
	}

	var builder strings.Builder
	endIndex := -1
	for i, arg := range args {
		if i > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(arg)
		if strings.Contains(arg, "]") {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return "", false, fmt.Errorf("unterminated bracket selector")
	}
	if endIndex != len(args)-1 {
		return "", false, fmt.Errorf("bracket selector must be the only argument")
	}

	return builder.String(), true, nil
}

// parse a single bracket selector string
func parseBracketSelector(selector string) ([]int, error) {
	trimmed := strings.TrimSpace(selector)
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return nil, fmt.Errorf("invalid bracket selector: %s", selector)
	}

	content := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if content == "" {
		return nil, fmt.Errorf("empty bracket selector")
	}

	fields := strings.FieldsFunc(content, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})

	ids := []int{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		expanded, err := expandIDToken(field)
		if err != nil {
			return nil, err
		}
		ids = append(ids, expanded...)
	}

	return dedupeIDs(ids), nil
}

// expand a token into one or more ids
func expandIDToken(token string) ([]int, error) {
	if token == "" {
		return nil, fmt.Errorf("empty task ID")
	}

	if strings.Count(token, "-") == 1 {
		parts := strings.SplitN(token, "-", 2)
		if parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid range: %s", token)
		}

		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid task ID: %s", parts[0])
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid task ID: %s", parts[1])
		}

		if start > end {
			start, end = end, start
		}

		ids := make([]int, 0, end-start+1)
		for id := start; id <= end; id++ {
			ids = append(ids, id)
		}
		return ids, nil
	}

	id, err := strconv.Atoi(token)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %s", token)
	}
	return []int{id}, nil
}

// remove duplicate ids while preserving order
func dedupeIDs(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	unique := make([]int, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
