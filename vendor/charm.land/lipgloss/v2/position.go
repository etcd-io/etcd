package lipgloss

import (
	"math"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Position represents a position along a horizontal or vertical axis. It's in
// situations where an axis is involved, like alignment, joining, placement and
// so on.
//
// A value of 0 represents the start (the left or top) and 1 represents the end
// (the right or bottom). 0.5 represents the center.
//
// There are constants Top, Bottom, Center, Left and Right in this package that
// can be used to aid readability.
type Position float64

func (p Position) value() float64 {
	return math.Min(1, math.Max(0, float64(p)))
}

// Position aliases.
const (
	Top    Position = 0.0
	Bottom Position = 1.0
	Center Position = 0.5
	Left   Position = 0.0
	Right  Position = 1.0
)

// Place places a string or text block vertically in an unstyled box of a given
// width or height.
func Place(width, height int, hPos, vPos Position, str string, opts ...WhitespaceOption) string {
	return PlaceVertical(height, vPos, PlaceHorizontal(width, hPos, str, opts...), opts...)
}

// PlaceHorizontal places a string or text block horizontally in an unstyled
// block of a given width. If the given width is shorter than the max width of
// the string (measured by its longest line) this will be a noop.
func PlaceHorizontal(width int, pos Position, str string, opts ...WhitespaceOption) string {
	lines, contentWidth := getLines(str)
	gap := width - contentWidth

	if gap <= 0 {
		return str
	}

	ws := newWhitespace(opts...)

	var b strings.Builder
	for i, l := range lines {
		// Is this line shorter than the longest line?
		short := max(0, contentWidth-ansi.StringWidth(l))

		switch pos {
		case Left:
			b.WriteString(l)
			b.WriteString(ws.render(gap + short))

		case Right:
			b.WriteString(ws.render(gap + short))
			b.WriteString(l)

		default: // somewhere in the middle
			totalGap := gap + short

			split := int(math.Round(float64(totalGap) * pos.value()))
			left := totalGap - split
			right := totalGap - left

			b.WriteString(ws.render(left))
			b.WriteString(l)
			b.WriteString(ws.render(right))
		}

		if i < len(lines)-1 {
			b.WriteRune('\n')
		}
	}

	return b.String()
}

// PlaceVertical places a string or text block vertically in an unstyled block
// of a given height. If the given height is shorter than the height of the
// string (measured by its newlines) then this will be a noop.
func PlaceVertical(height int, pos Position, str string, opts ...WhitespaceOption) string {
	contentHeight := strings.Count(str, "\n") + 1
	gap := height - contentHeight

	if gap <= 0 {
		return str
	}

	ws := newWhitespace(opts...)

	_, width := getLines(str)
	emptyLine := ws.render(width)
	b := strings.Builder{}

	switch pos {
	case Top:
		b.WriteString(str)
		b.WriteRune('\n')
		for i := range gap {
			b.WriteString(emptyLine)
			if i < gap-1 {
				b.WriteRune('\n')
			}
		}

	case Bottom:
		b.WriteString(strings.Repeat(emptyLine+"\n", gap))
		b.WriteString(str)

	default: // Somewhere in the middle
		split := int(math.Round(float64(gap) * pos.value()))
		top := gap - split
		bottom := gap - top

		b.WriteString(strings.Repeat(emptyLine+"\n", top))
		b.WriteString(str)

		for range bottom {
			b.WriteRune('\n')
			b.WriteString(emptyLine)
		}
	}

	return b.String()
}
