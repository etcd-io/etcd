package ansi

import (
	"strings"

	"github.com/charmbracelet/x/ansi/parser"
	"github.com/clipperhouse/displaywidth"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

// Cut the string, without adding any prefix or tail strings. This function is
// aware of ANSI escape codes and will not break them, and accounts for
// wide-characters (such as East-Asian characters and emojis).
// This treats the text as a sequence of graphemes.
func Cut(s string, left, right int) string {
	return cut(GraphemeWidth, s, left, right)
}

// CutWc the string, without adding any prefix or tail strings. This function is
// aware of ANSI escape codes and will not break them, and accounts for
// wide-characters (such as East-Asian characters and emojis).
// Note that the [left] parameter is inclusive, while [right] isn't,
// which is to say it'll return `[left, right)`.
//
// This treats the text as a sequence of wide characters and runes.
func CutWc(s string, left, right int) string {
	return cut(WcWidth, s, left, right)
}

func cut(m Method, s string, left, right int) string {
	if right <= left {
		return ""
	}

	truncate := Truncate
	truncateLeft := TruncateLeft
	if m == WcWidth {
		truncate = TruncateWc
		truncateLeft = TruncateWc
	}

	if left == 0 {
		return truncate(s, right, "")
	}
	return truncateLeft(truncate(s, right, ""), left, "")
}

// Truncate truncates a string to a given length, adding a tail to the end if
// the string is longer than the given length. This function is aware of ANSI
// escape codes and will not break them, and accounts for wide-characters (such
// as East-Asian characters and emojis).
// This treats the text as a sequence of graphemes.
func Truncate(s string, length int, tail string) string {
	return truncate(GraphemeWidth, s, length, tail)
}

// TruncateWc truncates a string to a given length, adding a tail to the end if
// the string is longer than the given length. This function is aware of ANSI
// escape codes and will not break them, and accounts for wide-characters (such
// as East-Asian characters and emojis).
// This treats the text as a sequence of wide characters and runes.
func TruncateWc(s string, length int, tail string) string {
	return truncate(WcWidth, s, length, tail)
}

func truncate(m Method, s string, length int, tail string) string {
	if sw := StringWidth(s); sw <= length {
		return s
	}

	tw := StringWidth(tail)
	length -= tw
	if length < 0 {
		return ""
	}

	var cluster string
	var buf strings.Builder
	curWidth := 0
	ignoring := false
	pstate := parser.GroundState // initial state
	i := 0

	// Here we iterate over the bytes of the string and collect printable
	// characters and runes. We also keep track of the width of the string
	// in cells.
	//
	// Once we reach the given length, we start ignoring characters and only
	// collect ANSI escape codes until we reach the end of string.
	for i < len(s) {
		state, action := parser.Table.Transition(pstate, s[i])
		if state == parser.Utf8State {
			// This action happens when we transition to the Utf8State.
			var width int
			cluster, width = FirstGraphemeCluster(s[i:], m)
			// increment the index by the length of the cluster
			i += len(cluster)
			curWidth += width

			// Are we ignoring? Skip to the next byte
			if ignoring {
				continue
			}

			// Is this gonna be too wide?
			// If so write the tail and stop collecting.
			if curWidth > length && !ignoring {
				ignoring = true
				buf.WriteString(tail)
			}

			if curWidth > length {
				continue
			}

			buf.WriteString(cluster)

			// Done collecting, now we're back in the ground state.
			pstate = parser.GroundState
			continue
		}

		switch action {
		case parser.PrintAction:
			// Is this gonna be too wide?
			// If so write the tail and stop collecting.
			if curWidth >= length && !ignoring {
				ignoring = true
				buf.WriteString(tail)
			}

			// Skip to the next byte if we're ignoring
			if ignoring {
				i++
				continue
			}

			// collects printable ASCII
			curWidth++
			fallthrough
		case parser.ExecuteAction:
			// execute action will be things like \n, which, if outside the cut,
			// should be ignored.
			if ignoring {
				i++
				continue
			}
			fallthrough
		default:
			buf.WriteByte(s[i])
			i++
		}

		// Transition to the next state.
		pstate = state

		// Once we reach the given length, we start ignoring runes and write
		// the tail to the buffer.
		if curWidth > length && !ignoring {
			ignoring = true
			buf.WriteString(tail)
		}
	}

	return buf.String()
}

// TruncateLeft truncates a string from the left side by removing n characters,
// adding a prefix to the beginning if the string is longer than n.
// This function is aware of ANSI escape codes and will not break them, and
// accounts for wide-characters (such as East-Asian characters and emojis).
// This treats the text as a sequence of graphemes.
func TruncateLeft(s string, n int, prefix string) string {
	return truncateLeft(GraphemeWidth, s, n, prefix)
}

// TruncateLeftWc truncates a string from the left side by removing n characters,
// adding a prefix to the beginning if the string is longer than n.
// This function is aware of ANSI escape codes and will not break them, and
// accounts for wide-characters (such as East-Asian characters and emojis).
// This treats the text as a sequence of wide characters and runes.
func TruncateLeftWc(s string, n int, prefix string) string {
	return truncateLeft(WcWidth, s, n, prefix)
}

func truncateLeft(m Method, s string, n int, prefix string) string {
	if n <= 0 {
		return s
	}

	var cluster string
	var buf strings.Builder
	curWidth := 0
	ignoring := true
	pstate := parser.GroundState
	i := 0

	for i < len(s) {
		if !ignoring {
			buf.WriteString(s[i:])
			break
		}

		state, action := parser.Table.Transition(pstate, s[i])
		if state == parser.Utf8State {
			var width int
			cluster, width = FirstGraphemeCluster(s[i:], m)

			i += len(cluster)
			curWidth += width

			if curWidth > n && ignoring {
				ignoring = false
				buf.WriteString(prefix)
			}

			if curWidth > n {
				buf.WriteString(cluster)
			}

			if ignoring {
				continue
			}

			pstate = parser.GroundState
			continue
		}

		switch action {
		case parser.PrintAction:
			curWidth++

			if curWidth > n && ignoring {
				ignoring = false
				buf.WriteString(prefix)
			}

			if ignoring {
				i++
				continue
			}

			fallthrough
		case parser.ExecuteAction:
			// execute action will be things like \n, which, if outside the cut,
			// should be ignored.
			if ignoring {
				i++
				continue
			}
			fallthrough
		default:
			buf.WriteByte(s[i])
			i++
		}

		pstate = state
		if curWidth > n && ignoring {
			ignoring = false
			buf.WriteString(prefix)
		}
	}

	return buf.String()
}

// ByteToGraphemeRange takes start and stop byte positions and converts them to
// grapheme-aware char positions.
// You can use this with [Truncate], [TruncateLeft], and [Cut].
func ByteToGraphemeRange(str string, byteStart, byteStop int) (charStart, charStop int) {
	bytePos, charPos := 0, 0
	gr := graphemes.FromString(str)
	for byteStart > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Value())
		charPos += max(1, displaywidth.String(gr.Value()))
	}
	charStart = charPos
	for byteStop > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Value())
		charPos += max(1, displaywidth.String(gr.Value()))
	}
	charStop = charPos
	return charStart, charStop
}
