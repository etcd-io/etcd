package uv

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

// Event represents an input event that can be received from an input source.
type Event interface{}

// EventStreamer is an interface that defines a method to stream events from an
// input source. It takes a context and a channel to send events to. The
// streamer should block until the context is done or an error occurs. The
// channel should never be closed by the streamer, as it is the responsibility
// of the consumer to close it when done.
type EventStreamer interface {
	StreamEvents(ctx context.Context, ch chan<- Event) error
}

// UnknownEvent represents an unknown event.
type UnknownEvent string

// String returns a string representation of the unknown event.
func (e UnknownEvent) String() string {
	return fmt.Sprintf("%q", string(e))
}

// UnknownCsiEvent represents an unknown CSI (Control Sequence Introducer) event.
type UnknownCsiEvent string

// String returns a string representation of the unknown CSI event.
func (e UnknownCsiEvent) String() string {
	return fmt.Sprintf("%q", string(e))
}

// UnknownSs3Event represents an unknown SS3 (Single Shift 3) event.
type UnknownSs3Event string

// String returns a string representation of the unknown SS3 event.
func (e UnknownSs3Event) String() string {
	return fmt.Sprintf("%q", string(e))
}

// UnknownOscEvent represents an unknown OSC (Operating System Command) event.
type UnknownOscEvent string

// String returns a string representation of the unknown OSC event.
func (e UnknownOscEvent) String() string {
	return fmt.Sprintf("%q", string(e))
}

// UnknownDcsEvent represents an unknown DCS (Device Control String) event.
type UnknownDcsEvent string

// String returns a string representation of the unknown DCS event.
func (e UnknownDcsEvent) String() string {
	return fmt.Sprintf("%q", string(e))
}

// UnknownSosEvent represents an unknown SOS (Start of String) event.
type UnknownSosEvent string

// String returns a string representation of the unknown SOS event.
func (e UnknownSosEvent) String() string {
	return fmt.Sprintf("%q", string(e))
}

// UnknownPmEvent represents an unknown PM (Privacy Message) event.
type UnknownPmEvent string

// String returns a string representation of the unknown PM event.
func (e UnknownPmEvent) String() string {
	return fmt.Sprintf("%q", string(e))
}

// UnknownApcEvent represents an unknown APC (Application Program Command) event.
type UnknownApcEvent string

// String returns a string representation of the unknown APC event.
func (e UnknownApcEvent) String() string {
	return fmt.Sprintf("%q", string(e))
}

// MultiEvent represents multiple messages event.
type MultiEvent []Event

// String returns a string representation of the multiple messages event.
func (e MultiEvent) String() string {
	var sb strings.Builder
	for _, ev := range e {
		sb.WriteString(fmt.Sprintf("%v\n", ev))
	}
	return sb.String()
}

// Size represents the size of the terminal window.
type Size struct {
	Width  int
	Height int
}

// Bounds returns the bounds corresponding to the size.
func (s Size) Bounds() Rectangle {
	return Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: s.Width, Y: s.Height},
	}
}

// WindowSizeEvent represents the window size in cells.
type WindowSizeEvent Size

// Bounds returns the bounds corresponding to the size.
func (s WindowSizeEvent) Bounds() Rectangle {
	return Size(s).Bounds()
}

// WindowPixelSizeEvent represents the window size in pixels.
type WindowPixelSizeEvent Size

// Bounds returns the bounds corresponding to the size.
func (s WindowPixelSizeEvent) Bounds() Rectangle {
	return Size(s).Bounds()
}

// CellSizeEvent represents the cell size in pixels.
type CellSizeEvent Size

// Bounds returns the bounds corresponding to the size.
func (s CellSizeEvent) Bounds() Rectangle {
	return Size(s).Bounds()
}

// KeyPressEvent represents a key press event.
type KeyPressEvent Key

// MatchString returns true if the [Key] matches one of the given strings.
//
// A string can be a key name like "enter", "tab", "a", or a printable
// character like "1" or " ". It can also have combinations of modifiers like
// "ctrl+a", "shift+enter", "alt+tab", "ctrl+shift+enter", etc.
func (k KeyPressEvent) MatchString(s ...string) bool {
	return Key(k).MatchString(s...)
}

// String implements [fmt.Stringer] and is quite useful for matching key
// events. For details, on what this returns see [Key.String].
func (k KeyPressEvent) String() string {
	return Key(k).String()
}

// Keystroke returns the keystroke representation of the [Key]. While less type
// safe than looking at the individual fields, it will usually be more
// convenient and readable to use this method when matching against keys.
//
// Note that modifier keys are always printed in the following order:
//   - ctrl
//   - alt
//   - shift
//   - meta
//   - hyper
//   - super
//
// For example, you'll always see "ctrl+shift+alt+a" and never
// "shift+ctrl+alt+a".
func (k KeyPressEvent) Keystroke() string {
	return Key(k).Keystroke()
}

// Key returns the underlying key event. This is a syntactic sugar for casting
// the key event to a [Key].
func (k KeyPressEvent) Key() Key {
	return Key(k)
}

// KeyReleaseEvent represents a key release event.
type KeyReleaseEvent Key

// MatchString returns true if the [Key] matches one of the given strings.
//
// A string can be a key name like "enter", "tab", "a", or a printable
// character like "1" or " ". It can also have combinations of modifiers like
// "ctrl+a", "shift+enter", "alt+tab", "ctrl+shift+enter", etc.
func (k KeyReleaseEvent) MatchString(s ...string) bool {
	return Key(k).MatchString(s...)
}

// String implements [fmt.Stringer] and is quite useful for matching key
// events. For details, on what this returns see [Key.String].
func (k KeyReleaseEvent) String() string {
	return Key(k).String()
}

// Keystroke returns the keystroke representation of the [Key]. While less type
// safe than looking at the individual fields, it will usually be more
// convenient and readable to use this method when matching against keys.
//
// Note that modifier keys are always printed in the following order:
//   - ctrl
//   - alt
//   - shift
//   - meta
//   - hyper
//   - super
//
// For example, you'll always see "ctrl+shift+alt+a" and never
// "shift+ctrl+alt+a".
func (k KeyReleaseEvent) Keystroke() string {
	return Key(k).Keystroke()
}

// Key returns the underlying key event. This is a convenience method and
// syntactic sugar to satisfy the [KeyEvent] interface, and cast the key event to
// [Key].
func (k KeyReleaseEvent) Key() Key {
	return Key(k)
}

// KeyEvent represents a key event. This can be either a key press or a key
// release event.
type KeyEvent interface {
	fmt.Stringer

	// Key returns the underlying key event.
	Key() Key
}

// MouseEvent represents a mouse message. This is a generic mouse message that
// can represent any kind of mouse event.
type MouseEvent interface {
	fmt.Stringer

	// Mouse returns the underlying mouse event.
	Mouse() Mouse
}

// MouseClickEvent represents a mouse button click event.
type MouseClickEvent Mouse

// String returns a string representation of the mouse click event.
func (e MouseClickEvent) String() string {
	return Mouse(e).String()
}

// Mouse returns the underlying mouse event. This is a convenience method and
// syntactic sugar to satisfy the [MouseEvent] interface, and cast the mouse
// event to [Mouse].
func (e MouseClickEvent) Mouse() Mouse {
	return Mouse(e)
}

// MouseReleaseEvent represents a mouse button release event.
type MouseReleaseEvent Mouse

// String returns a string representation of the mouse release event.
func (e MouseReleaseEvent) String() string {
	return Mouse(e).String()
}

// Mouse returns the underlying mouse event. This is a convenience method and
// syntactic sugar to satisfy the [MouseEvent] interface, and cast the mouse
// event to [Mouse].
func (e MouseReleaseEvent) Mouse() Mouse {
	return Mouse(e)
}

// MouseWheelEvent represents a mouse wheel message event.
type MouseWheelEvent Mouse

// String returns a string representation of the mouse wheel event.
func (e MouseWheelEvent) String() string {
	return Mouse(e).String()
}

// Mouse returns the underlying mouse event. This is a convenience method and
// syntactic sugar to satisfy the [MouseEvent] interface, and cast the mouse
// event to [Mouse].
func (e MouseWheelEvent) Mouse() Mouse {
	return Mouse(e)
}

// MouseMotionEvent represents a mouse motion event.
type MouseMotionEvent Mouse

// String returns a string representation of the mouse motion event.
func (e MouseMotionEvent) String() string {
	m := Mouse(e)
	if m.Button != 0 {
		return m.String() + "+motion"
	}
	return m.String() + "motion"
}

// Mouse returns the underlying mouse event. This is a convenience method and
// syntactic sugar to satisfy the [MouseEvent] interface, and cast the mouse
// event to [Mouse].
func (e MouseMotionEvent) Mouse() Mouse {
	return Mouse(e)
}

// CursorPositionEvent represents a cursor position event. Where X is the
// zero-based column and Y is the zero-based row.
type CursorPositionEvent struct {
	X, Y int
}

// FocusEvent represents a terminal focus event.
// This occurs when the terminal gains focus.
type FocusEvent struct{}

// BlurEvent represents a terminal blur event.
// This occurs when the terminal loses focus.
type BlurEvent struct{}

// DarkColorSchemeEvent is sent when the operating system is using a dark color
// scheme. This is typically used to notify applications of the current or new
// system color scheme.
type DarkColorSchemeEvent struct{}

// LightColorSchemeEvent is sent when the operating system is using a light color
// scheme. This is typically used to notify applications of the current or new
// system color scheme.
type LightColorSchemeEvent struct{}

// PasteEvent is an message that is emitted when a terminal receives pasted text
// using bracketed-paste.
type PasteEvent struct {
	// Content is the pasted text content.
	Content string
}

// String returns the pasted content as a string.
func (e PasteEvent) String() string {
	return e.Content
}

// PasteStartEvent is an message that is emitted when the terminal starts the
// bracketed-paste text.
type PasteStartEvent struct{}

// PasteEndEvent is an message that is emitted when the terminal ends the
// bracketed-paste text.
type PasteEndEvent struct{}

// TerminalVersionEvent is a message that represents the terminal version.
type TerminalVersionEvent struct {
	Name string
}

// String returns the terminal version as a string.
func (e TerminalVersionEvent) String() string {
	return e.Name
}

// ModifyOtherKeysEvent represents a modifyOtherKeys event.
//
//	0: disable
//	1: enable mode 1
//	2: enable mode 2
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
// See: https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyOtherKeys
type ModifyOtherKeysEvent struct {
	Mode int
}

// KittyGraphicsEvent represents a Kitty Graphics response event.
//
// See https://sw.kovidgoyal.net/kitty/graphics-protocol/
type KittyGraphicsEvent struct {
	Options kitty.Options
	Payload []byte
}

// KeyboardEnhancementsEvent represents a keyboard enhancements report event.
type KeyboardEnhancementsEvent struct {
	// Flags are the Kitty Keyboard Enhancement flags.
	//
	// Bit values:
	//
	//	00000001:  Disambiguate escape codes
	//	00000010:  Report event types
	//	00000100:  Report alternate keys
	//	00001000:  Report all keys as escape codes
	//	00010000:  Report associated text
	//
	// See: https://sw.kovidgoyal.net/kitty/keyboard-protocol/#keyboard-enhancements
	Flags int
}

// Contains reports whether m contains the given enhancements.
func (e KeyboardEnhancementsEvent) Contains(enhancements int) bool {
	return e.Flags&enhancements == enhancements
}

// SupportsKeyDisambiguation returns whether the terminal supports reporting
// disambiguous keys as escape codes.
func (e KeyboardEnhancementsEvent) SupportsKeyDisambiguation() bool {
	return e.Flags&ansi.KittyDisambiguateEscapeCodes != 0
}

// SupportsKeyReleases returns whether the terminal supports key release
// events.
func (e KeyboardEnhancementsEvent) SupportsKeyReleases() bool {
	return e.Flags&ansi.KittyReportEventTypes != 0
}

// SupportsUniformKeyLayout returns whether the terminal supports reporting key
// events as though they were on a PC-101 layout.
func (e KeyboardEnhancementsEvent) SupportsUniformKeyLayout() bool {
	return e.SupportsKeyDisambiguation() &&
		e.Flags&ansi.KittyReportAlternateKeys != 0 &&
		e.Flags&ansi.KittyReportAllKeysAsEscapeCodes != 0
}

// PrimaryDeviceAttributesEvent is an event that represents the terminal
// primary device attributes.
//
// Common attributes include:
//   - 1	132 columns
//   - 2	Printer port
//   - 4	Sixel
//   - 6	Selective erase
//   - 7	Soft character set (DRCS)
//   - 8	User-defined keys (UDKs)
//   - 9	National replacement character sets (NRCS) (International terminal only)
//   - 12	Yugoslavian (SCS)
//   - 15	Technical character set
//   - 18	Windowing capability
//   - 21	Horizontal scrolling
//   - 23	Greek
//   - 24	Turkish
//   - 42	ISO Latin-2 character set
//   - 44	PCTerm
//   - 45	Soft key map
//   - 46	ASCII emulation
//
// See [ansi.PrimaryDeviceAttributes] for more details.
type PrimaryDeviceAttributesEvent []int

// SecondaryDeviceAttributesEvent is an event that represents the terminal
// secondary device attributes.
//
// See [ansi.SecondaryDeviceAttributes] for more details.
type SecondaryDeviceAttributesEvent []int

// TertiaryDeviceAttributesEvent is an event that represents the terminal
// tertiary device attributes.
//
// See [ansi.TertiaryDeviceAttributes] for more details.
type TertiaryDeviceAttributesEvent string

// ModeReportEvent is a message that represents a mode report event (DECRPM).
//
// See: https://vt100.net/docs/vt510-rm/DECRPM.html
type ModeReportEvent struct {
	// Mode is the mode number.
	Mode ansi.Mode

	// Value is the mode value.
	Value ansi.ModeSetting
}

// ForegroundColorEvent represents a foreground color event. This event is
// emitted when the terminal requests the terminal foreground color using
// [ansi.RequestForegroundColor].
type ForegroundColorEvent struct{ color.Color }

// String returns the hex representation of the color.
func (e ForegroundColorEvent) String() string {
	return colorToHex(e.Color)
}

// IsDark returns whether the color is dark.
func (e ForegroundColorEvent) IsDark() bool {
	return isDarkColor(e.Color)
}

// BackgroundColorEvent represents a background color event. This event is
// emitted when the terminal requests the terminal background color using
// [ansi.RequestBackgroundColor].
type BackgroundColorEvent struct{ color.Color }

// String returns the hex representation of the color.
func (e BackgroundColorEvent) String() string {
	return colorToHex(e)
}

// IsDark returns whether the color is dark.
func (e BackgroundColorEvent) IsDark() bool {
	return isDarkColor(e.Color)
}

// CursorColorEvent represents a cursor color change event. This event is
// emitted when the program requests the terminal cursor color using
// [ansi.RequestCursorColor].
type CursorColorEvent struct{ color.Color }

// String returns the hex representation of the color.
func (e CursorColorEvent) String() string {
	return colorToHex(e)
}

// IsDark returns whether the color is dark.
func (e CursorColorEvent) IsDark() bool {
	return isDarkColor(e)
}

// WindowOpEvent is a window operation (XTWINOPS) report event. This is used to
// report various window operations such as reporting the window size or cell
// size.
type WindowOpEvent struct {
	Op   int
	Args []int
}

// CapabilityEvent represents a Termcap/Terminfo response event. Termcap
// responses are generated by the terminal in response to RequestTermcap
// (XTGETTCAP) requests.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands
type CapabilityEvent struct {
	Content string
}

// String returns the capability content.
func (e CapabilityEvent) String() string {
	return e.Content
}

// ClipboardSelection represents a clipboard selection. The most common
// clipboard selections are "system" and "primary" and selections.
type ClipboardSelection = byte

// Clipboard selections.
const (
	SystemClipboard  ClipboardSelection = ansi.SystemClipboard
	PrimaryClipboard ClipboardSelection = ansi.PrimaryClipboard
)

// ClipboardEvent is a clipboard read message event. This message is emitted when
// a terminal receives an OSC52 clipboard read message event.
type ClipboardEvent struct {
	Content   string
	Selection ClipboardSelection
}

// String returns the string representation of the clipboard message.
func (e ClipboardEvent) String() string {
	return e.Content
}

// Clipboard returns the clipboard selection. This can be either
// [SystemClipboard] 'c' or [PrimaryClipboard] 'p'.
func (e ClipboardEvent) Clipboard() ClipboardSelection {
	return e.Selection
}

// ignoredEvent represents a sequence event that is ignored by the terminal
// reader. This is used to ignore certain sequences that can be canceled.
type ignoredEvent string
