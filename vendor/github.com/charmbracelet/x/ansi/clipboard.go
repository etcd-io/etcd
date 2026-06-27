package ansi

import "encoding/base64"

// Clipboard names.
const (
	SystemClipboard  = 'c'
	PrimaryClipboard = 'p'
)

// SetClipboard returns a sequence for manipulating the clipboard.
//
//	OSC 52 ; Pc ; Pd ST
//	OSC 52 ; Pc ; Pd BEL
//
// Where Pc is the clipboard name and Pd is the base64 encoded data.
// Empty data or invalid base64 data will reset the clipboard.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
func SetClipboard(c byte, d string) string {
	if d != "" {
		d = base64.StdEncoding.EncodeToString([]byte(d))
	}
	return "\x1b]52;" + string(c) + ";" + d + "\x07"
}

// SetSystemClipboard returns a sequence for setting the system clipboard.
//
// This is equivalent to SetClipboard(SystemClipboard, d).
func SetSystemClipboard(d string) string {
	return SetClipboard(SystemClipboard, d)
}

// SetPrimaryClipboard returns a sequence for setting the primary clipboard.
//
// This is equivalent to SetClipboard(PrimaryClipboard, d).
func SetPrimaryClipboard(d string) string {
	return SetClipboard(PrimaryClipboard, d)
}

// ResetClipboard returns a sequence for resetting the clipboard.
//
// This is equivalent to SetClipboard(c, "").
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
func ResetClipboard(c byte) string {
	return SetClipboard(c, "")
}

// ResetSystemClipboard is a sequence for resetting the system clipboard.
//
// This is equivalent to ResetClipboard(SystemClipboard).
const ResetSystemClipboard = "\x1b]52;c;\x07"

// ResetPrimaryClipboard is a sequence for resetting the primary clipboard.
//
// This is equivalent to ResetClipboard(PrimaryClipboard).
const ResetPrimaryClipboard = "\x1b]52;p;\x07"

// RequestClipboard returns a sequence for requesting the clipboard.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
func RequestClipboard(c byte) string {
	return "\x1b]52;" + string(c) + ";?\x07"
}

// RequestSystemClipboard is a sequence for requesting the system clipboard.
//
// This is equivalent to RequestClipboard(SystemClipboard).
const RequestSystemClipboard = "\x1b]52;c;?\x07"

// RequestPrimaryClipboard is a sequence for requesting the primary clipboard.
//
// This is equivalent to RequestClipboard(PrimaryClipboard).
const RequestPrimaryClipboard = "\x1b]52;p;?\x07"
