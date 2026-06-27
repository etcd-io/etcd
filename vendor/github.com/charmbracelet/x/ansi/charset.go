package ansi

// SelectCharacterSet sets the G-set character designator to the specified
// character set.
//
//	ESC Ps Pd
//
// Where Ps is the G-set character designator, and Pd is the identifier.
// For 94-character sets, the designator can be one of:
//   - ( G0
//   - ) G1
//   - * G2
//   - + G3
//
// For 96-character sets, the designator can be one of:
//   - - G1
//   - . G2
//   - / G3
//
// Some common 94-character sets are:
//   - 0 DEC Special Drawing Set
//   - A United Kingdom (UK)
//   - B United States (USASCII)
//
// Examples:
//
//	ESC ( B  Select character set G0 = United States (USASCII)
//	ESC ( 0  Select character set G0 = Special Character and Line Drawing Set
//	ESC ) 0  Select character set G1 = Special Character and Line Drawing Set
//	ESC * A  Select character set G2 = United Kingdom (UK)
//
// See: https://vt100.net/docs/vt510-rm/SCS.html
func SelectCharacterSet(gset byte, charset byte) string {
	return "\x1b" + string(gset) + string(charset)
}

// SCS is an alias for SelectCharacterSet.
func SCS(gset byte, charset byte) string {
	return SelectCharacterSet(gset, charset)
}

// LS1R (Locking Shift 1 Right) shifts G1 into GR character set.
const LS1R = "\x1b~"

// LS2 (Locking Shift 2) shifts G2 into GL character set.
const LS2 = "\x1bn"

// LS2R (Locking Shift 2 Right) shifts G2 into GR character set.
const LS2R = "\x1b}"

// LS3 (Locking Shift 3) shifts G3 into GL character set.
const LS3 = "\x1bo"

// LS3R (Locking Shift 3 Right) shifts G3 into GR character set.
const LS3R = "\x1b|"
