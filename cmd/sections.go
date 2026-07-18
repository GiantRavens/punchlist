package cmd

import "strings"

// split a markdown body into before/section/after for a heading
func splitSection(body, heading string) (before, section, after string, found bool) {
	idx := indexOfHeadingLine(body, heading)
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

// indexOfHeadingLine finds heading only where it is a complete line: at the
// start of the body or right after a newline, and followed by a newline or
// end of body. A plain substring search corrupted task files whose prose
// merely mentioned a section heading (pin #28, specimen tasks/021).
func indexOfHeadingLine(body, heading string) int {
	searchFrom := 0
	for {
		idx := strings.Index(body[searchFrom:], heading)
		if idx == -1 {
			return -1
		}
		idx += searchFrom
		atLineStart := idx == 0 || body[idx-1] == '\n'
		end := idx + len(heading)
		atLineEnd := end == len(body) || body[end] == '\n' || body[end] == '\r'
		if atLineStart && atLineEnd {
			return idx
		}
		searchFrom = idx + 1
	}
}

// append a list entry to a section with spacing
// appendEntry adds a list entry to a section, emitting a TIGHT markdown
// list: consecutive "- " lines with no blanks between items (the idiomatic
// form for a chronological log, and what web renderers space correctly).
// A blank line still separates the section heading from its first item.
// Existing loose-list files (blank lines between items, the pre-1.3.2
// emission) stay valid — parsers accept both; only new entries are tight.
func appendEntry(section, entry string) string {
	section = strings.TrimRight(section, "\n")
	if section == "" {
		return entry + "\n\n"
	}
	separator := "\n"
	if !strings.HasPrefix(strings.TrimSpace(lastLine(section)), "- ") {
		// after the bare heading (or non-list text), open the list with a
		// blank line so the heading and list stay distinct blocks
		separator = "\n\n"
	}
	return section + separator + entry + "\n\n"
}

func lastLine(text string) string {
	if idx := strings.LastIndex(text, "\n"); idx != -1 {
		return text[idx+1:]
	}
	return text
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
