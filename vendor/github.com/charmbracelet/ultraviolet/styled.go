package uv

import (
	"bytes"
	"image/color"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
)

// StyledString is a string that can be decomposed into a series of styled
// lines and cells. It is used to disassemble a rendered string with ANSI
// escape codes into a series of cells that can be used in a [Buffer].
// A StyledString supports reading [ansi.SGR] and [ansi.Hyperlink] escape
// codes.
type StyledString struct {
	// Text is the original string that was used to create the styled string.
	Text string
	// Wrap determines whether the styled string should wrap to the next line.
	Wrap bool
	// Tail is the string that will be appended to the end of the line when the
	// string is truncated i.e. when [StyledString.Wrap] is false.
	Tail string
}

var _ Drawable = (*StyledString)(nil)

// NewStyledString creates a new [StyledString] for the given method and styled
// string. The method is used to calculate the width of each line.
func NewStyledString(str string) *StyledString {
	ss := new(StyledString)
	ss.Text = str
	return ss
}

// String returns the text of the styled string.
//
// It implements the [fmt.Stringer] interface.
func (s *StyledString) String() string {
	return s.Text
}

// Draw renders the styled string to the given buffer at the
// specified area.
func (s *StyledString) Draw(buf Screen, area Rectangle) {
	// Clear the area before drawing.
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			buf.SetCell(x, y, nil)
		}
	}
	str := s.Text
	// We need to normalize newlines "\n" to "\r\n" to emulate a raw terminal
	// output.
	str = strings.ReplaceAll(str, "\r\n", "\n")
	printString(buf, buf.WidthMethod(), area.Min.X, area.Min.Y, area, str, !s.Wrap, s.Tail)
}

// Height returns the number of lines in the styled string. This is the number
// of lines that the styled string will occupy when rendered to the screen.
func (s *StyledString) Height() int {
	return strings.Count(s.Text, "\n") + 1
}

// UnicodeWidth returns the cells width of the widest line in the styled string
// using the [ansi.GraphemeWidth] method.
func (s *StyledString) UnicodeWidth() int {
	w, _ := s.widthHeight(ansi.GraphemeWidth)
	return w
}

// WcWidth returns the cells width of the widest line in the styled string
// using the [ansi.WcWidth] method.
func (s *StyledString) WcWidth() int {
	w, _ := s.widthHeight(ansi.WcWidth)
	return w
}

func (s *StyledString) widthHeight(m ansi.Method) (w, h int) {
	lines := strings.Split(s.Text, "\n")
	h = len(lines)
	for _, l := range lines {
		w = max(w, m.StringWidth(l))
	}
	return
}

// Bounds returns the minimum area that can contain the whole styled string.
func (s *StyledString) Bounds() Rectangle {
	w, h := s.widthHeight(ansi.GraphemeWidth)
	return Rect(0, 0, w, h)
}

var parserPool = &sync.Pool{
	New: func() any {
		return ansi.NewParser()
	},
}

// printString draws a string starting at the given position.
func printString[T []byte | string](
	s Screen,
	m WidthMethod,
	x, y int,
	bounds Rectangle, str T,
	truncate bool, tail string,
) {
	// We don't need to use a large buffer parser here. [ansi.GetParser]
	// returns a 4MB parsers which are too large for our use case. We use our
	// own pool of parsers that are smaller and more efficient for our use
	// case.
	p := parserPool.Get().(*ansi.Parser)
	defer parserPool.Put(p)

	var tailc Cell
	if truncate && len(tail) > 0 {
		tailc = *NewCell(m, tail)
	}

	decoder := ansi.DecodeSequenceWc[T]
	if m == ansi.GraphemeWidth {
		decoder = ansi.DecodeSequence[T]
	}

	var cell Cell
	var style Style
	var link Link
	var state byte
	for len(str) > 0 {
		seq, width, n, newState := decoder(str, state, p)
		switch width {
		case 1, 2, 3, 4: // wide cells can go up to 4 cells wide
			cell.Width = width
			cell.Content = string(seq)

			if !truncate && x+cell.Width > bounds.Max.X && y+1 < bounds.Max.Y {
				// Wrap the string to the width of the window
				x = bounds.Min.X
				y++
			}

			pos := Pos(x, y)
			if pos.In(bounds) {
				if truncate && tailc.Width > 0 && x+cell.Width > bounds.Max.X-tailc.Width {
					// Truncate the string and append the tail if any.
					cell = tailc
					cell.Style = style
					cell.Link = link
					s.SetCell(x, y, &cell)
					x += tailc.Width
				} else {
					// Print the cell to the screen
					cell.Style = style
					cell.Link = link
					s.SetCell(x, y, &cell)
					x += width
				}
			}

			// String is too long for the line, truncate it.
			// Make sure we reset the cell for the next iteration.
			cell = Cell{}
		default:
			// Valid sequences always have a non-zero Cmd.
			// TODO: Handle cursor movement and other sequences
			switch {
			case ansi.HasCsiPrefix(seq) && p.Command() == 'm':
				// SGR - Select Graphic Rendition
				ReadStyle(p.Params(), &style)
			case ansi.HasOscPrefix(seq) && p.Command() == 8:
				// Hyperlinks
				ReadLink(p.Data(), &link)
			case ansi.Equal(seq, T("\n")):
				y++
				// Always treat a NL as CR-LF similar to Termios ONLCR.
				fallthrough
			case ansi.Equal(seq, T("\r")):
				x = bounds.Min.X
			default:
				cell.Content += string(seq)
			}
		}

		// Advance the state and data
		state = newState
		str = str[n:]
	}

	// Make sure to set the last cell if it's not empty.
	if !cell.IsZero() {
		s.SetCell(x, y, &cell)
		cell = Cell{}
	}
}

// ReadStyle reads a Select Graphic Rendition (SGR) escape sequences from a
// list of parameters into pen.
func ReadStyle(params ansi.Params, pen *Style) {
	if len(params) == 0 {
		*pen = Style{}
		return
	}

	for i := 0; i < len(params); i++ {
		param, hasMore, _ := params.Param(i, 0)
		switch param {
		case 0: // Reset
			*pen = Style{}
		case 1: // Bold
			pen.Attrs |= AttrBold
		case 2: // Dim/Faint
			pen.Attrs |= AttrFaint
		case 3: // Italic
			pen.Attrs |= AttrItalic
		case 4: // Underline
			nextParam, _, ok := params.Param(i+1, 0)
			if hasMore && ok { // Only accept subparameters i.e. separated by ":"
				switch nextParam {
				case 0, 1, 2, 3, 4, 5:
					i++
					switch nextParam {
					case 0: // No Underline
						pen.Underline = UnderlineStyleNone
					case 1: // Single Underline
						pen.Underline = UnderlineStyleSingle
					case 2: // Double Underline
						pen.Underline = UnderlineStyleDouble
					case 3: // Curly Underline
						pen.Underline = UnderlineStyleCurly
					case 4: // Dotted Underline
						pen.Underline = UnderlineStyleDotted
					case 5: // Dashed Underline
						pen.Underline = UnderlineStyleDashed
					}
				}
			} else {
				// Single Underline
				pen.Underline = UnderlineStyleSingle
			}
		case 5: // Slow Blink
			pen.Attrs |= AttrBlink
		case 6: // Rapid Blink
			pen.Attrs |= AttrRapidBlink
		case 7: // Reverse
			pen.Attrs |= AttrReverse
		case 8: // Conceal
			pen.Attrs |= AttrConceal
		case 9: // Crossed-out/Strikethrough
			pen.Attrs |= AttrStrikethrough
		case 22: // Normal Intensity (not bold or faint)
			pen.Attrs &^= (AttrBold | AttrFaint)
		case 23: // Not italic, not Fraktur
			pen.Attrs &^= AttrItalic
		case 24: // Not underlined
			pen.Underline = UnderlineStyleNone
		case 25: // Blink off
			pen.Attrs &^= (AttrBlink | AttrRapidBlink)
		case 27: // Positive (not reverse)
			pen.Attrs &^= AttrReverse
		case 28: // Reveal
			pen.Attrs &^= AttrConceal
		case 29: // Not crossed out
			pen.Attrs &^= AttrStrikethrough
		case 30, 31, 32, 33, 34, 35, 36, 37: // Set foreground
			pen.Fg = ansi.Black + ansi.BasicColor(param-30) //nolint:gosec
		case 38: // Set foreground 256 or truecolor
			var c color.Color
			n := ansi.ReadStyleColor(params[i:], &c)
			if n > 0 {
				pen.Fg = c
				i += n - 1
			}
		case 39: // Default foreground
			pen.Fg = nil
		case 40, 41, 42, 43, 44, 45, 46, 47: // Set background
			pen.Bg = ansi.Black + ansi.BasicColor(param-40) //nolint:gosec
		case 48: // Set background 256 or truecolor
			var c color.Color
			n := ansi.ReadStyleColor(params[i:], &c)
			if n > 0 {
				pen.Bg = c
				i += n - 1
			}
		case 49: // Default Background
			pen.Bg = nil
		case 58: // Set underline color
			var c color.Color
			n := ansi.ReadStyleColor(params[i:], &c)
			if n > 0 {
				pen.UnderlineColor = c
				i += n - 1
			}
		case 59: // Default underline color
			pen.UnderlineColor = nil
		case 90, 91, 92, 93, 94, 95, 96, 97: // Set bright foreground
			pen.Fg = ansi.BrightBlack + ansi.BasicColor(param-90) //nolint:gosec
		case 100, 101, 102, 103, 104, 105, 106, 107: // Set bright background
			pen.Bg = ansi.BrightBlack + ansi.BasicColor(param-100) //nolint:gosec
		}
	}
}

// ReadLink reads a hyperlink escape sequence from a data buffer into link.
func ReadLink(p []byte, link *Link) {
	params := bytes.Split(p, []byte{';'})
	if len(params) != 3 {
		return
	}
	link.Params = string(params[1])
	link.URL = string(params[2])
}
