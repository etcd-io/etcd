package lipgloss

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// StyleRanges applying styling to ranges in a string. Existing styles will be
// taken into account. Ranges should not overlap.
func StyleRanges(s string, ranges ...Range) string {
	if len(ranges) == 0 {
		return s
	}

	var buf strings.Builder
	lastIdx := 0
	stripped := ansi.Strip(s)

	// Use Truncate and TruncateLeft to style match.MatchedIndexes without
	// losing the original option style:
	for _, rng := range ranges {
		// Add the text before this match
		if rng.Start > lastIdx {
			buf.WriteString(ansi.Cut(s, lastIdx, rng.Start))
		}
		// Add the matched range with its highlight
		buf.WriteString(rng.Style.Render(ansi.Cut(stripped, rng.Start, rng.End)))
		lastIdx = rng.End
	}

	// Add any remaining text after the last match
	buf.WriteString(ansi.TruncateLeft(s, lastIdx, ""))

	return buf.String()
}

// NewRange returns a range and style that can be used with [StyleRanges].
func NewRange(start, end int, style Style) Range {
	return Range{start, end, style}
}

// Range is a range of text and associated styling to be used with
// [StyleRanges].
type Range struct {
	Start, End int
	Style      Style
}
