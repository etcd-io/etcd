package uv

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/muesli/cancelreader"
	"golang.org/x/sync/errgroup"
)

var (
	// ErrNotTerminal is returned when one of the I/O streams is not a terminal.
	ErrNotTerminal = fmt.Errorf("not a terminal")

	// ErrPlatformNotSupported is returned when the platform is not supported.
	ErrPlatformNotSupported = fmt.Errorf("platform not supported")

	// ErrRunning is returned when the terminal is already running.
	ErrRunning = fmt.Errorf("terminal is running")

	// ErrNotRunning is returned when the terminal is not running.
	ErrNotRunning = fmt.Errorf("terminal not running")
)

// Terminal represents a terminal screen that can be manipulated and drawn to.
// It handles reading events from the terminal using [TerminalReader].
type Terminal struct {
	// Terminal I/O streams and state.
	in          io.Reader
	out         io.Writer
	inTty       term.File
	inTtyState  *term.State
	outTty      term.File
	outTtyState *term.State
	running     atomic.Bool // Indicates if the terminal is running

	// Terminal type, screen and buffer.
	termtype  string            // The $TERM type.
	environ   Environ           // The environment variables.
	buf       *Buffer           // Reference to the last buffer used.
	scr       *TerminalRenderer // The actual screen to be drawn to.
	size      Size              // The last known full size of the terminal.
	pixSize   Size              // The last known pixel size of the terminal.
	method    ansi.Method       // The width method used by the terminal.
	profile   colorprofile.Profile
	useTabs   bool // Whether to use hard tabs or not.
	useBspace bool // Whether to use backspace or not.

	// Terminal input stream.
	cr       cancelreader.CancelReader
	rd       *TerminalReader
	winchn   *SizeNotifier      // The window size notifier for the terminal.
	evs      chan Event         // receiving event channel.
	evch     chan Event         // event loop channel
	evctx    context.Context    // context for the event channel.
	evcancel context.CancelFunc // The cancel function for the event channel.
	evloop   chan struct{}      // Channel to signal the event loop has exited.
	once     sync.Once
	errg     *errgroup.Group
	m        sync.RWMutex // Mutex to protect the terminal state.

	// Renderer state.
	state     state
	lastState *state
	prepend   []string

	logger Logger // The debug logger for I/O.
}

type state struct {
	altscreen bool
	curHidden bool
	cur       Position
}

// DefaultTerminal returns a new default terminal instance that uses
// [os.Stdin], [os.Stdout], and [os.Environ].
func DefaultTerminal() *Terminal {
	return NewTerminal(os.Stdin, os.Stdout, os.Environ())
}

// NewTerminal creates a new [Terminal] instance with the given I/O streams and
// environment variables.
func NewTerminal(in io.Reader, out io.Writer, env []string) *Terminal {
	t := new(Terminal)
	t.in = in
	t.out = out
	if f, ok := in.(term.File); ok {
		t.inTty = f
	}
	if f, ok := out.(term.File); ok {
		t.outTty = f
	}
	t.environ = env
	t.termtype = t.environ.Getenv("TERM")
	t.scr = NewTerminalRenderer(t.out, t.environ)
	t.buf = NewBuffer(0, 0)
	t.method = ansi.WcWidth // Default width method.
	t.SetColorProfile(colorprofile.Detect(out, env))
	t.evs = make(chan Event)
	t.evch = make(chan Event)
	t.once = sync.Once{}

	// Create a new context to manage input events.
	t.evctx, t.evcancel = context.WithCancel(context.Background())
	t.errg, t.evctx = errgroup.WithContext(t.evctx)

	// Window size changes only for non-Windows platforms.
	if !isWindows {
		// Create default input receivers.
		winchTty := t.inTty
		if winchTty == nil {
			winchTty = t.outTty
		}
		t.winchn = NewSizeNotifier(winchTty)
	}

	// Handle debugging I/O.
	debug, ok := os.LookupEnv("UV_DEBUG")
	if ok && len(debug) > 0 {
		f, err := os.OpenFile(debug, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			panic("failed to open debug file: " + err.Error())
		}

		logger := log.New(f, "uv: ", log.LstdFlags|log.Lshortfile)
		t.SetLogger(logger)
	}
	return t
}

// SetLogger sets the debug logger for the terminal. This is used to log debug
// information about the terminal I/O. By default, it is set to a no-op logger.
func (t *Terminal) SetLogger(logger Logger) {
	t.logger = logger
}

// ColorProfile returns the currently used color profile for the terminal.
func (t *Terminal) ColorProfile() colorprofile.Profile {
	return t.profile
}

// SetColorProfile sets a custom color profile for the terminal. This is useful
// for forcing a specific color output. By default, the terminal will use the
// system's color profile inferred by the environment variables.
func (t *Terminal) SetColorProfile(p colorprofile.Profile) {
	t.profile = p
}

// ColorModel returns the color model of the terminal screen.
func (t *Terminal) ColorModel() color.Model {
	return t.ColorProfile()
}

// SetWidthMethod sets the width method used by the terminal. This is typically
// used to determine how the terminal calculates the width of a single
// grapheme.
// The default method is [ansi.WcWidth].
func (t *Terminal) SetWidthMethod(method ansi.Method) {
	t.method = method
}

// WidthMethod returns the width method used by the terminal. This is typically
// used to determine how the terminal calculates the width of a single
// grapheme.
func (t *Terminal) WidthMethod() WidthMethod {
	return t.method
}

var _ color.Model = (*Terminal)(nil)

// Convert converts the given color to the terminal's color profile. This
// implements the [color.Model] interface, allowing you to convert any color to
// the terminal's preferred color model.
func (t *Terminal) Convert(c color.Color) color.Color {
	return t.profile.Convert(c)
}

// GetSize returns the size of the terminal screen. It errors if the size
// cannot be determined.
func (t *Terminal) GetSize() (width, height int, err error) {
	w, h, err := t.getSize()
	if err != nil {
		return 0, 0, fmt.Errorf("error getting terminal size: %w", err)
	}
	// Cache the last known size.
	t.m.Lock()
	t.size.Width = w
	t.size.Height = h
	t.m.Unlock()
	return w, h, nil
}

// Size returns the last known size of the terminal screen. This is updated
// whenever a window size change event is received. If you need to get the
// current size of the terminal, use [Terminal.GetSize].
func (t *Terminal) Size() Size {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.size
}

// Bounds returns the bounds of the terminal screen buffer. This is the
// rectangle that contains start and end points of the screen buffer.
// This is different from [Terminal.GetSize] which queries the size of the
// terminal window. The screen buffer can occupy a portion or all of the
// terminal window. Use [Terminal.Resize] to change the size of the screen
// buffer.
func (t *Terminal) Bounds() Rectangle {
	return Rect(0, 0, t.buf.Width(), t.buf.Height())
}

// SetCell sets the cell at the given x, y position in the terminal buffer.
func (t *Terminal) SetCell(x int, y int, c *Cell) {
	t.buf.SetCell(x, y, c)
}

// CellAt returns the cell at the given x, y position in the terminal buffer.
func (t *Terminal) CellAt(x int, y int) *Cell {
	return t.buf.CellAt(x, y)
}

var _ Screen = (*Terminal)(nil)

// Clear fills the terminal screen with empty cells, and clears the
// terminal screen.
//
// This is different from [Terminal.Erase], which only fills the screen
// buffer with empty cells without erasing the terminal screen first.
func (t *Terminal) Clear() {
	t.buf.Clear()
}

// ClearArea fills the given area of the terminal screen with empty cells.
func (t *Terminal) ClearArea(area Rectangle) {
	t.buf.ClearArea(area)
}

// Fill fills the terminal screen with the given cell. If the cell is nil, it
// fills the screen with empty cells.
func (t *Terminal) Fill(c *Cell) {
	t.buf.Fill(c)
}

// FillArea fills the given area of the terminal screen with the given cell.
// If the cell is nil, it fills the area with empty cells.
func (t *Terminal) FillArea(c *Cell, area Rectangle) {
	t.buf.FillArea(c, area)
}

// Clone returns a copy of the terminal screen buffer. This is useful for
// creating a snapshot of the current terminal state without modifying the
// original buffer.
func (t *Terminal) Clone() *Buffer {
	return t.buf.Clone()
}

// CloneArea clones the given area of the terminal screen and returns a new
// buffer with the same size as the area. The new buffer will contain the
// same cells as the area in the terminal screen.
func (t *Terminal) CloneArea(area Rectangle) *Buffer {
	return t.buf.CloneArea(area)
}

// Position returns the last known position of the cursor in the terminal.
func (t *Terminal) Position() (int, int) {
	return t.scr.Position()
}

// SetPosition sets the position of the cursor in the terminal. This is
// typically used when the cursor was moved manually outside of the [Terminal]
// context.
func (t *Terminal) SetPosition(x, y int) {
	t.scr.SetPosition(x, y)
}

// MoveTo moves the cursor to the given x, y position in the terminal. This
// won't take any effect until the next [Terminal.Display] call.
func (t *Terminal) MoveTo(x, y int) {
	t.state.cur = Pos(x, y)
}

func (t *Terminal) configureRenderer() {
	t.scr.SetColorProfile(t.profile)
	if t.useTabs {
		t.m.RLock()
		t.scr.SetTabStops(t.size.Width)
		t.m.RUnlock()
	}
	t.scr.SetBackspace(t.useBspace)
	t.scr.SetLogger(t.logger)
}

// Erase fills the screen buffer with empty cells, and wipe the terminal
// screen. This is different from [Terminal.Clear], which only fills the
// terminal with empty cells.
//
// This won't take any effect until the next [Terminal.Display].
func (t *Terminal) Erase() {
	t.buf.Touched = nil
	t.scr.Erase()
	t.Clear()
}

// Draw draws the given drawable component to the terminal screen buffer. It
// automatically resizes the screen buffer to match the size of the drawable
// component.
//
// This won't take any effect until the next [Terminal.Display].
func (t *Terminal) Draw(d Drawable) {
	frameArea := t.size.Bounds()
	if d == nil {
		// If the component is nil, we should clear the screen buffer.
		frameArea.Max.Y = 0
	}

	// We need to resizes the screen based on the frame height and
	// terminal width. This is because the frame height can change based on
	// the content of the frame. For example, if the frame contains a list
	// of items, the height of the frame will be the number of items in the
	// list. This is different from the alt screen buffer, which has a
	// fixed height and width.
	frameHeight := frameArea.Dy()
	switch layer := d.(type) {
	case *StyledString:
		frameHeight = layer.Height()
	case interface{ Height() int }:
		frameHeight = layer.Height()
	case interface{ Bounds() Rectangle }:
		frameHeight = layer.Bounds().Dy()
	}

	if frameHeight != frameArea.Dy() {
		frameArea.Max.Y = frameHeight
	}

	// Resize the screen buffer to match the frame area. This is necessary
	// to ensure that the screen buffer is the same size as the frame area
	// and to avoid rendering issues when the frame area is smaller than
	// the screen buffer.
	t.buf.Resize(frameArea.Dx(), frameArea.Dy())

	// Clear our screen buffer before copying the new frame into it to ensure
	// we erase any old content.
	t.buf.Clear()
	if d != nil {
		d.Draw(t, t.buf.Bounds())
	}

	// If the frame height is greater than the screen height, we drop the
	// lines from the top of the buffer.
	if frameHeight := frameArea.Dy(); frameHeight > t.size.Height {
		t.buf.Lines = t.buf.Lines[frameHeight-t.size.Height:]
	}
}

// Display computes the necessary changes to the terminal screen and renders
// the current buffer to the terminal screen.
//
// Typically, you would call this after modifying the terminal buffer using
// [Terminal.SetCell] or [Terminal.PrependString].
func (t *Terminal) Display() error {
	state := t.state // Capture the current state.

	// alternate screen buffer.
	altscreenChanged := t.lastState == nil || t.lastState.altscreen != state.altscreen
	if altscreenChanged {
		setAltScreen(t, state.altscreen)
	}

	// cursor visibility.
	cursorVisChanged := t.lastState == nil || t.lastState.curHidden != state.curHidden
	if cursorVisChanged || altscreenChanged {
		if state.curHidden {
			_, _ = t.scr.WriteString(ansi.ResetModeTextCursorEnable)
		} else {
			_, _ = t.scr.WriteString(ansi.SetModeTextCursorEnable)
		}
	}

	// render the buffer.
	t.scr.Render(t.buf)

	// add any prepended strings.
	if len(t.prepend) > 0 {
		for _, line := range t.prepend {
			prependLine(t, line)
		}
		t.prepend = t.prepend[:0]
	}

	if state.cur != Pos(-1, -1) {
		// MoveTo must come after [TerminalRenderer.Render] because the cursor
		// position might get updated during rendering.
		t.scr.MoveTo(state.cur.X, state.cur.Y)
	}

	if err := t.Flush(); err != nil {
		return fmt.Errorf("error flushing terminal: %w", err)
	}

	t.lastState = &state // Save the last state.

	return nil
}

// Flush flushes any pending renders to the terminal screen. This is typically
// used to flush the underlying screen buffer to the terminal.
//
// Use [Terminal.Buffered] to check how many bytes pending to be flushed.
func (t *Terminal) Flush() error {
	if err := t.scr.Flush(); err != nil {
		return fmt.Errorf("error flushing terminal: %w", err)
	}
	return nil
}

func prependLine(t *Terminal, line string) {
	strLines := strings.Split(line, "\n")
	for i, line := range strLines {
		// If the line is wider than the screen, truncate it.
		strLines[i] = ansi.Truncate(line, t.size.Width, "")
	}
	t.scr.PrependString(t.buf, strings.Join(strLines, "\n"))
}

// Buffered returns the number of bytes buffered for the flush operation.
func (t *Terminal) Buffered() int {
	return t.scr.Buffered()
}

// Touched returns the number of touched lines in the terminal buffer.
func (t *Terminal) Touched() int {
	return t.scr.Touched(t.buf)
}

// EnterAltScreen enters the alternate screen buffer. This is typically used
// for applications that want to take over the entire terminal screen.
//
// Note that this won't take any effect until the next [Terminal.Display] call.
func (t *Terminal) EnterAltScreen() {
	t.state.altscreen = true
}

// ExitAltScreen exits the alternate screen buffer and returns to the normal
// screen buffer.
//
// The [Terminal] manages the alternate screen buffer for you based on the
// [Viewport] used during [Terminal.Display]. This means that you don't need to
// call this unless you know what you're doing.
//
// Note that this won't take any effect until the next [Terminal.Display] call.
func (t *Terminal) ExitAltScreen() {
	t.state.altscreen = false
}

// ShowCursor shows the terminal cursor.
//
// The [Terminal] manages the visibility of the cursor for you based on the
// [Viewport] used during [Terminal.Display]. This means that you don't need to
// call this unless you know what you're doing.
//
// Note that this won't take any effect until the next [Terminal.Display] call.
func (t *Terminal) ShowCursor() {
	t.state.curHidden = false
}

// HideCursor hides the terminal cursor.
//
// The [Terminal] manages the visibility of the cursor for you based on the
// [Viewport] used during [Terminal.Display]. This means that you don't need to
// call this unless you know what you're doing.
//
// Note that this won't take any effect until the next [Terminal.Display] call.
func (t *Terminal) HideCursor() {
	t.state.curHidden = true
}

// Resize resizes the terminal screen buffer to the given width and height.
// This won't affect [Terminal.Size] or the terminal size, but it will resize
// the screen buffer used by the terminal.
func (t *Terminal) Resize(width, height int) error {
	// We need to reset the touched lines buffer to match the new height.
	t.buf.Touched = nil
	t.buf.Resize(width, height)
	if t.scr != nil {
		t.scr.Resize(width, height)
	}
	return nil
}

// Start prepares the terminal for use. It starts the input reader and
// initializes the terminal state. This should be called before using the
// terminal.
func (t *Terminal) Start() error {
	if t.running.Load() {
		return ErrRunning
	}

	if t.inTty == nil && t.outTty == nil {
		return ErrNotTerminal
	}

	// Create a new terminal renderer.
	t.scr = NewTerminalRenderer(t.out, t.environ)

	// First run, add some default states.
	if t.lastState == nil {
		t.EnterAltScreen()
		t.HideCursor()
		t.state.cur = Pos(-1, -1)
	}

	// Store the initial terminal size.
	w, h, err := t.GetSize()
	if err != nil {
		return fmt.Errorf("error getting initial terminal size: %w", err)
	}

	t.size = Size{Width: w, Height: h}

	if t.buf.Width() == 0 && t.buf.Height() == 0 {
		// If the buffer is not initialized, set it to the terminal size.
		t.buf.Resize(t.size.Width, t.size.Height)
		t.scr.Erase()
	}

	t.scr.Resize(t.buf.Width(), t.buf.Height())

	if err := t.initialize(); err != nil {
		_ = t.restore()
		return err
	}

	// We need to call [Terminal.optimizeMovements] before creating the screen
	// to populate [Terminal.useBspace] and [Terminal.useTabs].
	t.optimizeMovements()
	t.configureRenderer()

	t.running.Store(true)

	return t.initializeState()
}

// Pause pauses the terminal input reader. This is typically used to pause the
// terminal input reader when the terminal is not in use, such as when
// switching to another application.
func (t *Terminal) Pause() error {
	if !t.running.Load() {
		return ErrNotRunning
	}

	t.running.Store(false)

	if err := t.restore(); err != nil {
		return fmt.Errorf("error restoring terminal: %w", err)
	}
	return nil
}

// Resume resumes the terminal input reader. This is typically used to resume
// the terminal input reader when the terminal is in use again, such as when
// switching back to the terminal application.
func (t *Terminal) Resume() error {
	if t.running.Load() {
		return ErrRunning
	}

	t.running.Store(true)

	if err := t.initialize(); err != nil {
		return fmt.Errorf("error entering raw mode: %w", err)
	}

	return t.initializeState()
}

// Stop stops the terminal and restores the terminal to its original state.
// This is typically used to stop the terminal gracefully.
func (t *Terminal) Stop() error {
	return t.stop()
}

// Teardown is similar to [Terminal.Stop], but it also closes the input reader
// and the event channel as well as any other resources used by the terminal.
// This is typically used to completely shutdown the application.
func (t *Terminal) Teardown() error {
	if err := t.stop(); err != nil {
		return fmt.Errorf("error stopping terminal: %w", err)
	}
	_ = t.cr.Close()
	t.evcancel()
	return nil
}

// Wait waits for the terminal to shutdown and all pending events to be
// processed. This is typically used to wait for the terminal to shutdown
// gracefully.
func (t *Terminal) Wait() error {
	if err := t.errg.Wait(); err != nil {
		return fmt.Errorf("error waiting for terminal: %w", err)
	}
	return nil
}

// Shutdown gracefully tears down the terminal, restoring its original state
// and stopping the event channel. It waits for any pending events to be
// processed or the context to be done before closing the event channel.
func (t *Terminal) Shutdown(ctx context.Context) error {
	if err := t.Teardown(); err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() {
		errc <- t.Wait()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err() //nolint:wrapcheck
	case err := <-errc:
		return err
	}
}

// Events returns the event channel used by the terminal to send input events.
// Use [Terminal.SendEvent] to send events to the terminal event loop.
func (t *Terminal) Events() <-chan Event {
	return t.evs
}

// SendEvent sends the given event to the terminal event loop. This is
// typically used to send custom events to the terminal, such as
// application-specific events.
func (t *Terminal) SendEvent(ev Event) {
	select {
	case <-t.evctx.Done():
	case t.evch <- ev:
	}
}

// PrependString adds the given string to the top of the terminal screen. The
// string is split into lines and each line is added as a new line at the top
// of the screen. The added lines are not managed by the terminal and will not
// be cleared or updated by the [Terminal].
//
// This will truncate each line to the terminal width, so if the string is
// longer than the terminal width, it will be truncated to fit.
//
// Using this when the terminal is using the alternate screen or when occupying
// the whole screen may not produce any visible effects. This is because once
// the terminal writes the prepended lines, they will get overwritten by the
// next frame.
//
// Note that this won't take any effect until the next [Terminal.Display] call.
func (t *Terminal) PrependString(str string) {
	t.prepend = append(t.prepend, str)
}

// PrependLines adds lines of cells to the top of the terminal screen. The
// added line is unmanaged and will not be cleared or updated by the
// [Terminal].
//
// This will truncate each line to the terminal width, so if the string is
// longer than the terminal width, it will be truncated to fit.
//
// Using this when the terminal is using the alternate screen or when occupying
// the whole screen may not produce any visible effects. This is because once
// the terminal writes the prepended lines, they will get overwritten by the
// next frame.
//
// Note that this won't take any effect until the next [Terminal.Display] call.
func (t *Terminal) PrependLines(lines ...Line) {
	for _, l := range lines {
		t.prepend = append(t.prepend, l.Render())
	}
}

// Write writes the given bytes to the underlying terminal renderer.
// This is typically used to write arbitrary data to the terminal, usually
// escape sequences or control characters.
//
// This can affect the renderer state and the terminal screen, so it should be
// used with caution.
//
// Note that this won't take any effect until the next [Terminal.Display] or
// [Terminal.Flush] call.
func (t *Terminal) Write(p []byte) (n int, err error) {
	return t.scr.Write(p)
}

// WriteString writes the given string to the underlying terminal renderer.
// This is typically used to write arbitrary data to the terminal, usually
// escape sequences or control characters.
//
// This can affect the renderer state and the terminal screen, so it should be
// used with caution.
//
// Note that this won't take any effect until the next [Terminal.Display] or
// [Terminal.Flush] call.
func (t *Terminal) WriteString(s string) (n int, err error) {
	return t.scr.WriteString(s)
}

func (t *Terminal) stop() error {
	if !t.running.Load() {
		return ErrNotRunning
	}

	if err := t.restore(); err != nil {
		return fmt.Errorf("error restoring terminal: %w", err)
	}

	t.running.Store(false)
	return nil
}

func setAltScreen(t *Terminal, enable bool) {
	if enable {
		t.scr.EnterAltScreen()
	} else {
		t.scr.ExitAltScreen()
	}
}

func (t *Terminal) initializeState() error {
	if t.lastState == nil {
		setAltScreen(t, true)
		return nil
	}

	setAltScreen(t, t.lastState.altscreen)

	if t.lastState.curHidden {
		_, _ = t.scr.WriteString(ansi.ResetModeTextCursorEnable)
	} else {
		_, _ = t.scr.WriteString(ansi.SetModeTextCursorEnable)
	}
	if t.lastState.cur != Pos(-1, -1) {
		t.MoveTo(t.lastState.cur.X, t.lastState.cur.Y)
	}

	return t.scr.Flush()
}

func (t *Terminal) initialize() error {
	// Initialize the terminal IO streams.
	if err := t.makeRaw(); err != nil {
		return fmt.Errorf("error entering raw mode: %w", err)
	}

	// Initialize input.
	cr, err := NewCancelReader(t.in)
	if err != nil {
		return fmt.Errorf("error creating cancel reader: %w", err)
	}
	t.cr = cr
	t.rd = NewTerminalReader(t.cr, t.termtype)
	t.rd.SetLogger(t.logger)
	t.evloop = make(chan struct{})

	// Send the initial window size to the event channel.
	t.errg.Go(t.initialResizeEvent)

	// Start the window size notifier if it is available.
	if t.winchn != nil {
		if err := t.winchn.Start(); err != nil {
			return fmt.Errorf("error starting window size notifier: %w", err)
		}

		// Start SIGWINCH listener if available.
		t.errg.Go(t.resizeLoop)
	}

	// Input and event loops
	t.errg.Go(t.inputLoop)
	t.errg.Go(t.eventLoop)

	return nil
}

func (t *Terminal) initialResizeEvent() error {
	var cells, pixels Size
	var err error
	if t.winchn == nil {
		cells.Width, cells.Height, err = t.GetSize()
	} else {
		cells, pixels, err = t.winchn.GetWindowSize()
	}
	if err != nil {
		return err
	}

	events := []Event{
		WindowSizeEvent(cells),
	}
	if pixels.Width > 0 && pixels.Height > 0 {
		events = append(events, WindowPixelSizeEvent(pixels))
	}
	for _, e := range events {
		select {
		case <-t.evctx.Done():
			return nil
		case t.evch <- e:
		}
	}
	return nil
}

func (t *Terminal) resizeLoop() error {
	if t.winchn == nil {
		return nil
	}
	for {
		select {
		case <-t.evctx.Done():
			return nil
		case <-t.winchn.C:
			cells, pixels, err := t.winchn.GetWindowSize()
			if err != nil {
				return err
			}

			t.m.RLock()
			size, pixSize := t.size, t.pixSize
			t.m.RUnlock()

			if cells != size {
				select {
				case <-t.evctx.Done():
					return nil
				case t.evch <- WindowSizeEvent(cells):
				}
			}
			if pixels != pixSize && pixels.Width > 0 && pixels.Height > 0 {
				select {
				case <-t.evctx.Done():
					return nil
				case t.evch <- WindowPixelSizeEvent(pixels):
				}
			}

			t.m.Lock()
			t.size = cells
			if pixels.Width > 0 && pixels.Height > 0 {
				t.pixSize = pixels
			}
			t.m.Unlock()
		}
	}
}

func (t *Terminal) inputLoop() error {
	defer close(t.evloop)

	if err := t.rd.StreamEvents(t.evctx, t.evch); err != nil {
		return err
	}
	return nil
}

func (t *Terminal) eventLoop() error {
	for {
		select {
		case <-t.evctx.Done():
			return nil
		case ev, ok := <-t.evch:
			if !ok {
				return nil
			}
			switch ev := ev.(type) {
			case WindowSizeEvent:
				t.m.Lock()
				t.size = Size(ev)
				t.m.Unlock()
			case WindowPixelSizeEvent:
				t.m.Lock()
				t.pixSize = Size(ev)
				t.m.Unlock()
			}
			select {
			case <-t.evctx.Done():
				return nil
			case t.evs <- ev:
			}
		}
	}
}

// restoreTTY restores the terminal TTY to its original state.
func (t *Terminal) restoreTTY() error {
	if t.inTtyState != nil {
		if err := term.Restore(t.inTty.Fd(), t.inTtyState); err != nil {
			return fmt.Errorf("error restoring input terminal state: %w", err)
		}
		t.inTtyState = nil
	}
	if t.outTtyState != nil {
		if err := term.Restore(t.outTty.Fd(), t.outTtyState); err != nil {
			return fmt.Errorf("error restoring output terminal state: %w", err)
		}
		t.outTtyState = nil
	}
	if t.winchn != nil {
		if err := t.winchn.Stop(); err != nil {
			return fmt.Errorf("error stopping window size notifier: %w", err)
		}
	}

	return nil
}

// restoreState restores the terminal state, including modes, colors, and
// cursor position. If flush is false, it won't commit the changes to the
// terminal immediately.
func (t *Terminal) restoreState() error {
	if t.cr != nil {
		t.cr.Cancel()
		select {
		case <-t.evloop:
		case <-time.After(500 * time.Millisecond):
			// Timeout waiting for the event loop to exit.
		}
	}
	if ls := t.lastState; ls != nil {
		if ls.altscreen {
			setAltScreen(t, false)
		} else {
			// Go to the bottom of the screen.
			t.scr.MoveTo(0, t.buf.Height()-1)
			_, _ = t.WriteString("\r" + ansi.EraseScreenBelow)
		}
		if ls.curHidden {
			_, _ = t.scr.WriteString(ansi.SetModeTextCursorEnable)
		}
	}

	if err := t.scr.Flush(); err != nil {
		return fmt.Errorf("error flushing terminal: %w", err)
	}

	// Reset cursor position for next start.
	t.scr.SetPosition(-1, -1)

	return nil
}

// restore is a helper function that restores the terminal TTY and state. It also moves the cursor
// to the bottom of the screen to avoid overwriting any terminal content.
func (t *Terminal) restore() error {
	if err := t.restoreState(); err != nil {
		return err
	}
	if err := t.restoreTTY(); err != nil {
		return err
	}
	return nil
}
