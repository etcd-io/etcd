package ansi

import (
	"fmt"
	"image/color"

	"github.com/lucasb-eyer/go-colorful"
)

// HexColor is a [color.Color] that can be formatted as a hex string.
type HexColor string

// RGBA returns the RGBA values of the color.
func (h HexColor) RGBA() (r, g, b, a uint32) {
	hex := h.color()
	if hex == nil {
		return 0, 0, 0, 0
	}
	return hex.RGBA()
}

// Hex returns the hex representation of the color. If the color is invalid, it
// returns an empty string.
func (h HexColor) Hex() string {
	hex := h.color()
	if hex == nil {
		return ""
	}
	return hex.Hex()
}

// String returns the color as a hex string. If the color is nil, an empty
// string is returned.
func (h HexColor) String() string {
	return h.Hex()
}

// color returns the underlying color of the HexColor.
func (h HexColor) color() *colorful.Color {
	hex, err := colorful.Hex(string(h))
	if err != nil {
		return nil
	}
	return &hex
}

// XRGBColor is a [color.Color] that can be formatted as an XParseColor
// rgb: string.
//
// See: https://linux.die.net/man/3/xparsecolor
type XRGBColor struct {
	color.Color
}

// RGBA returns the RGBA values of the color.
func (x XRGBColor) RGBA() (r, g, b, a uint32) {
	if x.Color == nil {
		return 0, 0, 0, 0
	}
	return x.Color.RGBA()
}

// String returns the color as an XParseColor rgb: string. If the color is nil,
// an empty string is returned.
func (x XRGBColor) String() string {
	if x.Color == nil {
		return ""
	}
	r, g, b, _ := x.Color.RGBA()
	// Get the lower 8 bits
	return fmt.Sprintf("rgb:%04x/%04x/%04x", r, g, b)
}

// XRGBAColor is a [color.Color] that can be formatted as an XParseColor
// rgba: string.
//
// See: https://linux.die.net/man/3/xparsecolor
type XRGBAColor struct {
	color.Color
}

// RGBA returns the RGBA values of the color.
func (x XRGBAColor) RGBA() (r, g, b, a uint32) {
	if x.Color == nil {
		return 0, 0, 0, 0
	}
	return x.Color.RGBA()
}

// String returns the color as an XParseColor rgba: string. If the color is nil,
// an empty string is returned.
func (x XRGBAColor) String() string {
	if x.Color == nil {
		return ""
	}
	r, g, b, a := x.RGBA()
	// Get the lower 8 bits
	return fmt.Sprintf("rgba:%04x/%04x/%04x/%04x", r, g, b, a)
}

// SetForegroundColor returns a sequence that sets the default terminal
// foreground color.
//
//	OSC 10 ; color ST
//	OSC 10 ; color BEL
//
// Where color is the encoded color number. Most terminals support hex,
// XParseColor rgb: and rgba: strings. You could use [HexColor], [XRGBColor],
// or [XRGBAColor] to format the color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
func SetForegroundColor(s string) string {
	return "\x1b]10;" + s + "\x07"
}

// RequestForegroundColor is a sequence that requests the current default
// terminal foreground color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
const RequestForegroundColor = "\x1b]10;?\x07"

// ResetForegroundColor is a sequence that resets the default terminal
// foreground color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
const ResetForegroundColor = "\x1b]110\x07"

// SetBackgroundColor returns a sequence that sets the default terminal
// background color.
//
//	OSC 11 ; color ST
//	OSC 11 ; color BEL
//
// Where color is the encoded color number. Most terminals support hex,
// XParseColor rgb: and rgba: strings. You could use [HexColor], [XRGBColor],
// or [XRGBAColor] to format the color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
func SetBackgroundColor(s string) string {
	return "\x1b]11;" + s + "\x07"
}

// RequestBackgroundColor is a sequence that requests the current default
// terminal background color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
const RequestBackgroundColor = "\x1b]11;?\x07"

// ResetBackgroundColor is a sequence that resets the default terminal
// background color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
const ResetBackgroundColor = "\x1b]111\x07"

// SetCursorColor returns a sequence that sets the terminal cursor color.
//
//	OSC 12 ; color ST
//	OSC 12 ; color BEL
//
// Where color is the encoded color number. Most terminals support hex,
// XParseColor rgb: and rgba: strings. You could use [HexColor], [XRGBColor],
// or [XRGBAColor] to format the color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
func SetCursorColor(s string) string {
	return "\x1b]12;" + s + "\x07"
}

// RequestCursorColor is a sequence that requests the current terminal cursor
// color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
const RequestCursorColor = "\x1b]12;?\x07"

// ResetCursorColor is a sequence that resets the terminal cursor color.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
const ResetCursorColor = "\x1b]112\x07"
