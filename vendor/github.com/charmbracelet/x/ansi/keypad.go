package ansi

// Keypad Application Mode (DECKPAM) is a mode that determines whether the
// keypad sends application sequences or ANSI sequences.
//
// This works like enabling [DECNKM].
// Use [NumericKeypadMode] to set the numeric keypad mode.
//
//	ESC =
//
// See: https://vt100.net/docs/vt510-rm/DECKPAM.html
const (
	KeypadApplicationMode = "\x1b="
	DECKPAM               = KeypadApplicationMode
)

// Keypad Numeric Mode (DECKPNM) is a mode that determines whether the keypad
// sends application sequences or ANSI sequences.
//
// This works the same as disabling [DECNKM].
//
//	ESC >
//
// See: https://vt100.net/docs/vt510-rm/DECKPNM.html
const (
	KeypadNumericMode = "\x1b>"
	DECKPNM           = KeypadNumericMode
)
