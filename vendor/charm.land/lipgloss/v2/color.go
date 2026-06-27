package lipgloss

import (
	"cmp"
	"errors"
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/lucasb-eyer/go-colorful"
)

func clamp[T cmp.Ordered](v, low, high T) T {
	if high < low {
		high, low = low, high
	}
	return min(high, max(low, v))
}

// 4-bit color constants.
const (
	Black ansi.BasicColor = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White

	BrightBlack
	BrightRed
	BrightGreen
	BrightYellow
	BrightBlue
	BrightMagenta
	BrightCyan
	BrightWhite
)

var noColor = NoColor{}

// NoColor is used to specify the absence of color styling. When this is active
// foreground colors will be rendered with the terminal's default text color,
// and background colors will not be drawn at all.
//
// Example usage:
//
//	var style = someStyle.Background(lipgloss.NoColor{})
type NoColor struct{}

// RGBA returns the RGBA value of this color. Because we have to return
// something, despite this color being the absence of color, we're returning
// black with 100% opacity.
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
func (n NoColor) RGBA() (r, g, b, a uint32) {
	return 0x0, 0x0, 0x0, 0xFFFF //nolint:mnd
}

// Color specifies a color by hex or ANSI256 value. For example:
//
//	ansiColor := lipgloss.Color("1") // The same as lipgloss.Red
//	ansi256Color := lipgloss.Color("21")
//	hexColor := lipgloss.Color("#0000ff")
func Color(s string) color.Color {
	if strings.HasPrefix(s, "#") {
		c, err := parseHex(s)
		if err != nil {
			return noColor
		}
		return c
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		return noColor
	}

	if i < 0 {
		// Only positive numbers
		i = -i
	}

	if i < 16 {
		return ansi.BasicColor(i) //nolint:gosec
	} else if i < 256 {
		return ANSIColor(i) //nolint:gosec
	}

	r, g, b := uint8((i>>16)&0xff), uint8(i>>8&0xff), uint8(i&0xff) //nolint:gosec
	return color.RGBA{R: r, G: g, B: b, A: 0xff}
}

var errInvalidFormat = errors.New("invalid hex format") // pre-allocated.

// parseHex parses a hex color string and returns a color.RGBA. The string can be
// in the format #RRGGBB or #RGB. This is a more performant implementation of
// [colorful.Hex].
func parseHex(s string) (c color.RGBA, err error) {
	c.A = 0xff

	if len(s) == 0 || s[0] != '#' {
		return c, errInvalidFormat
	}

	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		err = errInvalidFormat
		return 0
	}

	switch len(s) {
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
		err = errInvalidFormat
	}
	return c, err
}

// RGBColor is a color specified by red, green, and blue values.
type RGBColor struct {
	R uint8
	G uint8
	B uint8
}

// RGBA returns the RGBA value of this color. This satisfies the Go Color
// interface.
func (c RGBColor) RGBA() (r, g, b, a uint32) {
	const shift = 8
	r |= uint32(c.R) << shift
	g |= uint32(c.G) << shift
	b |= uint32(c.B) << shift
	a = 0xFFFF
	return
}

// ANSIColor is a color specified by an ANSI256 color value.
//
// Example usage:
//
//	colorA := lipgloss.ANSIColor(8)
//	colorB := lipgloss.ANSIColor(134)
type ANSIColor = ansi.IndexedColor

// LightDarkFunc is a function that returns a color based on whether the
// terminal has a light or dark background. You can create one of these with
// [LightDark].
//
// Example:
//
//	lightDark := lipgloss.LightDark(hasDarkBackground)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
//
// For more info see [LightDark].
type LightDarkFunc func(light, dark color.Color) color.Color

// LightDark is a simple helper type that can be used to choose the appropriate
// color based on whether the terminal has a light or dark background.
//
//	lightDark := lipgloss.LightDark(hasDarkBackground)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
//
// In practice, there are slightly different workflows between Bubble Tea and
// Lip Gloss standalone.
//
// In Bubble Tea, listen for tea.BackgroundColorMsg, which automatically
// flows through Update on start. This message will be received whenever the
// background color changes:
//
//	case tea.BackgroundColorMsg:
//	    m.hasDarkBackground = msg.IsDark()
//
// Later, when you're rendering use:
//
//	lightDark := lipgloss.LightDark(m.hasDarkBackground)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
//
// In standalone Lip Gloss, the workflow is simpler:
//
//	hasDarkBG := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
//	lightDark := lipgloss.LightDark(hasDarkBG)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
func LightDark(isDark bool) LightDarkFunc {
	return func(light, dark color.Color) color.Color {
		if isDark {
			return dark
		}
		return light
	}
}

// isDarkColor returns whether the given color is dark (based on the luminance
// portion of the color as interpreted as HSL).
//
// Example usage:
//
//	color := lipgloss.Color("#0000ff")
//	if lipgloss.isDarkColor(color) {
//		fmt.Println("It's dark! I love darkness!")
//	} else {
//		fmt.Println("It's light! Cover your eyes!")
//	}
func isDarkColor(c color.Color) bool {
	col, ok := colorful.MakeColor(c)
	if !ok {
		return true
	}

	_, _, l := col.Hsl()
	return l < 0.5 //nolint:mnd
}

// CompleteFunc is a function that returns the appropriate color based on the
// given color profile.
//
// Example usage:
//
//	p := colorprofile.Detect(os.Stderr, os.Environ())
//	complete := lipgloss.Complete(p)
//	color := complete(
//		lipgloss.Color(1), // ANSI
//		lipgloss.Color(124), // ANSI256
//		lipgloss.Color("#ff34ac"), // TrueColor
//	)
//	fmt.Println("Ooh, pretty color: ", color)
//
// For more info see [Complete].
type CompleteFunc func(ansi, ansi256, truecolor color.Color) color.Color

// Complete returns a function that will return the appropriate color based on
// the given color profile.
//
// Example usage:
//
//	p := colorprofile.Detect(os.Stderr, os.Environ())
//	complete := lipgloss.Complete(p)
//	color := complete(
//	    lipgloss.Color(1), // ANSI
//	    lipgloss.Color(124), // ANSI256
//	    lipgloss.Color("#ff34ac"), // TrueColor
//	)
//	fmt.Println("Ooh, pretty color: ", color)
func Complete(p colorprofile.Profile) CompleteFunc {
	return func(ansi, ansi256, truecolor color.Color) color.Color {
		switch p { //nolint:exhaustive
		case colorprofile.ANSI:
			return ansi
		case colorprofile.ANSI256:
			return ansi256
		case colorprofile.TrueColor:
			return truecolor
		}
		return noColor
	}
}

// ensureNotTransparent ensures that the alpha value of a color is not 0, and if
// it is, we will set it to 1. This is useful for when we are converting from
// RGB -> RGBA, and the alpha value is lost in the conversion for gradient purposes.
func ensureNotTransparent(c color.Color) color.Color {
	_, _, _, a := c.RGBA()
	if a == 0 {
		return Alpha(c, 1)
	}
	return c
}

// Alpha adjusts the alpha value of a color using a 0-1 (clamped) float scale
// 0 = transparent, 1 = opaque.
func Alpha(c color.Color, alpha float64) color.Color {
	if c == nil {
		return nil
	}

	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: uint8(min(255, float64(r>>8))),
		G: uint8(min(255, float64(g>>8))),
		B: uint8(min(255, float64(b>>8))),
		A: uint8(clamp(alpha, 0, 1) * 255),
	}
}

// Complementary returns the complementary color (180° away on color wheel) of
// the given color. This is useful for creating a contrasting color.
func Complementary(c color.Color) color.Color {
	if c == nil {
		return nil
	}

	// Offset hue by 180°.
	cf, _ := colorful.MakeColor(ensureNotTransparent(c))

	h, s, v := cf.Hsv()
	h += 180
	if h >= 360 {
		h -= 360
	} else if h < 0 {
		h += 360
	}

	return colorful.Hsv(h, s, v).Clamped()
}

// Darken takes a color and makes it darker by a specific percentage (0-1, clamped).
func Darken(c color.Color, percent float64) color.Color {
	if c == nil {
		return nil
	}

	mult := 1.0 - clamp(percent, 0, 1)

	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(float64(r>>8) * mult),
		G: uint8(float64(g>>8) * mult),
		B: uint8(float64(b>>8) * mult),
		A: uint8(min(255, float64(a>>8))),
	}
}

// Lighten makes a color lighter by a specific percentage (0-1, clamped).
func Lighten(c color.Color, percent float64) color.Color {
	if c == nil {
		return nil
	}

	add := 255 * (clamp(percent, 0, 1))

	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(min(255, float64(r>>8)+add)),
		G: uint8(min(255, float64(g>>8)+add)),
		B: uint8(min(255, float64(b>>8)+add)),
		A: uint8(min(255, float64(a>>8))),
	}
}
