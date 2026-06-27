package lipgloss

import (
	"image/color"
	"strings"
)

// Set a value on the underlying rules map.
func (s *Style) set(key propKey, value any) {
	// We don't allow negative integers on any of our other values, so just keep
	// them at zero or above. We could use uints instead, but the
	// conversions are a little tedious, so we're sticking with ints for
	// sake of usability.
	switch key {
	case foregroundKey:
		s.fgColor = colorOrNil(value)
	case backgroundKey:
		s.bgColor = colorOrNil(value)
	case underlineColorKey:
		s.ulColor = colorOrNil(value)
	case underlineKey:
		s.ul = value.(Underline)
	case widthKey:
		s.width = max(0, value.(int))
	case heightKey:
		s.height = max(0, value.(int))
	case alignHorizontalKey:
		s.alignHorizontal = value.(Position)
	case alignVerticalKey:
		s.alignVertical = value.(Position)
	case paddingTopKey:
		s.paddingTop = max(0, value.(int))
	case paddingRightKey:
		s.paddingRight = max(0, value.(int))
	case paddingBottomKey:
		s.paddingBottom = max(0, value.(int))
	case paddingLeftKey:
		s.paddingLeft = max(0, value.(int))
	case paddingCharKey:
		s.paddingChar = value.(rune)
	case marginTopKey:
		s.marginTop = max(0, value.(int))
	case marginRightKey:
		s.marginRight = max(0, value.(int))
	case marginBottomKey:
		s.marginBottom = max(0, value.(int))
	case marginLeftKey:
		s.marginLeft = max(0, value.(int))
	case marginBackgroundKey:
		s.marginBgColor = colorOrNil(value)
	case marginCharKey:
		s.marginChar = value.(rune)
	case borderStyleKey:
		s.borderStyle = value.(Border)
	case borderTopForegroundKey:
		s.borderTopFgColor = colorOrNil(value)
	case borderRightForegroundKey:
		s.borderRightFgColor = colorOrNil(value)
	case borderBottomForegroundKey:
		s.borderBottomFgColor = colorOrNil(value)
	case borderLeftForegroundKey:
		s.borderLeftFgColor = colorOrNil(value)
	case borderForegroundBlendKey:
		s.borderBlendFgColor = value.([]color.Color)
	case borderForegroundBlendOffsetKey:
		s.borderForegroundBlendOffset = value.(int)
	case borderTopBackgroundKey:
		s.borderTopBgColor = colorOrNil(value)
	case borderRightBackgroundKey:
		s.borderRightBgColor = colorOrNil(value)
	case borderBottomBackgroundKey:
		s.borderBottomBgColor = colorOrNil(value)
	case borderLeftBackgroundKey:
		s.borderLeftBgColor = colorOrNil(value)
	case maxWidthKey:
		s.maxWidth = max(0, value.(int))
	case maxHeightKey:
		s.maxHeight = max(0, value.(int))
	case tabWidthKey:
		// TabWidth is the only property that may have a negative value (and
		// that negative value can be no less than -1).
		s.tabWidth = value.(int)
	case transformKey:
		s.transform = value.(func(string) string)
	case linkKey:
		s.link = value.(string)
	case linkParamsKey:
		s.linkParams = value.(string)
	default:
		if v, ok := value.(bool); ok { //nolint:nestif
			if v {
				s.attrs |= int(key)
			} else {
				s.attrs &^= int(key)
			}
		} else if attrs, ok := value.(int); ok {
			// bool attrs
			if attrs&int(key) != 0 {
				s.attrs |= int(key)
			} else {
				s.attrs &^= int(key)
			}
		}
	}

	// Set the prop on
	s.props = s.props.set(key)
}

// setFrom sets the property from another style.
func (s *Style) setFrom(key propKey, i Style) {
	switch key {
	case foregroundKey:
		s.set(foregroundKey, i.fgColor)
	case backgroundKey:
		s.set(backgroundKey, i.bgColor)
	case underlineColorKey:
		s.set(underlineColorKey, i.ulColor)
	case underlineKey:
		s.set(underlineKey, i.ul)
	case widthKey:
		s.set(widthKey, i.width)
	case heightKey:
		s.set(heightKey, i.height)
	case alignHorizontalKey:
		s.set(alignHorizontalKey, i.alignHorizontal)
	case alignVerticalKey:
		s.set(alignVerticalKey, i.alignVertical)
	case paddingTopKey:
		s.set(paddingTopKey, i.paddingTop)
	case paddingRightKey:
		s.set(paddingRightKey, i.paddingRight)
	case paddingBottomKey:
		s.set(paddingBottomKey, i.paddingBottom)
	case paddingLeftKey:
		s.set(paddingLeftKey, i.paddingLeft)
	case paddingCharKey:
		s.set(paddingCharKey, i.paddingChar)
	case marginTopKey:
		s.set(marginTopKey, i.marginTop)
	case marginRightKey:
		s.set(marginRightKey, i.marginRight)
	case marginBottomKey:
		s.set(marginBottomKey, i.marginBottom)
	case marginLeftKey:
		s.set(marginLeftKey, i.marginLeft)
	case marginBackgroundKey:
		s.set(marginBackgroundKey, i.marginBgColor)
	case marginCharKey:
		s.set(marginCharKey, i.marginChar)
	case borderStyleKey:
		s.set(borderStyleKey, i.borderStyle)
	case borderTopForegroundKey:
		s.set(borderTopForegroundKey, i.borderTopFgColor)
	case borderRightForegroundKey:
		s.set(borderRightForegroundKey, i.borderRightFgColor)
	case borderBottomForegroundKey:
		s.set(borderBottomForegroundKey, i.borderBottomFgColor)
	case borderLeftForegroundKey:
		s.set(borderLeftForegroundKey, i.borderLeftFgColor)
	case borderForegroundBlendKey:
		s.set(borderForegroundBlendKey, i.borderBlendFgColor)
	case borderForegroundBlendOffsetKey:
		s.set(borderForegroundBlendOffsetKey, i.borderForegroundBlendOffset)
	case borderTopBackgroundKey:
		s.set(borderTopBackgroundKey, i.borderTopBgColor)
	case borderRightBackgroundKey:
		s.set(borderRightBackgroundKey, i.borderRightBgColor)
	case borderBottomBackgroundKey:
		s.set(borderBottomBackgroundKey, i.borderBottomBgColor)
	case borderLeftBackgroundKey:
		s.set(borderLeftBackgroundKey, i.borderLeftBgColor)
	case maxWidthKey:
		s.set(maxWidthKey, i.maxWidth)
	case maxHeightKey:
		s.set(maxHeightKey, i.maxHeight)
	case tabWidthKey:
		s.set(tabWidthKey, i.tabWidth)
	case transformKey:
		s.set(transformKey, i.transform)
	default:
		// Set attributes for set bool properties
		s.set(key, i.attrs)
	}
}

func colorOrNil(c any) color.Color {
	if c, ok := c.(color.Color); ok {
		return c
	}
	return nil
}

// Bold sets a bold formatting rule.
func (s Style) Bold(v bool) Style {
	s.set(boldKey, v)
	return s
}

// Italic sets an italic formatting rule. In some terminal emulators this will
// render with "reverse" coloring if not italic font variant is available.
func (s Style) Italic(v bool) Style {
	s.set(italicKey, v)
	return s
}

// Underline sets an underline rule. By default, underlines will not be drawn on
// whitespace like margins and padding. To change this behavior set
// [Style.UnderlineSpaces].
func (s Style) Underline(v bool) Style {
	if v {
		return s.UnderlineStyle(UnderlineSingle)
	}
	return s.UnderlineStyle(UnderlineNone)
}

// UnderlineStyle sets the underline style. This can be used to set the underline
// to be a single, double, curly, dotted, or dashed line.
//
// Note that not all terminal emulators support underline styles. If a style is
// not supported, it will typically fall back to a single underline but this is
// not guaranteed. This depends on the terminal emulator being used.
func (s Style) UnderlineStyle(u Underline) Style {
	s.set(underlineKey, u)
	return s
}

// UnderlineColor sets the color of the underline. By default, the underline
// will be the same color as the foreground.
//
// Note that not all terminal emulators support colored underlines. If color is
// not supported, it might produce unexpected results. This depends on the
// terminal emulator being used.
func (s Style) UnderlineColor(c color.Color) Style {
	s.set(underlineColorKey, c)
	return s
}

// Strikethrough sets a strikethrough rule. By default, strikes will not be
// drawn on whitespace like margins and padding. To change this behavior set
// StrikethroughSpaces.
func (s Style) Strikethrough(v bool) Style {
	s.set(strikethroughKey, v)
	return s
}

// Reverse sets a rule for inverting foreground and background colors.
func (s Style) Reverse(v bool) Style {
	s.set(reverseKey, v)
	return s
}

// Blink sets a rule for blinking foreground text.
func (s Style) Blink(v bool) Style {
	s.set(blinkKey, v)
	return s
}

// Faint sets a rule for rendering the foreground color in a dimmer shade.
func (s Style) Faint(v bool) Style {
	s.set(faintKey, v)
	return s
}

// Foreground sets a foreground color.
//
//	// Sets the foreground to blue
//	s := lipgloss.NewStyle().Foreground(lipgloss.Color("#0000ff"))
//
//	// Removes the foreground color
//	s.Foreground(lipgloss.NoColor)
func (s Style) Foreground(c color.Color) Style {
	s.set(foregroundKey, c)
	return s
}

// Background sets a background color.
func (s Style) Background(c color.Color) Style {
	s.set(backgroundKey, c)
	return s
}

// Width sets the width of the block before applying margins. This means your
// styled content will exactly equal the size set here. Text will wrap based on
// Padding and Borders set on the style.
func (s Style) Width(i int) Style {
	s.set(widthKey, i)
	return s
}

// Height sets the height of the block before applying margins. If the height of
// the text block is less than this value after applying padding (or not), the
// block will be set to this height.
func (s Style) Height(i int) Style {
	s.set(heightKey, i)
	return s
}

// Align is a shorthand method for setting horizontal and vertical alignment.
//
// With one argument, the position value is applied to the horizontal alignment.
//
// With two arguments, the value is applied to the horizontal and vertical
// alignments, in that order.
func (s Style) Align(p ...Position) Style {
	if len(p) > 0 {
		s.set(alignHorizontalKey, p[0])
	}
	if len(p) > 1 {
		s.set(alignVerticalKey, p[1])
	}
	return s
}

// AlignHorizontal sets a horizontal text alignment rule.
func (s Style) AlignHorizontal(p Position) Style {
	s.set(alignHorizontalKey, p)
	return s
}

// AlignVertical sets a vertical text alignment rule.
func (s Style) AlignVertical(p Position) Style {
	s.set(alignVerticalKey, p)
	return s
}

// Padding is a shorthand method for setting padding on all sides at once.
//
// With one argument, the value is applied to all sides.
//
// With two arguments, the value is applied to the vertical and horizontal
// sides, in that order.
//
// With three arguments, the value is applied to the top side, the horizontal
// sides, and the bottom side, in that order.
//
// With four arguments, the value is applied clockwise starting from the top
// side, followed by the right side, then the bottom, and finally the left.
//
// With more than four arguments no padding will be added.
func (s Style) Padding(i ...int) Style {
	top, right, bottom, left, ok := whichSidesInt(i...)
	if !ok {
		return s
	}

	s.set(paddingTopKey, top)
	s.set(paddingRightKey, right)
	s.set(paddingBottomKey, bottom)
	s.set(paddingLeftKey, left)
	return s
}

// PaddingLeft adds padding on the left.
func (s Style) PaddingLeft(i int) Style {
	s.set(paddingLeftKey, i)
	return s
}

// PaddingRight adds padding on the right.
func (s Style) PaddingRight(i int) Style {
	s.set(paddingRightKey, i)
	return s
}

// PaddingTop adds padding to the top of the block.
func (s Style) PaddingTop(i int) Style {
	s.set(paddingTopKey, i)
	return s
}

// PaddingBottom adds padding to the bottom of the block.
func (s Style) PaddingBottom(i int) Style {
	s.set(paddingBottomKey, i)
	return s
}

// PaddingChar sets the character used for padding. This is useful for
// rendering blocks with a specific character, such as a space or a dot.
// Example of using [NBSP] as padding to prevent line breaks:
//
//	```go
//	s := lipgloss.NewStyle().PaddingChar(lipgloss.NBSP)
//	```
func (s Style) PaddingChar(r rune) Style {
	s.set(paddingCharKey, r)
	return s
}

// ColorWhitespace determines whether or not the background color should be
// applied to the padding. This is true by default as it's more than likely the
// desired and expected behavior, but it can be disabled for certain graphic
// effects.
//
// Deprecated: Just use margins and padding.
func (s Style) ColorWhitespace(v bool) Style {
	s.set(colorWhitespaceKey, v)
	return s
}

// Margin is a shorthand method for setting margins on all sides at once.
//
// With one argument, the value is applied to all sides.
//
// With two arguments, the value is applied to the vertical and horizontal
// sides, in that order.
//
// With three arguments, the value is applied to the top side, the horizontal
// sides, and the bottom side, in that order.
//
// With four arguments, the value is applied clockwise starting from the top
// side, followed by the right side, then the bottom, and finally the left.
//
// With more than four arguments no margin will be added.
func (s Style) Margin(i ...int) Style {
	top, right, bottom, left, ok := whichSidesInt(i...)
	if !ok {
		return s
	}

	s.set(marginTopKey, top)
	s.set(marginRightKey, right)
	s.set(marginBottomKey, bottom)
	s.set(marginLeftKey, left)
	return s
}

// MarginLeft sets the value of the left margin.
func (s Style) MarginLeft(i int) Style {
	s.set(marginLeftKey, i)
	return s
}

// MarginRight sets the value of the right margin.
func (s Style) MarginRight(i int) Style {
	s.set(marginRightKey, i)
	return s
}

// MarginTop sets the value of the top margin.
func (s Style) MarginTop(i int) Style {
	s.set(marginTopKey, i)
	return s
}

// MarginBottom sets the value of the bottom margin.
func (s Style) MarginBottom(i int) Style {
	s.set(marginBottomKey, i)
	return s
}

// MarginBackground sets the background color of the margin. Note that this is
// also set when inheriting from a style with a background color. In that case
// the background color on that style will set the margin color on this style.
func (s Style) MarginBackground(c color.Color) Style {
	s.set(marginBackgroundKey, c)
	return s
}

// MarginChar sets the character used for the margin. This is useful for
// rendering blocks with a specific character, such as a space or a dot.
func (s Style) MarginChar(r rune) Style {
	s.set(marginCharKey, r)
	return s
}

// Border is shorthand for setting the border style and which sides should
// have a border at once. The variadic argument sides works as follows:
//
// With one value, the value is applied to all sides.
//
// With two values, the values are applied to the vertical and horizontal
// sides, in that order.
//
// With three values, the values are applied to the top side, the horizontal
// sides, and the bottom side, in that order.
//
// With four values, the values are applied clockwise starting from the top
// side, followed by the right side, then the bottom, and finally the left.
//
// With more than four arguments the border will be applied to all sides.
//
// Examples:
//
//	// Applies borders to the top and bottom only
//	lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false)
//
//	// Applies rounded borders to the right and bottom only
//	lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), false, true, true, false)
func (s Style) Border(b Border, sides ...bool) Style {
	s.set(borderStyleKey, b)

	top, right, bottom, left, ok := whichSidesBool(sides...)
	if !ok {
		top = true
		right = true
		bottom = true
		left = true
	}

	s.set(borderTopKey, top)
	s.set(borderRightKey, right)
	s.set(borderBottomKey, bottom)
	s.set(borderLeftKey, left)

	return s
}

// BorderStyle defines the Border on a style. A Border contains a series of
// definitions for the sides and corners of a border.
//
// Note that if border visibility has not been set for any sides when setting
// the border style, the border will be enabled for all sides during rendering.
//
// You can define border characters as you'd like, though several default
// styles are included: NormalBorder(), RoundedBorder(), BlockBorder(),
// OuterHalfBlockBorder(), InnerHalfBlockBorder(), ThickBorder(),
// and DoubleBorder().
//
// Example:
//
//	lipgloss.NewStyle().BorderStyle(lipgloss.ThickBorder())
func (s Style) BorderStyle(b Border) Style {
	s.set(borderStyleKey, b)
	return s
}

// BorderTop determines whether or not to draw a top border.
func (s Style) BorderTop(v bool) Style {
	s.set(borderTopKey, v)
	return s
}

// BorderRight determines whether or not to draw a right border.
func (s Style) BorderRight(v bool) Style {
	s.set(borderRightKey, v)
	return s
}

// BorderBottom determines whether or not to draw a bottom border.
func (s Style) BorderBottom(v bool) Style {
	s.set(borderBottomKey, v)
	return s
}

// BorderLeft determines whether or not to draw a left border.
func (s Style) BorderLeft(v bool) Style {
	s.set(borderLeftKey, v)
	return s
}

// BorderForeground is a shorthand function for setting all of the
// foreground colors of the borders at once. The arguments work as follows:
//
// With one argument, the argument is applied to all sides.
//
// With two arguments, the arguments are applied to the vertical and horizontal
// sides, in that order.
//
// With three arguments, the arguments are applied to the top side, the
// horizontal sides, and the bottom side, in that order.
//
// With four arguments, the arguments are applied clockwise starting from the
// top side, followed by the right side, then the bottom, and finally the left.
//
// With more than four arguments nothing will be set.
func (s Style) BorderForeground(c ...color.Color) Style {
	if len(c) == 0 {
		return s
	}

	top, right, bottom, left, ok := whichSidesColor(c...)
	if !ok {
		return s
	}

	s.set(borderTopForegroundKey, top)
	s.set(borderRightForegroundKey, right)
	s.set(borderBottomForegroundKey, bottom)
	s.set(borderLeftForegroundKey, left)

	return s
}

// BorderTopForeground set the foreground color for the top of the border.
func (s Style) BorderTopForeground(c color.Color) Style {
	s.set(borderTopForegroundKey, c)
	return s
}

// BorderRightForeground sets the foreground color for the right side of the
// border.
func (s Style) BorderRightForeground(c color.Color) Style {
	s.set(borderRightForegroundKey, c)
	return s
}

// BorderBottomForeground sets the foreground color for the bottom of the
// border.
func (s Style) BorderBottomForeground(c color.Color) Style {
	s.set(borderBottomForegroundKey, c)
	return s
}

// BorderLeftForeground sets the foreground color for the left side of the
// border.
func (s Style) BorderLeftForeground(c color.Color) Style {
	s.set(borderLeftForegroundKey, c)
	return s
}

// BorderForegroundBlend sets the foreground colors for the border blend. At least
// 2 colors are required to use blending, otherwise this will no-op with 0 colors,
// and pass to BorderForeground with 1 color. This will override all other border
// foreground colors when used.
//
// When providing colors, in most cases (e.g. when all border sides are enabled),
// you will want to provide a wrapping-set of colors, so the start and end color
// are either the same, or very similar. For example:
//
//	lipgloss.NewStyle().BorderForegroundBlend(
//		lipgloss.Color("#00FA68"),
//		lipgloss.Color("#9900FF"),
//		lipgloss.Color("#ED5353"),
//		lipgloss.Color("#9900FF"),
//		lipgloss.Color("#00FA68"),
//	)
func (s Style) BorderForegroundBlend(c ...color.Color) Style {
	if len(c) == 0 {
		return s
	}

	// Insufficient colors to use blending, pass to BorderForeground.
	if len(c) == 1 {
		return s.BorderForeground(c...)
	}

	s.set(borderForegroundBlendKey, c)
	return s
}

// BorderForegroundBlendOffset sets the border blend offset cells, starting from
// the top left corner. Value can be positive or negative, and does not need to
// equal the dimensions of the border region. Direction (when positive) is as
// follows ("o" is starting point):
//
//	  o -------->
//	  ┌──────────┐
//	^ │          │ |
//	| │          │ |
//	| │          │ |
//	| │          │ v
//	  └──────────┘
//	   <---------
func (s Style) BorderForegroundBlendOffset(v int) Style {
	s.set(borderForegroundBlendOffsetKey, v)
	return s
}

// BorderBackground is a shorthand function for setting all of the
// background colors of the borders at once. The arguments work as follows:
//
// With one argument, the argument is applied to all sides.
//
// With two arguments, the arguments are applied to the vertical and horizontal
// sides, in that order.
//
// With three arguments, the arguments are applied to the top side, the
// horizontal sides, and the bottom side, in that order.
//
// With four arguments, the arguments are applied clockwise starting from the
// top side, followed by the right side, then the bottom, and finally the left.
//
// With more than four arguments nothing will be set.
func (s Style) BorderBackground(c ...color.Color) Style {
	if len(c) == 0 {
		return s
	}

	top, right, bottom, left, ok := whichSidesColor(c...)
	if !ok {
		return s
	}

	s.set(borderTopBackgroundKey, top)
	s.set(borderRightBackgroundKey, right)
	s.set(borderBottomBackgroundKey, bottom)
	s.set(borderLeftBackgroundKey, left)

	return s
}

// BorderTopBackground sets the background color of the top of the border.
func (s Style) BorderTopBackground(c color.Color) Style {
	s.set(borderTopBackgroundKey, c)
	return s
}

// BorderRightBackground sets the background color of right side the border.
func (s Style) BorderRightBackground(c color.Color) Style {
	s.set(borderRightBackgroundKey, c)
	return s
}

// BorderBottomBackground sets the background color of the bottom of the
// border.
func (s Style) BorderBottomBackground(c color.Color) Style {
	s.set(borderBottomBackgroundKey, c)
	return s
}

// BorderLeftBackground set the background color of the left side of the
// border.
func (s Style) BorderLeftBackground(c color.Color) Style {
	s.set(borderLeftBackgroundKey, c)
	return s
}

// Inline makes rendering output one line and disables the rendering of
// margins, padding and borders. This is useful when you need a style to apply
// only to font rendering and don't want it to change any physical dimensions.
// It works well with Style.MaxWidth.
//
// Because this in intended to be used at the time of render, this method will
// not mutate the style and instead return a copy.
//
// Example:
//
//	var userInput string = "..."
//	var userStyle = text.Style{ /* ... */ }
//	fmt.Println(userStyle.Inline(true).Render(userInput))
func (s Style) Inline(v bool) Style {
	o := s // copy
	o.set(inlineKey, v)
	return o
}

// MaxWidth applies a max width to a given style. This is useful in enforcing
// a certain width at render time, particularly with arbitrary strings and
// styles.
//
// Because this in intended to be used at the time of render, this method will
// not mutate the style and instead return a copy.
//
// Example:
//
//	var userInput string = "..."
//	var userStyle = text.Style{ /* ... */ }
//	fmt.Println(userStyle.MaxWidth(16).Render(userInput))
func (s Style) MaxWidth(n int) Style {
	o := s // copy
	o.set(maxWidthKey, n)
	return o
}

// MaxHeight applies a max height to a given style. This is useful in enforcing
// a certain height at render time, particularly with arbitrary strings and
// styles.
//
// Because this in intended to be used at the time of render, this method will
// not mutate the style and instead returns a copy.
func (s Style) MaxHeight(n int) Style {
	o := s // copy
	o.set(maxHeightKey, n)
	return o
}

// NoTabConversion can be passed to [Style.TabWidth] to disable the replacement
// of tabs with spaces at render time.
const NoTabConversion = -1

// TabWidth sets the number of spaces that a tab (/t) should be rendered as.
// When set to 0, tabs will be removed. To disable the replacement of tabs with
// spaces entirely, set this to [NoTabConversion].
//
// By default, tabs will be replaced with 4 spaces.
func (s Style) TabWidth(n int) Style {
	if n <= -1 {
		n = -1
	}
	s.set(tabWidthKey, n)
	return s
}

// UnderlineSpaces determines whether to underline spaces between words. By
// default, this is true. Spaces can also be underlined without underlining the
// text itself.
func (s Style) UnderlineSpaces(v bool) Style {
	s.set(underlineSpacesKey, v)
	return s
}

// StrikethroughSpaces determines whether to apply strikethroughs to spaces
// between words. By default, this is true. Spaces can also be struck without
// underlining the text itself.
func (s Style) StrikethroughSpaces(v bool) Style {
	s.set(strikethroughSpacesKey, v)
	return s
}

// Transform applies a given function to a string at render time, allowing for
// the string being rendered to be manipuated.
//
// Example:
//
//	s := NewStyle().Transform(strings.ToUpper)
//	fmt.Println(s.Render("raow!") // "RAOW!"
func (s Style) Transform(fn func(string) string) Style {
	s.set(transformKey, fn)
	return s
}

// Hyperlink sets a hyperlink on a style. This is useful for rendering text that
// can be clicked on in a terminal emulator that supports hyperlinks.
//
// Example:
//
//	s := lipgloss.NewStyle().Hyperlink("https://charm.sh")
//	s := lipgloss.NewStyle().Hyperlink("https://charm.sh", "id=1")
func (s Style) Hyperlink(link string, params ...string) Style {
	s.set(linkKey, link)
	if len(params) > 0 {
		s.set(linkParamsKey, strings.Join(params, ":"))
	}
	return s
}

// whichSidesInt is a helper method for setting values on sides of a block based
// on the number of arguments. It follows the CSS shorthand rules for blocks
// like margin, padding. and borders. Here are how the rules work:
//
// 0 args:  do nothing
// 1 arg:   all sides
// 2 args:  top -> bottom
// 3 args:  top -> horizontal -> bottom
// 4 args:  top -> right -> bottom -> left
// 5+ args: do nothing.
func whichSidesInt(i ...int) (top, right, bottom, left int, ok bool) {
	switch len(i) {
	case 1:
		top = i[0]
		bottom = i[0]
		left = i[0]
		right = i[0]
		ok = true
	case 2: //nolint:mnd
		top = i[0]
		bottom = i[0]
		left = i[1]
		right = i[1]
		ok = true
	case 3: //nolint:mnd
		top = i[0]
		left = i[1]
		right = i[1]
		bottom = i[2]
		ok = true
	case 4: //nolint:mnd
		top = i[0]
		right = i[1]
		bottom = i[2]
		left = i[3]
		ok = true
	}
	return top, right, bottom, left, ok
}

// whichSidesBool is like whichSidesInt, except it operates on a series of
// boolean values. See the comment on whichSidesInt for details on how this
// works.
func whichSidesBool(i ...bool) (top, right, bottom, left bool, ok bool) {
	switch len(i) {
	case 1:
		top = i[0]
		bottom = i[0]
		left = i[0]
		right = i[0]
		ok = true
	case 2: //nolint:mnd
		top = i[0]
		bottom = i[0]
		left = i[1]
		right = i[1]
		ok = true
	case 3: //nolint:mnd
		top = i[0]
		left = i[1]
		right = i[1]
		bottom = i[2]
		ok = true
	case 4: //nolint:mnd
		top = i[0]
		right = i[1]
		bottom = i[2]
		left = i[3]
		ok = true
	}
	return top, right, bottom, left, ok
}

// whichSidesColor is like whichSides, except it operates on a series of
// boolean values. See the comment on whichSidesInt for details on how this
// works.
func whichSidesColor(i ...color.Color) (top, right, bottom, left color.Color, ok bool) {
	switch len(i) {
	case 1:
		top = i[0]
		bottom = i[0]
		left = i[0]
		right = i[0]
		ok = true
	case 2: //nolint:mnd
		top = i[0]
		bottom = i[0]
		left = i[1]
		right = i[1]
		ok = true
	case 3: //nolint:mnd
		top = i[0]
		left = i[1]
		right = i[1]
		bottom = i[2]
		ok = true
	case 4: //nolint:mnd
		top = i[0]
		right = i[1]
		bottom = i[2]
		left = i[3]
		ok = true
	}
	return top, right, bottom, left, ok
}
