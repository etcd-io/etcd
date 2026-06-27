package ansi

import (
	"strconv"
	"strings"
)

// ModeSetting represents a mode setting.
type ModeSetting byte

// ModeSetting constants.
const (
	ModeNotRecognized ModeSetting = iota
	ModeSet
	ModeReset
	ModePermanentlySet
	ModePermanentlyReset
)

// IsNotRecognized returns true if the mode is not recognized.
func (m ModeSetting) IsNotRecognized() bool {
	return m == ModeNotRecognized
}

// IsSet returns true if the mode is set or permanently set.
func (m ModeSetting) IsSet() bool {
	return m == ModeSet || m == ModePermanentlySet
}

// IsReset returns true if the mode is reset or permanently reset.
func (m ModeSetting) IsReset() bool {
	return m == ModeReset || m == ModePermanentlyReset
}

// IsPermanentlySet returns true if the mode is permanently set.
func (m ModeSetting) IsPermanentlySet() bool {
	return m == ModePermanentlySet
}

// IsPermanentlyReset returns true if the mode is permanently reset.
func (m ModeSetting) IsPermanentlyReset() bool {
	return m == ModePermanentlyReset
}

// Mode represents an interface for terminal modes.
// Modes can be set, reset, and requested.
type Mode interface {
	Mode() int
}

// SetMode (SM) or (DECSET) returns a sequence to set a mode.
// The mode arguments are a list of modes to set.
//
// If one of the modes is a [DECMode], the function will returns two escape
// sequences.
//
// ANSI format:
//
//	CSI Pd ; ... ; Pd h
//
// DEC format:
//
//	CSI ? Pd ; ... ; Pd h
//
// See: https://vt100.net/docs/vt510-rm/SM.html
func SetMode(modes ...Mode) string {
	return setMode(false, modes...)
}

// SM is an alias for [SetMode].
func SM(modes ...Mode) string {
	return SetMode(modes...)
}

// DECSET is an alias for [SetMode].
func DECSET(modes ...Mode) string {
	return SetMode(modes...)
}

// ResetMode (RM) or (DECRST) returns a sequence to reset a mode.
// The mode arguments are a list of modes to reset.
//
// If one of the modes is a [DECMode], the function will returns two escape
// sequences.
//
// ANSI format:
//
//	CSI Pd ; ... ; Pd l
//
// DEC format:
//
//	CSI ? Pd ; ... ; Pd l
//
// See: https://vt100.net/docs/vt510-rm/RM.html
func ResetMode(modes ...Mode) string {
	return setMode(true, modes...)
}

// RM is an alias for [ResetMode].
func RM(modes ...Mode) string {
	return ResetMode(modes...)
}

// DECRST is an alias for [ResetMode].
func DECRST(modes ...Mode) string {
	return ResetMode(modes...)
}

func setMode(reset bool, modes ...Mode) (s string) {
	if len(modes) == 0 {
		return s
	}

	cmd := "h"
	if reset {
		cmd = "l"
	}

	seq := "\x1b["
	if len(modes) == 1 {
		switch modes[0].(type) {
		case DECMode:
			seq += "?"
		}
		return seq + strconv.Itoa(modes[0].Mode()) + cmd
	}

	dec := make([]string, 0, len(modes)/2)
	ansi := make([]string, 0, len(modes)/2)
	for _, m := range modes {
		switch m.(type) {
		case DECMode:
			dec = append(dec, strconv.Itoa(m.Mode()))
		case ANSIMode:
			ansi = append(ansi, strconv.Itoa(m.Mode()))
		}
	}

	if len(ansi) > 0 {
		s += seq + strings.Join(ansi, ";") + cmd
	}
	if len(dec) > 0 {
		s += seq + "?" + strings.Join(dec, ";") + cmd
	}
	return s
}

// RequestMode (DECRQM) returns a sequence to request a mode from the terminal.
// The terminal responds with a report mode function [DECRPM].
//
// ANSI format:
//
//	CSI Pa $ p
//
// DEC format:
//
//	CSI ? Pa $ p
//
// See: https://vt100.net/docs/vt510-rm/DECRQM.html
func RequestMode(m Mode) string {
	seq := "\x1b["
	switch m.(type) {
	case DECMode:
		seq += "?"
	}
	return seq + strconv.Itoa(m.Mode()) + "$p"
}

// DECRQM is an alias for [RequestMode].
func DECRQM(m Mode) string {
	return RequestMode(m)
}

// ReportMode (DECRPM) returns a sequence that the terminal sends to the host
// in response to a mode request [DECRQM].
//
// ANSI format:
//
//	CSI Pa ; Ps ; $ y
//
// DEC format:
//
//	CSI ? Pa ; Ps $ y
//
// Where Pa is the mode number, and Ps is the mode value.
//
//	0: Not recognized
//	1: Set
//	2: Reset
//	3: Permanent set
//	4: Permanent reset
//
// See: https://vt100.net/docs/vt510-rm/DECRPM.html
func ReportMode(mode Mode, value ModeSetting) string {
	if value > 4 {
		value = 0
	}
	switch mode.(type) {
	case DECMode:
		return "\x1b[?" + strconv.Itoa(mode.Mode()) + ";" + strconv.Itoa(int(value)) + "$y"
	}
	return "\x1b[" + strconv.Itoa(mode.Mode()) + ";" + strconv.Itoa(int(value)) + "$y"
}

// DECRPM is an alias for [ReportMode].
func DECRPM(mode Mode, value ModeSetting) string {
	return ReportMode(mode, value)
}

// ANSIMode represents an ANSI terminal mode.
type ANSIMode int //nolint:revive

// Mode returns the ANSI mode as an integer.
func (m ANSIMode) Mode() int {
	return int(m)
}

// DECMode represents a private DEC terminal mode.
type DECMode int

// Mode returns the DEC mode as an integer.
func (m DECMode) Mode() int {
	return int(m)
}

// Keyboard Action Mode (KAM) is a mode that controls locking of the keyboard.
// When the keyboard is locked, it cannot send data to the terminal.
//
// See: https://vt100.net/docs/vt510-rm/KAM.html
const (
	ModeKeyboardAction = ANSIMode(2)
	KAM                = ModeKeyboardAction

	SetModeKeyboardAction     = "\x1b[2h"
	ResetModeKeyboardAction   = "\x1b[2l"
	RequestModeKeyboardAction = "\x1b[2$p"
)

// Insert/Replace Mode (IRM) is a mode that determines whether characters are
// inserted or replaced when typed.
//
// When enabled, characters are inserted at the cursor position pushing the
// characters to the right. When disabled, characters replace the character at
// the cursor position.
//
// See: https://vt100.net/docs/vt510-rm/IRM.html
const (
	ModeInsertReplace = ANSIMode(4)
	IRM               = ModeInsertReplace

	SetModeInsertReplace     = "\x1b[4h"
	ResetModeInsertReplace   = "\x1b[4l"
	RequestModeInsertReplace = "\x1b[4$p"
)

// BiDirectional Support Mode (BDSM) is a mode that determines whether the
// terminal supports bidirectional text. When enabled, the terminal supports
// bidirectional text and is set to implicit bidirectional mode. When disabled,
// the terminal does not support bidirectional text.
//
// See ECMA-48 7.2.1.
const (
	ModeBiDirectionalSupport = ANSIMode(8)
	BDSM                     = ModeBiDirectionalSupport

	SetModeBiDirectionalSupport     = "\x1b[8h"
	ResetModeBiDirectionalSupport   = "\x1b[8l"
	RequestModeBiDirectionalSupport = "\x1b[8$p"
)

// Send Receive Mode (SRM) or Local Echo Mode is a mode that determines whether
// the terminal echoes characters back to the host. When enabled, the terminal
// sends characters to the host as they are typed.
//
// See: https://vt100.net/docs/vt510-rm/SRM.html
const (
	ModeSendReceive = ANSIMode(12)
	ModeLocalEcho   = ModeSendReceive
	SRM             = ModeSendReceive

	SetModeSendReceive     = "\x1b[12h"
	ResetModeSendReceive   = "\x1b[12l"
	RequestModeSendReceive = "\x1b[12$p"

	SetModeLocalEcho     = "\x1b[12h"
	ResetModeLocalEcho   = "\x1b[12l"
	RequestModeLocalEcho = "\x1b[12$p"
)

// Line Feed/New Line Mode (LNM) is a mode that determines whether the terminal
// interprets the line feed character as a new line.
//
// When enabled, the terminal interprets the line feed character as a new line.
// When disabled, the terminal interprets the line feed character as a line feed.
//
// A new line moves the cursor to the first position of the next line.
// A line feed moves the cursor down one line without changing the column
// scrolling the screen if necessary.
//
// See: https://vt100.net/docs/vt510-rm/LNM.html
const (
	ModeLineFeedNewLine = ANSIMode(20)
	LNM                 = ModeLineFeedNewLine

	SetModeLineFeedNewLine     = "\x1b[20h"
	ResetModeLineFeedNewLine   = "\x1b[20l"
	RequestModeLineFeedNewLine = "\x1b[20$p"
)

// Cursor Keys Mode (DECCKM) is a mode that determines whether the cursor keys
// send ANSI cursor sequences or application sequences.
//
// See: https://vt100.net/docs/vt510-rm/DECCKM.html
const (
	ModeCursorKeys = DECMode(1)
	DECCKM         = ModeCursorKeys

	SetModeCursorKeys     = "\x1b[?1h"
	ResetModeCursorKeys   = "\x1b[?1l"
	RequestModeCursorKeys = "\x1b[?1$p"
)

// Origin Mode (DECOM) is a mode that determines whether the cursor moves to the
// home position or the margin position.
//
// See: https://vt100.net/docs/vt510-rm/DECOM.html
const (
	ModeOrigin = DECMode(6)
	DECOM      = ModeOrigin

	SetModeOrigin     = "\x1b[?6h"
	ResetModeOrigin   = "\x1b[?6l"
	RequestModeOrigin = "\x1b[?6$p"
)

// Auto Wrap Mode (DECAWM) is a mode that determines whether the cursor wraps
// to the next line when it reaches the right margin.
//
// See: https://vt100.net/docs/vt510-rm/DECAWM.html
const (
	ModeAutoWrap = DECMode(7)
	DECAWM       = ModeAutoWrap

	SetModeAutoWrap     = "\x1b[?7h"
	ResetModeAutoWrap   = "\x1b[?7l"
	RequestModeAutoWrap = "\x1b[?7$p"
)

// X10 Mouse Mode is a mode that determines whether the mouse reports on button
// presses.
//
// The terminal responds with the following encoding:
//
//	CSI M CbCxCy
//
// Where Cb is the button-1, where it can be 1, 2, or 3.
// Cx and Cy are the x and y coordinates of the mouse event.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseX10 = DECMode(9)

	SetModeMouseX10     = "\x1b[?9h"
	ResetModeMouseX10   = "\x1b[?9l"
	RequestModeMouseX10 = "\x1b[?9$p"
)

// Text Cursor Enable Mode (DECTCEM) is a mode that shows/hides the cursor.
//
// See: https://vt100.net/docs/vt510-rm/DECTCEM.html
const (
	ModeTextCursorEnable = DECMode(25)
	DECTCEM              = ModeTextCursorEnable

	SetModeTextCursorEnable     = "\x1b[?25h"
	ResetModeTextCursorEnable   = "\x1b[?25l"
	RequestModeTextCursorEnable = "\x1b[?25$p"
)

// These are aliases for [SetModeTextCursorEnable] and [ResetModeTextCursorEnable].
const (
	ShowCursor = SetModeTextCursorEnable
	HideCursor = ResetModeTextCursorEnable
)

// Numeric Keypad Mode (DECNKM) is a mode that determines whether the keypad
// sends application sequences or numeric sequences.
//
// This works like [DECKPAM] and [DECKPNM], but uses different sequences.
//
// See: https://vt100.net/docs/vt510-rm/DECNKM.html
const (
	ModeNumericKeypad = DECMode(66)
	DECNKM            = ModeNumericKeypad

	SetModeNumericKeypad     = "\x1b[?66h"
	ResetModeNumericKeypad   = "\x1b[?66l"
	RequestModeNumericKeypad = "\x1b[?66$p"
)

// Backarrow Key Mode (DECBKM) is a mode that determines whether the backspace
// key sends a backspace or delete character. Disabled by default.
//
// See: https://vt100.net/docs/vt510-rm/DECBKM.html
const (
	ModeBackarrowKey = DECMode(67)
	DECBKM           = ModeBackarrowKey

	SetModeBackarrowKey     = "\x1b[?67h"
	ResetModeBackarrowKey   = "\x1b[?67l"
	RequestModeBackarrowKey = "\x1b[?67$p"
)

// Left Right Margin Mode (DECLRMM) is a mode that determines whether the left
// and right margins can be set with [DECSLRM].
//
// See: https://vt100.net/docs/vt510-rm/DECLRMM.html
const (
	ModeLeftRightMargin = DECMode(69)
	DECLRMM             = ModeLeftRightMargin

	SetModeLeftRightMargin     = "\x1b[?69h"
	ResetModeLeftRightMargin   = "\x1b[?69l"
	RequestModeLeftRightMargin = "\x1b[?69$p"
)

// Normal Mouse Mode is a mode that determines whether the mouse reports on
// button presses and releases. It will also report modifier keys, wheel
// events, and extra buttons.
//
// It uses the same encoding as [ModeMouseX10] with a few differences:
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseNormal = DECMode(1000)

	SetModeMouseNormal     = "\x1b[?1000h"
	ResetModeMouseNormal   = "\x1b[?1000l"
	RequestModeMouseNormal = "\x1b[?1000$p"
)

// Highlight Mouse Tracking is a mode that determines whether the mouse reports
// on button presses, releases, and highlighted cells.
//
// It uses the same encoding as [ModeMouseNormal] with a few differences:
//
// On highlight events, the terminal responds with the following encoding:
//
//	CSI t CxCy
//	CSI T CxCyCxCyCxCy
//
// Where the parameters are startx, starty, endx, endy, mousex, and mousey.
const (
	ModeMouseHighlight = DECMode(1001)

	SetModeMouseHighlight     = "\x1b[?1001h"
	ResetModeMouseHighlight   = "\x1b[?1001l"
	RequestModeMouseHighlight = "\x1b[?1001$p"
)

// VT Hilite Mouse Tracking is a mode that determines whether the mouse reports on
// button presses, releases, and highlighted cells.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
//

// Button Event Mouse Tracking is essentially the same as [ModeMouseNormal],
// but it also reports button-motion events when a button is pressed.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseButtonEvent = DECMode(1002)

	SetModeMouseButtonEvent     = "\x1b[?1002h"
	ResetModeMouseButtonEvent   = "\x1b[?1002l"
	RequestModeMouseButtonEvent = "\x1b[?1002$p"
)

// Any Event Mouse Tracking is the same as [ModeMouseButtonEvent], except that
// all motion events are reported even if no mouse buttons are pressed.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseAnyEvent = DECMode(1003)

	SetModeMouseAnyEvent     = "\x1b[?1003h"
	ResetModeMouseAnyEvent   = "\x1b[?1003l"
	RequestModeMouseAnyEvent = "\x1b[?1003$p"
)

// Focus Event Mode is a mode that determines whether the terminal reports focus
// and blur events.
//
// The terminal sends the following encoding:
//
//	CSI I // Focus In
//	CSI O // Focus Out
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Focus-Tracking
const (
	ModeFocusEvent = DECMode(1004)

	SetModeFocusEvent     = "\x1b[?1004h"
	ResetModeFocusEvent   = "\x1b[?1004l"
	RequestModeFocusEvent = "\x1b[?1004$p"
)

// SGR Extended Mouse Mode is a mode that changes the mouse tracking encoding
// to use SGR parameters.
//
// The terminal responds with the following encoding:
//
//	CSI < Cb ; Cx ; Cy M
//
// Where Cb is the same as [ModeMouseNormal], and Cx and Cy are the x and y.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseExtSgr = DECMode(1006)

	SetModeMouseExtSgr     = "\x1b[?1006h"
	ResetModeMouseExtSgr   = "\x1b[?1006l"
	RequestModeMouseExtSgr = "\x1b[?1006$p"
)

// UTF-8 Extended Mouse Mode is a mode that changes the mouse tracking encoding
// to use UTF-8 parameters.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseExtUtf8 = DECMode(1005)

	SetModeMouseExtUtf8     = "\x1b[?1005h"
	ResetModeMouseExtUtf8   = "\x1b[?1005l"
	RequestModeMouseExtUtf8 = "\x1b[?1005$p"
)

// URXVT Extended Mouse Mode is a mode that changes the mouse tracking encoding
// to use an alternate encoding.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseExtUrxvt = DECMode(1015)

	SetModeMouseExtUrxvt     = "\x1b[?1015h"
	ResetModeMouseExtUrxvt   = "\x1b[?1015l"
	RequestModeMouseExtUrxvt = "\x1b[?1015$p"
)

// SGR Pixel Extended Mouse Mode is a mode that changes the mouse tracking
// encoding to use SGR parameters with pixel coordinates.
//
// This is similar to [ModeMouseExtSgr], but also reports pixel coordinates.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ModeMouseExtSgrPixel = DECMode(1016)

	SetModeMouseExtSgrPixel     = "\x1b[?1016h"
	ResetModeMouseExtSgrPixel   = "\x1b[?1016l"
	RequestModeMouseExtSgrPixel = "\x1b[?1016$p"
)

// Alternate Screen Mode is a mode that determines whether the alternate screen
// buffer is active. When this mode is enabled, the alternate screen buffer is
// cleared.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-The-Alternate-Screen-Buffer
const (
	ModeAltScreen = DECMode(1047)

	SetModeAltScreen     = "\x1b[?1047h"
	ResetModeAltScreen   = "\x1b[?1047l"
	RequestModeAltScreen = "\x1b[?1047$p"
)

// Save Cursor Mode is a mode that saves the cursor position.
// This is equivalent to [SaveCursor] and [RestoreCursor].
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-The-Alternate-Screen-Buffer
const (
	ModeSaveCursor = DECMode(1048)

	SetModeSaveCursor     = "\x1b[?1048h"
	ResetModeSaveCursor   = "\x1b[?1048l"
	RequestModeSaveCursor = "\x1b[?1048$p"
)

// Alternate Screen Save Cursor Mode is a mode that saves the cursor position as in
// [ModeSaveCursor], switches to the alternate screen buffer as in [ModeAltScreen],
// and clears the screen on switch.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-The-Alternate-Screen-Buffer
const (
	ModeAltScreenSaveCursor = DECMode(1049)

	SetModeAltScreenSaveCursor     = "\x1b[?1049h"
	ResetModeAltScreenSaveCursor   = "\x1b[?1049l"
	RequestModeAltScreenSaveCursor = "\x1b[?1049$p"
)

// Bracketed Paste Mode is a mode that determines whether pasted text is
// bracketed with escape sequences.
//
// See: https://cirw.in/blog/bracketed-paste
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Bracketed-Paste-Mode
const (
	ModeBracketedPaste = DECMode(2004)

	SetModeBracketedPaste     = "\x1b[?2004h"
	ResetModeBracketedPaste   = "\x1b[?2004l"
	RequestModeBracketedPaste = "\x1b[?2004$p"
)

// Synchronized Output Mode is a mode that determines whether output is
// synchronized with the terminal.
//
// See: https://gist.github.com/christianparpart/d8a62cc1ab659194337d73e399004036
const (
	ModeSynchronizedOutput = DECMode(2026)

	SetModeSynchronizedOutput     = "\x1b[?2026h"
	ResetModeSynchronizedOutput   = "\x1b[?2026l"
	RequestModeSynchronizedOutput = "\x1b[?2026$p"
)

// Unicode Core Mode is a mode that determines whether the terminal should use
// Unicode grapheme clustering to calculate the width of glyphs for each
// terminal cell.
//
// See: https://github.com/contour-terminal/terminal-unicode-core
const (
	ModeUnicodeCore = DECMode(2027)

	SetModeUnicodeCore     = "\x1b[?2027h"
	ResetModeUnicodeCore   = "\x1b[?2027l"
	RequestModeUnicodeCore = "\x1b[?2027$p"
)

//

// ModeLightDark is a mode that enables reporting the operating system's color
// scheme (light or dark) preference. It reports the color scheme as a [DSR]
// and [LightDarkReport] escape sequences encoded as follows:
//
//	CSI ? 997 ; 1 n   for dark mode
//	CSI ? 997 ; 2 n   for light mode
//
// The color preference can also be requested via the following [DSR] and
// [RequestLightDarkReport] escape sequences:
//
//	CSI ? 996 n
//
// See: https://contour-terminal.org/vt-extensions/color-palette-update-notifications/
const (
	ModeLightDark = DECMode(2031)

	SetModeLightDark     = "\x1b[?2031h"
	ResetModeLightDark   = "\x1b[?2031l"
	RequestModeLightDark = "\x1b[?2031$p"
)

// ModeInBandResize is a mode that reports terminal resize events as escape
// sequences. This is useful for systems that do not support [SIGWINCH] like
// Windows.
//
// The terminal then sends the following encoding:
//
//	CSI 48 ; cellsHeight ; cellsWidth ; pixelHeight ; pixelWidth t
//
// See: https://gist.github.com/rockorager/e695fb2924d36b2bcf1fff4a3704bd83
const (
	ModeInBandResize = DECMode(2048)

	SetModeInBandResize     = "\x1b[?2048h"
	ResetModeInBandResize   = "\x1b[?2048l"
	RequestModeInBandResize = "\x1b[?2048$p"
)

// Win32Input is a mode that determines whether input is processed by the
// Win32 console and Conpty.
//
// See: https://github.com/microsoft/terminal/blob/main/doc/specs/%234999%20-%20Improved%20keyboard%20handling%20in%20Conpty.md
const (
	ModeWin32Input = DECMode(9001)

	SetModeWin32Input     = "\x1b[?9001h"
	ResetModeWin32Input   = "\x1b[?9001l"
	RequestModeWin32Input = "\x1b[?9001$p"
)
