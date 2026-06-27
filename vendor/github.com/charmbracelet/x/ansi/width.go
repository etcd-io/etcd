package ansi

import (
	"bytes"

	"github.com/charmbracelet/x/ansi/parser"
)

// Strip removes ANSI escape codes from a string.
func Strip(s string) string {
	var (
		buf    bytes.Buffer         // buffer for collecting printable characters
		ri     int                  // rune index
		rw     int                  // rune width
		pstate = parser.GroundState // initial state
	)

	// This implements a subset of the Parser to only collect runes and
	// printable characters.
	for i := range len(s) {
		if pstate == parser.Utf8State {
			// During this state, collect rw bytes to form a valid rune in the
			// buffer. After getting all the rune bytes into the buffer,
			// transition to GroundState and reset the counters.
			buf.WriteByte(s[i])
			ri++
			if ri < rw {
				continue
			}
			pstate = parser.GroundState
			ri = 0
			rw = 0
			continue
		}

		state, action := parser.Table.Transition(pstate, s[i])
		switch action {
		case parser.CollectAction:
			if state == parser.Utf8State {
				// This action happens when we transition to the Utf8State.
				rw = utf8ByteLen(s[i])
				buf.WriteByte(s[i])
				ri++
			}
		case parser.PrintAction, parser.ExecuteAction:
			// collects printable ASCII and non-printable characters
			buf.WriteByte(s[i])
		}

		// Transition to the next state.
		// The Utf8State is managed separately above.
		if pstate != parser.Utf8State {
			pstate = state
		}
	}

	return buf.String()
}

// StringWidth returns the width of a string in cells. This is the number of
// cells that the string will occupy when printed in a terminal. ANSI escape
// codes are ignored and wide characters (such as East Asians and emojis) are
// accounted for.
// This treats the text as a sequence of grapheme clusters.
func StringWidth(s string) int {
	return stringWidth(GraphemeWidth, s)
}

// StringWidthWc returns the width of a string in cells. This is the number of
// cells that the string will occupy when printed in a terminal. ANSI escape
// codes are ignored and wide characters (such as East Asians and emojis) are
// accounted for.
// This treats the text as a sequence of wide characters and runes.
func StringWidthWc(s string) int {
	return stringWidth(WcWidth, s)
}

func stringWidth(m Method, s string) int {
	if s == "" {
		return 0
	}

	var (
		pstate = parser.GroundState // initial state
		width  int
	)

	for i := 0; i < len(s); i++ {
		state, action := parser.Table.Transition(pstate, s[i])
		if action == parser.PrintAction || state == parser.Utf8State {
			cluster, w := FirstGraphemeCluster(s[i:], m)
			width += w

			i += len(cluster) - 1
			pstate = parser.GroundState
			continue
		}

		if action == parser.PrintAction {
			width++
		}

		pstate = state
	}

	return width
}
