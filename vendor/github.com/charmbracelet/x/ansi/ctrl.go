package ansi

import (
	"strconv"
	"strings"
)

// RequestNameVersion (XTVERSION) is a control sequence that requests the
// terminal's name and version. It responds with a DSR sequence identifying the
// terminal.
//
//	CSI > 0 q
//	DCS > | text ST
//
// See https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-PC-Style-Function-Keys
const (
	RequestNameVersion = "\x1b[>q"
	XTVERSION          = RequestNameVersion
)

// RequestXTVersion is a control sequence that requests the terminal's XTVERSION. It responds with a DSR sequence identifying the version.
//
//	CSI > Ps q
//	DCS > | text ST
//
// See https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-PC-Style-Function-Keys
//
// Deprecated: use [RequestNameVersion] instead.
const RequestXTVersion = RequestNameVersion

// PrimaryDeviceAttributes (DA1) is a control sequence that reports the
// terminal's primary device attributes.
//
//	CSI c
//	CSI 0 c
//	CSI ? Ps ; ... c
//
// If no attributes are given, or if the attribute is 0, this function returns
// the request sequence. Otherwise, it returns the response sequence.
//
// Common attributes include:
//   - 1	132 columns
//   - 2	Printer port
//   - 4	Sixel
//   - 6	Selective erase
//   - 7	Soft character set (DRCS)
//   - 8	User-defined keys (UDKs)
//   - 9	National replacement character sets (NRCS) (International terminal only)
//   - 12	Yugoslavian (SCS)
//   - 15	Technical character set
//   - 18	Windowing capability
//   - 21	Horizontal scrolling
//   - 23	Greek
//   - 24	Turkish
//   - 42	ISO Latin-2 character set
//   - 44	PCTerm
//   - 45	Soft key map
//   - 46	ASCII emulation
//
// See https://vt100.net/docs/vt510-rm/DA1.html
func PrimaryDeviceAttributes(attrs ...int) string {
	if len(attrs) == 0 {
		return RequestPrimaryDeviceAttributes
	} else if len(attrs) == 1 && attrs[0] == 0 {
		return "\x1b[0c"
	}

	as := make([]string, len(attrs))
	for i, a := range attrs {
		as[i] = strconv.Itoa(a)
	}
	return "\x1b[?" + strings.Join(as, ";") + "c"
}

// DA1 is an alias for [PrimaryDeviceAttributes].
func DA1(attrs ...int) string {
	return PrimaryDeviceAttributes(attrs...)
}

// RequestPrimaryDeviceAttributes is a control sequence that requests the
// terminal's primary device attributes (DA1).
//
//	CSI c
//
// See https://vt100.net/docs/vt510-rm/DA1.html
const RequestPrimaryDeviceAttributes = "\x1b[c"

// SecondaryDeviceAttributes (DA2) is a control sequence that reports the
// terminal's secondary device attributes.
//
//	CSI > c
//	CSI > 0 c
//	CSI > Ps ; ... c
//
// See https://vt100.net/docs/vt510-rm/DA2.html
func SecondaryDeviceAttributes(attrs ...int) string {
	if len(attrs) == 0 {
		return RequestSecondaryDeviceAttributes
	}

	as := make([]string, len(attrs))
	for i, a := range attrs {
		as[i] = strconv.Itoa(a)
	}
	return "\x1b[>" + strings.Join(as, ";") + "c"
}

// DA2 is an alias for [SecondaryDeviceAttributes].
func DA2(attrs ...int) string {
	return SecondaryDeviceAttributes(attrs...)
}

// RequestSecondaryDeviceAttributes is a control sequence that requests the
// terminal's secondary device attributes (DA2).
//
//	CSI > c
//
// See https://vt100.net/docs/vt510-rm/DA2.html
const RequestSecondaryDeviceAttributes = "\x1b[>c"

// TertiaryDeviceAttributes (DA3) is a control sequence that reports the
// terminal's tertiary device attributes.
//
//	CSI = c
//	CSI = 0 c
//	DCS ! | Text ST
//
// Where Text is the unit ID for the terminal.
//
// If no unit ID is given, or if the unit ID is 0, this function returns the
// request sequence. Otherwise, it returns the response sequence.
//
// See https://vt100.net/docs/vt510-rm/DA3.html
func TertiaryDeviceAttributes(unitID string) string {
	switch unitID {
	case "":
		return RequestTertiaryDeviceAttributes
	case "0":
		return "\x1b[=0c"
	}

	return "\x1bP!|" + unitID + "\x1b\\"
}

// DA3 is an alias for [TertiaryDeviceAttributes].
func DA3(unitID string) string {
	return TertiaryDeviceAttributes(unitID)
}

// RequestTertiaryDeviceAttributes is a control sequence that requests the
// terminal's tertiary device attributes (DA3).
//
//	CSI = c
//
// See https://vt100.net/docs/vt510-rm/DA3.html
const RequestTertiaryDeviceAttributes = "\x1b[=c"
