package ansi

import (
	"fmt"
	"image/color"
)

// SetPalette sets the palette color for the given index. The index is a 16
// color index between 0 and 15. The color is a 24-bit RGB color.
//
//	OSC P n rrggbb BEL
//
// Where n is the color index in hex (0-f), and rrggbb is the color in
// hexadecimal format (e.g., ff0000 for red).
//
// This sequence is specific to the Linux Console and may not work in other
// terminal emulators.
//
// See https://man7.org/linux/man-pages/man4/console_codes.4.html
func SetPalette(i int, c color.Color) string {
	if c == nil || i < 0 || i > 15 {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b]P%x%02x%02x%02x\x07", i, r>>8, g>>8, b>>8)
}

// ResetPalette resets the color palette to the default values.
//
// This sequence is specific to the Linux Console and may not work in other
// terminal emulators.
//
// See https://man7.org/linux/man-pages/man4/console_codes.4.html
const ResetPalette = "\x1b]R\x07"
