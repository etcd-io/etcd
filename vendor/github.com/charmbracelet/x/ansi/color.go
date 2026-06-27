package ansi

import (
	"image/color"

	"github.com/lucasb-eyer/go-colorful"
)

// Color is a color that can be used in a terminal. ANSI (including
// ANSI256) and 24-bit "true colors" fall under this category.
type Color interface {
	color.Color
}

// BasicColor is an ANSI 3-bit or 4-bit color with a value from 0 to 15.
type BasicColor uint8

var _ Color = BasicColor(0)

const (
	// Black is the ANSI black color.
	Black BasicColor = iota

	// Red is the ANSI red color.
	Red

	// Green is the ANSI green color.
	Green

	// Yellow is the ANSI yellow color.
	Yellow

	// Blue is the ANSI blue color.
	Blue

	// Magenta is the ANSI magenta color.
	Magenta

	// Cyan is the ANSI cyan color.
	Cyan

	// White is the ANSI white color.
	White

	// BrightBlack is the ANSI bright black color.
	BrightBlack

	// BrightRed is the ANSI bright red color.
	BrightRed

	// BrightGreen is the ANSI bright green color.
	BrightGreen

	// BrightYellow is the ANSI bright yellow color.
	BrightYellow

	// BrightBlue is the ANSI bright blue color.
	BrightBlue

	// BrightMagenta is the ANSI bright magenta color.
	BrightMagenta

	// BrightCyan is the ANSI bright cyan color.
	BrightCyan

	// BrightWhite is the ANSI bright white color.
	BrightWhite
)

// RGBA returns the red, green, blue and alpha components of the color. It
// satisfies the color.Color interface.
func (c BasicColor) RGBA() (uint32, uint32, uint32, uint32) {
	ansi := uint32(c)
	if ansi > 15 {
		return 0, 0, 0, 0xffff
	}

	return ansiToRGB(byte(ansi)).RGBA()
}

// IndexedColor is an ANSI 256 (8-bit) color with a value from 0 to 255.
type IndexedColor uint8

var _ Color = IndexedColor(0)

// RGBA returns the red, green, blue and alpha components of the color. It
// satisfies the color.Color interface.
func (c IndexedColor) RGBA() (uint32, uint32, uint32, uint32) {
	return ansiToRGB(byte(c)).RGBA()
}

// ExtendedColor is an ANSI 256 (8-bit) color with a value from 0 to 255.
//
// Deprecated: use [IndexedColor] instead.
type ExtendedColor = IndexedColor

// TrueColor is a 24-bit color that can be used in the terminal.
// This can be used to represent RGB colors.
//
// For example, the color red can be represented as:
//
//	TrueColor(0xff0000)
//
// Deprecated: use [RGBColor] instead.
type TrueColor uint32

var _ Color = TrueColor(0)

// RGBA returns the red, green, blue and alpha components of the color. It
// satisfies the color.Color interface.
func (c TrueColor) RGBA() (uint32, uint32, uint32, uint32) {
	r, g, b := hexToRGB(uint32(c))
	return toRGBA(r, g, b)
}

// RGBColor is a 24-bit color that can be used in the terminal.
// This can be used to represent RGB colors.
type RGBColor struct {
	R uint8
	G uint8
	B uint8
}

// RGBA returns the red, green, blue and alpha components of the color. It
// satisfies the color.Color interface.
func (c RGBColor) RGBA() (uint32, uint32, uint32, uint32) {
	return toRGBA(uint32(c.R), uint32(c.G), uint32(c.B))
}

// ansiToRGB converts an ANSI color to a 24-bit RGB color.
//
//	r, g, b := ansiToRGB(57)
func ansiToRGB(ansi byte) color.Color {
	return ansiHex[ansi]
}

// hexToRGB converts a number in hexadecimal format to red, green, and blue
// values.
//
//	r, g, b := hexToRGB(0x0000FF)
func hexToRGB(hex uint32) (uint32, uint32, uint32) {
	return hex >> 16 & 0xff, hex >> 8 & 0xff, hex & 0xff
}

// toRGBA converts an RGB 8-bit color values to 32-bit color values suitable
// for color.Color.
//
// color.Color requires 16-bit color values, so we duplicate the 8-bit values
// to fill the 16-bit values.
//
// This always returns 0xffff (opaque) for the alpha channel.
func toRGBA(r, g, b uint32) (uint32, uint32, uint32, uint32) {
	r |= r << 8
	g |= g << 8
	b |= b << 8
	return r, g, b, 0xffff
}

//nolint:unused
func distSq(r1, g1, b1, r2, g2, b2 int) int {
	return ((r1-r2)*(r1-r2) + (g1-g2)*(g1-g2) + (b1-b2)*(b1-b2))
}

func to6Cube[T int | float64](v T) int {
	if v < 48 {
		return 0
	}
	if v < 115 {
		return 1
	}
	return int((v - 35) / 40)
}

// Convert256 converts a [color.Color], usually a 24-bit color, to xterm(1) 256
// color palette.
//
// xterm provides a 6x6x6 color cube (16 - 231) and 24 greys (232 - 255). We
// map our RGB color to the closest in the cube, also work out the closest
// grey, and use the nearest of the two based on the lightness of the color.
//
// Note that the xterm has much lower resolution for darker colors (they are
// not evenly spread out), so our 6 levels are not evenly spread: 0x0, 0x5f
// (95), 0x87 (135), 0xaf (175), 0xd7 (215) and 0xff (255). Greys are more
// evenly spread (8, 18, 28 ... 238).
func Convert256(c color.Color) IndexedColor {
	// If the color is already an IndexedColor, return it.
	if i, ok := c.(IndexedColor); ok {
		return i
	}

	// Note: this is mostly ported from tmux/colour.c.
	col, ok := colorful.MakeColor(c)
	if !ok {
		return IndexedColor(0)
	}

	r := col.R * 255
	g := col.G * 255
	b := col.B * 255

	q2c := [6]int{0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff}

	// Map RGB to 6x6x6 cube.
	qr := to6Cube(r)
	cr := q2c[qr]
	qg := to6Cube(g)
	cg := q2c[qg]
	qb := to6Cube(b)
	cb := q2c[qb]

	// If we have hit the color exactly, return early.
	ci := (36 * qr) + (6 * qg) + qb
	if cr == int(r) && cg == int(g) && cb == int(b) {
		return IndexedColor(16 + ci) //nolint:gosec
	}

	// Work out the closest grey (average of RGB).
	greyAvg := int(r+g+b) / 3
	var greyIdx int
	if greyAvg > 238 {
		greyIdx = 23
	} else {
		greyIdx = (greyAvg - 3) / 10
	}
	grey := 8 + (10 * greyIdx)

	// Return the one which is nearer to the original input rgb value
	// XXX: This is where it differs from tmux's implementation, we prefer the
	// closer color to the original in terms of light distances rather than the
	// cube distance.
	c2 := colorful.Color{R: float64(cr) / 255.0, G: float64(cg) / 255.0, B: float64(cb) / 255.0}
	g2 := colorful.Color{R: float64(grey) / 255.0, G: float64(grey) / 255.0, B: float64(grey) / 255.0}
	colorDist := col.DistanceHSLuv(c2)
	grayDist := col.DistanceHSLuv(g2)

	if colorDist <= grayDist {
		return IndexedColor(16 + ci) //nolint:gosec
	}
	return IndexedColor(232 + greyIdx) //nolint:gosec

	// // Is grey or 6x6x6 color closest?
	// d := distSq(cr, cg, cb, int(r), int(g), int(b))
	// if distSq(grey, grey, grey, int(r), int(g), int(b)) < d {
	// 	return IndexedColor(232 + greyIdx) //nolint:gosec
	// }
	// return IndexedColor(16 + ci) //nolint:gosec
}

// Convert16 converts a [color.Color] to a 16-color ANSI color. It will first
// try to find a match in the 256 xterm(1) color palette, and then map that to
// the 16-color ANSI palette.
func Convert16(c color.Color) BasicColor {
	switch c := c.(type) {
	case BasicColor:
		// If the color is already a BasicColor, return it.
		return c
	case IndexedColor:
		// If the color is already an IndexedColor, return the corresponding
		// BasicColor.
		return ansi256To16[c]
	default:
		c256 := Convert256(c)
		return ansi256To16[c256]
	}
}

// RGB values of ANSI colors (0-255).
var ansiHex = [...]color.RGBA{
	0:   {R: 0x00, G: 0x00, B: 0x00, A: 0xff}, //   "#000000"
	1:   {R: 0x80, G: 0x00, B: 0x00, A: 0xff}, //   "#800000"
	2:   {R: 0x00, G: 0x80, B: 0x00, A: 0xff}, //   "#008000"
	3:   {R: 0x80, G: 0x80, B: 0x00, A: 0xff}, //   "#808000"
	4:   {R: 0x00, G: 0x00, B: 0x80, A: 0xff}, //   "#000080"
	5:   {R: 0x80, G: 0x00, B: 0x80, A: 0xff}, //   "#800080"
	6:   {R: 0x00, G: 0x80, B: 0x80, A: 0xff}, //   "#008080"
	7:   {R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}, //   "#c0c0c0"
	8:   {R: 0x80, G: 0x80, B: 0x80, A: 0xff}, //   "#808080"
	9:   {R: 0xff, G: 0x00, B: 0x00, A: 0xff}, //   "#ff0000"
	10:  {R: 0x00, G: 0xff, B: 0x00, A: 0xff}, //  "#00ff00"
	11:  {R: 0xff, G: 0xff, B: 0x00, A: 0xff}, //  "#ffff00"
	12:  {R: 0x00, G: 0x00, B: 0xff, A: 0xff}, //  "#0000ff"
	13:  {R: 0xff, G: 0x00, B: 0xff, A: 0xff}, //  "#ff00ff"
	14:  {R: 0x00, G: 0xff, B: 0xff, A: 0xff}, //  "#00ffff"
	15:  {R: 0xff, G: 0xff, B: 0xff, A: 0xff}, //  "#ffffff"
	16:  {R: 0x00, G: 0x00, B: 0x00, A: 0xff}, //  "#000000"
	17:  {R: 0x00, G: 0x00, B: 0x5f, A: 0xff}, //  "#00005f"
	18:  {R: 0x00, G: 0x00, B: 0x87, A: 0xff}, //  "#000087"
	19:  {R: 0x00, G: 0x00, B: 0xaf, A: 0xff}, //  "#0000af"
	20:  {R: 0x00, G: 0x00, B: 0xd7, A: 0xff}, //  "#0000d7"
	21:  {R: 0x00, G: 0x00, B: 0xff, A: 0xff}, //  "#0000ff"
	22:  {R: 0x00, G: 0x5f, B: 0x00, A: 0xff}, //  "#005f00"
	23:  {R: 0x00, G: 0x5f, B: 0x5f, A: 0xff}, //  "#005f5f"
	24:  {R: 0x00, G: 0x5f, B: 0x87, A: 0xff}, //  "#005f87"
	25:  {R: 0x00, G: 0x5f, B: 0xaf, A: 0xff}, //  "#005faf"
	26:  {R: 0x00, G: 0x5f, B: 0xd7, A: 0xff}, //  "#005fd7"
	27:  {R: 0x00, G: 0x5f, B: 0xff, A: 0xff}, //  "#005fff"
	28:  {R: 0x00, G: 0x87, B: 0x00, A: 0xff}, //  "#008700"
	29:  {R: 0x00, G: 0x87, B: 0x5f, A: 0xff}, //  "#00875f"
	30:  {R: 0x00, G: 0x87, B: 0x87, A: 0xff}, //  "#008787"
	31:  {R: 0x00, G: 0x87, B: 0xaf, A: 0xff}, //  "#0087af"
	32:  {R: 0x00, G: 0x87, B: 0xd7, A: 0xff}, //  "#0087d7"
	33:  {R: 0x00, G: 0x87, B: 0xff, A: 0xff}, //  "#0087ff"
	34:  {R: 0x00, G: 0xaf, B: 0x00, A: 0xff}, //  "#00af00"
	35:  {R: 0x00, G: 0xaf, B: 0x5f, A: 0xff}, //  "#00af5f"
	36:  {R: 0x00, G: 0xaf, B: 0x87, A: 0xff}, //  "#00af87"
	37:  {R: 0x00, G: 0xaf, B: 0xaf, A: 0xff}, //  "#00afaf"
	38:  {R: 0x00, G: 0xaf, B: 0xd7, A: 0xff}, //  "#00afd7"
	39:  {R: 0x00, G: 0xaf, B: 0xff, A: 0xff}, //  "#00afff"
	40:  {R: 0x00, G: 0xd7, B: 0x00, A: 0xff}, //  "#00d700"
	41:  {R: 0x00, G: 0xd7, B: 0x5f, A: 0xff}, //  "#00d75f"
	42:  {R: 0x00, G: 0xd7, B: 0x87, A: 0xff}, //  "#00d787"
	43:  {R: 0x00, G: 0xd7, B: 0xaf, A: 0xff}, //  "#00d7af"
	44:  {R: 0x00, G: 0xd7, B: 0xd7, A: 0xff}, //  "#00d7d7"
	45:  {R: 0x00, G: 0xd7, B: 0xff, A: 0xff}, //  "#00d7ff"
	46:  {R: 0x00, G: 0xff, B: 0x00, A: 0xff}, //  "#00ff00"
	47:  {R: 0x00, G: 0xff, B: 0x5f, A: 0xff}, //  "#00ff5f"
	48:  {R: 0x00, G: 0xff, B: 0x87, A: 0xff}, //  "#00ff87"
	49:  {R: 0x00, G: 0xff, B: 0xaf, A: 0xff}, //  "#00ffaf"
	50:  {R: 0x00, G: 0xff, B: 0xd7, A: 0xff}, //  "#00ffd7"
	51:  {R: 0x00, G: 0xff, B: 0xff, A: 0xff}, //  "#00ffff"
	52:  {R: 0x5f, G: 0x00, B: 0x00, A: 0xff}, //  "#5f0000"
	53:  {R: 0x5f, G: 0x00, B: 0x5f, A: 0xff}, //  "#5f005f"
	54:  {R: 0x5f, G: 0x00, B: 0x87, A: 0xff}, //  "#5f0087"
	55:  {R: 0x5f, G: 0x00, B: 0xaf, A: 0xff}, //  "#5f00af"
	56:  {R: 0x5f, G: 0x00, B: 0xd7, A: 0xff}, //  "#5f00d7"
	57:  {R: 0x5f, G: 0x00, B: 0xff, A: 0xff}, //  "#5f00ff"
	58:  {R: 0x5f, G: 0x5f, B: 0x00, A: 0xff}, //  "#5f5f00"
	59:  {R: 0x5f, G: 0x5f, B: 0x5f, A: 0xff}, //  "#5f5f5f"
	60:  {R: 0x5f, G: 0x5f, B: 0x87, A: 0xff}, //  "#5f5f87"
	61:  {R: 0x5f, G: 0x5f, B: 0xaf, A: 0xff}, //  "#5f5faf"
	62:  {R: 0x5f, G: 0x5f, B: 0xd7, A: 0xff}, //  "#5f5fd7"
	63:  {R: 0x5f, G: 0x5f, B: 0xff, A: 0xff}, //  "#5f5fff"
	64:  {R: 0x5f, G: 0x87, B: 0x00, A: 0xff}, //  "#5f8700"
	65:  {R: 0x5f, G: 0x87, B: 0x5f, A: 0xff}, //  "#5f875f"
	66:  {R: 0x5f, G: 0x87, B: 0x87, A: 0xff}, //  "#5f8787"
	67:  {R: 0x5f, G: 0x87, B: 0xaf, A: 0xff}, //  "#5f87af"
	68:  {R: 0x5f, G: 0x87, B: 0xd7, A: 0xff}, //  "#5f87d7"
	69:  {R: 0x5f, G: 0x87, B: 0xff, A: 0xff}, //  "#5f87ff"
	70:  {R: 0x5f, G: 0xaf, B: 0x00, A: 0xff}, //  "#5faf00"
	71:  {R: 0x5f, G: 0xaf, B: 0x5f, A: 0xff}, //  "#5faf5f"
	72:  {R: 0x5f, G: 0xaf, B: 0x87, A: 0xff}, //  "#5faf87"
	73:  {R: 0x5f, G: 0xaf, B: 0xaf, A: 0xff}, //  "#5fafaf"
	74:  {R: 0x5f, G: 0xaf, B: 0xd7, A: 0xff}, //  "#5fafd7"
	75:  {R: 0x5f, G: 0xaf, B: 0xff, A: 0xff}, //  "#5fafff"
	76:  {R: 0x5f, G: 0xd7, B: 0x00, A: 0xff}, //  "#5fd700"
	77:  {R: 0x5f, G: 0xd7, B: 0x5f, A: 0xff}, //  "#5fd75f"
	78:  {R: 0x5f, G: 0xd7, B: 0x87, A: 0xff}, //  "#5fd787"
	79:  {R: 0x5f, G: 0xd7, B: 0xaf, A: 0xff}, //  "#5fd7af"
	80:  {R: 0x5f, G: 0xd7, B: 0xd7, A: 0xff}, //  "#5fd7d7"
	81:  {R: 0x5f, G: 0xd7, B: 0xff, A: 0xff}, //  "#5fd7ff"
	82:  {R: 0x5f, G: 0xff, B: 0x00, A: 0xff}, //  "#5fff00"
	83:  {R: 0x5f, G: 0xff, B: 0x5f, A: 0xff}, //  "#5fff5f"
	84:  {R: 0x5f, G: 0xff, B: 0x87, A: 0xff}, //  "#5fff87"
	85:  {R: 0x5f, G: 0xff, B: 0xaf, A: 0xff}, //  "#5fffaf"
	86:  {R: 0x5f, G: 0xff, B: 0xd7, A: 0xff}, //  "#5fffd7"
	87:  {R: 0x5f, G: 0xff, B: 0xff, A: 0xff}, //  "#5fffff"
	88:  {R: 0x87, G: 0x00, B: 0x00, A: 0xff}, //  "#870000"
	89:  {R: 0x87, G: 0x00, B: 0x5f, A: 0xff}, //  "#87005f"
	90:  {R: 0x87, G: 0x00, B: 0x87, A: 0xff}, //  "#870087"
	91:  {R: 0x87, G: 0x00, B: 0xaf, A: 0xff}, //  "#8700af"
	92:  {R: 0x87, G: 0x00, B: 0xd7, A: 0xff}, //  "#8700d7"
	93:  {R: 0x87, G: 0x00, B: 0xff, A: 0xff}, //  "#8700ff"
	94:  {R: 0x87, G: 0x5f, B: 0x00, A: 0xff}, //  "#875f00"
	95:  {R: 0x87, G: 0x5f, B: 0x5f, A: 0xff}, //  "#875f5f"
	96:  {R: 0x87, G: 0x5f, B: 0x87, A: 0xff}, //  "#875f87"
	97:  {R: 0x87, G: 0x5f, B: 0xaf, A: 0xff}, //  "#875faf"
	98:  {R: 0x87, G: 0x5f, B: 0xd7, A: 0xff}, //  "#875fd7"
	99:  {R: 0x87, G: 0x5f, B: 0xff, A: 0xff}, //  "#875fff"
	100: {R: 0x87, G: 0x87, B: 0x00, A: 0xff}, // "#878700"
	101: {R: 0x87, G: 0x87, B: 0x5f, A: 0xff}, // "#87875f"
	102: {R: 0x87, G: 0x87, B: 0x87, A: 0xff}, // "#878787"
	103: {R: 0x87, G: 0x87, B: 0xaf, A: 0xff}, // "#8787af"
	104: {R: 0x87, G: 0x87, B: 0xd7, A: 0xff}, // "#8787d7"
	105: {R: 0x87, G: 0x87, B: 0xff, A: 0xff}, // "#8787ff"
	106: {R: 0x87, G: 0xaf, B: 0x00, A: 0xff}, // "#87af00"
	107: {R: 0x87, G: 0xaf, B: 0x5f, A: 0xff}, // "#87af5f"
	108: {R: 0x87, G: 0xaf, B: 0x87, A: 0xff}, // "#87af87"
	109: {R: 0x87, G: 0xaf, B: 0xaf, A: 0xff}, // "#87afaf"
	110: {R: 0x87, G: 0xaf, B: 0xd7, A: 0xff}, // "#87afd7"
	111: {R: 0x87, G: 0xaf, B: 0xff, A: 0xff}, // "#87afff"
	112: {R: 0x87, G: 0xd7, B: 0x00, A: 0xff}, // "#87d700"
	113: {R: 0x87, G: 0xd7, B: 0x5f, A: 0xff}, // "#87d75f"
	114: {R: 0x87, G: 0xd7, B: 0x87, A: 0xff}, // "#87d787"
	115: {R: 0x87, G: 0xd7, B: 0xaf, A: 0xff}, // "#87d7af"
	116: {R: 0x87, G: 0xd7, B: 0xd7, A: 0xff}, // "#87d7d7"
	117: {R: 0x87, G: 0xd7, B: 0xff, A: 0xff}, // "#87d7ff"
	118: {R: 0x87, G: 0xff, B: 0x00, A: 0xff}, // "#87ff00"
	119: {R: 0x87, G: 0xff, B: 0x5f, A: 0xff}, // "#87ff5f"
	120: {R: 0x87, G: 0xff, B: 0x87, A: 0xff}, // "#87ff87"
	121: {R: 0x87, G: 0xff, B: 0xaf, A: 0xff}, // "#87ffaf"
	122: {R: 0x87, G: 0xff, B: 0xd7, A: 0xff}, // "#87ffd7"
	123: {R: 0x87, G: 0xff, B: 0xff, A: 0xff}, // "#87ffff"
	124: {R: 0xaf, G: 0x00, B: 0x00, A: 0xff}, // "#af0000"
	125: {R: 0xaf, G: 0x00, B: 0x5f, A: 0xff}, // "#af005f"
	126: {R: 0xaf, G: 0x00, B: 0x87, A: 0xff}, // "#af0087"
	127: {R: 0xaf, G: 0x00, B: 0xaf, A: 0xff}, // "#af00af"
	128: {R: 0xaf, G: 0x00, B: 0xd7, A: 0xff}, // "#af00d7"
	129: {R: 0xaf, G: 0x00, B: 0xff, A: 0xff}, // "#af00ff"
	130: {R: 0xaf, G: 0x5f, B: 0x00, A: 0xff}, // "#af5f00"
	131: {R: 0xaf, G: 0x5f, B: 0x5f, A: 0xff}, // "#af5f5f"
	132: {R: 0xaf, G: 0x5f, B: 0x87, A: 0xff}, // "#af5f87"
	133: {R: 0xaf, G: 0x5f, B: 0xaf, A: 0xff}, // "#af5faf"
	134: {R: 0xaf, G: 0x5f, B: 0xd7, A: 0xff}, // "#af5fd7"
	135: {R: 0xaf, G: 0x5f, B: 0xff, A: 0xff}, // "#af5fff"
	136: {R: 0xaf, G: 0x87, B: 0x00, A: 0xff}, // "#af8700"
	137: {R: 0xaf, G: 0x87, B: 0x5f, A: 0xff}, // "#af875f"
	138: {R: 0xaf, G: 0x87, B: 0x87, A: 0xff}, // "#af8787"
	139: {R: 0xaf, G: 0x87, B: 0xaf, A: 0xff}, // "#af87af"
	140: {R: 0xaf, G: 0x87, B: 0xd7, A: 0xff}, // "#af87d7"
	141: {R: 0xaf, G: 0x87, B: 0xff, A: 0xff}, // "#af87ff"
	142: {R: 0xaf, G: 0xaf, B: 0x00, A: 0xff}, // "#afaf00"
	143: {R: 0xaf, G: 0xaf, B: 0x5f, A: 0xff}, // "#afaf5f"
	144: {R: 0xaf, G: 0xaf, B: 0x87, A: 0xff}, // "#afaf87"
	145: {R: 0xaf, G: 0xaf, B: 0xaf, A: 0xff}, // "#afafaf"
	146: {R: 0xaf, G: 0xaf, B: 0xd7, A: 0xff}, // "#afafd7"
	147: {R: 0xaf, G: 0xaf, B: 0xff, A: 0xff}, // "#afafff"
	148: {R: 0xaf, G: 0xd7, B: 0x00, A: 0xff}, // "#afd700"
	149: {R: 0xaf, G: 0xd7, B: 0x5f, A: 0xff}, // "#afd75f"
	150: {R: 0xaf, G: 0xd7, B: 0x87, A: 0xff}, // "#afd787"
	151: {R: 0xaf, G: 0xd7, B: 0xaf, A: 0xff}, // "#afd7af"
	152: {R: 0xaf, G: 0xd7, B: 0xd7, A: 0xff}, // "#afd7d7"
	153: {R: 0xaf, G: 0xd7, B: 0xff, A: 0xff}, // "#afd7ff"
	154: {R: 0xaf, G: 0xff, B: 0x00, A: 0xff}, // "#afff00"
	155: {R: 0xaf, G: 0xff, B: 0x5f, A: 0xff}, // "#afff5f"
	156: {R: 0xaf, G: 0xff, B: 0x87, A: 0xff}, // "#afff87"
	157: {R: 0xaf, G: 0xff, B: 0xaf, A: 0xff}, // "#afffaf"
	158: {R: 0xaf, G: 0xff, B: 0xd7, A: 0xff}, // "#afffd7"
	159: {R: 0xaf, G: 0xff, B: 0xff, A: 0xff}, // "#afffff"
	160: {R: 0xd7, G: 0x00, B: 0x00, A: 0xff}, // "#d70000"
	161: {R: 0xd7, G: 0x00, B: 0x5f, A: 0xff}, // "#d7005f"
	162: {R: 0xd7, G: 0x00, B: 0x87, A: 0xff}, // "#d70087"
	163: {R: 0xd7, G: 0x00, B: 0xaf, A: 0xff}, // "#d700af"
	164: {R: 0xd7, G: 0x00, B: 0xd7, A: 0xff}, // "#d700d7"
	165: {R: 0xd7, G: 0x00, B: 0xff, A: 0xff}, // "#d700ff"
	166: {R: 0xd7, G: 0x5f, B: 0x00, A: 0xff}, // "#d75f00"
	167: {R: 0xd7, G: 0x5f, B: 0x5f, A: 0xff}, // "#d75f5f"
	168: {R: 0xd7, G: 0x5f, B: 0x87, A: 0xff}, // "#d75f87"
	169: {R: 0xd7, G: 0x5f, B: 0xaf, A: 0xff}, // "#d75faf"
	170: {R: 0xd7, G: 0x5f, B: 0xd7, A: 0xff}, // "#d75fd7"
	171: {R: 0xd7, G: 0x5f, B: 0xff, A: 0xff}, // "#d75fff"
	172: {R: 0xd7, G: 0x87, B: 0x00, A: 0xff}, // "#d78700"
	173: {R: 0xd7, G: 0x87, B: 0x5f, A: 0xff}, // "#d7875f"
	174: {R: 0xd7, G: 0x87, B: 0x87, A: 0xff}, // "#d78787"
	175: {R: 0xd7, G: 0x87, B: 0xaf, A: 0xff}, // "#d787af"
	176: {R: 0xd7, G: 0x87, B: 0xd7, A: 0xff}, // "#d787d7"
	177: {R: 0xd7, G: 0x87, B: 0xff, A: 0xff}, // "#d787ff"
	178: {R: 0xd7, G: 0xaf, B: 0x00, A: 0xff}, // "#d7af00"
	179: {R: 0xd7, G: 0xaf, B: 0x5f, A: 0xff}, // "#d7af5f"
	180: {R: 0xd7, G: 0xaf, B: 0x87, A: 0xff}, // "#d7af87"
	181: {R: 0xd7, G: 0xaf, B: 0xaf, A: 0xff}, // "#d7afaf"
	182: {R: 0xd7, G: 0xaf, B: 0xd7, A: 0xff}, // "#d7afd7"
	183: {R: 0xd7, G: 0xaf, B: 0xff, A: 0xff}, // "#d7afff"
	184: {R: 0xd7, G: 0xd7, B: 0x00, A: 0xff}, // "#d7d700"
	185: {R: 0xd7, G: 0xd7, B: 0x5f, A: 0xff}, // "#d7d75f"
	186: {R: 0xd7, G: 0xd7, B: 0x87, A: 0xff}, // "#d7d787"
	187: {R: 0xd7, G: 0xd7, B: 0xaf, A: 0xff}, // "#d7d7af"
	188: {R: 0xd7, G: 0xd7, B: 0xd7, A: 0xff}, // "#d7d7d7"
	189: {R: 0xd7, G: 0xd7, B: 0xff, A: 0xff}, // "#d7d7ff"
	190: {R: 0xd7, G: 0xff, B: 0x00, A: 0xff}, // "#d7ff00"
	191: {R: 0xd7, G: 0xff, B: 0x5f, A: 0xff}, // "#d7ff5f"
	192: {R: 0xd7, G: 0xff, B: 0x87, A: 0xff}, // "#d7ff87"
	193: {R: 0xd7, G: 0xff, B: 0xaf, A: 0xff}, // "#d7ffaf"
	194: {R: 0xd7, G: 0xff, B: 0xd7, A: 0xff}, // "#d7ffd7"
	195: {R: 0xd7, G: 0xff, B: 0xff, A: 0xff}, // "#d7ffff"
	196: {R: 0xff, G: 0x00, B: 0x00, A: 0xff}, // "#ff0000"
	197: {R: 0xff, G: 0x00, B: 0x5f, A: 0xff}, // "#ff005f"
	198: {R: 0xff, G: 0x00, B: 0x87, A: 0xff}, // "#ff0087"
	199: {R: 0xff, G: 0x00, B: 0xaf, A: 0xff}, // "#ff00af"
	200: {R: 0xff, G: 0x00, B: 0xd7, A: 0xff}, // "#ff00d7"
	201: {R: 0xff, G: 0x00, B: 0xff, A: 0xff}, // "#ff00ff"
	202: {R: 0xff, G: 0x5f, B: 0x00, A: 0xff}, // "#ff5f00"
	203: {R: 0xff, G: 0x5f, B: 0x5f, A: 0xff}, // "#ff5f5f"
	204: {R: 0xff, G: 0x5f, B: 0x87, A: 0xff}, // "#ff5f87"
	205: {R: 0xff, G: 0x5f, B: 0xaf, A: 0xff}, // "#ff5faf"
	206: {R: 0xff, G: 0x5f, B: 0xd7, A: 0xff}, // "#ff5fd7"
	207: {R: 0xff, G: 0x5f, B: 0xff, A: 0xff}, // "#ff5fff"
	208: {R: 0xff, G: 0x87, B: 0x00, A: 0xff}, // "#ff8700"
	209: {R: 0xff, G: 0x87, B: 0x5f, A: 0xff}, // "#ff875f"
	210: {R: 0xff, G: 0x87, B: 0x87, A: 0xff}, // "#ff8787"
	211: {R: 0xff, G: 0x87, B: 0xaf, A: 0xff}, // "#ff87af"
	212: {R: 0xff, G: 0x87, B: 0xd7, A: 0xff}, // "#ff87d7"
	213: {R: 0xff, G: 0x87, B: 0xff, A: 0xff}, // "#ff87ff"
	214: {R: 0xff, G: 0xaf, B: 0x00, A: 0xff}, // "#ffaf00"
	215: {R: 0xff, G: 0xaf, B: 0x5f, A: 0xff}, // "#ffaf5f"
	216: {R: 0xff, G: 0xaf, B: 0x87, A: 0xff}, // "#ffaf87"
	217: {R: 0xff, G: 0xaf, B: 0xaf, A: 0xff}, // "#ffafaf"
	218: {R: 0xff, G: 0xaf, B: 0xd7, A: 0xff}, // "#ffafd7"
	219: {R: 0xff, G: 0xaf, B: 0xff, A: 0xff}, // "#ffafff"
	220: {R: 0xff, G: 0xd7, B: 0x00, A: 0xff}, // "#ffd700"
	221: {R: 0xff, G: 0xd7, B: 0x5f, A: 0xff}, // "#ffd75f"
	222: {R: 0xff, G: 0xd7, B: 0x87, A: 0xff}, // "#ffd787"
	223: {R: 0xff, G: 0xd7, B: 0xaf, A: 0xff}, // "#ffd7af"
	224: {R: 0xff, G: 0xd7, B: 0xd7, A: 0xff}, // "#ffd7d7"
	225: {R: 0xff, G: 0xd7, B: 0xff, A: 0xff}, // "#ffd7ff"
	226: {R: 0xff, G: 0xff, B: 0x00, A: 0xff}, // "#ffff00"
	227: {R: 0xff, G: 0xff, B: 0x5f, A: 0xff}, // "#ffff5f"
	228: {R: 0xff, G: 0xff, B: 0x87, A: 0xff}, // "#ffff87"
	229: {R: 0xff, G: 0xff, B: 0xaf, A: 0xff}, // "#ffffaf"
	230: {R: 0xff, G: 0xff, B: 0xd7, A: 0xff}, // "#ffffd7"
	231: {R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // "#ffffff"
	232: {R: 0x08, G: 0x08, B: 0x08, A: 0xff}, // "#080808"
	233: {R: 0x12, G: 0x12, B: 0x12, A: 0xff}, // "#121212"
	234: {R: 0x1c, G: 0x1c, B: 0x1c, A: 0xff}, // "#1c1c1c"
	235: {R: 0x26, G: 0x26, B: 0x26, A: 0xff}, // "#262626"
	236: {R: 0x30, G: 0x30, B: 0x30, A: 0xff}, // "#303030"
	237: {R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff}, // "#3a3a3a"
	238: {R: 0x44, G: 0x44, B: 0x44, A: 0xff}, // "#444444"
	239: {R: 0x4e, G: 0x4e, B: 0x4e, A: 0xff}, // "#4e4e4e"
	240: {R: 0x58, G: 0x58, B: 0x58, A: 0xff}, // "#585858"
	241: {R: 0x62, G: 0x62, B: 0x62, A: 0xff}, // "#626262"
	242: {R: 0x6c, G: 0x6c, B: 0x6c, A: 0xff}, // "#6c6c6c"
	243: {R: 0x76, G: 0x76, B: 0x76, A: 0xff}, // "#767676"
	244: {R: 0x80, G: 0x80, B: 0x80, A: 0xff}, // "#808080"
	245: {R: 0x8a, G: 0x8a, B: 0x8a, A: 0xff}, // "#8a8a8a"
	246: {R: 0x94, G: 0x94, B: 0x94, A: 0xff}, // "#949494"
	247: {R: 0x9e, G: 0x9e, B: 0x9e, A: 0xff}, // "#9e9e9e"
	248: {R: 0xa8, G: 0xa8, B: 0xa8, A: 0xff}, // "#a8a8a8"
	249: {R: 0xb2, G: 0xb2, B: 0xb2, A: 0xff}, // "#b2b2b2"
	250: {R: 0xbc, G: 0xbc, B: 0xbc, A: 0xff}, // "#bcbcbc"
	251: {R: 0xc6, G: 0xc6, B: 0xc6, A: 0xff}, // "#c6c6c6"
	252: {R: 0xd0, G: 0xd0, B: 0xd0, A: 0xff}, // "#d0d0d0"
	253: {R: 0xda, G: 0xda, B: 0xda, A: 0xff}, // "#dadada"
	254: {R: 0xe4, G: 0xe4, B: 0xe4, A: 0xff}, // "#e4e4e4"
	255: {R: 0xee, G: 0xee, B: 0xee, A: 0xff}, // "#eeeeee"
}

var ansi256To16 = [...]BasicColor{
	0:   0,
	1:   1,
	2:   2,
	3:   3,
	4:   4,
	5:   5,
	6:   6,
	7:   7,
	8:   8,
	9:   9,
	10:  10,
	11:  11,
	12:  12,
	13:  13,
	14:  14,
	15:  15,
	16:  0,
	17:  4,
	18:  4,
	19:  4,
	20:  12,
	21:  12,
	22:  2,
	23:  6,
	24:  4,
	25:  4,
	26:  12,
	27:  12,
	28:  2,
	29:  2,
	30:  6,
	31:  4,
	32:  12,
	33:  12,
	34:  2,
	35:  2,
	36:  2,
	37:  6,
	38:  12,
	39:  12,
	40:  10,
	41:  10,
	42:  10,
	43:  10,
	44:  14,
	45:  12,
	46:  10,
	47:  10,
	48:  10,
	49:  10,
	50:  10,
	51:  14,
	52:  1,
	53:  5,
	54:  4,
	55:  4,
	56:  12,
	57:  12,
	58:  3,
	59:  8,
	60:  4,
	61:  4,
	62:  12,
	63:  12,
	64:  2,
	65:  2,
	66:  6,
	67:  4,
	68:  12,
	69:  12,
	70:  2,
	71:  2,
	72:  2,
	73:  6,
	74:  12,
	75:  12,
	76:  10,
	77:  10,
	78:  10,
	79:  10,
	80:  14,
	81:  12,
	82:  10,
	83:  10,
	84:  10,
	85:  10,
	86:  10,
	87:  14,
	88:  1,
	89:  1,
	90:  5,
	91:  4,
	92:  12,
	93:  12,
	94:  1,
	95:  1,
	96:  5,
	97:  4,
	98:  12,
	99:  12,
	100: 3,
	101: 3,
	102: 8,
	103: 4,
	104: 12,
	105: 12,
	106: 2,
	107: 2,
	108: 2,
	109: 6,
	110: 12,
	111: 12,
	112: 10,
	113: 10,
	114: 10,
	115: 10,
	116: 14,
	117: 12,
	118: 10,
	119: 10,
	120: 10,
	121: 10,
	122: 10,
	123: 14,
	124: 1,
	125: 1,
	126: 1,
	127: 5,
	128: 12,
	129: 12,
	130: 1,
	131: 1,
	132: 1,
	133: 5,
	134: 12,
	135: 12,
	136: 1,
	137: 1,
	138: 1,
	139: 5,
	140: 12,
	141: 12,
	142: 3,
	143: 3,
	144: 3,
	145: 7,
	146: 12,
	147: 12,
	148: 10,
	149: 10,
	150: 10,
	151: 10,
	152: 14,
	153: 12,
	154: 10,
	155: 10,
	156: 10,
	157: 10,
	158: 10,
	159: 14,
	160: 9,
	161: 9,
	162: 9,
	163: 9,
	164: 13,
	165: 12,
	166: 9,
	167: 9,
	168: 9,
	169: 9,
	170: 13,
	171: 12,
	172: 9,
	173: 9,
	174: 9,
	175: 9,
	176: 13,
	177: 12,
	178: 9,
	179: 9,
	180: 9,
	181: 9,
	182: 13,
	183: 12,
	184: 11,
	185: 11,
	186: 11,
	187: 11,
	188: 7,
	189: 12,
	190: 10,
	191: 10,
	192: 10,
	193: 10,
	194: 10,
	195: 14,
	196: 9,
	197: 9,
	198: 9,
	199: 9,
	200: 9,
	201: 13,
	202: 9,
	203: 9,
	204: 9,
	205: 9,
	206: 9,
	207: 13,
	208: 9,
	209: 9,
	210: 9,
	211: 9,
	212: 9,
	213: 13,
	214: 9,
	215: 9,
	216: 9,
	217: 9,
	218: 9,
	219: 13,
	220: 9,
	221: 9,
	222: 9,
	223: 9,
	224: 9,
	225: 13,
	226: 11,
	227: 11,
	228: 11,
	229: 11,
	230: 11,
	231: 15,
	232: 0,
	233: 0,
	234: 0,
	235: 0,
	236: 0,
	237: 0,
	238: 8,
	239: 8,
	240: 8,
	241: 8,
	242: 8,
	243: 8,
	244: 7,
	245: 7,
	246: 7,
	247: 7,
	248: 7,
	249: 7,
	250: 15,
	251: 15,
	252: 15,
	253: 15,
	254: 15,
	255: 15,
}
