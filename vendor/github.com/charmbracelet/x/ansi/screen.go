package ansi

import (
	"strconv"
	"strings"
)

// EraseDisplay (ED) clears the display or parts of the display. A screen is
// the shown part of the terminal display excluding the scrollback buffer.
// Possible values:
//
// Default is 0.
//
//	 0: Clear from cursor to end of screen.
//	 1: Clear from cursor to beginning of the screen.
//	 2: Clear entire screen (and moves cursor to upper left on DOS).
//	 3: Clear entire display which delete all lines saved in the scrollback buffer (xterm).
//
//	CSI <n> J
//
// See: https://vt100.net/docs/vt510-rm/ED.html
func EraseDisplay(n int) string {
	var s string
	if n > 0 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "J"
}

// ED is an alias for [EraseDisplay].
func ED(n int) string {
	return EraseDisplay(n)
}

// EraseDisplay constants.
// These are the possible values for the EraseDisplay function.
const (
	EraseScreenBelow   = "\x1b[J"
	EraseScreenAbove   = "\x1b[1J"
	EraseEntireScreen  = "\x1b[2J"
	EraseEntireDisplay = "\x1b[3J"
)

// EraseLine (EL) clears the current line or parts of the line. Possible values:
//
//	0: Clear from cursor to end of line.
//	1: Clear from cursor to beginning of the line.
//	2: Clear entire line.
//
// The cursor position is not affected.
//
//	CSI <n> K
//
// See: https://vt100.net/docs/vt510-rm/EL.html
func EraseLine(n int) string {
	var s string
	if n > 0 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "K"
}

// EL is an alias for [EraseLine].
func EL(n int) string {
	return EraseLine(n)
}

// EraseLine constants.
// These are the possible values for the EraseLine function.
const (
	EraseLineRight  = "\x1b[K"
	EraseLineLeft   = "\x1b[1K"
	EraseEntireLine = "\x1b[2K"
)

// ScrollUp (SU) scrolls the screen up n lines. New lines are added at the
// bottom of the screen.
//
//	CSI Pn S
//
// See: https://vt100.net/docs/vt510-rm/SU.html
func ScrollUp(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "S"
}

// PanDown is an alias for [ScrollUp].
func PanDown(n int) string {
	return ScrollUp(n)
}

// SU is an alias for [ScrollUp].
func SU(n int) string {
	return ScrollUp(n)
}

// ScrollDown (SD) scrolls the screen down n lines. New lines are added at the
// top of the screen.
//
//	CSI Pn T
//
// See: https://vt100.net/docs/vt510-rm/SD.html
func ScrollDown(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "T"
}

// PanUp is an alias for [ScrollDown].
func PanUp(n int) string {
	return ScrollDown(n)
}

// SD is an alias for [ScrollDown].
func SD(n int) string {
	return ScrollDown(n)
}

// InsertLine (IL) inserts n blank lines at the current cursor position.
// Existing lines are moved down.
//
//	CSI Pn L
//
// See: https://vt100.net/docs/vt510-rm/IL.html
func InsertLine(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "L"
}

// IL is an alias for [InsertLine].
func IL(n int) string {
	return InsertLine(n)
}

// DeleteLine (DL) deletes n lines at the current cursor position. Existing
// lines are moved up.
//
//	CSI Pn M
//
// See: https://vt100.net/docs/vt510-rm/DL.html
func DeleteLine(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "M"
}

// DL is an alias for [DeleteLine].
func DL(n int) string {
	return DeleteLine(n)
}

// SetTopBottomMargins (DECSTBM) sets the top and bottom margins for the scrolling
// region. The default is the entire screen.
//
// Default is 1 and the bottom of the screen.
//
//	CSI Pt ; Pb r
//
// See: https://vt100.net/docs/vt510-rm/DECSTBM.html
func SetTopBottomMargins(top, bot int) string {
	var t, b string
	if top > 0 {
		t = strconv.Itoa(top)
	}
	if bot > 0 {
		b = strconv.Itoa(bot)
	}
	return "\x1b[" + t + ";" + b + "r"
}

// DECSTBM is an alias for [SetTopBottomMargins].
func DECSTBM(top, bot int) string {
	return SetTopBottomMargins(top, bot)
}

// SetLeftRightMargins (DECSLRM) sets the left and right margins for the scrolling
// region.
//
// Default is 1 and the right of the screen.
//
//	CSI Pl ; Pr s
//
// See: https://vt100.net/docs/vt510-rm/DECSLRM.html
func SetLeftRightMargins(left, right int) string {
	var l, r string
	if left > 0 {
		l = strconv.Itoa(left)
	}
	if right > 0 {
		r = strconv.Itoa(right)
	}
	return "\x1b[" + l + ";" + r + "s"
}

// DECSLRM is an alias for [SetLeftRightMargins].
func DECSLRM(left, right int) string {
	return SetLeftRightMargins(left, right)
}

// SetScrollingRegion (DECSTBM) sets the top and bottom margins for the scrolling
// region. The default is the entire screen.
//
//	CSI <top> ; <bottom> r
//
// See: https://vt100.net/docs/vt510-rm/DECSTBM.html
//
// Deprecated: use [SetTopBottomMargins] instead.
func SetScrollingRegion(t, b int) string {
	if t < 0 {
		t = 0
	}
	if b < 0 {
		b = 0
	}
	return "\x1b[" + strconv.Itoa(t) + ";" + strconv.Itoa(b) + "r"
}

// InsertCharacter (ICH) inserts n blank characters at the current cursor
// position. Existing characters move to the right. Characters moved past the
// right margin are lost. ICH has no effect outside the scrolling margins.
//
// Default is 1.
//
//	CSI Pn @
//
// See: https://vt100.net/docs/vt510-rm/ICH.html
func InsertCharacter(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "@"
}

// ICH is an alias for [InsertCharacter].
func ICH(n int) string {
	return InsertCharacter(n)
}

// DeleteCharacter (DCH) deletes n characters at the current cursor position.
// As the characters are deleted, the remaining characters move to the left and
// the cursor remains at the same position.
//
// Default is 1.
//
//	CSI Pn P
//
// See: https://vt100.net/docs/vt510-rm/DCH.html
func DeleteCharacter(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "P"
}

// DCH is an alias for [DeleteCharacter].
func DCH(n int) string {
	return DeleteCharacter(n)
}

// SetTabEvery8Columns (DECST8C) sets the tab stops at every 8 columns.
//
//	CSI ? 5 W
//
// See: https://vt100.net/docs/vt510-rm/DECST8C.html
const (
	SetTabEvery8Columns = "\x1b[?5W"
	DECST8C             = SetTabEvery8Columns
)

// HorizontalTabSet (HTS) sets a horizontal tab stop at the current cursor
// column.
//
// This is equivalent to [HTS].
//
//	ESC H
//
// See: https://vt100.net/docs/vt510-rm/HTS.html
const HorizontalTabSet = "\x1bH"

// TabClear (TBC) clears tab stops.
//
// Default is 0.
//
// Possible values:
// 0: Clear tab stop at the current column. (default)
// 3: Clear all tab stops.
//
//	CSI Pn g
//
// See: https://vt100.net/docs/vt510-rm/TBC.html
func TabClear(n int) string {
	var s string
	if n > 0 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "g"
}

// TBC is an alias for [TabClear].
func TBC(n int) string {
	return TabClear(n)
}

// RequestPresentationStateReport (DECRQPSR) requests the terminal to send a
// report of the presentation state. This includes the cursor information [DECCIR],
// and tab stop [DECTABSR] reports.
//
// Default is 0.
//
// Possible values:
// 0: Error, request ignored.
// 1: Cursor information report [DECCIR].
// 2: Tab stop report [DECTABSR].
//
//	CSI Ps $ w
//
// See: https://vt100.net/docs/vt510-rm/DECRQPSR.html
func RequestPresentationStateReport(n int) string {
	var s string
	if n > 0 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "$w"
}

// DECRQPSR is an alias for [RequestPresentationStateReport].
func DECRQPSR(n int) string {
	return RequestPresentationStateReport(n)
}

// TabStopReport (DECTABSR) is the response to a tab stop report request.
// It reports the tab stops set in the terminal.
//
// The response is a list of tab stops separated by a slash (/) character.
//
//	DCS 2 $ u D ... D ST
//
// Where D is a decimal number representing a tab stop.
//
// See: https://vt100.net/docs/vt510-rm/DECTABSR.html
func TabStopReport(stops ...int) string {
	var s []string //nolint:prealloc
	for _, v := range stops {
		s = append(s, strconv.Itoa(v))
	}
	return "\x1bP2$u" + strings.Join(s, "/") + "\x1b\\"
}

// DECTABSR is an alias for [TabStopReport].
func DECTABSR(stops ...int) string {
	return TabStopReport(stops...)
}

// CursorInformationReport (DECCIR) is the response to a cursor information
// report request. It reports the cursor position, visual attributes, and
// character protection attributes. It also reports the status of origin mode
// [DECOM] and the current active character set.
//
// The response is a list of values separated by a semicolon (;) character.
//
//	DCS 1 $ u D ... D ST
//
// Where D is a decimal number representing a value.
//
// See: https://vt100.net/docs/vt510-rm/DECCIR.html
func CursorInformationReport(values ...int) string {
	var s []string //nolint:prealloc
	for _, v := range values {
		s = append(s, strconv.Itoa(v))
	}
	return "\x1bP1$u" + strings.Join(s, ";") + "\x1b\\"
}

// DECCIR is an alias for [CursorInformationReport].
func DECCIR(values ...int) string {
	return CursorInformationReport(values...)
}

// RepeatPreviousCharacter (REP) repeats the previous character n times.
// This is identical to typing the same character n times.
//
// Default is 1.
//
//	CSI Pn b
//
// See: ECMA-48 § 8.3.103.
func RepeatPreviousCharacter(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "b"
}

// REP is an alias for [RepeatPreviousCharacter].
func REP(n int) string {
	return RepeatPreviousCharacter(n)
}
