package cmd

import "strings"

// split a markdown body into before/section/after for a heading
func splitSection(body, heading string) (before, section, after string, found bool) {
	idx := strings.Index(body, heading)
	if idx == -1 {
		return body, "", "", false
	}

	before = body[:idx]
	rest := body[idx:]
	searchStart := len(heading)
	nextIdx := strings.Index(rest[searchStart:], "\n## ")
	if nextIdx == -1 {
		return before, rest, "", true
	}
	nextIdx += searchStart
	section = rest[:nextIdx]
	after = rest[nextIdx:]
	return before, section, after, true
}

// append a list entry to a section with spacing
func appendEntry(section, entry string) string {
	section = strings.TrimRight(section, "\n")
	if section == "" {
		return entry + "\n\n"
	}
	return section + "\n\n" + entry + "\n\n"
}

// join markdown blocks with blank lines and trim edges
func joinBlocks(blocks ...string) string {
	cleaned := make([]string, 0, len(blocks))
	for _, b := range blocks {
		b = strings.Trim(b, "\n")
		if b == "" {
			continue
		}
		cleaned = append(cleaned, b)
	}
	return strings.Join(cleaned, "\n\n")
}
