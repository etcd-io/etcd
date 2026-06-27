package lipgloss

import (
	"image/color"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

const (
	// NBSP is the non-breaking space rune.
	NBSP            = '\u00A0'
	tabWidthDefault = 4
)

// Property for a key.
type propKey int64

// Available properties.
const (
	// Boolean props come first.
	boldKey propKey = 1 << iota
	italicKey
	strikethroughKey
	reverseKey
	blinkKey
	faintKey
	underlineSpacesKey
	strikethroughSpacesKey
	colorWhitespaceKey

	// Non-boolean props.
	underlineKey
	foregroundKey
	backgroundKey
	underlineColorKey
	widthKey
	heightKey
	alignHorizontalKey
	alignVerticalKey

	// Padding.
	paddingTopKey
	paddingRightKey
	paddingBottomKey
	paddingLeftKey
	paddingCharKey

	// Margins.
	marginTopKey
	marginRightKey
	marginBottomKey
	marginLeftKey
	marginBackgroundKey
	marginCharKey

	// Border runes.
	borderStyleKey

	// Border edges.
	borderTopKey
	borderRightKey
	borderBottomKey
	borderLeftKey

	// Border foreground colors.
	borderTopForegroundKey
	borderRightForegroundKey
	borderBottomForegroundKey
	borderLeftForegroundKey
	borderForegroundBlendKey
	borderForegroundBlendOffsetKey

	// Border background colors.
	borderTopBackgroundKey
	borderRightBackgroundKey
	borderBottomBackgroundKey
	borderLeftBackgroundKey

	inlineKey
	maxWidthKey
	maxHeightKey
	tabWidthKey

	transformKey

	// Hyperlink.
	linkKey
	linkParamsKey
)

// props is a set of properties.
type props int64

// set sets a property.
func (p props) set(k propKey) props {
	return p | props(k)
}

// unset unsets a property.
func (p props) unset(k propKey) props {
	return p &^ props(k)
}

// has checks if a property is set.
func (p props) has(k propKey) bool {
	return p&props(k) != 0
}

// Underline is the style of the underline.
//
// Caveats:
// - Not all terminals support all underline styles.
// - Some terminals may render unsupported styles as standard underlines.
// - Terminal themes may affect the visibility of different underline styles.
type Underline = ansi.Underline

const (
	// UnderlineNone is no underline.
	UnderlineNone = ansi.UnderlineNone
	// UnderlineSingle is a single underline. This is the default when underline is enabled.
	UnderlineSingle = ansi.UnderlineSingle
	// UnderlineDouble is a double underline.
	UnderlineDouble = ansi.UnderlineDouble
	// UnderlineCurly is a curly underline.
	UnderlineCurly = ansi.UnderlineCurly
	// UnderlineDotted is a dotted underline.
	UnderlineDotted = ansi.UnderlineDotted
	// UnderlineDashed is a dashed underline.
	UnderlineDashed = ansi.UnderlineDashed
)

// NewStyle returns a new, empty Style. While it's syntactic sugar for the
// [Style]{} primitive, it's recommended to use this function for creating styles
// in case the underlying implementation changes.
func NewStyle() Style {
	return Style{}
}

// Style contains a set of rules that comprise a style as a whole.
type Style struct {
	props props
	value string

	// hyperlink
	link, linkParams string

	// we store bool props values here
	attrs int

	// props that have values
	fgColor color.Color
	bgColor color.Color
	ulColor color.Color

	ul Underline

	width  int
	height int

	alignHorizontal Position
	alignVertical   Position

	paddingTop    int
	paddingRight  int
	paddingBottom int
	paddingLeft   int
	paddingChar   rune

	marginTop     int
	marginRight   int
	marginBottom  int
	marginLeft    int
	marginBgColor color.Color
	marginChar    rune

	borderStyle                 Border
	borderTopFgColor            color.Color
	borderRightFgColor          color.Color
	borderBottomFgColor         color.Color
	borderLeftFgColor           color.Color
	borderBlendFgColor          []color.Color
	borderForegroundBlendOffset int
	borderTopBgColor            color.Color
	borderRightBgColor          color.Color
	borderBottomBgColor         color.Color
	borderLeftBgColor           color.Color

	maxWidth  int
	maxHeight int
	tabWidth  int

	transform func(string) string
}

// joinString joins a list of strings into a single string separated with a
// space.
func joinString(strs ...string) string {
	return strings.Join(strs, " ")
}

// SetString sets the underlying string value for this style. To render once
// the underlying string is set, use the [Style.String]. This method is
// a convenience for cases when having a stringer implementation is handy, such
// as when using fmt.Sprintf. You can also simply define a style and render out
// strings directly with [Style.Render].
func (s Style) SetString(strs ...string) Style {
	s.value = joinString(strs...)
	return s
}

// Value returns the raw, unformatted, underlying string value for this style.
func (s Style) Value() string {
	return s.value
}

// String implements stringer for a Style, returning the rendered result based
// on the rules in this style. An underlying string value must be set with
// Style.SetString prior to using this method.
func (s Style) String() string {
	return s.Render()
}

// Copy returns a copy of this style, including any underlying string values.
//
// Deprecated: to copy just use assignment (i.e. a := b). All methods also
// return a new style.
func (s Style) Copy() Style {
	return s
}

// Inherit overlays the style in the argument onto this style by copying each explicitly
// set value from the argument style onto this style if it is not already explicitly set.
// Existing set values are kept intact and not overwritten.
//
// Margins, padding, and underlying string values are not inherited.
func (s Style) Inherit(i Style) Style {
	for k := boldKey; k <= transformKey; k <<= 1 {
		if !i.isSet(k) {
			continue
		}

		switch k { //nolint:exhaustive
		case marginTopKey, marginRightKey, marginBottomKey, marginLeftKey:
			// Margins are not inherited
			continue
		case paddingTopKey, paddingRightKey, paddingBottomKey, paddingLeftKey:
			// Padding is not inherited
			continue
		case backgroundKey:
			// The margins also inherit the background color
			if !s.isSet(marginBackgroundKey) && !i.isSet(marginBackgroundKey) {
				s.set(marginBackgroundKey, i.bgColor)
			}
		}

		if s.isSet(k) {
			continue
		}

		s.setFrom(k, i)
	}
	return s
}

// Render applies the defined style formatting to a given string.
func (s Style) Render(strs ...string) string {
	if s.value != "" {
		strs = append([]string{s.value}, strs...)
	}

	var (
		str = joinString(strs...)

		te           ansi.Style
		teSpace      ansi.Style
		teWhitespace ansi.Style

		bold          = s.getAsBool(boldKey, false)
		italic        = s.getAsBool(italicKey, false)
		strikethrough = s.getAsBool(strikethroughKey, false)
		reverse       = s.getAsBool(reverseKey, false)
		blink         = s.getAsBool(blinkKey, false)
		faint         = s.getAsBool(faintKey, false)

		fg = s.getAsColor(foregroundKey)
		bg = s.getAsColor(backgroundKey)
		ul = s.getAsColor(underlineColorKey)

		underline       = s.ul != UnderlineNone
		width           = s.getAsInt(widthKey)
		height          = s.getAsInt(heightKey)
		horizontalAlign = s.getAsPosition(alignHorizontalKey)
		verticalAlign   = s.getAsPosition(alignVerticalKey)

		topPadding    = s.getAsInt(paddingTopKey)
		rightPadding  = s.getAsInt(paddingRightKey)
		bottomPadding = s.getAsInt(paddingBottomKey)
		leftPadding   = s.getAsInt(paddingLeftKey)

		horizontalBorderSize = s.GetHorizontalBorderSize()
		verticalBorderSize   = s.GetVerticalBorderSize()

		colorWhitespace = s.getAsBool(colorWhitespaceKey, true)
		inline          = s.getAsBool(inlineKey, false)
		maxWidth        = s.getAsInt(maxWidthKey)
		maxHeight       = s.getAsInt(maxHeightKey)

		underlineSpaces     = s.getAsBool(underlineSpacesKey, false) || (underline && s.getAsBool(underlineSpacesKey, true))
		strikethroughSpaces = s.getAsBool(strikethroughSpacesKey, false) || (strikethrough && s.getAsBool(strikethroughSpacesKey, true))

		// Do we need to style whitespace (padding and space outside
		// paragraphs) separately?
		styleWhitespace = reverse

		// Do we need to style spaces separately?
		useSpaceStyler = (underline && !underlineSpaces) || (strikethrough && !strikethroughSpaces) || underlineSpaces || strikethroughSpaces

		transform = s.getAsTransform(transformKey)

		link, linkParams = s.GetHyperlink()
	)

	if transform != nil {
		str = transform(str)
	}

	if s.props == 0 {
		return s.maybeConvertTabs(str)
	}

	if bold {
		te = te.Bold()
	}
	if italic {
		te = te.Italic(true)
	}
	if underline {
		te = te.Underline(true)
	}
	if reverse {
		teWhitespace = teWhitespace.Reverse(true)
		te = te.Reverse(true)
	}
	if blink {
		te = te.Blink(true)
	}
	if faint {
		te = te.Faint()
	}

	if fg != noColor {
		te = te.ForegroundColor(fg)
		if styleWhitespace {
			teWhitespace = teWhitespace.ForegroundColor(fg)
		}
		if useSpaceStyler {
			teSpace = teSpace.ForegroundColor(fg)
		}
	}

	if bg != noColor {
		te = te.BackgroundColor(bg)
		if colorWhitespace {
			teWhitespace = teWhitespace.BackgroundColor(bg)
		}
		if useSpaceStyler {
			teSpace = teSpace.BackgroundColor(bg)
		}
	}

	if ul != noColor {
		te = te.UnderlineColor(ul)
		if colorWhitespace {
			teWhitespace = teWhitespace.UnderlineColor(ul)
		}
		if useSpaceStyler {
			teSpace = teSpace.UnderlineColor(ul)
		}
	}

	if underline {
		te = te.UnderlineStyle(s.ul)
	}
	if strikethrough {
		te = te.Strikethrough(true)
	}

	if underlineSpaces {
		teSpace = teSpace.Underline(true)
	}
	if strikethroughSpaces {
		teSpace = teSpace.Strikethrough(true)
	}

	// Potentially convert tabs to spaces
	str = s.maybeConvertTabs(str)
	// carriage returns can cause strange behaviour when rendering.
	str = strings.ReplaceAll(str, "\r\n", "\n")

	// Strip newlines in single line mode
	if inline {
		str = strings.ReplaceAll(str, "\n", "")
	}

	// Include borders in block size.
	width -= horizontalBorderSize
	height -= verticalBorderSize

	// Word wrap
	if !inline && width > 0 {
		wrapAt := width - leftPadding - rightPadding
		str = Wrap(str, wrapAt, "")
	}

	// Render core text
	{
		var b strings.Builder

		isFirst := true
		for line := range strings.SplitSeq(str, "\n") {
			if isFirst {
				isFirst = false
			} else {
				b.WriteRune('\n')
			}
			if useSpaceStyler {
				// Look for spaces and apply a different styler
				for _, r := range line {
					if unicode.IsSpace(r) {
						b.WriteString(teSpace.Styled(string(r)))
						continue
					}
					b.WriteString(te.Styled(string(r)))
				}
			} else {
				b.WriteString(te.Styled(line))
			}
		}

		str = b.String()

		if len(link) > 0 {
			str = ansi.SetHyperlink(link, linkParams) + str + ansi.ResetHyperlink()
		}
	}

	// Padding
	if !inline { //nolint:nestif
		padChar := s.paddingChar
		if padChar == 0 {
			padChar = ' '
		}
		if leftPadding > 0 {
			var st *ansi.Style
			if colorWhitespace || styleWhitespace {
				st = &teWhitespace
			}
			str = padLeft(str, leftPadding, st, padChar)
		}

		if rightPadding > 0 {
			var st *ansi.Style
			if colorWhitespace || styleWhitespace {
				st = &teWhitespace
			}
			str = padRight(str, rightPadding, st, padChar)
		}

		if topPadding > 0 {
			str = strings.Repeat("\n", topPadding) + str
		}

		if bottomPadding > 0 {
			str += strings.Repeat("\n", bottomPadding)
		}
	}

	// Height
	if height > 0 {
		str = alignTextVertical(str, verticalAlign, height, nil)
	}

	// Set alignment. This will also pad short lines with spaces so that all
	// lines are the same length, so we run it under a few different conditions
	// beyond alignment.
	{
		numLines := strings.Count(str, "\n")

		if numLines != 0 || width != 0 {
			var st *ansi.Style
			if colorWhitespace || styleWhitespace {
				st = &teWhitespace
			}
			str = alignTextHorizontal(str, horizontalAlign, width, st)
		}
	}

	if !inline {
		str = s.applyBorder(str)
		str = s.applyMargins(str, inline)
	}

	// Truncate according to MaxWidth
	if maxWidth > 0 {
		lines := strings.Split(str, "\n")

		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], maxWidth, "")
		}

		str = strings.Join(lines, "\n")
	}

	// Truncate according to MaxHeight
	if maxHeight > 0 {
		lines := strings.Split(str, "\n")
		height := min(maxHeight, len(lines))
		if len(lines) > 0 {
			str = strings.Join(lines[:height], "\n")
		}
	}

	return str
}

func (s Style) maybeConvertTabs(str string) string {
	tw := tabWidthDefault
	if s.isSet(tabWidthKey) {
		tw = s.getAsInt(tabWidthKey)
	}
	switch tw {
	case -1:
		return str
	case 0:
		return strings.ReplaceAll(str, "\t", "")
	default:
		return strings.ReplaceAll(str, "\t", strings.Repeat(" ", tw))
	}
}

func (s Style) applyMargins(str string, inline bool) string {
	var (
		topMargin    = s.getAsInt(marginTopKey)
		rightMargin  = s.getAsInt(marginRightKey)
		bottomMargin = s.getAsInt(marginBottomKey)
		leftMargin   = s.getAsInt(marginLeftKey)

		style ansi.Style
	)

	bgc := s.getAsColor(marginBackgroundKey)
	if bgc != noColor {
		style = style.BackgroundColor(bgc)
	}

	// Add left and right margin
	marginChar := s.marginChar
	if marginChar == 0 {
		marginChar = ' '
	}
	str = padLeft(str, leftMargin, &style, marginChar)
	str = padRight(str, rightMargin, &style, marginChar)

	// Top/bottom margin
	if !inline {
		_, width := getLines(str)
		spaces := strings.Repeat(" ", width)

		if topMargin > 0 {
			str = style.Styled(strings.Repeat(spaces+"\n", topMargin)) + str
		}
		if bottomMargin > 0 {
			str += style.Styled(strings.Repeat("\n"+spaces, bottomMargin))
		}
	}

	return str
}

// Apply left padding.
func padLeft(str string, n int, style *ansi.Style, r rune) string {
	return pad(str, -n, style, r)
}

// Apply right padding.
func padRight(str string, n int, style *ansi.Style, r rune) string {
	return pad(str, n, style, r)
}

// pad adds padding to either the left or right side of a string.
// Positive values add to the right side while negative values
// add to the left side.
// r is the rune to use for padding. We use " " for margins and
// "\u00A0" for padding so that the padding is preserved when the
// string is copied and pasted.
func pad(str string, n int, style *ansi.Style, r rune) string {
	if n == 0 {
		return str
	}

	sp := strings.Repeat(string(r), abs(n))
	if style != nil {
		sp = style.Styled(sp)
	}

	b := strings.Builder{}
	isFirst := true
	for line := range strings.SplitSeq(str, "\n") {
		if isFirst {
			isFirst = false
		} else {
			b.WriteRune('\n')
		}
		switch {
		// pad right
		case n > 0:
			b.WriteString(line)
			b.WriteString(sp)
		// pad left
		default:
			b.WriteString(sp)
			b.WriteString(line)
		}
	}

	return b.String()
}

func abs(a int) int {
	if a < 0 {
		return -a
	}

	return a
}
