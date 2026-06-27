package ansi

import (
	"strconv"
	"strings"
)

// StatusReport represents a terminal status report.
type StatusReport interface {
	// StatusReport returns the status report identifier.
	StatusReport() int
}

// ANSIStatusReport represents an ANSI terminal status report.
type ANSIStatusReport int //nolint:revive

// StatusReport returns the status report identifier.
func (s ANSIStatusReport) StatusReport() int {
	return int(s)
}

// DECStatusReport represents a DEC terminal status report.
type DECStatusReport int

// StatusReport returns the status report identifier.
func (s DECStatusReport) StatusReport() int {
	return int(s)
}

// DeviceStatusReport (DSR) is a control sequence that reports the terminal's
// status.
// The terminal responds with a DSR sequence.
//
//	CSI Ps n
//	CSI ? Ps n
//
// If one of the statuses is a [DECStatus], the sequence will use the DEC
// format.
//
// See also https://vt100.net/docs/vt510-rm/DSR.html
func DeviceStatusReport(statues ...StatusReport) string {
	var dec bool
	list := make([]string, len(statues))
	seq := "\x1b["
	for i, status := range statues {
		list[i] = strconv.Itoa(status.StatusReport())
		switch status.(type) {
		case DECStatusReport:
			dec = true
		}
	}
	if dec {
		seq += "?"
	}
	return seq + strings.Join(list, ";") + "n"
}

// DSR is an alias for [DeviceStatusReport].
func DSR(status StatusReport) string {
	return DeviceStatusReport(status)
}

// RequestCursorPositionReport is an escape sequence that requests the current
// cursor position.
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
const RequestCursorPositionReport = "\x1b[6n"

// RequestExtendedCursorPositionReport (DECXCPR) is a sequence for requesting
// the cursor position report including the current page number.
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
const RequestExtendedCursorPositionReport = "\x1b[?6n"

// RequestLightDarkReport is a control sequence that requests the terminal to
// report its operating system light/dark color preference. Supported terminals
// should respond with a [LightDarkReport] sequence as follows:
//
//	CSI ? 997 ; 1 n   for dark mode
//	CSI ? 997 ; 2 n   for light mode
//
// See: https://contour-terminal.org/vt-extensions/color-palette-update-notifications/
const RequestLightDarkReport = "\x1b[?996n"

// CursorPositionReport (CPR) is a control sequence that reports the cursor's
// position.
//
//	CSI Pl ; Pc R
//
// Where Pl is the line number and Pc is the column number.
//
// See also https://vt100.net/docs/vt510-rm/CPR.html
func CursorPositionReport(line, column int) string {
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	return "\x1b[" + strconv.Itoa(line) + ";" + strconv.Itoa(column) + "R"
}

// CPR is an alias for [CursorPositionReport].
func CPR(line, column int) string {
	return CursorPositionReport(line, column)
}

// ExtendedCursorPositionReport (DECXCPR) is a control sequence that reports the
// cursor's position along with the page number (optional).
//
//	CSI ? Pl ; Pc R
//	CSI ? Pl ; Pc ; Pv R
//
// Where Pl is the line number, Pc is the column number, and Pv is the page
// number.
//
// If the page number is zero or negative, the returned sequence won't include
// the page number.
//
// See also https://vt100.net/docs/vt510-rm/DECXCPR.html
func ExtendedCursorPositionReport(line, column, page int) string {
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	if page < 1 {
		return "\x1b[?" + strconv.Itoa(line) + ";" + strconv.Itoa(column) + "R"
	}
	return "\x1b[?" + strconv.Itoa(line) + ";" + strconv.Itoa(column) + ";" + strconv.Itoa(page) + "R"
}

// DECXCPR is an alias for [ExtendedCursorPositionReport].
func DECXCPR(line, column, page int) string {
	return ExtendedCursorPositionReport(line, column, page)
}

// LightDarkReport is a control sequence that reports the terminal's operating
// system light/dark color preference.
//
//	CSI ? 997 ; 1 n   for dark mode
//	CSI ? 997 ; 2 n   for light mode
//
// See: https://contour-terminal.org/vt-extensions/color-palette-update-notifications/
func LightDarkReport(dark bool) string {
	if dark {
		return "\x1b[?997;1n"
	}
	return "\x1b[?997;2n"
}
