package ansi

import (
	"strconv"
)

// SaveCursor (DECSC) is an escape sequence that saves the current cursor
// position.
//
//	ESC 7
//
// See: https://vt100.net/docs/vt510-rm/DECSC.html
const (
	SaveCursor = "\x1b7"
	DECSC      = SaveCursor
)

// RestoreCursor (DECRC) is an escape sequence that restores the cursor
// position.
//
//	ESC 8
//
// See: https://vt100.net/docs/vt510-rm/DECRC.html
const (
	RestoreCursor = "\x1b8"
	DECRC         = RestoreCursor
)

// RequestCursorPosition is an escape sequence that requests the current cursor
// position.
//
//	CSI 6 n
//
// The terminal will report the cursor position as a CSI sequence in the
// following format:
//
//	CSI Pl ; Pc R
//
// Where Pl is the line number and Pc is the column number.
// See: https://vt100.net/docs/vt510-rm/CPR.html
//
// Deprecated: use [RequestCursorPositionReport] instead.
const RequestCursorPosition = "\x1b[6n"

// RequestExtendedCursorPosition (DECXCPR) is a sequence for requesting the
// cursor position report including the current page number.
//
//	CSI ? 6 n
//
// The terminal will report the cursor position as a CSI sequence in the
// following format:
//
//	CSI ? Pl ; Pc ; Pp R
//
// Where Pl is the line number, Pc is the column number, and Pp is the page
// number.
// See: https://vt100.net/docs/vt510-rm/DECXCPR.html
//
// Deprecated: use [RequestExtendedCursorPositionReport] instead.
const RequestExtendedCursorPosition = "\x1b[?6n"

// CursorUp (CUU) returns a sequence for moving the cursor up n cells.
//
//	CSI n A
//
// See: https://vt100.net/docs/vt510-rm/CUU.html
func CursorUp(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "A"
}

// CUU is an alias for [CursorUp].
func CUU(n int) string {
	return CursorUp(n)
}

// CUU1 is a sequence for moving the cursor up one cell.
const CUU1 = "\x1b[A"

// CursorUp1 is a sequence for moving the cursor up one cell.
//
// This is equivalent to CursorUp(1).
//
// Deprecated: use [CUU1] instead.
const CursorUp1 = "\x1b[A"

// CursorDown (CUD) returns a sequence for moving the cursor down n cells.
//
//	CSI n B
//
// See: https://vt100.net/docs/vt510-rm/CUD.html
func CursorDown(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "B"
}

// CUD is an alias for [CursorDown].
func CUD(n int) string {
	return CursorDown(n)
}

// CUD1 is a sequence for moving the cursor down one cell.
const CUD1 = "\x1b[B"

// CursorDown1 is a sequence for moving the cursor down one cell.
//
// This is equivalent to CursorDown(1).
//
// Deprecated: use [CUD1] instead.
const CursorDown1 = "\x1b[B"

// CursorForward (CUF) returns a sequence for moving the cursor right n cells.
//
// # CSI n C
//
// See: https://vt100.net/docs/vt510-rm/CUF.html
func CursorForward(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "C"
}

// CUF is an alias for [CursorForward].
func CUF(n int) string {
	return CursorForward(n)
}

// CUF1 is a sequence for moving the cursor right one cell.
const CUF1 = "\x1b[C"

// CursorRight (CUF) returns a sequence for moving the cursor right n cells.
//
//	CSI n C
//
// See: https://vt100.net/docs/vt510-rm/CUF.html
//
// Deprecated: use [CursorForward] instead.
func CursorRight(n int) string {
	return CursorForward(n)
}

// CursorRight1 is a sequence for moving the cursor right one cell.
//
// This is equivalent to CursorRight(1).
//
// Deprecated: use [CUF1] instead.
const CursorRight1 = CUF1

// CursorBackward (CUB) returns a sequence for moving the cursor left n cells.
//
// # CSI n D
//
// See: https://vt100.net/docs/vt510-rm/CUB.html
func CursorBackward(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "D"
}

// CUB is an alias for [CursorBackward].
func CUB(n int) string {
	return CursorBackward(n)
}

// CUB1 is a sequence for moving the cursor left one cell.
const CUB1 = "\x1b[D"

// CursorLeft (CUB) returns a sequence for moving the cursor left n cells.
//
//	CSI n D
//
// See: https://vt100.net/docs/vt510-rm/CUB.html
//
// Deprecated: use [CursorBackward] instead.
func CursorLeft(n int) string {
	return CursorBackward(n)
}

// CursorLeft1 is a sequence for moving the cursor left one cell.
//
// This is equivalent to CursorLeft(1).
//
// Deprecated: use [CUB1] instead.
const CursorLeft1 = CUB1

// CursorNextLine (CNL) returns a sequence for moving the cursor to the
// beginning of the next line n times.
//
//	CSI n E
//
// See: https://vt100.net/docs/vt510-rm/CNL.html
func CursorNextLine(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "E"
}

// CNL is an alias for [CursorNextLine].
func CNL(n int) string {
	return CursorNextLine(n)
}

// CursorPreviousLine (CPL) returns a sequence for moving the cursor to the
// beginning of the previous line n times.
//
//	CSI n F
//
// See: https://vt100.net/docs/vt510-rm/CPL.html
func CursorPreviousLine(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "F"
}

// CPL is an alias for [CursorPreviousLine].
func CPL(n int) string {
	return CursorPreviousLine(n)
}

// CursorHorizontalAbsolute (CHA) returns a sequence for moving the cursor to
// the given column.
//
// Default is 1.
//
//	CSI n G
//
// See: https://vt100.net/docs/vt510-rm/CHA.html
func CursorHorizontalAbsolute(col int) string {
	var s string
	if col > 0 {
		s = strconv.Itoa(col)
	}
	return "\x1b[" + s + "G"
}

// CHA is an alias for [CursorHorizontalAbsolute].
func CHA(col int) string {
	return CursorHorizontalAbsolute(col)
}

// CursorPosition (CUP) returns a sequence for setting the cursor to the
// given row and column.
//
// Default is 1,1.
//
//	CSI n ; m H
//
// See: https://vt100.net/docs/vt510-rm/CUP.html
func CursorPosition(col, row int) string {
	if row <= 1 && col <= 1 {
		return CursorHomePosition
	}

	var r, c string
	if row > 0 {
		r = strconv.Itoa(row)
	}
	if col > 0 {
		c = strconv.Itoa(col)
	}
	return "\x1b[" + r + ";" + c + "H"
}

// CUP is an alias for [CursorPosition].
func CUP(col, row int) string {
	return CursorPosition(col, row)
}

// CursorHomePosition is a sequence for moving the cursor to the upper left
// corner of the scrolling region.
//
// This is equivalent to [CursorPosition](1, 1).
const CursorHomePosition = "\x1b[H"

// SetCursorPosition (CUP) returns a sequence for setting the cursor to the
// given row and column.
//
//	CSI n ; m H
//
// See: https://vt100.net/docs/vt510-rm/CUP.html
//
// Deprecated: use [CursorPosition] instead.
func SetCursorPosition(col, row int) string {
	if row <= 0 && col <= 0 {
		return HomeCursorPosition
	}

	var r, c string
	if row > 0 {
		r = strconv.Itoa(row)
	}
	if col > 0 {
		c = strconv.Itoa(col)
	}
	return "\x1b[" + r + ";" + c + "H"
}

// HomeCursorPosition is a sequence for moving the cursor to the upper left
// corner of the scrolling region. This is equivalent to `SetCursorPosition(1, 1)`.
//
// Deprecated: use [CursorHomePosition] instead.
const HomeCursorPosition = CursorHomePosition

// MoveCursor (CUP) returns a sequence for setting the cursor to the
// given row and column.
//
//	CSI n ; m H
//
// See: https://vt100.net/docs/vt510-rm/CUP.html
//
// Deprecated: use [CursorPosition] instead.
func MoveCursor(col, row int) string {
	return SetCursorPosition(col, row)
}

// CursorOrigin is a sequence for moving the cursor to the upper left corner of
// the display. This is equivalent to `SetCursorPosition(1, 1)`.
//
// Deprecated: use [CursorHomePosition] instead.
const CursorOrigin = "\x1b[1;1H"

// MoveCursorOrigin is a sequence for moving the cursor to the upper left
// corner of the display. This is equivalent to `SetCursorPosition(1, 1)`.
//
// Deprecated: use [CursorHomePosition] instead.
const MoveCursorOrigin = CursorOrigin

// CursorHorizontalForwardTab (CHT) returns a sequence for moving the cursor to
// the next tab stop n times.
//
// Default is 1.
//
//	CSI n I
//
// See: https://vt100.net/docs/vt510-rm/CHT.html
func CursorHorizontalForwardTab(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "I"
}

// CHT is an alias for [CursorHorizontalForwardTab].
func CHT(n int) string {
	return CursorHorizontalForwardTab(n)
}

// EraseCharacter (ECH) returns a sequence for erasing n characters from the
// screen. This doesn't affect other cell attributes.
//
// Default is 1.
//
//	CSI n X
//
// See: https://vt100.net/docs/vt510-rm/ECH.html
func EraseCharacter(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "X"
}

// ECH is an alias for [EraseCharacter].
func ECH(n int) string {
	return EraseCharacter(n)
}

// CursorBackwardTab (CBT) returns a sequence for moving the cursor to the
// previous tab stop n times.
//
// Default is 1.
//
//	CSI n Z
//
// See: https://vt100.net/docs/vt510-rm/CBT.html
func CursorBackwardTab(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "Z"
}

// CBT is an alias for [CursorBackwardTab].
func CBT(n int) string {
	return CursorBackwardTab(n)
}

// VerticalPositionAbsolute (VPA) returns a sequence for moving the cursor to
// the given row.
//
// Default is 1.
//
//	CSI n d
//
// See: https://vt100.net/docs/vt510-rm/VPA.html
func VerticalPositionAbsolute(row int) string {
	var s string
	if row > 0 {
		s = strconv.Itoa(row)
	}
	return "\x1b[" + s + "d"
}

// VPA is an alias for [VerticalPositionAbsolute].
func VPA(row int) string {
	return VerticalPositionAbsolute(row)
}

// VerticalPositionRelative (VPR) returns a sequence for moving the cursor down
// n rows relative to the current position.
//
// Default is 1.
//
//	CSI n e
//
// See: https://vt100.net/docs/vt510-rm/VPR.html
func VerticalPositionRelative(n int) string {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "e"
}

// VPR is an alias for [VerticalPositionRelative].
func VPR(n int) string {
	return VerticalPositionRelative(n)
}

// HorizontalVerticalPosition (HVP) returns a sequence for moving the cursor to
// the given row and column.
//
// Default is 1,1.
//
//	CSI n ; m f
//
// This has the same effect as [CursorPosition].
//
// See: https://vt100.net/docs/vt510-rm/HVP.html
func HorizontalVerticalPosition(col, row int) string {
	var r, c string
	if row > 0 {
		r = strconv.Itoa(row)
	}
	if col > 0 {
		c = strconv.Itoa(col)
	}
	return "\x1b[" + r + ";" + c + "f"
}

// HVP is an alias for [HorizontalVerticalPosition].
func HVP(col, row int) string {
	return HorizontalVerticalPosition(col, row)
}

// HorizontalVerticalHomePosition is a sequence for moving the cursor to the
// upper left corner of the scrolling region. This is equivalent to
// `HorizontalVerticalPosition(1, 1)`.
const HorizontalVerticalHomePosition = "\x1b[f"

// SaveCurrentCursorPosition (SCOSC) is a sequence for saving the current cursor
// position for SCO console mode.
//
//	CSI s
//
// This acts like [DECSC], except the page number where the cursor is located
// is not saved.
//
// See: https://vt100.net/docs/vt510-rm/SCOSC.html
const (
	SaveCurrentCursorPosition = "\x1b[s"
	SCOSC                     = SaveCurrentCursorPosition
)

// SaveCursorPosition (SCP or SCOSC) is a sequence for saving the cursor
// position.
//
//	CSI s
//
// This acts like Save, except the page number where the cursor is located is
// not saved.
//
// See: https://vt100.net/docs/vt510-rm/SCOSC.html
//
// Deprecated: use [SaveCurrentCursorPosition] instead.
const SaveCursorPosition = "\x1b[s"

// RestoreCurrentCursorPosition (SCORC) is a sequence for restoring the current
// cursor position for SCO console mode.
//
//	CSI u
//
// This acts like [DECRC], except the page number where the cursor was saved is
// not restored.
//
// See: https://vt100.net/docs/vt510-rm/SCORC.html
const (
	RestoreCurrentCursorPosition = "\x1b[u"
	SCORC                        = RestoreCurrentCursorPosition
)

// RestoreCursorPosition (RCP or SCORC) is a sequence for restoring the cursor
// position.
//
//	CSI u
//
// This acts like Restore, except the cursor stays on the same page where the
// cursor was saved.
//
// See: https://vt100.net/docs/vt510-rm/SCORC.html
//
// Deprecated: use [RestoreCurrentCursorPosition] instead.
const RestoreCursorPosition = "\x1b[u"

// SetCursorStyle (DECSCUSR) returns a sequence for changing the cursor style.
//
// Default is 1.
//
//	CSI Ps SP q
//
// Where Ps is the cursor style:
//
//	0: Blinking block
//	1: Blinking block (default)
//	2: Steady block
//	3: Blinking underline
//	4: Steady underline
//	5: Blinking bar (xterm)
//	6: Steady bar (xterm)
//
// See: https://vt100.net/docs/vt510-rm/DECSCUSR.html
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h4-Functions-using-CSI-_-ordered-by-the-final-character-lparen-s-rparen:CSI-Ps-SP-q.1D81
func SetCursorStyle(style int) string {
	if style < 0 {
		style = 0
	}
	return "\x1b[" + strconv.Itoa(style) + " q"
}

// DECSCUSR is an alias for [SetCursorStyle].
func DECSCUSR(style int) string {
	return SetCursorStyle(style)
}

// SetPointerShape returns a sequence for changing the mouse pointer cursor
// shape. Use "default" for the default pointer shape.
//
//	OSC 22 ; Pt ST
//	OSC 22 ; Pt BEL
//
// Where Pt is the pointer shape name. The name can be anything that the
// operating system can understand. Some common names are:
//
//   - copy
//   - crosshair
//   - default
//   - ew-resize
//   - n-resize
//   - text
//   - wait
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Operating-System-Commands
func SetPointerShape(shape string) string {
	return "\x1b]22;" + shape + "\x07"
}

// ReverseIndex (RI) is an escape sequence for moving the cursor up one line in
// the same column. If the cursor is at the top margin, the screen scrolls
// down.
//
// This has the same effect as [RI].
const ReverseIndex = "\x1bM"

// HorizontalPositionAbsolute (HPA) returns a sequence for moving the cursor to
// the given column. This has the same effect as [CUP].
//
// Default is 1.
//
//	CSI n \`
//
// See: https://vt100.net/docs/vt510-rm/HPA.html
func HorizontalPositionAbsolute(col int) string {
	var s string
	if col > 0 {
		s = strconv.Itoa(col)
	}
	return "\x1b[" + s + "`"
}

// HPA is an alias for [HorizontalPositionAbsolute].
func HPA(col int) string {
	return HorizontalPositionAbsolute(col)
}

// HorizontalPositionRelative (HPR) returns a sequence for moving the cursor
// right n columns relative to the current position. This has the same effect
// as [CUP].
//
// Default is 1.
//
//	CSI n a
//
// See: https://vt100.net/docs/vt510-rm/HPR.html
func HorizontalPositionRelative(n int) string {
	var s string
	if n > 0 {
		s = strconv.Itoa(n)
	}
	return "\x1b[" + s + "a"
}

// HPR is an alias for [HorizontalPositionRelative].
func HPR(n int) string {
	return HorizontalPositionRelative(n)
}

// Index (IND) is an escape sequence for moving the cursor down one line in the
// same column. If the cursor is at the bottom margin, the screen scrolls up.
// This has the same effect as [IND].
const Index = "\x1bD"
