package lipgloss

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// GetBold returns the style's bold value. If no value is set false is returned.
func (s Style) GetBold() bool {
	return s.getAsBool(boldKey, false)
}

// GetItalic returns the style's italic value. If no value is set false is
// returned.
func (s Style) GetItalic() bool {
	return s.getAsBool(italicKey, false)
}

// GetUnderline returns the style's underline value. If no value is set false is
// returned.
func (s Style) GetUnderline() bool {
	return s.ul != UnderlineNone
}

// GetUnderlineStyle returns the style's underline style. If no value is set
// UnderlineNone is returned.
func (s Style) GetUnderlineStyle() Underline {
	return s.ul
}

// GetUnderlineColor returns the style's underline color. If no value is set
// NoColor{} is returned.
func (s Style) GetUnderlineColor() color.Color {
	return s.getAsColor(underlineColorKey)
}

// GetStrikethrough returns the style's strikethrough value. If no value is set false
// is returned.
func (s Style) GetStrikethrough() bool {
	return s.getAsBool(strikethroughKey, false)
}

// GetReverse returns the style's reverse value. If no value is set false is
// returned.
func (s Style) GetReverse() bool {
	return s.getAsBool(reverseKey, false)
}

// GetBlink returns the style's blink value. If no value is set false is
// returned.
func (s Style) GetBlink() bool {
	return s.getAsBool(blinkKey, false)
}

// GetFaint returns the style's faint value. If no value is set false is
// returned.
func (s Style) GetFaint() bool {
	return s.getAsBool(faintKey, false)
}

// GetForeground returns the style's foreground color. If no value is set
// NoColor{} is returned.
func (s Style) GetForeground() color.Color {
	return s.getAsColor(foregroundKey)
}

// GetBackground returns the style's background color. If no value is set
// NoColor{} is returned.
func (s Style) GetBackground() color.Color {
	return s.getAsColor(backgroundKey)
}

// GetWidth returns the style's width setting. If no width is set 0 is
// returned.
func (s Style) GetWidth() int {
	return s.getAsInt(widthKey)
}

// GetHeight returns the style's height setting. If no height is set 0 is
// returned.
func (s Style) GetHeight() int {
	return s.getAsInt(heightKey)
}

// GetAlign returns the style's implicit horizontal alignment setting.
// If no alignment is set Position.Left is returned.
func (s Style) GetAlign() Position {
	v := s.getAsPosition(alignHorizontalKey)
	if v == Position(0) {
		return Left
	}
	return v
}

// GetAlignHorizontal returns the style's implicit horizontal alignment setting.
// If no alignment is set Position.Left is returned.
func (s Style) GetAlignHorizontal() Position {
	v := s.getAsPosition(alignHorizontalKey)
	if v == Position(0) {
		return Left
	}
	return v
}

// GetAlignVertical returns the style's implicit vertical alignment setting.
// If no alignment is set Position.Top is returned.
func (s Style) GetAlignVertical() Position {
	v := s.getAsPosition(alignVerticalKey)
	if v == Position(0) {
		return Top
	}
	return v
}

// GetPadding returns the style's top, right, bottom, and left padding values,
// in that order. 0 is returned for unset values.
func (s Style) GetPadding() (top, right, bottom, left int) {
	return s.getAsInt(paddingTopKey),
		s.getAsInt(paddingRightKey),
		s.getAsInt(paddingBottomKey),
		s.getAsInt(paddingLeftKey)
}

// GetPaddingTop returns the style's top padding. If no value is set 0 is
// returned.
func (s Style) GetPaddingTop() int {
	return s.getAsInt(paddingTopKey)
}

// GetPaddingRight returns the style's right padding. If no value is set 0 is
// returned.
func (s Style) GetPaddingRight() int {
	return s.getAsInt(paddingRightKey)
}

// GetPaddingBottom returns the style's bottom padding. If no value is set 0 is
// returned.
func (s Style) GetPaddingBottom() int {
	return s.getAsInt(paddingBottomKey)
}

// GetPaddingLeft returns the style's left padding. If no value is set 0 is
// returned.
func (s Style) GetPaddingLeft() int {
	return s.getAsInt(paddingLeftKey)
}

// GetPaddingChar returns the style's padding character. If no value is set a
// space is returned.
func (s Style) GetPaddingChar() rune {
	char := s.getAsRune(paddingCharKey)
	if char == 0 {
		return ' '
	}
	return char
}

// GetHorizontalPadding returns the style's left and right padding. Unset
// values are measured as 0.
func (s Style) GetHorizontalPadding() int {
	return s.getAsInt(paddingLeftKey) + s.getAsInt(paddingRightKey)
}

// GetVerticalPadding returns the style's top and bottom padding. Unset values
// are measured as 0.
func (s Style) GetVerticalPadding() int {
	return s.getAsInt(paddingTopKey) + s.getAsInt(paddingBottomKey)
}

// GetColorWhitespace returns the style's whitespace coloring setting. If no
// value is set false is returned.
func (s Style) GetColorWhitespace() bool {
	return s.getAsBool(colorWhitespaceKey, false)
}

// GetMargin returns the style's top, right, bottom, and left margins, in that
// order. 0 is returned for unset values.
func (s Style) GetMargin() (top, right, bottom, left int) {
	return s.getAsInt(marginTopKey),
		s.getAsInt(marginRightKey),
		s.getAsInt(marginBottomKey),
		s.getAsInt(marginLeftKey)
}

// GetMarginTop returns the style's top margin. If no value is set 0 is
// returned.
func (s Style) GetMarginTop() int {
	return s.getAsInt(marginTopKey)
}

// GetMarginRight returns the style's right margin. If no value is set 0 is
// returned.
func (s Style) GetMarginRight() int {
	return s.getAsInt(marginRightKey)
}

// GetMarginBottom returns the style's bottom margin. If no value is set 0 is
// returned.
func (s Style) GetMarginBottom() int {
	return s.getAsInt(marginBottomKey)
}

// GetMarginLeft returns the style's left margin. If no value is set 0 is
// returned.
func (s Style) GetMarginLeft() int {
	return s.getAsInt(marginLeftKey)
}

// GetMarginChar returns the style's padding character. If no value is set a
// space is returned.
func (s Style) GetMarginChar() rune {
	char := s.getAsRune(marginCharKey)
	if char == 0 {
		return ' '
	}
	return char
}

// GetHorizontalMargins returns the style's left and right margins. Unset
// values are measured as 0.
func (s Style) GetHorizontalMargins() int {
	return s.getAsInt(marginLeftKey) + s.getAsInt(marginRightKey)
}

// GetVerticalMargins returns the style's top and bottom margins. Unset values
// are measured as 0.
func (s Style) GetVerticalMargins() int {
	return s.getAsInt(marginTopKey) + s.getAsInt(marginBottomKey)
}

// GetBorder returns the style's border style (type Border) and value for the
// top, right, bottom, and left in that order. If no value is set for the
// border style, Border{} is returned. For all other unset values false is
// returned.
func (s Style) GetBorder() (b Border, top, right, bottom, left bool) {
	return s.getBorderStyle(),
		s.getAsBool(borderTopKey, false),
		s.getAsBool(borderRightKey, false),
		s.getAsBool(borderBottomKey, false),
		s.getAsBool(borderLeftKey, false)
}

// GetBorderStyle returns the style's border style (type Border). If no value
// is set Border{} is returned.
func (s Style) GetBorderStyle() Border {
	return s.getBorderStyle()
}

// GetBorderTop returns the style's top border setting. If no value is set
// false is returned.
func (s Style) GetBorderTop() bool {
	return s.getAsBool(borderTopKey, false)
}

// GetBorderRight returns the style's right border setting. If no value is set
// false is returned.
func (s Style) GetBorderRight() bool {
	return s.getAsBool(borderRightKey, false)
}

// GetBorderBottom returns the style's bottom border setting. If no value is
// set false is returned.
func (s Style) GetBorderBottom() bool {
	return s.getAsBool(borderBottomKey, false)
}

// GetBorderLeft returns the style's left border setting. If no value is
// set false is returned.
func (s Style) GetBorderLeft() bool {
	return s.getAsBool(borderLeftKey, false)
}

// GetBorderTopForeground returns the style's border top foreground color. If
// no value is set NoColor{} is returned.
func (s Style) GetBorderTopForeground() color.Color {
	return s.getAsColor(borderTopForegroundKey)
}

// GetBorderRightForeground returns the style's border right foreground color.
// If no value is set NoColor{} is returned.
func (s Style) GetBorderRightForeground() color.Color {
	return s.getAsColor(borderRightForegroundKey)
}

// GetBorderBottomForeground returns the style's border bottom foreground
// color.  If no value is set NoColor{} is returned.
func (s Style) GetBorderBottomForeground() color.Color {
	return s.getAsColor(borderBottomForegroundKey)
}

// GetBorderLeftForeground returns the style's border left foreground
// color.  If no value is set NoColor{} is returned.
func (s Style) GetBorderLeftForeground() color.Color {
	return s.getAsColor(borderLeftForegroundKey)
}

// GetBorderForegroundBlend returns the style's border blend foreground
// colors. If no value is set, nil is returned.
func (s Style) GetBorderForegroundBlend() []color.Color {
	return s.getAsColors(borderForegroundBlendKey)
}

// GetBorderForegroundBlendOffset returns the style's border blend offset. If no
// value is set, 0 is returned.
func (s Style) GetBorderForegroundBlendOffset() int {
	return s.getAsInt(borderForegroundBlendOffsetKey)
}

// GetBorderTopBackground returns the style's border top background color. If
// no value is set NoColor{} is returned.
func (s Style) GetBorderTopBackground() color.Color {
	return s.getAsColor(borderTopBackgroundKey)
}

// GetBorderRightBackground returns the style's border right background color.
// If no value is set NoColor{} is returned.
func (s Style) GetBorderRightBackground() color.Color {
	return s.getAsColor(borderRightBackgroundKey)
}

// GetBorderBottomBackground returns the style's border bottom background
// color.  If no value is set NoColor{} is returned.
func (s Style) GetBorderBottomBackground() color.Color {
	return s.getAsColor(borderBottomBackgroundKey)
}

// GetBorderLeftBackground returns the style's border left background
// color.  If no value is set NoColor{} is returned.
func (s Style) GetBorderLeftBackground() color.Color {
	return s.getAsColor(borderLeftBackgroundKey)
}

// GetBorderTopWidth returns the width of the top border. If borders contain
// runes of varying widths, the widest rune is returned. If no border exists on
// the top edge, 0 is returned.
//
// Deprecated: This function simply calls Style.GetBorderTopSize.
func (s Style) GetBorderTopWidth() int {
	return s.GetBorderTopSize()
}

// GetBorderTopSize returns the width of the top border. If borders contain
// runes of varying widths, the widest rune is returned. If no border exists on
// the top edge, 0 is returned.
func (s Style) GetBorderTopSize() int {
	if s.isBorderStyleSetWithoutSides() {
		return 1
	}
	if !s.getAsBool(borderTopKey, false) {
		return 0
	}
	return s.getBorderStyle().GetTopSize()
}

// GetBorderLeftSize returns the width of the left border. If borders contain
// runes of varying widths, the widest rune is returned. If no border exists on
// the left edge, 0 is returned.
func (s Style) GetBorderLeftSize() int {
	if s.isBorderStyleSetWithoutSides() {
		return 1
	}
	if !s.getAsBool(borderLeftKey, false) {
		return 0
	}
	return s.getBorderStyle().GetLeftSize()
}

// GetBorderBottomSize returns the width of the bottom border. If borders
// contain runes of varying widths, the widest rune is returned. If no border
// exists on the left edge, 0 is returned.
func (s Style) GetBorderBottomSize() int {
	if s.isBorderStyleSetWithoutSides() {
		return 1
	}
	if !s.getAsBool(borderBottomKey, false) {
		return 0
	}
	return s.getBorderStyle().GetBottomSize()
}

// GetBorderRightSize returns the width of the right border. If borders
// contain runes of varying widths, the widest rune is returned. If no border
// exists on the right edge, 0 is returned.
func (s Style) GetBorderRightSize() int {
	if s.isBorderStyleSetWithoutSides() {
		return 1
	}
	if !s.getAsBool(borderRightKey, false) {
		return 0
	}
	return s.getBorderStyle().GetRightSize()
}

// GetHorizontalBorderSize returns the width of the horizontal borders. If
// borders contain runes of varying widths, the widest rune is returned. If no
// border exists on the horizontal edges, 0 is returned.
func (s Style) GetHorizontalBorderSize() int {
	return s.GetBorderLeftSize() + s.GetBorderRightSize()
}

// GetVerticalBorderSize returns the width of the vertical borders. If
// borders contain runes of varying widths, the widest rune is returned. If no
// border exists on the vertical edges, 0 is returned.
func (s Style) GetVerticalBorderSize() int {
	return s.GetBorderTopSize() + s.GetBorderBottomSize()
}

// GetInline returns the style's inline setting. If no value is set false is
// returned.
func (s Style) GetInline() bool {
	return s.getAsBool(inlineKey, false)
}

// GetMaxWidth returns the style's max width setting. If no value is set 0 is
// returned.
func (s Style) GetMaxWidth() int {
	return s.getAsInt(maxWidthKey)
}

// GetMaxHeight returns the style's max height setting. If no value is set 0 is
// returned.
func (s Style) GetMaxHeight() int {
	return s.getAsInt(maxHeightKey)
}

// GetTabWidth returns the style's tab width setting. If no value is set 4 is
// returned which is the implicit default.
func (s Style) GetTabWidth() int {
	return s.getAsInt(tabWidthKey)
}

// GetUnderlineSpaces returns whether or not the style is set to underline
// spaces. If not value is set false is returned.
func (s Style) GetUnderlineSpaces() bool {
	return s.getAsBool(underlineSpacesKey, false)
}

// GetStrikethroughSpaces returns whether or not the style is set to strikethrough
// spaces. If not value is set false is returned.
func (s Style) GetStrikethroughSpaces() bool {
	return s.getAsBool(strikethroughSpacesKey, false)
}

// GetHorizontalFrameSize returns the sum of the style's horizontal margins, padding
// and border widths.
//
// Provisional: this method may be renamed.
func (s Style) GetHorizontalFrameSize() int {
	return s.GetHorizontalMargins() + s.GetHorizontalPadding() + s.GetHorizontalBorderSize()
}

// GetVerticalFrameSize returns the sum of the style's vertical margins, padding
// and border widths.
//
// Provisional: this method may be renamed.
func (s Style) GetVerticalFrameSize() int {
	return s.GetVerticalMargins() + s.GetVerticalPadding() + s.GetVerticalBorderSize()
}

// GetFrameSize returns the sum of the margins, padding and border width for
// both the horizontal and vertical margins.
func (s Style) GetFrameSize() (x, y int) {
	return s.GetHorizontalFrameSize(), s.GetVerticalFrameSize()
}

// GetTransform returns the transform set on the style. If no transform is set
// nil is returned.
func (s Style) GetTransform() func(string) string {
	return s.getAsTransform(transformKey)
}

// GetHyperlink returns the hyperlink along with its parameters. If no
// hyperlink is set, empty strings are returned.
func (s Style) GetHyperlink() (link, params string) {
	if s.isSet(linkKey) {
		link = s.link
	}
	if s.isSet(linkParamsKey) {
		params = s.linkParams
	}
	return
}

// Returns whether or not the given property is set.
func (s Style) isSet(k propKey) bool {
	return s.props.has(k)
}

func (s Style) getAsRune(k propKey) rune {
	if !s.isSet(k) {
		return 0
	}
	switch k { //nolint:exhaustive
	case paddingCharKey:
		return s.paddingChar
	case marginCharKey:
		return s.marginChar
	}
	return 0
}

func (s Style) getAsBool(k propKey, defaultVal bool) bool {
	if !s.isSet(k) {
		return defaultVal
	}
	return s.attrs&int(k) != 0
}

func (s Style) getAsColors(k propKey) (colors []color.Color) {
	if !s.isSet(k) {
		return nil
	}

	switch k { //nolint:exhaustive
	case borderForegroundBlendKey:
		return s.borderBlendFgColor
	}

	return nil
}

func (s Style) getAsColor(k propKey) color.Color {
	if !s.isSet(k) {
		return noColor
	}

	var c color.Color
	switch k { //nolint:exhaustive
	case foregroundKey:
		c = s.fgColor
	case backgroundKey:
		c = s.bgColor
	case marginBackgroundKey:
		c = s.marginBgColor
	case borderTopForegroundKey:
		c = s.borderTopFgColor
	case borderRightForegroundKey:
		c = s.borderRightFgColor
	case borderBottomForegroundKey:
		c = s.borderBottomFgColor
	case borderLeftForegroundKey:
		c = s.borderLeftFgColor
	case borderTopBackgroundKey:
		c = s.borderTopBgColor
	case borderRightBackgroundKey:
		c = s.borderRightBgColor
	case borderBottomBackgroundKey:
		c = s.borderBottomBgColor
	case borderLeftBackgroundKey:
		c = s.borderLeftBgColor
	case underlineColorKey:
		c = s.ulColor
	}

	if c != nil {
		return c
	}

	return noColor
}

func (s Style) getAsInt(k propKey) int {
	if !s.isSet(k) {
		return 0
	}
	switch k { //nolint:exhaustive
	case widthKey:
		return s.width
	case heightKey:
		return s.height
	case paddingTopKey:
		return s.paddingTop
	case paddingRightKey:
		return s.paddingRight
	case paddingBottomKey:
		return s.paddingBottom
	case paddingLeftKey:
		return s.paddingLeft
	case marginTopKey:
		return s.marginTop
	case marginRightKey:
		return s.marginRight
	case marginBottomKey:
		return s.marginBottom
	case marginLeftKey:
		return s.marginLeft
	case borderForegroundBlendOffsetKey:
		return s.borderForegroundBlendOffset
	case maxWidthKey:
		return s.maxWidth
	case maxHeightKey:
		return s.maxHeight
	case tabWidthKey:
		return s.tabWidth
	}
	return 0
}

func (s Style) getAsPosition(k propKey) Position {
	if !s.isSet(k) {
		return Position(0)
	}
	switch k { //nolint:exhaustive
	case alignHorizontalKey:
		return s.alignHorizontal
	case alignVerticalKey:
		return s.alignVertical
	}
	return Position(0)
}

func (s Style) getBorderStyle() Border {
	if !s.isSet(borderStyleKey) {
		return noBorder
	}
	return s.borderStyle
}

func (s Style) getAsTransform(propKey) func(string) string {
	if !s.isSet(transformKey) {
		return nil
	}
	return s.transform
}

// Split a string into lines, additionally returning the size of the widest
// line.
func getLines(s string) (lines []string, widest int) {
	s = strings.ReplaceAll(s, "\t", "    ")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines = strings.Split(s, "\n")

	for _, l := range lines {
		w := ansi.StringWidth(l)
		if widest < w {
			widest = w
		}
	}

	return lines, widest
}

// isBorderStyleSetWithoutSides returns true if the border style is set but no
// sides are set. This is used to determine if the border should be rendered by
// default.
func (s Style) isBorderStyleSetWithoutSides() bool {
	var (
		border    = s.getBorderStyle()
		topSet    = s.isSet(borderTopKey)
		rightSet  = s.isSet(borderRightKey)
		bottomSet = s.isSet(borderBottomKey)
		leftSet   = s.isSet(borderLeftKey)
	)
	return border != noBorder && !(topSet || rightSet || bottomSet || leftSet) //nolint:staticcheck
}
