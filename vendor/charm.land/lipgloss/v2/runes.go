package lipgloss

import (
	"strings"
)

// StyleRunes apply a given style to runes at the given indices in the string.
// Note that you must provide styling options for both matched and unmatched
// runes. Indices out of bounds will be ignored.
func StyleRunes(str string, indices []int, matched, unmatched Style) string {
	// Convert slice of indices to a map for easier lookups
	m := make(map[int]struct{})
	for _, i := range indices {
		m[i] = struct{}{}
	}

	var (
		out   strings.Builder
		group strings.Builder
		style Style
		runes = []rune(str)
	)

	for i, r := range runes {
		group.WriteRune(r)

		_, matches := m[i]
		_, nextMatches := m[i+1]

		if matches != nextMatches || i == len(runes)-1 {
			// Flush
			if matches {
				style = matched
			} else {
				style = unmatched
			}
			out.WriteString(style.Render(group.String()))
			group.Reset()
		}
	}

	return out.String()
}
