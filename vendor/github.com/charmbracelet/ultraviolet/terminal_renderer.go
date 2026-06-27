package uv

import (
	"bytes"
	"errors"
	"hash/maphash"
	"io"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// ErrInvalidDimensions is returned when the dimensions of a window are invalid
// for the operation.
var ErrInvalidDimensions = errors.New("invalid dimensions")

// capabilities represents a mask of supported ANSI escape sequences.
type capabilities uint

const (
	// Vertical Position Absolute [ansi.VPA].
	capVPA capabilities = 1 << iota
	// Horizontal Position Absolute [ansi.HPA].
	capHPA
	// Cursor Horizontal Absolute [ansi.CHA].
	capCHA
	// Cursor Horizontal Tab [ansi.CHT].
	capCHT
	// Cursor Backward Tab [ansi.CBT].
	capCBT
	// Repeat Previous Character [ansi.REP].
	capREP
	// Erase Character [ansi.ECH].
	capECH
	// Insert Character [ansi.ICH].
	capICH
	// Scroll Down [ansi.SD].
	capSD
	// Scroll Up [ansi.SU].
	capSU
	// These capabilities depend on the tty termios settings and are not
	// enabled by default.
	// Tabulation [ansi.HT].
	capHT
	// Backspace [ansi.BS].
	capBS

	noCaps  capabilities = 0
	allCaps              = capVPA | capHPA | capCHA | capCHT | capCBT | capREP | capECH | capICH | capSD | capSU
)

// Set sets the given capabilities.
func (v *capabilities) Set(c capabilities) {
	*v |= c
}

// Reset resets the given capabilities.
func (v *capabilities) Reset(c capabilities) {
	*v &^= c
}

// Contains returns whether the capabilities contains the given capability.
func (v capabilities) Contains(c capabilities) bool {
	return v&c == c
}

// cursor represents a terminal cursor.
type cursor struct {
	Cell
	Position
}

// LineData represents the metadata for a line.
type LineData struct {
	// First and last changed cell indices.
	FirstCell, LastCell int
	// Old index used for scrolling
	oldIndex int //nolint:unused
}

// tFlag is a bitmask of terminal flags.
type tFlag uint

// Terminal writer flags.
const (
	tRelativeCursor tFlag = 1 << iota
	tFullscreen
	tMapNewline
)

// Set sets the given flags.
func (v *tFlag) Set(c tFlag) {
	*v |= c
}

// Reset resets the given flags.
func (v *tFlag) Reset(c tFlag) {
	*v &^= c
}

// Contains returns whether the terminal flags contains the given flags.
func (v tFlag) Contains(c tFlag) bool {
	return v&c == c
}

// TerminalRenderer is a terminal screen render and lazy writer that buffers
// the output until it is flushed. It handles rendering a screen from a
// [Buffer] to the terminal with the minimal necessary escape sequences to
// transition the terminal to the new buffer state. It uses various escape
// sequence optimizations to reduce the number of bytes sent to the terminal.
// It's designed to be lazy and only flush the output when necessary by calling
// the [TerminalRenderer.Flush] method.
//
// The renderer handles the terminal's alternate screen and cursor visibility
// via the [TerminalRenderer.EnterAltScreen], [TerminalRenderer.ExitAltScreen],
// [TerminalRenderer.ShowCursor] and [TerminalRenderer.HideCursor] methods.
// Using these methods will queue the appropriate escape sequences to enter or
// exit the alternate screen and show or hide the cursor respectively to be
// flushed to the terminal.
//
// Use the [io.Writer] and [io.StringWriter] interfaces to queue custom
// commands the renderer.
//
// The renderer is not thread-safe, the caller must protect the renderer when
// using it from multiple goroutines.
type TerminalRenderer struct {
	w                io.Writer
	buf              *bytes.Buffer // buffer for writing to the screen
	curbuf           *Buffer       // the current buffer
	tabs             *TabStops
	hasher           maphash.Hash
	oldhash, newhash []uint64     // the old and new hash values for each line
	hashtab          []hashmap    // the hashmap table
	oldnum           []int        // old indices from previous hash
	cur, saved       cursor       // the current and saved cursors
	flags            tFlag        // terminal writer flags.
	term             string       // the terminal type
	scrollHeight     int          // keeps track of how many lines we've scrolled down (inline mode)
	clear            bool         // whether to force clear the screen
	caps             capabilities // terminal control sequence capabilities
	atPhantom        bool         // whether the cursor is out of bounds and at a phantom cell
	logger           Logger       // The logger used for debugging.

	// profile is the color profile to use when downsampling colors. This is
	// used to determine the appropriate color the terminal can display.
	profile colorprofile.Profile
}

// NewTerminalRenderer returns a new [TerminalRenderer] that uses the given
// writer, terminal type, and initializes the width of the terminal. The
// terminal type is used to determine the capabilities of the terminal and
// should be set to the value of the TERM environment variable.
//
// The renderer will try to detect the color profile from the output and the
// given environment variables. Use [TerminalRenderer.SetColorProfile] method
// to set a specific color profile for downsampling.
//
// See [TerminalRenderer] for more information on how to use the renderer.
func NewTerminalRenderer(w io.Writer, env []string) (s *TerminalRenderer) {
	s = new(TerminalRenderer)
	s.w = w
	s.profile = colorprofile.Detect(w, env)
	s.buf = new(bytes.Buffer)
	s.term = Environ(env).Getenv("TERM")
	s.caps = xtermCaps(s.term)
	s.cur = cursor{Cell: EmptyCell, Position: Pos(-1, -1)} // start at -1 to force a move
	s.saved = s.cur
	s.scrollHeight = 0
	s.oldhash, s.newhash = nil, nil
	return
}

// SetLogger sets the logger to use for debugging. If nil, no logging will be
// performed.
func (s *TerminalRenderer) SetLogger(logger Logger) {
	s.logger = logger
}

// SetColorProfile sets the color profile to use for downsampling colors. This
// is used to determine the appropriate color the terminal can display.
func (s *TerminalRenderer) SetColorProfile(profile colorprofile.Profile) {
	s.profile = profile
}

// SetMapNewline sets whether the terminal is currently mapping newlines to
// CRLF or carriage return and line feed. This is used to correctly determine
// how to move the cursor when writing to the screen.
func (s *TerminalRenderer) SetMapNewline(v bool) {
	if v {
		s.flags.Set(tMapNewline)
	} else {
		s.flags.Reset(tMapNewline)
	}
}

// SetBackspace sets whether to use backspace as a movement optimization.
func (s *TerminalRenderer) SetBackspace(v bool) {
	if v {
		s.caps.Set(capBS)
	} else {
		s.caps.Reset(capBS)
	}
}

// SetTabStops sets the tab stops for the terminal and enables hard tabs
// movement optimizations. Use -1 to disable hard tabs. This option is ignored
// when the terminal type is "linux" as it does not support hard tabs.
func (s *TerminalRenderer) SetTabStops(width int) {
	if width < 0 || strings.HasPrefix(s.term, "linux") {
		// Linux terminal does not support hard tabs.
		s.caps.Reset(capHT)
	} else {
		s.caps.Set(capHT)
		s.tabs = DefaultTabStops(width)
	}
}

// SetFullscreen sets whether whole screen is being used. This is usually
// paired with the alternate screen mode, and used to control cursor movements
// and optimizations when the terminal is occupying the whole screen.
func (s *TerminalRenderer) SetFullscreen(v bool) {
	if v {
		s.flags.Set(tFullscreen)
	} else {
		s.flags.Reset(tFullscreen)
	}
}

// Fullscreen returns whether whole screen is being used. This is usually
// paired with the alternate screen mode, and used to control cursor movements
// and optimizations when the terminal is occupying the whole screen.
func (s *TerminalRenderer) Fullscreen() bool {
	return s.flags.Contains(tFullscreen)
}

// SetRelativeCursor sets whether to use relative cursor movements.
func (s *TerminalRenderer) SetRelativeCursor(v bool) {
	if v {
		s.flags.Set(tRelativeCursor)
	} else {
		s.flags.Reset(tRelativeCursor)
	}
}

// SaveCursor saves the current cursor position and styles. This can be used
// when a program enters the alternate screen buffer mode using DECSET 1049
// [ansi.ModeAltScreenSaveCursor] and wants to restore the cursor position and
// styles later.
func (s *TerminalRenderer) SaveCursor() {
	s.saved = s.cur
}

// RestoreCursor restores the saved cursor position and styles. This can be
// used when a program exits the alternate screen buffer mode using DECRST 1049
// [ansi.ModeAltScreenSaveCursor] and wants to restore the cursor position and
// styles saved earlier.
func (s *TerminalRenderer) RestoreCursor() {
	s.cur = s.saved
}

// EnterAltScreen is a helper that queues the [ansi.ModeAltScreenSaveCursor]
// escape sequence to enter the alternate screen buffer and save cursor mode.
//
// It saves the current cursor properties, enables
// [TerminalRenderer.SetFullscreen] flag, and disables
// [TerminalRenderer.SetRelativeCursor] flag. On the next call to
// [TerminalRenderer.Render], it will move the cursor home and clear the screen
// to prepare for rendering the new buffer.
//
// Note: you might want to reapply the cursor visibility state after calling
// this method, as some terminals reset the cursor visibility when switching to
// the alternate screen.
func (s *TerminalRenderer) EnterAltScreen() {
	s.SaveCursor()
	s.buf.WriteString(ansi.SetModeAltScreenSaveCursor)
	s.SetFullscreen(true)
	s.SetRelativeCursor(false)
	s.Erase()
}

// ExitAltScreen is a helper that queues the [ansi.ModeAltScreenSaveCursor]
// escape sequence to exit the alternate screen buffer and restore cursor mode.
//
// It restores the saved cursor properties, disables
// [TerminalRenderer.SetFullscreen] flag, and enables
// [TerminalRenderer.SetRelativeCursor] flag. On the next call to
// [TerminalRenderer.Render], it will move the cursor to the first line and
// clear everything below the cursor to prepare for rendering the new buffer.
//
// Note: you might want to reapply the cursor visibility state after calling
// this method, as some terminals reset the cursor visibility when switching to
// the alternate screen.
func (s *TerminalRenderer) ExitAltScreen() {
	s.Erase()
	s.SetRelativeCursor(true)
	s.SetFullscreen(false)
	s.buf.WriteString(ansi.ResetModeAltScreenSaveCursor)
	s.RestoreCursor()
}

// PrependString adds the lines of the given string to the top of the terminal
// screen. The lines prepended are not managed by the renderer and will not be
// cleared or updated by the renderer.
//
// Using this when the terminal is using the alternate screen or when occupying
// the whole screen may not produce any visible effects. This is because
// once the terminal writes the prepended lines, they will get overwritten
// by the next frame.
func (s *TerminalRenderer) PrependString(newbuf *Buffer, str string) {
	if len(str) == 0 {
		return
	}

	// TODO: Use scrolling region if available.
	// TODO: Use [Screen.Write] [io.Writer] interface.

	// We need to scroll the screen up by the number of lines in the queue.
	// We can't use [ansi.SU] because we want the cursor to move down until
	// it reaches the bottom of the screen.
	w, h := newbuf.Width(), newbuf.Height()
	s.move(newbuf, 0, h-1)
	lines := strings.Split(str, "\n")
	offset := 0
	for _, line := range lines {
		lineWidth := ansi.StringWidth(line)
		if w > 0 && lineWidth > w {
			offset += (lineWidth / w)
		}
		if lineWidth == 0 || lineWidth%w != 0 {
			offset++
		}
	}

	s.buf.WriteString(strings.Repeat("\n", offset))
	s.cur.Y += offset
	// XXX: Now go to the top of the screen, insert new lines, and write
	// the queued strings. It is important to use [Screen.moveCursor]
	// instead of [Screen.move] because we don't want to perform any checks
	// on the cursor position.
	s.moveCursor(newbuf, 0, 0, false)
	s.buf.WriteString(ansi.InsertLine(offset))
	for _, line := range lines {
		s.buf.WriteString(line)
		s.buf.WriteString("\r\n")
	}
}

// moveCursor moves the cursor to the specified position.
//
// It is safe to call this function with a nil [Buffer], in that case, it won't
// be using any optimizations that depend on the buffer.
func (s *TerminalRenderer) moveCursor(newbuf *Buffer, x, y int, overwrite bool) {
	if !s.flags.Contains(tFullscreen) && s.flags.Contains(tRelativeCursor) &&
		s.cur.X == -1 && s.cur.Y == -1 {
		// First cursor movement in inline mode, move the cursor to the first
		// column before moving to the target position.
		_ = s.buf.WriteByte('\r')
		s.cur.X, s.cur.Y = 0, 0
	}
	seq, scrollHeight := moveCursor(s, newbuf, x, y, overwrite)
	// If we scrolled the screen, we need to update the scroll height.
	s.scrollHeight = max(s.scrollHeight, scrollHeight)
	_, _ = s.buf.WriteString(seq)
	s.cur.X, s.cur.Y = x, y
}

// move moves the cursor to the specified position in the buffer.
//
// It is safe to call this function with a nil [Buffer], in that case, it won't
// be using any optimizations that depend on the buffer.
func (s *TerminalRenderer) move(newbuf *Buffer, x, y int) {
	// XXX: Make sure we use the max height and width of the buffer in case
	// we're in the middle of a resize operation.
	var width, height int
	if s.curbuf != nil {
		width, height = s.curbuf.Width(), s.curbuf.Height()
	}
	if newbuf != nil {
		width = max(newbuf.Width(), width)
		height = max(newbuf.Height(), height)
	}

	if width > 0 && x >= width {
		// Handle autowrap
		y += (x / width)
		x %= width
	}

	// XXX: Disable styles if there's any
	// Some move operations such as [ansi.LF] can apply styles to the new
	// cursor position, thus, we need to reset the styles before moving the
	// cursor.
	blank := s.clearBlank()
	resetPen := y != s.cur.Y && !blank.Equal(&EmptyCell)
	if resetPen {
		s.updatePen(nil)
	}

	// Reset wrap around (phantom cursor) state
	if s.atPhantom {
		s.cur.X = 0
		_ = s.buf.WriteByte('\r')
		s.atPhantom = false // reset phantom cell state
	}

	// TODO: Investigate if we need to handle this case and/or if we need the
	// following code.
	//
	// if width > 0 && s.cur.X >= width {
	// 	l := (s.cur.X + 1) / width
	//
	// 	s.cur.Y += l
	// 	if height > 0 && s.cur.Y >= height {
	// 		l -= s.cur.Y - height - 1
	// 	}
	//
	// 	if l > 0 {
	// 		s.cur.X = 0
	// 		s.buf.WriteString("\r" + strings.Repeat("\n", l)) //nolint:errcheck
	// 	}
	// }

	if height > 0 {
		if s.cur.Y > height-1 {
			s.cur.Y = height - 1
		}
		if y > height-1 {
			y = height - 1
		}
	}

	if x == s.cur.X && y == s.cur.Y {
		// We give up later because we need to run checks for the phantom cell
		// and others before we can determine if we can give up.
		return
	}

	// We set the new cursor in tscreen.moveCursor].
	s.moveCursor(newbuf, x, y, true) // Overwrite cells if possible
}

// cellEqual returns whether the two cells are equal. A nil cell is considered
// a [EmptyCell].
func cellEqual(a, b *Cell) bool {
	if a == b {
		return true
	}
	if a == nil {
		a = &EmptyCell
	}
	if b == nil {
		b = &EmptyCell
	}
	return a.Equal(b)
}

// putCell draws a cell at the current cursor position.
func (s *TerminalRenderer) putCell(newbuf *Buffer, cell *Cell) {
	width, height := newbuf.Width(), newbuf.Height()
	if s.flags.Contains(tFullscreen) && s.cur.X == width-1 && s.cur.Y == height-1 {
		s.putCellLR(newbuf, cell)
	} else {
		s.putAttrCell(newbuf, cell)
	}
}

// wrapCursor wraps the cursor to the next line.
func (s *TerminalRenderer) wrapCursor() {
	const autoRightMargin = true
	if autoRightMargin {
		// Assume we have auto wrap mode enabled.
		s.cur.X = 0
		s.cur.Y++
	} else {
		s.cur.X--
	}
}

func (s *TerminalRenderer) putAttrCell(newbuf *Buffer, cell *Cell) {
	if cell != nil && cell.IsZero() {
		// XXX: Zero width cells are special and should not be written to the
		// screen no matter what other attributes they have.
		// Zero width cells are used for wide characters that are split into
		// multiple cells.
		return
	}

	// We're at pending wrap state (phantom cell), incoming cell should
	// wrap.
	if s.atPhantom {
		s.wrapCursor()
		s.atPhantom = false
	}

	s.updatePen(cell)
	cellWidth := 1
	if cell == nil {
		_ = s.buf.WriteByte(' ')
	} else {
		_, _ = s.buf.WriteString(cell.Content)
		cellWidth = cell.Width
	}

	s.cur.X += cellWidth
	if s.cur.X >= newbuf.Width() {
		s.atPhantom = true
	}
}

// putCellLR draws a cell at the lower right corner of the screen.
func (s *TerminalRenderer) putCellLR(newbuf *Buffer, cell *Cell) {
	// Optimize for the lower right corner cell.
	curX := s.cur.X
	if cell == nil || !cell.IsZero() {
		_, _ = s.buf.WriteString(ansi.ResetModeAutoWrap)
		s.putAttrCell(newbuf, cell)
		// Writing to lower-right corner cell should not wrap.
		s.atPhantom = false
		s.cur.X = curX
		_, _ = s.buf.WriteString(ansi.SetModeAutoWrap)
	}
}

// updatePen updates the cursor pen styles.
func (s *TerminalRenderer) updatePen(cell *Cell) {
	if cell == nil {
		if !s.cur.Style.IsZero() {
			_, _ = s.buf.WriteString(ansi.ResetStyle)
			s.cur.Style = Style{} // Reset style
		}
		if !s.cur.Link.IsZero() {
			_, _ = s.buf.WriteString(ansi.ResetHyperlink())
		}
		return
	}

	// Downsample pen when we don't have a [colorprofile.TrueColor],
	// otherwise, use the original style.
	newStyle := ConvertStyle(cell.Style, s.profile)
	newLink := ConvertLink(cell.Link, s.profile)
	oldStyle := ConvertStyle(s.cur.Style, s.profile)
	oldLink := ConvertLink(s.cur.Link, s.profile)

	if !newStyle.Equal(&oldStyle) {
		seq := newStyle.Diff(&oldStyle)
		if newStyle.IsZero() && len(seq) > len(ansi.ResetStyle) {
			seq = ansi.ResetStyle
		}
		_, _ = s.buf.WriteString(seq)
		s.cur.Style = cell.Style // Copy the original style
	}
	if !newLink.Equal(&oldLink) {
		_, _ = s.buf.WriteString(ansi.SetHyperlink(newLink.URL, newLink.Params))
		s.cur.Link = cell.Link // Copy the original link
	}
}

// canClearWith checks whether the given cell can be used by clearing commands
// like [ansi.EL] to clear the screen. It tests if a cell is empty i.e. space
// or blank and doesn't include any bad style attributes such as [AttrReverse].
func canClearWith(c *Cell) bool {
	if c == nil {
		return true
	}
	if c.Width != 1 || len(c.Content) != 1 || c.Content != " " {
		return false
	}
	// NOTE: This assumes that the terminal supports bce terminfo capability
	// which all xterm-compatible terminals and terminals that use xterm*
	// terminal types do.
	return c.Style.Underline == UnderlineNone &&
		c.Style.Attrs&^(AttrBold|AttrFaint|AttrItalic|AttrBlink|AttrRapidBlink) == 0 &&
		c.Link.IsZero()
}

// emitRange emits a range of cells to the buffer. It it equivalent to calling
// tscreen.putCell] for each cell in the range. This is optimized to use
// [ansi.ECH] and [ansi.REP].
// Returns whether the cursor is at the end of interval or somewhere in the
// middle.
func (s *TerminalRenderer) emitRange(newbuf *Buffer, line Line, n int) (eoi bool) {
	hasECH := s.caps.Contains(capECH)
	hasREP := s.caps.Contains(capREP)
	if hasECH || hasREP { //nolint:nestif
		for n > 0 {
			var count int
			for n > 1 && !cellEqual(line.At(0), line.At(1)) {
				s.putCell(newbuf, line.At(0))
				line = line[1:]
				n--
			}

			cell0 := line[0]
			if n == 1 {
				s.putCell(newbuf, &cell0)
				return false
			}

			count = 2
			for count < n && cellEqual(line.At(count), &cell0) {
				count++
			}

			ech := ansi.EraseCharacter(count)
			cup := ansi.CursorPosition(s.cur.X+count, s.cur.Y)
			rep := ansi.RepeatPreviousCharacter(count)
			if hasECH && count > len(ech)+len(cup) && canClearWith(&cell0) {
				s.updatePen(&cell0)
				_, _ = s.buf.WriteString(ech)

				// If this is the last cell, we don't need to move the cursor.
				if count < n {
					s.move(newbuf, s.cur.X+count, s.cur.Y)
				} else {
					return true // cursor in the middle
				}
			} else if hasREP && count > len(rep) &&
				(len(cell0.Content) == 1 && cell0.Content[0] >= ansi.US && cell0.Content[0] < ansi.DEL) {
				// We only support ASCII characters. Most terminals will handle
				// non-ASCII characters correctly, but some might not, ahem xterm.
				//
				// NOTE: [ansi.REP] only repeats the last rune and won't work
				// if the last cell contains multiple runes.

				wrapPossible := s.cur.X+count >= newbuf.Width()
				repCount := count
				if wrapPossible {
					repCount--
				}

				s.updatePen(&cell0)
				s.putCell(newbuf, &cell0)
				repCount-- // cell0 is a single width cell ASCII character

				_, _ = s.buf.WriteString(ansi.RepeatPreviousCharacter(repCount))
				s.cur.X += repCount
				if wrapPossible {
					s.putCell(newbuf, &cell0)
				}
			} else {
				for i := 0; i < count; i++ {
					s.putCell(newbuf, line.At(i))
				}
			}

			line = line[clamp(count, 0, len(line)):]
			n -= count
		}

		return false
	}

	for i := 0; i < n; i++ {
		s.putCell(newbuf, line.At(i))
	}

	return false
}

// putRange puts a range of cells from the old line to the new line.
// Returns whether the cursor is at the end of interval or somewhere in the
// middle.
func (s *TerminalRenderer) putRange(newbuf *Buffer, oldLine, newLine Line, y, start, end int) (eoi bool) {
	inline := min(len(ansi.CursorPosition(start+1, y+1)),
		min(len(ansi.HorizontalPositionAbsolute(start+1)),
			len(ansi.CursorForward(start+1))))
	if (end - start + 1) > inline { //nolint:nestif
		var j, same int
		for j, same = start, 0; j <= end; j++ {
			oldCell, newCell := oldLine.At(j), newLine.At(j)
			if same == 0 && oldCell != nil && oldCell.IsZero() {
				continue
			}
			if cellEqual(oldCell, newCell) {
				same++
			} else {
				if same > end-start {
					s.emitRange(newbuf, newLine[start:], j-same-start)
					s.move(newbuf, j, y)
					start = j
				}
				same = 0
			}
		}

		i := s.emitRange(newbuf, newLine[start:], j-same-start)

		// Always return 1 for the next [tScreen.move] after a
		// [tScreen.putRange] if we found identical characters at end of
		// interval.
		if same == 0 {
			return i
		}
		return true
	}

	return s.emitRange(newbuf, newLine[start:], end-start+1)
}

// clearToEnd clears the screen from the current cursor position to the end of
// line.
func (s *TerminalRenderer) clearToEnd(newbuf *Buffer, blank *Cell, force bool) {
	if s.cur.Y >= 0 {
		curline := s.curbuf.Line(s.cur.Y)
		// We use the newbuf width because the current buffer might be smaller
		// than the new buffer during a resize operation and we want to detect
		// that.
		for j := s.cur.X; j < newbuf.Width(); j++ {
			if j >= 0 {
				c := curline.At(j)
				if !cellEqual(c, blank) {
					curline.Set(j, blank)
					force = true
				}
			}
		}
	}

	if force {
		s.updatePen(blank)
		count := newbuf.Width() - s.cur.X
		if s.el0Cost() <= count {
			_, _ = s.buf.WriteString(ansi.EraseLineRight)
		} else {
			for i := 0; i < count; i++ {
				s.putCell(newbuf, blank)
			}
		}
	}
}

// clearBlank returns a blank cell based on the current cursor background color.
func (s *TerminalRenderer) clearBlank() *Cell {
	return &s.cur.Cell
}

// insertCells inserts the count cells pointed by the given line at the current
// cursor position.
func (s *TerminalRenderer) insertCells(newbuf *Buffer, line Line, count int) {
	supportsICH := s.caps.Contains(capICH)
	if supportsICH {
		// Use [ansi.ICH] as an optimization.
		_, _ = s.buf.WriteString(ansi.InsertCharacter(count))
	} else {
		// Otherwise, use [ansi.IRM] mode.
		_, _ = s.buf.WriteString(ansi.SetModeInsertReplace)
	}

	for i := 0; count > 0; i++ {
		s.putAttrCell(newbuf, line.At(i))
		count--
	}

	if !supportsICH {
		_, _ = s.buf.WriteString(ansi.ResetModeInsertReplace)
	}
}

// el0Cost returns the cost of using [ansi.EL] 0 i.e. [ansi.EraseLineRight]. If
// this terminal supports background color erase, it can be cheaper to use
// [ansi.EL] 0 i.e. [ansi.EraseLineRight] to clear
// trailing spaces.
func (s *TerminalRenderer) el0Cost() int {
	if s.caps != noCaps {
		return 0
	}
	return len(ansi.EraseLineRight)
}

// transformLine transforms the given line in the current window to the
// corresponding line in the new window. It uses [ansi.ICH] and [ansi.DCH] to
// insert or delete characters.
func (s *TerminalRenderer) transformLine(newbuf *Buffer, y int) {
	var firstCell, oLastCell, nLastCell int // first, old last, new last index
	oldLine := s.curbuf.Line(y)
	newLine := newbuf.Line(y)

	// Find the first changed cell in the line
	blank := newLine.At(0)

	// It might be cheaper to clear leading spaces with [ansi.EL] 1 i.e.
	// [ansi.EraseLineLeft].
	if canClearWith(blank) { //nolint:nestif
		var oFirstCell, nFirstCell int
		for oFirstCell = 0; oFirstCell < s.curbuf.Width(); oFirstCell++ {
			if !cellEqual(oldLine.At(oFirstCell), blank) {
				break
			}
		}
		for nFirstCell = 0; nFirstCell < newbuf.Width(); nFirstCell++ {
			if !cellEqual(newLine.At(nFirstCell), blank) {
				break
			}
		}

		if nFirstCell == oFirstCell {
			firstCell = nFirstCell

			// Find the first differing cell
			for firstCell < newbuf.Width() &&
				cellEqual(oldLine.At(firstCell), newLine.At(firstCell)) {
				firstCell++
			}
		} else if oFirstCell > nFirstCell {
			firstCell = nFirstCell
		} else if oFirstCell < nFirstCell {
			firstCell = oFirstCell
			el1Cost := len(ansi.EraseLineLeft)
			if el1Cost < nFirstCell-oFirstCell {
				if nFirstCell >= newbuf.Width() {
					s.move(newbuf, 0, y)
					s.updatePen(blank)
					_, _ = s.buf.WriteString(ansi.EraseLineRight)
				} else {
					s.move(newbuf, nFirstCell-1, y)
					s.updatePen(blank)
					_, _ = s.buf.WriteString(ansi.EraseLineLeft)
				}

				for firstCell < nFirstCell {
					oldLine.Set(firstCell, blank)
					firstCell++
				}
			}
		}
	} else {
		// Find the first differing cell
		for firstCell < newbuf.Width() && cellEqual(newLine.At(firstCell), oldLine.At(firstCell)) {
			firstCell++
		}
	}

	// If we didn't find one, we're done
	if firstCell >= newbuf.Width() {
		return
	}

	blank = newLine.At(newbuf.Width() - 1)
	if blank != nil && !canClearWith(blank) {
		// Find the last differing cell
		nLastCell = newbuf.Width() - 1
		for nLastCell > firstCell && cellEqual(newLine.At(nLastCell), oldLine.At(nLastCell)) {
			nLastCell--
		}

		if nLastCell >= firstCell {
			s.move(newbuf, firstCell, y)
			s.putRange(newbuf, oldLine, newLine, y, firstCell, nLastCell)
			if firstCell < len(oldLine) && firstCell < len(newLine) {
				copy(oldLine[firstCell:], newLine[firstCell:])
			} else {
				copy(oldLine, newLine)
			}
		}

		return
	}

	// Find last non-blank cell in the old line.
	// We always use the newbuf width to detect new cell changes.
	oLastCell = newbuf.Width() - 1
	for oLastCell > firstCell && cellEqual(oldLine.At(oLastCell), blank) {
		oLastCell--
	}

	// Find last non-blank cell in the new line.
	// We always use the newbuf width to detect new cell changes.
	nLastCell = newbuf.Width() - 1
	for nLastCell > firstCell && cellEqual(newLine.At(nLastCell), blank) {
		nLastCell--
	}

	if nLastCell == firstCell && s.el0Cost() < oLastCell-nLastCell { //nolint:nestif
		s.move(newbuf, firstCell, y)
		if !cellEqual(newLine.At(firstCell), blank) {
			s.putCell(newbuf, newLine.At(firstCell))
		}
		s.clearToEnd(newbuf, blank, false)
	} else if nLastCell != oLastCell &&
		!cellEqual(newLine.At(nLastCell), oldLine.At(oLastCell)) {
		s.move(newbuf, firstCell, y)
		if oLastCell-nLastCell > s.el0Cost() {
			if s.putRange(newbuf, oldLine, newLine, y, firstCell, nLastCell) {
				s.move(newbuf, nLastCell+1, y)
			}
			s.clearToEnd(newbuf, blank, false)
		} else {
			n := max(nLastCell, oLastCell)
			s.putRange(newbuf, oldLine, newLine, y, firstCell, n)
		}
	} else {
		nLastNonBlank := nLastCell
		oLastNonBlank := oLastCell

		// Find the last cells that really differ.
		// Can be -1 if no cells differ.
		for cellEqual(newLine.At(nLastCell), oldLine.At(oLastCell)) {
			if !cellEqual(newLine.At(nLastCell-1), oldLine.At(oLastCell-1)) {
				break
			}
			nLastCell--
			oLastCell--
			if nLastCell == -1 || oLastCell == -1 {
				break
			}
		}

		n := min(oLastCell, nLastCell)
		if n >= firstCell {
			s.move(newbuf, firstCell, y)
			s.putRange(newbuf, oldLine, newLine, y, firstCell, n)
		}

		if oLastCell < nLastCell {
			m := max(nLastNonBlank, oLastNonBlank)
			if n != 0 {
				for n > 0 {
					wide := newLine.At(n + 1)
					if wide == nil || !wide.IsZero() {
						break
					}
					n--
					oLastCell--
				}
			} else if n >= firstCell && newLine.At(n) != nil && newLine.At(n).Width > 1 {
				next := newLine.At(n + 1)
				for next != nil && next.IsZero() {
					n++
					oLastCell++
				}
			}

			if oLastCell >= 0 && nLastCell >= 0 {
				// Only move the cursor when we actually have changes to make.
				// This prevents unnecessary cursor movements when we're adding
				// new columns with blank cells.
				s.move(newbuf, n+1, y)
			}
			ichCost := 3 + nLastCell - oLastCell
			if s.caps.Contains(capICH) && (nLastCell < nLastNonBlank || ichCost > (m-n)) {
				s.putRange(newbuf, oldLine, newLine, y, n+1, m)
			} else {
				s.insertCells(newbuf, newLine[n+1:], nLastCell-oLastCell)
			}
		} else if oLastCell > nLastCell {
			s.move(newbuf, n+1, y)
			dchCost := 3 + oLastCell - nLastCell
			if dchCost > len(ansi.EraseLineRight)+nLastNonBlank-(n+1) {
				if s.putRange(newbuf, oldLine, newLine, y, n+1, nLastNonBlank) {
					s.move(newbuf, nLastNonBlank+1, y)
				}
				s.clearToEnd(newbuf, blank, false)
			} else {
				s.updatePen(blank)
				s.deleteCells(oLastCell - nLastCell)
			}
		}
	}

	// Update the old line with the new line
	if firstCell < len(oldLine) && firstCell < len(newLine) {
		copy(oldLine[firstCell:], newLine[firstCell:])
	} else {
		copy(oldLine, newLine)
	}
}

// deleteCells deletes the count cells at the current cursor position and moves
// the rest of the line to the left. This is equivalent to [ansi.DCH].
func (s *TerminalRenderer) deleteCells(count int) {
	// [ansi.DCH] will shift in cells from the right margin so we need to
	// ensure that they are the right style.
	_, _ = s.buf.WriteString(ansi.DeleteCharacter(count))
}

// clearToBottom clears the screen from the current cursor position to the end
// of the screen.
func (s *TerminalRenderer) clearToBottom(blank *Cell) {
	row, col := s.cur.Y, s.cur.X
	if row < 0 {
		row = 0
	}

	s.updatePen(blank)
	_, _ = s.buf.WriteString(ansi.EraseScreenBelow)
	// Clear the rest of the current line
	s.curbuf.ClearArea(Rect(col, row, s.curbuf.Width()-col, 1))
	// Clear everything below the current line
	s.curbuf.ClearArea(Rect(0, row+1, s.curbuf.Width(), s.curbuf.Height()-row-1))
}

// clearBottom tests if clearing the end of the screen would satisfy part of
// the screen update. Scan backwards through lines in the screen checking if
// each is blank and one or more are changed.
// It returns the top line.
func (s *TerminalRenderer) clearBottom(newbuf *Buffer, total int) (top int) {
	if total <= 0 {
		return 0
	}

	top = total
	last := min(s.curbuf.Width(), newbuf.Width())
	blank := s.clearBlank()
	canClearWithBlank := canClearWith(blank)

	if canClearWithBlank { //nolint:nestif
		var row int
		for row = total - 1; row >= 0; row-- {
			oldLine := s.curbuf.Line(row)
			newLine := newbuf.Line(row)

			var col int
			ok := true
			for col = 0; ok && col < last; col++ {
				ok = cellEqual(newLine.At(col), blank)
			}
			if !ok {
				break
			}

			for col = 0; ok && col < last; col++ {
				ok = cellEqual(oldLine.At(col), blank)
			}
			if !ok {
				top = row
			}
		}

		if top < total {
			s.move(newbuf, 0, max(0, top-1)) // top is 1-based
			s.clearToBottom(blank)
			if s.oldhash != nil && s.newhash != nil &&
				row < len(s.oldhash) && row < len(s.newhash) {
				for row := top; row < newbuf.Height(); row++ {
					s.oldhash[row] = s.newhash[row]
				}
			}
		}
	}

	return top
}

// clearScreen clears the screen and put cursor at home.
func (s *TerminalRenderer) clearScreen(blank *Cell) {
	s.updatePen(blank)
	_, _ = s.buf.WriteString(ansi.CursorHomePosition)
	_, _ = s.buf.WriteString(ansi.EraseEntireScreen)
	s.cur.X, s.cur.Y = 0, 0
	s.curbuf.Fill(blank)
}

// clearBelow clears everything below and including the row.
func (s *TerminalRenderer) clearBelow(newbuf *Buffer, blank *Cell, row int) {
	s.move(newbuf, 0, row)
	s.clearToBottom(blank)
}

// clearUpdate forces a screen redraw.
func (s *TerminalRenderer) clearUpdate(newbuf *Buffer) {
	blank := s.clearBlank()
	var nonEmpty int
	if s.flags.Contains(tFullscreen) {
		// XXX: We're using the maximum height of the two buffers to ensure we
		// write newly added lines to the screen in
		// [terminalWriter.transformLine].
		nonEmpty = max(s.curbuf.Height(), newbuf.Height())
		s.clearScreen(blank)
	} else {
		nonEmpty = newbuf.Height()
		// FIXME: Investigate the double [ansi.ClearScreenBelow] call.
		// Commenting the line below out seems to work but it might cause other
		// bugs.
		s.clearBelow(newbuf, blank, 0)
	}
	nonEmpty = s.clearBottom(newbuf, nonEmpty)
	for i := 0; i < nonEmpty && i < newbuf.Height(); i++ {
		s.transformLine(newbuf, i)
	}
}

func (s *TerminalRenderer) logf(format string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Printf(format, args...)
}

// Buffered returns the number of bytes buffered for the next flush.
func (s *TerminalRenderer) Buffered() int {
	return s.buf.Len()
}

// Flush flushes the buffer to the screen.
func (s *TerminalRenderer) Flush() (err error) {
	// Write the buffer
	if n := s.buf.Len(); n > 0 {
		bts := s.buf.Bytes()
		if s.logger != nil {
			s.logf("output: %q", bts)
		}
		_, err = s.w.Write(bts)
		s.buf.Reset()
	}
	return
}

// Touched returns the number of lines that have been touched or changed.
func (s *TerminalRenderer) Touched(buf *Buffer) (n int) {
	if buf.Touched == nil {
		return buf.Height()
	}
	for _, ch := range buf.Touched {
		if ch != nil {
			n++
		}
	}
	return
}

// Redraw forces a full redraw of the screen. It's equivalent to calling
// [TerminalRenderer.Erase] and [TerminalRenderer.Render].
func (s *TerminalRenderer) Redraw(newbuf *Buffer) {
	s.clear = true
	s.Render(newbuf)
}

// Render renders changes of the screen to the internal buffer. Call
// [terminalWriter.Flush] to flush pending changes to the screen.
func (s *TerminalRenderer) Render(newbuf *Buffer) {
	// Do we need to render anything?
	touchedLines := s.Touched(newbuf)
	if !s.clear && touchedLines == 0 {
		return
	}

	if s.curbuf == nil || s.curbuf.Bounds().Empty() {
		// Initialize the current buffer
		s.curbuf = NewBuffer(newbuf.Width(), newbuf.Height())
	}

	newWidth, newHeight := newbuf.Width(), newbuf.Height()
	curWidth, curHeight := s.curbuf.Width(), s.curbuf.Height()

	if curWidth != newWidth || curHeight != newHeight {
		s.oldhash, s.newhash = nil, nil
	}

	// TODO: Investigate whether this is necessary. Theoretically, terminals
	// can add/remove tab stops and we should be able to handle that. We could
	// use [ansi.DECTABSR] to read the tab stops, but that's not implemented in
	// most terminals :/
	// // Are we using hard tabs? If so, ensure tabs are using the
	// // default interval using [ansi.DECST8C].
	// if s.opts.HardTabs && !s.initTabs {
	// 	s.buf.WriteString(ansi.SetTabEvery8Columns)
	// 	s.initTabs = true
	// }

	var nonEmpty int

	// XXX: In inline mode, after a screen resize, we need to clear the extra
	// lines at the bottom of the screen. This is because in inline mode, we
	// don't use the full screen height and the current buffer size might be
	// larger than the new buffer size.
	partialClear := !s.flags.Contains(tFullscreen) && s.cur.X != -1 && s.cur.Y != -1 &&
		curWidth == newWidth &&
		curHeight > 0 &&
		curHeight > newHeight

	if !s.clear && partialClear {
		s.clearBelow(newbuf, nil, newHeight-1)
	}

	if s.clear { //nolint:nestif
		s.clearUpdate(newbuf)
		s.clear = false
	} else if touchedLines > 0 {
		// On Windows, there's a bug with Windows Terminal where [ansi.DECSTBM]
		// misbehaves and moves the cursor outside of the scrolling region. For
		// now, we disable the optimizations completely on Windows.
		// See https://github.com/microsoft/terminal/issues/19016
		if s.flags.Contains(tFullscreen) && !isWindows {
			// Optimize scrolling for the alternate screen buffer.
			// TODO: Should we optimize for inline mode as well? If so, we need
			// to know the actual cursor position to use [ansi.DECSTBM].
			s.scrollOptimize(newbuf)
		}

		var changedLines int
		var i int

		if s.flags.Contains(tFullscreen) {
			nonEmpty = min(curHeight, newHeight)
		} else {
			nonEmpty = newHeight
		}

		nonEmpty = s.clearBottom(newbuf, nonEmpty)
		for i = 0; i < nonEmpty && i < newHeight; i++ {
			if newbuf.Touched == nil || i >= len(newbuf.Touched) || (newbuf.Touched[i] != nil &&
				(newbuf.Touched[i].FirstCell != -1 || newbuf.Touched[i].LastCell != -1)) {
				s.transformLine(newbuf, i)
				changedLines++
			}

			// Mark line changed successfully.
			if i < len(newbuf.Touched) && i <= newbuf.Height()-1 {
				newbuf.Touched[i] = &LineData{
					FirstCell: -1, LastCell: -1,
				}
			}
			if i < len(s.curbuf.Touched) && i < s.curbuf.Height()-1 {
				s.curbuf.Touched[i] = &LineData{
					FirstCell: -1, LastCell: -1,
				}
			}
		}
	}

	if !s.flags.Contains(tFullscreen) && s.scrollHeight < newHeight-1 {
		s.move(newbuf, 0, newHeight-1)
	}

	// Sync windows and screen
	newbuf.Touched = make([]*LineData, newHeight)
	for i := range newbuf.Touched {
		newbuf.Touched[i] = &LineData{
			FirstCell: -1, LastCell: -1,
		}
	}
	for i := range s.curbuf.Touched {
		s.curbuf.Touched[i] = &LineData{
			FirstCell: -1, LastCell: -1,
		}
	}

	if curWidth != newWidth || curHeight != newHeight {
		// Resize the old buffer to match the new buffer.
		s.curbuf.Resize(newWidth, newHeight)
		// Sync new lines to old lines
		for i := curHeight - 1; i < newHeight; i++ {
			copy(s.curbuf.Line(i), newbuf.Line(i))
		}
	}

	s.updatePen(nil) // nil indicates a blank cell with no styles
}

// Erase marks the screen to be fully erased on the next render.
func (s *TerminalRenderer) Erase() {
	s.clear = true
}

// Resize updates the terminal screen tab stops. This is used to calculate
// terminal tab stops for hard tab optimizations.
func (s *TerminalRenderer) Resize(width, _ int) {
	if s.tabs != nil {
		s.tabs.Resize(width)
	}
	s.scrollHeight = 0
}

// Position returns the cursor position in the screen buffer after applying any
// pending transformations from the underlying buffer.
func (s *TerminalRenderer) Position() (x, y int) {
	return s.cur.X, s.cur.Y
}

// SetPosition changes the logical cursor position. This can be used when we
// change the cursor position outside of the screen and need to update the
// screen cursor position.
// This changes the cursor position for both normal and alternate screen
// buffers.
func (s *TerminalRenderer) SetPosition(x, y int) {
	s.cur.X, s.cur.Y = x, y
}

// WriteString writes the given string to the underlying buffer.
func (s *TerminalRenderer) WriteString(str string) (int, error) {
	return s.buf.WriteString(str) //nolint:wrapcheck
}

// Write writes the given bytes to the underlying buffer.
func (s *TerminalRenderer) Write(b []byte) (int, error) {
	return s.buf.Write(b) //nolint:wrapcheck
}

// MoveTo calculates and writes the shortest sequence to move the cursor to the
// given position. It uses the current cursor position and the new position to
// calculate the shortest amount of sequences to move the cursor.
func (s *TerminalRenderer) MoveTo(x, y int) {
	s.move(nil, x, y)
}

// notLocal returns whether the coordinates are not considered local movement
// using the defined thresholds.
// This takes the number of columns, and the coordinates of the current and
// target positions.
func notLocal(cols, fx, fy, tx, ty int) bool {
	// The typical distance for a [ansi.CUP] sequence. Anything less than this
	// is considered local movement.
	const longDist = 8 - 1
	return (tx > longDist) &&
		(tx < cols-1-longDist) &&
		(abs(ty-fy)+abs(tx-fx) > longDist)
}

// relativeCursorMove returns the relative cursor movement sequence using one or two
// of the following sequences [ansi.CUU], [ansi.CUD], [ansi.CUF], [ansi.CUB],
// [ansi.VPA], [ansi.HPA].
// When overwrite is true, this will try to optimize the sequence by using the
// screen cells values to move the cursor instead of using escape sequences.
//
// It is safe to call this function with a nil [Buffer]. In that case, it won't
// use any optimizations that require the new buffer such as overwrite.
func relativeCursorMove(s *TerminalRenderer, newbuf *Buffer, fx, fy, tx, ty int, overwrite, useTabs, useBackspace bool) (string, int) {
	var seq strings.Builder
	var scrollHeight int
	if newbuf == nil {
		overwrite = false // We can't overwrite the current buffer.
	}

	if ty != fy { //nolint:nestif
		var yseq string
		if s.caps.Contains(capVPA) && !s.flags.Contains(tRelativeCursor) {
			yseq = ansi.VerticalPositionAbsolute(ty + 1)
		}

		if ty > fy {
			n := ty - fy
			if cud := ansi.CursorDown(n); yseq == "" || len(cud) < len(yseq) {
				yseq = cud
			}
			shouldScroll := !s.flags.Contains(tFullscreen) && ty > s.scrollHeight
			if shouldScroll || n < len(yseq) { // n is the cost of using newline characters
				yseq = strings.Repeat("\n", n)
				scrollHeight = ty
				if s.flags.Contains(tMapNewline) {
					fx = 0
				}
			}
		} else if ty < fy {
			n := fy - ty
			if cuu := ansi.CursorUp(n); yseq == "" || len(cuu) < len(yseq) {
				yseq = cuu
			}
			if n == 1 && fy-1 > 0 {
				// TODO: Ensure we're not unintentionally scrolling the screen up.
				yseq = ansi.ReverseIndex
			}
		}

		seq.WriteString(yseq)
	}

	if tx != fx { //nolint:nestif
		var xseq string
		if !s.flags.Contains(tRelativeCursor) {
			if s.caps.Contains(capHPA) {
				xseq = ansi.HorizontalPositionAbsolute(tx + 1)
			} else if s.caps.Contains(capCHA) {
				xseq = ansi.CursorHorizontalAbsolute(tx + 1)
			}
		}

		if tx > fx {
			n := tx - fx
			if useTabs && s.tabs != nil {
				var tabs int
				var col int
				for col = fx; s.tabs.Next(col) <= tx; col = s.tabs.Next(col) {
					tabs++
					if col == s.tabs.Next(col) || col >= s.tabs.Width()-1 {
						break
					}
				}

				if tabs > 0 {
					cht := ansi.CursorHorizontalForwardTab(tabs)
					tab := strings.Repeat("\t", tabs)
					if false && s.caps.Contains(capCHT) && len(cht) < len(tab) {
						// TODO: The linux console and some terminals such as
						// Alacritty don't support [ansi.CHT]. Enable this when
						// we have a way to detect this, or after 5 years when
						// we're sure everyone has updated their terminals :P
						seq.WriteString(cht)
					} else {
						seq.WriteString(tab)
					}

					n = tx - col
					fx = col
				}
			}

			if cuf := ansi.CursorForward(n); xseq == "" || len(cuf) < len(xseq) {
				xseq = cuf
			}

			// If we have no attribute and style changes, overwrite is cheaper.
			var ovw string
			if overwrite && ty >= 0 {
				for i := 0; i < n; i++ {
					cell := newbuf.CellAt(fx+i, ty)
					if cell != nil && cell.Width > 0 {
						i += cell.Width - 1
						if !cell.Style.Equal(&s.cur.Style) || !cell.Link.Equal(&s.cur.Link) {
							overwrite = false
							break
						}
					}
				}
			}

			if overwrite && ty >= 0 {
				for i := 0; i < n; i++ {
					cell := newbuf.CellAt(fx+i, ty)
					if cell != nil && cell.Width > 0 {
						ovw += cell.String()
						i += cell.Width - 1
					} else {
						ovw += " "
					}
				}
			}

			if overwrite && len(ovw) < len(xseq) {
				xseq = ovw
			}
		} else if tx < fx {
			n := fx - tx
			if useTabs && s.tabs != nil && s.caps.Contains(capCBT) {
				// VT100 does not support backward tabs [ansi.CBT].

				col := fx

				var cbt int // cursor backward tabs count
				for s.tabs.Prev(col) >= tx {
					col = s.tabs.Prev(col)
					cbt++
					if col == s.tabs.Prev(col) || col <= 0 {
						break
					}
				}

				if cbt > 0 {
					seq.WriteString(ansi.CursorBackwardTab(cbt))
					n = col - tx
				}
			}

			if cub := ansi.CursorBackward(n); xseq == "" || len(cub) < len(xseq) {
				xseq = cub
			}

			if useBackspace && n < len(xseq) {
				xseq = strings.Repeat("\b", n)
			}
		}

		seq.WriteString(xseq)
	}

	return seq.String(), scrollHeight
}

// moveCursor moves and returns the cursor movement sequence to move the cursor
// to the specified position.
// When overwrite is true, this will try to optimize the sequence by using the
// screen cells values to move the cursor instead of using escape sequences.
//
// It is safe to call this function with a nil [Buffer]. In that case, it won't
// use any optimizations that require the new buffer such as overwrite.
func moveCursor(s *TerminalRenderer, newbuf *Buffer, x, y int, overwrite bool) (seq string, scrollHeight int) {
	fx, fy := s.cur.X, s.cur.Y

	if !s.flags.Contains(tRelativeCursor) {
		width := -1 // Use -1 to indicate that we don't know the width of the screen.
		if s.tabs != nil {
			width = s.tabs.Width()
		}
		if newbuf != nil && width == -1 {
			// Even though this might not be accurate, we can use the new
			// buffer width as a fallback. Technically, if the new buffer
			// didn't have the width of the terminal, this would give us a
			// wrong result from [notLocal].
			width = newbuf.Width()
		}
		// Method #0: Use [ansi.CUP] if the distance is long.
		seq = ansi.CursorPosition(x+1, y+1)
		if fx == -1 || fy == -1 || width == -1 || notLocal(width, fx, fy, x, y) {
			return seq, 0
		}
	}

	// Optimize based on options.
	trials := 0
	if s.caps.Contains(capHT) {
		trials |= 2 // 0b10 in binary
	}
	if s.caps.Contains(capBS) {
		trials |= 1 // 0b01 in binary
	}

	// Try all possible combinations of hard tabs and backspace optimizations.
	for i := 0; i <= trials; i++ {
		// Skip combinations that are not enabled.
		if i & ^trials != 0 {
			continue
		}

		useHardTabs := i&2 != 0
		useBackspace := i&1 != 0

		// Method #1: Use local movement sequences.
		nseq1, nscrollHeight1 := relativeCursorMove(s, newbuf, fx, fy, x, y, overwrite, useHardTabs, useBackspace)
		if (i == 0 && len(seq) == 0) || len(nseq1) < len(seq) {
			seq = nseq1
			scrollHeight = max(scrollHeight, nscrollHeight1)
		}

		// Method #2: Use [ansi.CR] and local movement sequences.
		nseq2, nscrollHeight2 := relativeCursorMove(s, newbuf, 0, fy, x, y, overwrite, useHardTabs, useBackspace)
		nseq2 = "\r" + nseq2
		if len(nseq2) < len(seq) {
			seq = nseq2
			scrollHeight = max(scrollHeight, nscrollHeight2)
		}

		if !s.flags.Contains(tRelativeCursor) {
			// Method #3: Use [ansi.CursorHomePosition] and local movement sequences.
			nseq3, nscrollHeight3 := relativeCursorMove(s, newbuf, 0, 0, x, y, overwrite, useHardTabs, useBackspace)
			nseq3 = ansi.CursorHomePosition + nseq3
			if len(nseq3) < len(seq) {
				seq = nseq3
				scrollHeight = max(scrollHeight, nscrollHeight3)
			}
		}
	}

	return seq, scrollHeight
}

// xtermCaps returns whether the terminal is xterm-like. This means that the
// terminal supports ECMA-48 and ANSI X3.64 escape sequences.
// xtermCaps returns a list of control sequence capabilities for the given
// terminal type. This only supports a subset of sequences that can
// be different among terminals.
// NOTE: A hybrid approach would be to support Terminfo databases for a full
// set of capabilities.
func xtermCaps(termtype string) (v capabilities) {
	parts := strings.Split(termtype, "-")
	if len(parts) == 0 {
		return v
	}

	switch parts[0] {
	case
		"contour",
		"foot",
		"ghostty",
		"kitty",
		"rio",
		"st",
		"tmux",
		"wezterm":
		v = allCaps
	case "xterm":
		switch {
		case len(parts) > 1 && parts[1] == "ghostty":
			fallthrough
		case len(parts) > 1 && parts[1] == "kitty":
			fallthrough
		case len(parts) > 1 && parts[1] == "rio":
			// These terminals can be defined as xterm- variants for
			// compatibility with applications that check for xterm.
			v = allCaps
		default:
			// NOTE: We exclude capHPA from allCaps because terminals like
			// Konsole don't support it. Xterm terminfo defines HPA as CHA
			// which means we can use CHA instead of HPA.
			v = allCaps
			v &^= capHPA
			v &^= capCHT
			v &^= capREP
		}
	case "alacritty":
		v = allCaps
		v &^= capCHT // NOTE: alacritty added support for [ansi.CHT] in 2024-12-28 #62d5b13.
	case "screen":
		// See https://www.gnu.org/software/screen/manual/screen.html#Control-Sequences-1
		v = allCaps
		v &^= capREP
	case "linux":
		// See https://man7.org/linux/man-pages/man4/console_codes.4.html
		v = capVPA | capCHA | capHPA | capECH | capICH
	}

	return v
}
