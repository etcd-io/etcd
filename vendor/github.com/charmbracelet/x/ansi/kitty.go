package ansi

import "strconv"

// Kitty keyboard protocol progressive enhancement flags.
// See: https://sw.kovidgoyal.net/kitty/keyboard-protocol/#progressive-enhancement
const (
	KittyDisambiguateEscapeCodes = 1 << iota
	KittyReportEventTypes
	KittyReportAlternateKeys
	KittyReportAllKeysAsEscapeCodes
	KittyReportAssociatedKeys

	KittyAllFlags = KittyDisambiguateEscapeCodes | KittyReportEventTypes |
		KittyReportAlternateKeys | KittyReportAllKeysAsEscapeCodes | KittyReportAssociatedKeys
)

// RequestKittyKeyboard is a sequence to request the terminal Kitty keyboard
// protocol enabled flags.
//
// See: https://sw.kovidgoyal.net/kitty/keyboard-protocol/
const RequestKittyKeyboard = "\x1b[?u"

// KittyKeyboard returns a sequence to request keyboard enhancements from the terminal.
// The flags argument is a bitmask of the Kitty keyboard protocol flags. While
// mode specifies how the flags should be interpreted.
//
// Possible values for flags mask:
//
//	1:  Disambiguate escape codes
//	2:  Report event types
//	4:  Report alternate keys
//	8:  Report all keys as escape codes
//	16: Report associated text
//
// Possible values for mode:
//
//	1: Set given flags and unset all others
//	2: Set given flags and keep existing flags unchanged
//	3: Unset given flags and keep existing flags unchanged
//
// See https://sw.kovidgoyal.net/kitty/keyboard-protocol/#progressive-enhancement
func KittyKeyboard(flags, mode int) string {
	return "\x1b[=" + strconv.Itoa(flags) + ";" + strconv.Itoa(mode) + "u"
}

// PushKittyKeyboard returns a sequence to push the given flags to the terminal
// Kitty Keyboard stack.
//
// Possible values for flags mask:
//
//	0:  Disable all features
//	1:  Disambiguate escape codes
//	2:  Report event types
//	4:  Report alternate keys
//	8:  Report all keys as escape codes
//	16: Report associated text
//
//	CSI > flags u
//
// See https://sw.kovidgoyal.net/kitty/keyboard-protocol/#progressive-enhancement
func PushKittyKeyboard(flags int) string {
	var f string
	if flags > 0 {
		f = strconv.Itoa(flags)
	}

	return "\x1b[>" + f + "u"
}

// DisableKittyKeyboard is a sequence to push zero into the terminal Kitty
// Keyboard stack to disable the protocol.
//
// This is equivalent to PushKittyKeyboard(0).
const DisableKittyKeyboard = "\x1b[>u"

// PopKittyKeyboard returns a sequence to pop n number of flags from the
// terminal Kitty Keyboard stack.
//
//	CSI < flags u
//
// See https://sw.kovidgoyal.net/kitty/keyboard-protocol/#progressive-enhancement
func PopKittyKeyboard(n int) string {
	var num string
	if n > 0 {
		num = strconv.Itoa(n)
	}

	return "\x1b[<" + num + "u"
}
