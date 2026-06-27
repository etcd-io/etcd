package ansi

import (
	"image/color"
	"strconv"
	"strings"
)

// ResetStyle is a SGR (Select Graphic Rendition) style sequence that resets
// all attributes.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
const ResetStyle = "\x1b[m"

// Attr is a SGR (Select Graphic Rendition) style attribute.
type Attr = int

// Style represents an ANSI SGR (Select Graphic Rendition) style.
type Style []string

// NewStyle returns a new style with the given attributes. Attributes are SGR
// (Select Graphic Rendition) codes that control text formatting like bold,
// italic, colors, etc.
func NewStyle(attrs ...Attr) Style {
	if len(attrs) == 0 {
		return Style{}
	}
	s := make(Style, 0, len(attrs))
	for _, a := range attrs {
		attr, ok := attrStrings[a]
		if ok {
			s = append(s, attr)
		} else {
			if a < 0 {
				a = 0
			}
			s = append(s, strconv.Itoa(a))
		}
	}
	return s
}

// String returns the ANSI SGR (Select Graphic Rendition) style sequence for
// the given style.
func (s Style) String() string {
	if len(s) == 0 {
		return ResetStyle
	}
	return "\x1b[" + strings.Join(s, ";") + "m"
}

// Styled returns a styled string with the given style applied. The style is
// applied at the beginning and reset at the end of the string.
func (s Style) Styled(str string) string {
	if len(s) == 0 {
		return str
	}
	return s.String() + str + ResetStyle
}

// Reset appends the reset style attribute to the style. This resets all
// formatting attributes to their defaults.
func (s Style) Reset() Style {
	return append(s, attrReset)
}

// Bold appends the bold or normal intensity style attribute to the style.
// You can use [Style.Normal] to reset to normal intensity.
func (s Style) Bold() Style {
	return append(s, attrBold)
}

// Faint appends the faint or normal intensity style attribute to the style.
// You can use [Style.Normal] to reset to normal intensity.
func (s Style) Faint() Style {
	return append(s, attrFaint)
}

// Italic appends the italic or no italic style attribute to the style.
// When v is true, text is rendered in italic. When false, italic is disabled.
func (s Style) Italic(v bool) Style {
	if v {
		return append(s, attrItalic)
	}
	return append(s, attrNoItalic)
}

// Underline appends the underline or no underline style attribute to the style.
// When v is true, text is underlined. When false, underline is disabled.
func (s Style) Underline(v bool) Style {
	if v {
		return append(s, attrUnderline)
	}
	return append(s, attrNoUnderline)
}

// UnderlineStyle appends the underline style attribute to the style.
// Supports various underline styles including single, double, curly, dotted,
// and dashed.
func (s Style) UnderlineStyle(u Underline) Style {
	switch u {
	case UnderlineNone:
		return s.Underline(false)
	case UnderlineSingle:
		return s.Underline(true)
	case UnderlineDouble:
		return append(s, underlineDouble)
	case UnderlineCurly:
		return append(s, underlineCurly)
	case UnderlineDotted:
		return append(s, underlineDotted)
	case UnderlineDashed:
		return append(s, underlineDashed)
	}
	return s
}

// Blink appends the slow blink or no blink style attribute to the style.
// When v is true, text blinks slowly (less than 150 per minute). When false,
// blinking is disabled.
func (s Style) Blink(v bool) Style {
	if v {
		return append(s, attrBlink)
	}
	return append(s, attrNoBlink)
}

// RapidBlink appends the rapid blink or no blink style attribute to the style.
// When v is true, text blinks rapidly (150+ per minute). When false, blinking
// is disabled.
//
// Note that this is not widely supported in terminal emulators.
func (s Style) RapidBlink(v bool) Style {
	if v {
		return append(s, attrRapidBlink)
	}
	return append(s, attrNoBlink)
}

// Reverse appends the reverse or no reverse style attribute to the style.
// When v is true, foreground and background colors are swapped. When false,
// reverse video is disabled.
func (s Style) Reverse(v bool) Style {
	if v {
		return append(s, attrReverse)
	}
	return append(s, attrNoReverse)
}

// Conceal appends the conceal or no conceal style attribute to the style.
// When v is true, text is hidden/concealed. When false, concealment is
// disabled.
func (s Style) Conceal(v bool) Style {
	if v {
		return append(s, attrConceal)
	}
	return append(s, attrNoConceal)
}

// Strikethrough appends the strikethrough or no strikethrough style attribute
// to the style. When v is true, text is rendered with a horizontal line through
// it. When false, strikethrough is disabled.
func (s Style) Strikethrough(v bool) Style {
	if v {
		return append(s, attrStrikethrough)
	}
	return append(s, attrNoStrikethrough)
}

// Normal appends the normal intensity style attribute to the style. This
// resets [Style.Bold] and [Style.Faint] attributes.
func (s Style) Normal() Style {
	return append(s, attrNormalIntensity)
}

// NoItalic appends the no italic style attribute to the style.
//
// Deprecated: use [Style.Italic](false) instead.
func (s Style) NoItalic() Style {
	return append(s, attrNoItalic)
}

// NoUnderline appends the no underline style attribute to the style.
//
// Deprecated: use [Style.Underline](false) instead.
func (s Style) NoUnderline() Style {
	return append(s, attrNoUnderline)
}

// NoBlink appends the no blink style attribute to the style.
//
// Deprecated: use [Style.Blink](false) or [Style.RapidBlink](false) instead.
func (s Style) NoBlink() Style {
	return append(s, attrNoBlink)
}

// NoReverse appends the no reverse style attribute to the style.
//
// Deprecated: use [Style.Reverse](false) instead.
func (s Style) NoReverse() Style {
	return append(s, attrNoReverse)
}

// NoConceal appends the no conceal style attribute to the style.
//
// Deprecated: use [Style.Conceal](false) instead.
func (s Style) NoConceal() Style {
	return append(s, attrNoConceal)
}

// NoStrikethrough appends the no strikethrough style attribute to the style.
//
// Deprecated: use [Style.Strikethrough](false) instead.
func (s Style) NoStrikethrough() Style {
	return append(s, attrNoStrikethrough)
}

// DefaultForegroundColor appends the default foreground color style attribute to the style.
//
// Deprecated: use [Style.ForegroundColor](nil) instead.
func (s Style) DefaultForegroundColor() Style {
	return append(s, attrDefaultForegroundColor)
}

// DefaultBackgroundColor appends the default background color style attribute to the style.
//
// Deprecated: use [Style.BackgroundColor](nil) instead.
func (s Style) DefaultBackgroundColor() Style {
	return append(s, attrDefaultBackgroundColor)
}

// DefaultUnderlineColor appends the default underline color style attribute to the style.
//
// Deprecated: use [Style.UnderlineColor](nil) instead.
func (s Style) DefaultUnderlineColor() Style {
	return append(s, attrDefaultUnderlineColor)
}

// ForegroundColor appends the foreground color style attribute to the style.
// If c is nil, the default foreground color is used. Supports [BasicColor],
// [IndexedColor] (256-color), and [color.Color] (24-bit RGB).
func (s Style) ForegroundColor(c Color) Style {
	if c == nil {
		return append(s, attrDefaultForegroundColor)
	}
	return append(s, foregroundColorString(c))
}

// BackgroundColor appends the background color style attribute to the style.
// If c is nil, the default background color is used. Supports [BasicColor],
// [IndexedColor] (256-color), and [color.Color] (24-bit RGB).
func (s Style) BackgroundColor(c Color) Style {
	if c == nil {
		return append(s, attrDefaultBackgroundColor)
	}
	return append(s, backgroundColorString(c))
}

// UnderlineColor appends the underline color style attribute to the style.
// If c is nil, the default underline color is used. Supports [BasicColor],
// [IndexedColor] (256-color), and [color.Color] (24-bit RGB).
func (s Style) UnderlineColor(c Color) Style {
	if c == nil {
		return append(s, attrDefaultUnderlineColor)
	}
	return append(s, underlineColorString(c))
}

// Underline represents an ANSI SGR (Select Graphic Rendition) underline style.
type Underline = byte

// UnderlineStyle represents an ANSI SGR (Select Graphic Rendition) underline
// style.
//
// Deprecated: use [Underline] instead.
type UnderlineStyle = byte

const (
	underlineDouble = "4:2"
	underlineCurly  = "4:3"
	underlineDotted = "4:4"
	underlineDashed = "4:5"
)

// Underline styles constants.
const (
	UnderlineNone Underline = iota
	UnderlineSingle
	UnderlineDouble
	UnderlineCurly
	UnderlineDotted
	UnderlineDashed
)

// Underline styles constants.
//
// Deprecated: use [UnderlineNone], [UnderlineSingle], etc. instead.
const (
	NoUnderlineStyle Underline = iota
	SingleUnderlineStyle
	DoubleUnderlineStyle
	CurlyUnderlineStyle
	DottedUnderlineStyle
	DashedUnderlineStyle
)

// Underline styles constants.
//
// Deprecated: use [UnderlineNone], [UnderlineSingle], etc. instead.
const (
	UnderlineStyleNone Underline = iota
	UnderlineStyleSingle
	UnderlineStyleDouble
	UnderlineStyleCurly
	UnderlineStyleDotted
	UnderlineStyleDashed
)

// SGR (Select Graphic Rendition) style attributes.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
const (
	AttrReset                        Attr = 0
	AttrBold                         Attr = 1
	AttrFaint                        Attr = 2
	AttrItalic                       Attr = 3
	AttrUnderline                    Attr = 4
	AttrBlink                        Attr = 5
	AttrRapidBlink                   Attr = 6
	AttrReverse                      Attr = 7
	AttrConceal                      Attr = 8
	AttrStrikethrough                Attr = 9
	AttrNormalIntensity              Attr = 22
	AttrNoItalic                     Attr = 23
	AttrNoUnderline                  Attr = 24
	AttrNoBlink                      Attr = 25
	AttrNoReverse                    Attr = 27
	AttrNoConceal                    Attr = 28
	AttrNoStrikethrough              Attr = 29
	AttrBlackForegroundColor         Attr = 30
	AttrRedForegroundColor           Attr = 31
	AttrGreenForegroundColor         Attr = 32
	AttrYellowForegroundColor        Attr = 33
	AttrBlueForegroundColor          Attr = 34
	AttrMagentaForegroundColor       Attr = 35
	AttrCyanForegroundColor          Attr = 36
	AttrWhiteForegroundColor         Attr = 37
	AttrExtendedForegroundColor      Attr = 38
	AttrDefaultForegroundColor       Attr = 39
	AttrBlackBackgroundColor         Attr = 40
	AttrRedBackgroundColor           Attr = 41
	AttrGreenBackgroundColor         Attr = 42
	AttrYellowBackgroundColor        Attr = 43
	AttrBlueBackgroundColor          Attr = 44
	AttrMagentaBackgroundColor       Attr = 45
	AttrCyanBackgroundColor          Attr = 46
	AttrWhiteBackgroundColor         Attr = 47
	AttrExtendedBackgroundColor      Attr = 48
	AttrDefaultBackgroundColor       Attr = 49
	AttrExtendedUnderlineColor       Attr = 58
	AttrDefaultUnderlineColor        Attr = 59
	AttrBrightBlackForegroundColor   Attr = 90
	AttrBrightRedForegroundColor     Attr = 91
	AttrBrightGreenForegroundColor   Attr = 92
	AttrBrightYellowForegroundColor  Attr = 93
	AttrBrightBlueForegroundColor    Attr = 94
	AttrBrightMagentaForegroundColor Attr = 95
	AttrBrightCyanForegroundColor    Attr = 96
	AttrBrightWhiteForegroundColor   Attr = 97
	AttrBrightBlackBackgroundColor   Attr = 100
	AttrBrightRedBackgroundColor     Attr = 101
	AttrBrightGreenBackgroundColor   Attr = 102
	AttrBrightYellowBackgroundColor  Attr = 103
	AttrBrightBlueBackgroundColor    Attr = 104
	AttrBrightMagentaBackgroundColor Attr = 105
	AttrBrightCyanBackgroundColor    Attr = 106
	AttrBrightWhiteBackgroundColor   Attr = 107

	AttrRGBColorIntroducer      Attr = 2
	AttrExtendedColorIntroducer Attr = 5
)

// SGR (Select Graphic Rendition) style attributes.
//
// Deprecated: use Attr* constants instead.
const (
	ResetAttr                        = AttrReset
	BoldAttr                         = AttrBold
	FaintAttr                        = AttrFaint
	ItalicAttr                       = AttrItalic
	UnderlineAttr                    = AttrUnderline
	SlowBlinkAttr                    = AttrBlink
	RapidBlinkAttr                   = AttrRapidBlink
	ReverseAttr                      = AttrReverse
	ConcealAttr                      = AttrConceal
	StrikethroughAttr                = AttrStrikethrough
	NormalIntensityAttr              = AttrNormalIntensity
	NoItalicAttr                     = AttrNoItalic
	NoUnderlineAttr                  = AttrNoUnderline
	NoBlinkAttr                      = AttrNoBlink
	NoReverseAttr                    = AttrNoReverse
	NoConcealAttr                    = AttrNoConceal
	NoStrikethroughAttr              = AttrNoStrikethrough
	BlackForegroundColorAttr         = AttrBlackForegroundColor
	RedForegroundColorAttr           = AttrRedForegroundColor
	GreenForegroundColorAttr         = AttrGreenForegroundColor
	YellowForegroundColorAttr        = AttrYellowForegroundColor
	BlueForegroundColorAttr          = AttrBlueForegroundColor
	MagentaForegroundColorAttr       = AttrMagentaForegroundColor
	CyanForegroundColorAttr          = AttrCyanForegroundColor
	WhiteForegroundColorAttr         = AttrWhiteForegroundColor
	ExtendedForegroundColorAttr      = AttrExtendedForegroundColor
	DefaultForegroundColorAttr       = AttrDefaultForegroundColor
	BlackBackgroundColorAttr         = AttrBlackBackgroundColor
	RedBackgroundColorAttr           = AttrRedBackgroundColor
	GreenBackgroundColorAttr         = AttrGreenBackgroundColor
	YellowBackgroundColorAttr        = AttrYellowBackgroundColor
	BlueBackgroundColorAttr          = AttrBlueBackgroundColor
	MagentaBackgroundColorAttr       = AttrMagentaBackgroundColor
	CyanBackgroundColorAttr          = AttrCyanBackgroundColor
	WhiteBackgroundColorAttr         = AttrWhiteBackgroundColor
	ExtendedBackgroundColorAttr      = AttrExtendedBackgroundColor
	DefaultBackgroundColorAttr       = AttrDefaultBackgroundColor
	ExtendedUnderlineColorAttr       = AttrExtendedUnderlineColor
	DefaultUnderlineColorAttr        = AttrDefaultUnderlineColor
	BrightBlackForegroundColorAttr   = AttrBrightBlackForegroundColor
	BrightRedForegroundColorAttr     = AttrBrightRedForegroundColor
	BrightGreenForegroundColorAttr   = AttrBrightGreenForegroundColor
	BrightYellowForegroundColorAttr  = AttrBrightYellowForegroundColor
	BrightBlueForegroundColorAttr    = AttrBrightBlueForegroundColor
	BrightMagentaForegroundColorAttr = AttrBrightMagentaForegroundColor
	BrightCyanForegroundColorAttr    = AttrBrightCyanForegroundColor
	BrightWhiteForegroundColorAttr   = AttrBrightWhiteForegroundColor
	BrightBlackBackgroundColorAttr   = AttrBrightBlackBackgroundColor
	BrightRedBackgroundColorAttr     = AttrBrightRedBackgroundColor
	BrightGreenBackgroundColorAttr   = AttrBrightGreenBackgroundColor
	BrightYellowBackgroundColorAttr  = AttrBrightYellowBackgroundColor
	BrightBlueBackgroundColorAttr    = AttrBrightBlueBackgroundColor
	BrightMagentaBackgroundColorAttr = AttrBrightMagentaBackgroundColor
	BrightCyanBackgroundColorAttr    = AttrBrightCyanBackgroundColor
	BrightWhiteBackgroundColorAttr   = AttrBrightWhiteBackgroundColor
	RGBColorIntroducerAttr           = AttrRGBColorIntroducer
	ExtendedColorIntroducerAttr      = AttrExtendedColorIntroducer
)

const (
	attrReset                        = "0"
	attrBold                         = "1"
	attrFaint                        = "2"
	attrItalic                       = "3"
	attrUnderline                    = "4"
	attrBlink                        = "5"
	attrRapidBlink                   = "6"
	attrReverse                      = "7"
	attrConceal                      = "8"
	attrStrikethrough                = "9"
	attrNormalIntensity              = "22"
	attrNoItalic                     = "23"
	attrNoUnderline                  = "24"
	attrNoBlink                      = "25"
	attrNoReverse                    = "27"
	attrNoConceal                    = "28"
	attrNoStrikethrough              = "29"
	attrBlackForegroundColor         = "30"
	attrRedForegroundColor           = "31"
	attrGreenForegroundColor         = "32"
	attrYellowForegroundColor        = "33"
	attrBlueForegroundColor          = "34"
	attrMagentaForegroundColor       = "35"
	attrCyanForegroundColor          = "36"
	attrWhiteForegroundColor         = "37"
	attrExtendedForegroundColor      = "38"
	attrDefaultForegroundColor       = "39"
	attrBlackBackgroundColor         = "40"
	attrRedBackgroundColor           = "41"
	attrGreenBackgroundColor         = "42"
	attrYellowBackgroundColor        = "43"
	attrBlueBackgroundColor          = "44"
	attrMagentaBackgroundColor       = "45"
	attrCyanBackgroundColor          = "46"
	attrWhiteBackgroundColor         = "47"
	attrExtendedBackgroundColor      = "48"
	attrDefaultBackgroundColor       = "49"
	attrExtendedUnderlineColor       = "58"
	attrDefaultUnderlineColor        = "59"
	attrBrightBlackForegroundColor   = "90"
	attrBrightRedForegroundColor     = "91"
	attrBrightGreenForegroundColor   = "92"
	attrBrightYellowForegroundColor  = "93"
	attrBrightBlueForegroundColor    = "94"
	attrBrightMagentaForegroundColor = "95"
	attrBrightCyanForegroundColor    = "96"
	attrBrightWhiteForegroundColor   = "97"
	attrBrightBlackBackgroundColor   = "100"
	attrBrightRedBackgroundColor     = "101"
	attrBrightGreenBackgroundColor   = "102"
	attrBrightYellowBackgroundColor  = "103"
	attrBrightBlueBackgroundColor    = "104"
	attrBrightMagentaBackgroundColor = "105"
	attrBrightCyanBackgroundColor    = "106"
	attrBrightWhiteBackgroundColor   = "107"
)

// foregroundColorString returns the style SGR attribute for the given
// foreground color.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
func foregroundColorString(c Color) string {
	switch c := c.(type) {
	case nil:
		return attrDefaultForegroundColor
	case BasicColor:
		// 3-bit or 4-bit ANSI foreground
		// "3<n>" or "9<n>" where n is the color number from 0 to 7
		switch c {
		case Black:
			return attrBlackForegroundColor
		case Red:
			return attrRedForegroundColor
		case Green:
			return attrGreenForegroundColor
		case Yellow:
			return attrYellowForegroundColor
		case Blue:
			return attrBlueForegroundColor
		case Magenta:
			return attrMagentaForegroundColor
		case Cyan:
			return attrCyanForegroundColor
		case White:
			return attrWhiteForegroundColor
		case BrightBlack:
			return attrBrightBlackForegroundColor
		case BrightRed:
			return attrBrightRedForegroundColor
		case BrightGreen:
			return attrBrightGreenForegroundColor
		case BrightYellow:
			return attrBrightYellowForegroundColor
		case BrightBlue:
			return attrBrightBlueForegroundColor
		case BrightMagenta:
			return attrBrightMagentaForegroundColor
		case BrightCyan:
			return attrBrightCyanForegroundColor
		case BrightWhite:
			return attrBrightWhiteForegroundColor
		}
	case ExtendedColor:
		// 256-color ANSI foreground
		// "38;5;<n>"
		return "38;5;" + strconv.FormatUint(uint64(c), 10)
	case TrueColor, color.Color:
		// 24-bit "true color" foreground
		// "38;2;<r>;<g>;<b>"
		r, g, b, _ := c.RGBA()
		return "38;2;" +
			strconv.FormatUint(uint64(shift(r)), 10) + ";" +
			strconv.FormatUint(uint64(shift(g)), 10) + ";" +
			strconv.FormatUint(uint64(shift(b)), 10)
	}
	return attrDefaultForegroundColor
}

// backgroundColorString returns the style SGR attribute for the given
// background color.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
func backgroundColorString(c Color) string {
	switch c := c.(type) {
	case nil:
		return attrDefaultBackgroundColor
	case BasicColor:
		// 3-bit or 4-bit ANSI foreground
		// "4<n>" or "10<n>" where n is the color number from 0 to 7
		switch c {
		case Black:
			return attrBlackBackgroundColor
		case Red:
			return attrRedBackgroundColor
		case Green:
			return attrGreenBackgroundColor
		case Yellow:
			return attrYellowBackgroundColor
		case Blue:
			return attrBlueBackgroundColor
		case Magenta:
			return attrMagentaBackgroundColor
		case Cyan:
			return attrCyanBackgroundColor
		case White:
			return attrWhiteBackgroundColor
		case BrightBlack:
			return attrBrightBlackBackgroundColor
		case BrightRed:
			return attrBrightRedBackgroundColor
		case BrightGreen:
			return attrBrightGreenBackgroundColor
		case BrightYellow:
			return attrBrightYellowBackgroundColor
		case BrightBlue:
			return attrBrightBlueBackgroundColor
		case BrightMagenta:
			return attrBrightMagentaBackgroundColor
		case BrightCyan:
			return attrBrightCyanBackgroundColor
		case BrightWhite:
			return attrBrightWhiteBackgroundColor
		}
	case ExtendedColor:
		// 256-color ANSI foreground
		// "48;5;<n>"
		return "48;5;" + strconv.FormatUint(uint64(c), 10)
	case TrueColor, color.Color:
		// 24-bit "true color" foreground
		// "38;2;<r>;<g>;<b>"
		r, g, b, _ := c.RGBA()
		return "48;2;" +
			strconv.FormatUint(uint64(shift(r)), 10) + ";" +
			strconv.FormatUint(uint64(shift(g)), 10) + ";" +
			strconv.FormatUint(uint64(shift(b)), 10)
	}
	return attrDefaultBackgroundColor
}

// underlineColorString returns the style SGR attribute for the given underline
// color.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
func underlineColorString(c Color) string {
	switch c := c.(type) {
	case nil:
		return attrDefaultUnderlineColor
	// NOTE: we can't use 3-bit and 4-bit ANSI color codes with underline
	// color, use 256-color instead.
	//
	// 256-color ANSI underline color
	// "58;5;<n>"
	case BasicColor:
		return "58;5;" + strconv.FormatUint(uint64(c), 10)
	case ExtendedColor:
		return "58;5;" + strconv.FormatUint(uint64(c), 10)
	case TrueColor, color.Color:
		// 24-bit "true color" foreground
		// "38;2;<r>;<g>;<b>"
		r, g, b, _ := c.RGBA()
		return "58;2;" +
			strconv.FormatUint(uint64(shift(r)), 10) + ";" +
			strconv.FormatUint(uint64(shift(g)), 10) + ";" +
			strconv.FormatUint(uint64(shift(b)), 10)
	}
	return attrDefaultUnderlineColor
}

// ReadStyleColor decodes a color from a slice of parameters. It returns the
// number of parameters read and the color. This function is used to read SGR
// color parameters following the ITU T.416 standard.
//
// It supports reading the following color types:
//   - 0: implementation defined
//   - 1: transparent
//   - 2: RGB direct color
//   - 3: CMY direct color
//   - 4: CMYK direct color
//   - 5: indexed color
//   - 6: RGBA direct color (WezTerm extension)
//
// The parameters can be separated by semicolons (;) or colons (:). Mixing
// separators is not allowed.
//
// The specs supports defining a color space id, a color tolerance value, and a
// tolerance color space id. However, these values have no effect on the
// returned color and will be ignored.
//
// This implementation includes a few modifications to the specs:
//  1. Support for legacy color values separated by semicolons (;) with respect to RGB, and indexed colors
//  2. Support ignoring and omitting the color space id (second parameter) with respect to RGB colors
//  3. Support ignoring and omitting the 6th parameter with respect to RGB and CMY colors
//  4. Support reading RGBA colors
func ReadStyleColor(params Params, co *color.Color) int {
	if len(params) < 2 { // Need at least SGR type and color type
		return 0
	}

	// First parameter indicates one of 38, 48, or 58 (foreground, background, or underline)
	s := params[0]
	p := params[1]
	colorType := p.Param(0)
	n := 2

	paramsfn := func() (p1, p2, p3, p4 int) {
		// Where should we start reading the color?
		switch {
		case s.HasMore() && p.HasMore() && len(params) > 8 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && params[5].HasMore() && params[6].HasMore() && params[7].HasMore():
			// We have color space id, a 6th parameter, a tolerance value, and a tolerance color space
			n += 7
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), params[6].Param(0)
		case s.HasMore() && p.HasMore() && len(params) > 7 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && params[5].HasMore() && params[6].HasMore():
			// We have color space id, a 6th parameter, and a tolerance value
			n += 6
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), params[6].Param(0)
		case s.HasMore() && p.HasMore() && len(params) > 6 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && params[5].HasMore():
			// We have color space id and a 6th parameter
			// 48 : 4 : : 1 : 2 : 3 :4
			n += 5
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), params[6].Param(0)
		case s.HasMore() && p.HasMore() && len(params) > 5 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && !params[5].HasMore():
			// We have color space
			// 48 : 3 : : 1 : 2 : 3
			n += 4
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), -1
		case s.HasMore() && p.HasMore() && p.Param(0) == 2 && params[2].HasMore() && params[3].HasMore() && !params[4].HasMore():
			// We have color values separated by colons (:)
			// 48 : 2 : 1 : 2 : 3
			fallthrough
		case !s.HasMore() && !p.HasMore() && p.Param(0) == 2 && !params[2].HasMore() && !params[3].HasMore() && !params[4].HasMore():
			// Support legacy color values separated by semicolons (;)
			// 48 ; 2 ; 1 ; 2 ; 3
			n += 3
			return params[2].Param(0), params[3].Param(0), params[4].Param(0), -1
		}
		// Ambiguous SGR color
		return -1, -1, -1, -1
	}

	switch colorType {
	case 0: // implementation defined
		return 2
	case 1: // transparent
		*co = color.Transparent
		return 2
	case 2: // RGB direct color
		if len(params) < 5 {
			return 0
		}

		r, g, b, _ := paramsfn()
		if r == -1 || g == -1 || b == -1 {
			return 0
		}

		*co = color.RGBA{
			R: uint8(r), //nolint:gosec
			G: uint8(g), //nolint:gosec
			B: uint8(b), //nolint:gosec
			A: 0xff,
		}
		return n

	case 3: // CMY direct color
		if len(params) < 5 {
			return 0
		}

		c, m, y, _ := paramsfn()
		if c == -1 || m == -1 || y == -1 {
			return 0
		}

		*co = color.CMYK{
			C: uint8(c), //nolint:gosec
			M: uint8(m), //nolint:gosec
			Y: uint8(y), //nolint:gosec
			K: 0,
		}
		return n

	case 4: // CMYK direct color
		if len(params) < 6 {
			return 0
		}

		c, m, y, k := paramsfn()
		if c == -1 || m == -1 || y == -1 || k == -1 {
			return 0
		}

		*co = color.CMYK{
			C: uint8(c), //nolint:gosec
			M: uint8(m), //nolint:gosec
			Y: uint8(y), //nolint:gosec
			K: uint8(k), //nolint:gosec
		}
		return n

	case 5: // indexed color
		if len(params) < 3 {
			return 0
		}
		switch {
		case s.HasMore() && p.HasMore() && !params[2].HasMore():
			// Colon separated indexed color
			// 38 : 5 : 234
		case !s.HasMore() && !p.HasMore() && !params[2].HasMore():
			// Legacy semicolon indexed color
			// 38 ; 5 ; 234
		default:
			return 0
		}
		*co = ExtendedColor(params[2].Param(0)) //nolint:gosec
		return 3

	case 6: // RGBA direct color
		if len(params) < 6 {
			return 0
		}

		r, g, b, a := paramsfn()
		if r == -1 || g == -1 || b == -1 || a == -1 {
			return 0
		}

		*co = color.RGBA{
			R: uint8(r), //nolint:gosec
			G: uint8(g), //nolint:gosec
			B: uint8(b), //nolint:gosec
			A: uint8(a), //nolint:gosec
		}
		return n

	default:
		return 0
	}
}
