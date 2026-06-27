package lipgloss

import (
	"image/color"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"
	"github.com/rivo/uniseg"
)

// Border contains a series of values which comprise the various parts of a
// border.
type Border struct {
	Top          string
	Bottom       string
	Left         string
	Right        string
	TopLeft      string
	TopRight     string
	BottomLeft   string
	BottomRight  string
	MiddleLeft   string
	MiddleRight  string
	Middle       string
	MiddleTop    string
	MiddleBottom string
}

// GetTopSize returns the width of the top border. If borders contain runes of
// varying widths, the widest rune is returned. If no border exists on the top
// edge, 0 is returned.
func (b Border) GetTopSize() int {
	return getBorderEdgeWidth(b.TopLeft, b.Top, b.TopRight)
}

// GetRightSize returns the width of the right border. If borders contain
// runes of varying widths, the widest rune is returned. If no border exists on
// the right edge, 0 is returned.
func (b Border) GetRightSize() int {
	return getBorderEdgeWidth(b.TopRight, b.Right, b.BottomRight)
}

// GetBottomSize returns the width of the bottom border. If borders contain
// runes of varying widths, the widest rune is returned. If no border exists on
// the bottom edge, 0 is returned.
func (b Border) GetBottomSize() int {
	return getBorderEdgeWidth(b.BottomLeft, b.Bottom, b.BottomRight)
}

// GetLeftSize returns the width of the left border. If borders contain runes
// of varying widths, the widest rune is returned. If no border exists on the
// left edge, 0 is returned.
func (b Border) GetLeftSize() int {
	return getBorderEdgeWidth(b.TopLeft, b.Left, b.BottomLeft)
}

func getBorderEdgeWidth(borderParts ...string) (maxWidth int) {
	for _, piece := range borderParts {
		maxWidth = max(maxWidth, maxRuneWidth(piece))
	}
	return maxWidth
}

var (
	noBorder = Border{}

	normalBorder = Border{
		Top:          "─",
		Bottom:       "─",
		Left:         "│",
		Right:        "│",
		TopLeft:      "┌",
		TopRight:     "┐",
		BottomLeft:   "└",
		BottomRight:  "┘",
		MiddleLeft:   "├",
		MiddleRight:  "┤",
		Middle:       "┼",
		MiddleTop:    "┬",
		MiddleBottom: "┴",
	}

	roundedBorder = Border{
		Top:          "─",
		Bottom:       "─",
		Left:         "│",
		Right:        "│",
		TopLeft:      "╭",
		TopRight:     "╮",
		BottomLeft:   "╰",
		BottomRight:  "╯",
		MiddleLeft:   "├",
		MiddleRight:  "┤",
		Middle:       "┼",
		MiddleTop:    "┬",
		MiddleBottom: "┴",
	}

	blockBorder = Border{
		Top:          "█",
		Bottom:       "█",
		Left:         "█",
		Right:        "█",
		TopLeft:      "█",
		TopRight:     "█",
		BottomLeft:   "█",
		BottomRight:  "█",
		MiddleLeft:   "█",
		MiddleRight:  "█",
		Middle:       "█",
		MiddleTop:    "█",
		MiddleBottom: "█",
	}

	outerHalfBlockBorder = Border{
		Top:         "▀",
		Bottom:      "▄",
		Left:        "▌",
		Right:       "▐",
		TopLeft:     "▛",
		TopRight:    "▜",
		BottomLeft:  "▙",
		BottomRight: "▟",
	}

	innerHalfBlockBorder = Border{
		Top:         "▄",
		Bottom:      "▀",
		Left:        "▐",
		Right:       "▌",
		TopLeft:     "▗",
		TopRight:    "▖",
		BottomLeft:  "▝",
		BottomRight: "▘",
	}

	thickBorder = Border{
		Top:          "━",
		Bottom:       "━",
		Left:         "┃",
		Right:        "┃",
		TopLeft:      "┏",
		TopRight:     "┓",
		BottomLeft:   "┗",
		BottomRight:  "┛",
		MiddleLeft:   "┣",
		MiddleRight:  "┫",
		Middle:       "╋",
		MiddleTop:    "┳",
		MiddleBottom: "┻",
	}

	doubleBorder = Border{
		Top:          "═",
		Bottom:       "═",
		Left:         "║",
		Right:        "║",
		TopLeft:      "╔",
		TopRight:     "╗",
		BottomLeft:   "╚",
		BottomRight:  "╝",
		MiddleLeft:   "╠",
		MiddleRight:  "╣",
		Middle:       "╬",
		MiddleTop:    "╦",
		MiddleBottom: "╩",
	}

	hiddenBorder = Border{
		Top:          " ",
		Bottom:       " ",
		Left:         " ",
		Right:        " ",
		TopLeft:      " ",
		TopRight:     " ",
		BottomLeft:   " ",
		BottomRight:  " ",
		MiddleLeft:   " ",
		MiddleRight:  " ",
		Middle:       " ",
		MiddleTop:    " ",
		MiddleBottom: " ",
	}

	markdownBorder = Border{
		Top:          "-",
		Bottom:       "-",
		Left:         "|",
		Right:        "|",
		TopLeft:      "|",
		TopRight:     "|",
		BottomLeft:   "|",
		BottomRight:  "|",
		MiddleLeft:   "|",
		MiddleRight:  "|",
		Middle:       "|",
		MiddleTop:    "|",
		MiddleBottom: "|",
	}

	asciiBorder = Border{
		Top:          "-",
		Bottom:       "-",
		Left:         "|",
		Right:        "|",
		TopLeft:      "+",
		TopRight:     "+",
		BottomLeft:   "+",
		BottomRight:  "+",
		MiddleLeft:   "+",
		MiddleRight:  "+",
		Middle:       "+",
		MiddleTop:    "+",
		MiddleBottom: "+",
	}
)

// NormalBorder returns a standard-type border with a normal weight and 90
// degree corners.
func NormalBorder() Border {
	return normalBorder
}

// RoundedBorder returns a border with rounded corners.
func RoundedBorder() Border {
	return roundedBorder
}

// BlockBorder returns a border that takes the whole block.
func BlockBorder() Border {
	return blockBorder
}

// OuterHalfBlockBorder returns a half-block border that sits outside the frame.
func OuterHalfBlockBorder() Border {
	return outerHalfBlockBorder
}

// InnerHalfBlockBorder returns a half-block border that sits inside the frame.
func InnerHalfBlockBorder() Border {
	return innerHalfBlockBorder
}

// ThickBorder returns a border that's thicker than the one returned by
// NormalBorder.
func ThickBorder() Border {
	return thickBorder
}

// DoubleBorder returns a border comprised of two thin strokes.
func DoubleBorder() Border {
	return doubleBorder
}

// HiddenBorder returns a border that renders as a series of single-cell
// spaces. It's useful for cases when you want to remove a standard border but
// maintain layout positioning. This said, you can still apply a background
// color to a hidden border.
func HiddenBorder() Border {
	return hiddenBorder
}

// MarkdownBorder return a table border in markdown style.
//
// Make sure to disable top and bottom border for the best result. This will
// ensure that the output is valid markdown.
//
//	table.New().Border(lipgloss.MarkdownBorder()).BorderTop(false).BorderBottom(false)
func MarkdownBorder() Border {
	return markdownBorder
}

// ASCIIBorder returns a table border with ASCII characters.
func ASCIIBorder() Border {
	return asciiBorder
}

type borderBlend struct {
	topGradient    []color.Color
	rightGradient  []color.Color
	bottomGradient []color.Color
	leftGradient   []color.Color
}

func (s Style) borderBlend(width, height int, colors ...color.Color) *borderBlend {
	gradient := Blend1D(
		(height+width+2)*2,
		colors...,
	)

	// Rotate array forward or reverse based on the offset if provided.
	if r := -s.getAsInt(borderForegroundBlendOffsetKey); r != 0 {
		n := len(gradient)
		r %= n
		if r < 0 {
			r += n
		}
		slices.Reverse(gradient[:r])
		slices.Reverse(gradient[r:])
		slices.Reverse(gradient)
	}

	offset := 0
	getFromOffset := func(size int) (s []color.Color) {
		s = gradient[offset : offset+size]
		offset += size
		return s
	}

	blend := &borderBlend{
		topGradient:    getFromOffset(width + 2),
		rightGradient:  getFromOffset(height),
		bottomGradient: getFromOffset(width + 2),
		leftGradient:   getFromOffset(height),
	}

	// bottom and left gradients are reversed because they are drawn in reverse order.
	slices.Reverse(blend.bottomGradient)
	slices.Reverse(blend.leftGradient)

	return blend
}

func (s Style) applyBorder(str string) string {
	var (
		border    = s.getBorderStyle()
		hasTop    = s.getAsBool(borderTopKey, false)
		hasRight  = s.getAsBool(borderRightKey, false)
		hasBottom = s.getAsBool(borderBottomKey, false)
		hasLeft   = s.getAsBool(borderLeftKey, false)
	)

	// If a border is set and no sides have been specifically turned on or off
	// render borders on all sides.
	if s.isBorderStyleSetWithoutSides() {
		hasTop = true
		hasRight = true
		hasBottom = true
		hasLeft = true
	}

	// If no border is set or all borders are been disabled, abort.
	if border == noBorder || (!hasTop && !hasRight && !hasBottom && !hasLeft) {
		return str
	}

	lines, width := getLines(str)

	if hasLeft {
		if border.Left == "" {
			border.Left = " "
		}
		width += maxRuneWidth(border.Left)
	}

	if hasRight {
		if border.Right == "" {
			border.Right = " "
		}
		width += maxRuneWidth(border.Right)
	}

	// If corners should be rendered but are set with the empty string, fill them
	// with a single space.
	if hasTop && hasLeft && border.TopLeft == "" {
		border.TopLeft = " "
	}
	if hasTop && hasRight && border.TopRight == "" {
		border.TopRight = " "
	}
	if hasBottom && hasLeft && border.BottomLeft == "" {
		border.BottomLeft = " "
	}
	if hasBottom && hasRight && border.BottomRight == "" {
		border.BottomRight = " "
	}

	// Figure out which corners we should actually be using based on which
	// sides are set to show.
	if hasTop {
		switch {
		case !hasLeft && !hasRight:
			border.TopLeft = ""
			border.TopRight = ""
		case !hasLeft:
			border.TopLeft = ""
		case !hasRight:
			border.TopRight = ""
		}
	}
	if hasBottom {
		switch {
		case !hasLeft && !hasRight:
			border.BottomLeft = ""
			border.BottomRight = ""
		case !hasLeft:
			border.BottomLeft = ""
		case !hasRight:
			border.BottomRight = ""
		}
	}

	// For now, limit corners to one rune.
	border.TopLeft = getFirstRuneAsString(border.TopLeft)
	border.TopRight = getFirstRuneAsString(border.TopRight)
	border.BottomRight = getFirstRuneAsString(border.BottomRight)
	border.BottomLeft = getFirstRuneAsString(border.BottomLeft)

	var topFG, rightFG, bottomFG, leftFG color.Color
	var (
		blendFG  = s.getAsColors(borderForegroundBlendKey)
		topBG    = s.getAsColor(borderTopBackgroundKey)
		rightBG  = s.getAsColor(borderRightBackgroundKey)
		bottomBG = s.getAsColor(borderBottomBackgroundKey)
		leftBG   = s.getAsColor(borderLeftBackgroundKey)
	)

	var blend *borderBlend
	if len(blendFG) > 0 {
		blend = s.borderBlend(width, len(lines), blendFG...)
	} else {
		topFG = s.getAsColor(borderTopForegroundKey)
		rightFG = s.getAsColor(borderRightForegroundKey)
		bottomFG = s.getAsColor(borderBottomForegroundKey)
		leftFG = s.getAsColor(borderLeftForegroundKey)
	}

	var out strings.Builder

	// Render top
	if hasTop {
		top := renderHorizontalEdge(border.TopLeft, border.Top, border.TopRight, width)
		if blend != nil {
			out.WriteString(s.styleBorderBlend(top, blend.topGradient, topBG))
		} else {
			out.WriteString(s.styleBorder(top, topFG, topBG))
		}
		out.WriteRune('\n')
	}

	leftRunes := []rune(border.Left)
	leftIndex := 0

	rightRunes := []rune(border.Right)
	rightIndex := 0

	// Render sides
	var r string
	for i, l := range lines {
		if hasLeft {
			r = string(leftRunes[leftIndex])
			leftIndex++
			if leftIndex >= len(leftRunes) {
				leftIndex = 0
			}
			if blend != nil {
				out.WriteString(s.styleBorder(r, blend.leftGradient[i], leftBG))
			} else {
				out.WriteString(s.styleBorder(r, leftFG, leftBG))
			}
		}
		out.WriteString(l)
		if hasRight {
			r = string(rightRunes[rightIndex])
			rightIndex++
			if rightIndex >= len(rightRunes) {
				rightIndex = 0
			}
			if blend != nil {
				out.WriteString(s.styleBorder(r, blend.rightGradient[i], rightBG))
			} else {
				out.WriteString(s.styleBorder(r, rightFG, rightBG))
			}
		}
		if i < len(lines)-1 {
			out.WriteRune('\n')
		}
	}

	// Render bottom
	if hasBottom {
		bottom := renderHorizontalEdge(border.BottomLeft, border.Bottom, border.BottomRight, width)
		out.WriteRune('\n')
		if blend != nil {
			out.WriteString(s.styleBorderBlend(bottom, blend.bottomGradient, bottomBG))
		} else {
			out.WriteString(s.styleBorder(bottom, bottomFG, bottomBG))
		}
	}

	return out.String()
}

// Render the horizontal (top or bottom) portion of a border.
func renderHorizontalEdge(left, middle, right string, width int) string {
	if middle == "" {
		middle = " "
	}

	leftWidth := ansi.StringWidth(left)
	rightWidth := ansi.StringWidth(right)

	runes := []rune(middle)
	j := 0

	out := strings.Builder{}
	out.WriteString(left)

	for i := 0; i < width-leftWidth-rightWidth; {
		r := runes[j]
		out.WriteRune(r)
		i += ansi.StringWidth(string(r))
		j++
		if j >= len(runes) {
			j = 0
		}
	}

	out.WriteString(right)
	return out.String()
}

// styleBorder applies foreground and background styling to a border.
func (s Style) styleBorder(border string, fg, bg color.Color) string {
	if fg == noColor && bg == noColor {
		return border
	}
	var style ansi.Style
	if fg != noColor {
		style = style.ForegroundColor(fg)
	}
	if bg != noColor {
		style = style.BackgroundColor(bg)
	}
	return style.Styled(border)
}

// styleBorderBlend applies foreground and background styling to a border, using blending.
func (s Style) styleBorderBlend(border string, fg []color.Color, bg color.Color) string {
	var out strings.Builder
	var style ansi.Style
	var i int

	gr := uniseg.NewGraphemes(border)
	for gr.Next() {
		style = style[:0]
		if fg[i] != noColor {
			style = style.ForegroundColor(fg[i])
		}
		if bg != noColor {
			style = style.BackgroundColor(bg)
		}
		_, _ = out.WriteString(style.String())
		_, _ = out.Write(gr.Bytes())
		i++
	}
	_, _ = out.WriteString(ansi.ResetStyle)
	return out.String()
}

func maxRuneWidth(str string) int {
	switch len(str) {
	case 0:
		return 0
	case 1:
		return displaywidth.String(str)
	}

	var width int

	g := displaywidth.StringGraphemes(str)
	for g.Next() {
		width = max(width, g.Width())
	}
	return width
}

func getFirstRuneAsString(str string) string {
	if str == "" {
		return str
	}
	_, size := utf8.DecodeRuneInString(str)
	return str[:size]
}
