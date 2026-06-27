package ansi

// Keyboard Action Mode (KAM) controls locking of the keyboard.
//
// Deprecated: use [ModeKeyboardAction] instead.
const (
	KeyboardActionMode = ANSIMode(2)

	SetKeyboardActionMode     = "\x1b[2h"
	ResetKeyboardActionMode   = "\x1b[2l"
	RequestKeyboardActionMode = "\x1b[2$p"
)

// Insert/Replace Mode (IRM) determines whether characters are inserted or replaced.
//
// Deprecated: use [ModeInsertReplace] instead.
const (
	InsertReplaceMode = ANSIMode(4)

	SetInsertReplaceMode     = "\x1b[4h"
	ResetInsertReplaceMode   = "\x1b[4l"
	RequestInsertReplaceMode = "\x1b[4$p"
)

// BiDirectional Support Mode (BDSM) determines whether the terminal supports bidirectional text.
//
// Deprecated: use [ModeBiDirectionalSupport] instead.
const (
	BiDirectionalSupportMode = ANSIMode(8)

	SetBiDirectionalSupportMode     = "\x1b[8h"
	ResetBiDirectionalSupportMode   = "\x1b[8l"
	RequestBiDirectionalSupportMode = "\x1b[8$p"
)

// Send Receive Mode (SRM) or Local Echo Mode determines whether the terminal echoes characters.
//
// Deprecated: use [ModeSendReceive] instead.
const (
	SendReceiveMode = ANSIMode(12)
	LocalEchoMode   = SendReceiveMode

	SetSendReceiveMode     = "\x1b[12h"
	ResetSendReceiveMode   = "\x1b[12l"
	RequestSendReceiveMode = "\x1b[12$p"

	SetLocalEchoMode     = "\x1b[12h"
	ResetLocalEchoMode   = "\x1b[12l"
	RequestLocalEchoMode = "\x1b[12$p"
)

// Line Feed/New Line Mode (LNM) determines whether the terminal interprets line feed as new line.
//
// Deprecated: use [ModeLineFeedNewLine] instead.
const (
	LineFeedNewLineMode = ANSIMode(20)

	SetLineFeedNewLineMode     = "\x1b[20h"
	ResetLineFeedNewLineMode   = "\x1b[20l"
	RequestLineFeedNewLineMode = "\x1b[20$p"
)

// Cursor Keys Mode (DECCKM) determines whether cursor keys send ANSI or application sequences.
//
// Deprecated: use [ModeCursorKeys] instead.
const (
	CursorKeysMode = DECMode(1)

	SetCursorKeysMode     = "\x1b[?1h"
	ResetCursorKeysMode   = "\x1b[?1l"
	RequestCursorKeysMode = "\x1b[?1$p"
)

// Cursor Keys mode.
//
// Deprecated: use [SetModeCursorKeys] and [ResetModeCursorKeys] instead.
const (
	EnableCursorKeys  = "\x1b[?1h"
	DisableCursorKeys = "\x1b[?1l"
)

// Origin Mode (DECOM) determines whether the cursor moves to home or margin position.
//
// Deprecated: use [ModeOrigin] instead.
const (
	OriginMode = DECMode(6)

	SetOriginMode     = "\x1b[?6h"
	ResetOriginMode   = "\x1b[?6l"
	RequestOriginMode = "\x1b[?6$p"
)

// Auto Wrap Mode (DECAWM) determines whether the cursor wraps to the next line.
//
// Deprecated: use [ModeAutoWrap] instead.
const (
	AutoWrapMode = DECMode(7)

	SetAutoWrapMode     = "\x1b[?7h"
	ResetAutoWrapMode   = "\x1b[?7l"
	RequestAutoWrapMode = "\x1b[?7$p"
)

// X10 Mouse Mode determines whether the mouse reports on button presses.
//
// Deprecated: use [ModeMouseX10] instead.
const (
	X10MouseMode = DECMode(9)

	SetX10MouseMode     = "\x1b[?9h"
	ResetX10MouseMode   = "\x1b[?9l"
	RequestX10MouseMode = "\x1b[?9$p"
)

// Text Cursor Enable Mode (DECTCEM) shows/hides the cursor.
//
// Deprecated: use [ModeTextCursorEnable] instead.
const (
	TextCursorEnableMode = DECMode(25)

	SetTextCursorEnableMode     = "\x1b[?25h"
	ResetTextCursorEnableMode   = "\x1b[?25l"
	RequestTextCursorEnableMode = "\x1b[?25$p"
)

// Text Cursor Enable mode.
//
// Deprecated: use [SetModeTextCursorEnable] and [ResetModeTextCursorEnable] instead.
const (
	CursorEnableMode        = DECMode(25)
	RequestCursorVisibility = "\x1b[?25$p"
)

// Numeric Keypad Mode (DECNKM) determines whether the keypad sends application or numeric sequences.
//
// Deprecated: use [ModeNumericKeypad] instead.
const (
	NumericKeypadMode = DECMode(66)

	SetNumericKeypadMode     = "\x1b[?66h"
	ResetNumericKeypadMode   = "\x1b[?66l"
	RequestNumericKeypadMode = "\x1b[?66$p"
)

// Backarrow Key Mode (DECBKM) determines whether the backspace key sends backspace or delete.
//
// Deprecated: use [ModeBackarrowKey] instead.
const (
	BackarrowKeyMode = DECMode(67)

	SetBackarrowKeyMode     = "\x1b[?67h"
	ResetBackarrowKeyMode   = "\x1b[?67l"
	RequestBackarrowKeyMode = "\x1b[?67$p"
)

// Left Right Margin Mode (DECLRMM) determines whether left and right margins can be set.
//
// Deprecated: use [ModeLeftRightMargin] instead.
const (
	LeftRightMarginMode = DECMode(69)

	SetLeftRightMarginMode     = "\x1b[?69h"
	ResetLeftRightMarginMode   = "\x1b[?69l"
	RequestLeftRightMarginMode = "\x1b[?69$p"
)

// Normal Mouse Mode determines whether the mouse reports on button presses and releases.
//
// Deprecated: use [ModeMouseNormal] instead.
const (
	NormalMouseMode = DECMode(1000)

	SetNormalMouseMode     = "\x1b[?1000h"
	ResetNormalMouseMode   = "\x1b[?1000l"
	RequestNormalMouseMode = "\x1b[?1000$p"
)

// VT Mouse Tracking mode.
//
// Deprecated: use [ModeMouseNormal] instead.
const (
	MouseMode = DECMode(1000)

	EnableMouse  = "\x1b[?1000h"
	DisableMouse = "\x1b[?1000l"
	RequestMouse = "\x1b[?1000$p"
)

// Highlight Mouse Tracking determines whether the mouse reports on button presses and highlighted cells.
//
// Deprecated: use [ModeMouseHighlight] instead.
const (
	HighlightMouseMode = DECMode(1001)

	SetHighlightMouseMode     = "\x1b[?1001h"
	ResetHighlightMouseMode   = "\x1b[?1001l"
	RequestHighlightMouseMode = "\x1b[?1001$p"
)

// VT Hilite Mouse Tracking mode.
//
// Deprecated: use [ModeMouseHighlight] instead.
const (
	MouseHiliteMode = DECMode(1001)

	EnableMouseHilite  = "\x1b[?1001h"
	DisableMouseHilite = "\x1b[?1001l"
	RequestMouseHilite = "\x1b[?1001$p"
)

// Button Event Mouse Tracking reports button-motion events when a button is pressed.
//
// Deprecated: use [ModeMouseButtonEvent] instead.
const (
	ButtonEventMouseMode = DECMode(1002)

	SetButtonEventMouseMode     = "\x1b[?1002h"
	ResetButtonEventMouseMode   = "\x1b[?1002l"
	RequestButtonEventMouseMode = "\x1b[?1002$p"
)

// Cell Motion Mouse Tracking mode.
//
// Deprecated: use [ModeMouseButtonEvent] instead.
const (
	MouseCellMotionMode = DECMode(1002)

	EnableMouseCellMotion  = "\x1b[?1002h"
	DisableMouseCellMotion = "\x1b[?1002l"
	RequestMouseCellMotion = "\x1b[?1002$p"
)

// Any Event Mouse Tracking reports all motion events.
//
// Deprecated: use [ModeMouseAnyEvent] instead.
const (
	AnyEventMouseMode = DECMode(1003)

	SetAnyEventMouseMode     = "\x1b[?1003h"
	ResetAnyEventMouseMode   = "\x1b[?1003l"
	RequestAnyEventMouseMode = "\x1b[?1003$p"
)

// All Mouse Tracking mode.
//
// Deprecated: use [ModeMouseAnyEvent] instead.
const (
	MouseAllMotionMode = DECMode(1003)

	EnableMouseAllMotion  = "\x1b[?1003h"
	DisableMouseAllMotion = "\x1b[?1003l"
	RequestMouseAllMotion = "\x1b[?1003$p"
)

// Focus Event Mode determines whether the terminal reports focus and blur events.
//
// Deprecated: use [ModeFocusEvent] instead.
const (
	FocusEventMode = DECMode(1004)

	SetFocusEventMode     = "\x1b[?1004h"
	ResetFocusEventMode   = "\x1b[?1004l"
	RequestFocusEventMode = "\x1b[?1004$p"
)

// Focus reporting mode.
//
// Deprecated: use [SetModeFocusEvent], [ResetModeFocusEvent], and
// [RequestModeFocusEvent] instead.
const (
	ReportFocusMode = DECMode(1004)

	EnableReportFocus  = "\x1b[?1004h"
	DisableReportFocus = "\x1b[?1004l"
	RequestReportFocus = "\x1b[?1004$p"
)

// UTF-8 Extended Mouse Mode changes the mouse tracking encoding to use UTF-8 parameters.
//
// Deprecated: use [ModeMouseExtUtf8] instead.
const (
	Utf8ExtMouseMode = DECMode(1005)

	SetUtf8ExtMouseMode     = "\x1b[?1005h"
	ResetUtf8ExtMouseMode   = "\x1b[?1005l"
	RequestUtf8ExtMouseMode = "\x1b[?1005$p"
)

// SGR Extended Mouse Mode changes the mouse tracking encoding to use SGR parameters.
//
// Deprecated: use [ModeMouseExtSgr] instead.
const (
	SgrExtMouseMode = DECMode(1006)

	SetSgrExtMouseMode     = "\x1b[?1006h"
	ResetSgrExtMouseMode   = "\x1b[?1006l"
	RequestSgrExtMouseMode = "\x1b[?1006$p"
)

// Mouse SGR Extended mode.
//
// Deprecated: use [ModeMouseExtSgr], [SetModeMouseExtSgr],
// [ResetModeMouseExtSgr], and [RequestModeMouseExtSgr] instead.
const (
	MouseSgrExtMode    = DECMode(1006)
	EnableMouseSgrExt  = "\x1b[?1006h"
	DisableMouseSgrExt = "\x1b[?1006l"
	RequestMouseSgrExt = "\x1b[?1006$p"
)

// URXVT Extended Mouse Mode changes the mouse tracking encoding to use an alternate encoding.
//
// Deprecated: use [ModeMouseUrxvtExt] instead.
const (
	UrxvtExtMouseMode = DECMode(1015)

	SetUrxvtExtMouseMode     = "\x1b[?1015h"
	ResetUrxvtExtMouseMode   = "\x1b[?1015l"
	RequestUrxvtExtMouseMode = "\x1b[?1015$p"
)

// SGR Pixel Extended Mouse Mode changes the mouse tracking encoding to use SGR parameters with pixel coordinates.
//
// Deprecated: use [ModeMouseExtSgrPixel] instead.
const (
	SgrPixelExtMouseMode = DECMode(1016)

	SetSgrPixelExtMouseMode     = "\x1b[?1016h"
	ResetSgrPixelExtMouseMode   = "\x1b[?1016l"
	RequestSgrPixelExtMouseMode = "\x1b[?1016$p"
)

// Alternate Screen Mode determines whether the alternate screen buffer is active.
//
// Deprecated: use [ModeAltScreen] instead.
const (
	AltScreenMode = DECMode(1047)

	SetAltScreenMode     = "\x1b[?1047h"
	ResetAltScreenMode   = "\x1b[?1047l"
	RequestAltScreenMode = "\x1b[?1047$p"
)

// Save Cursor Mode saves the cursor position.
//
// Deprecated: use [ModeSaveCursor] instead.
const (
	SaveCursorMode = DECMode(1048)

	SetSaveCursorMode     = "\x1b[?1048h"
	ResetSaveCursorMode   = "\x1b[?1048l"
	RequestSaveCursorMode = "\x1b[?1048$p"
)

// Alternate Screen Save Cursor Mode saves the cursor position and switches to alternate screen.
//
// Deprecated: use [ModeAltScreenSaveCursor] instead.
const (
	AltScreenSaveCursorMode = DECMode(1049)

	SetAltScreenSaveCursorMode     = "\x1b[?1049h"
	ResetAltScreenSaveCursorMode   = "\x1b[?1049l"
	RequestAltScreenSaveCursorMode = "\x1b[?1049$p"
)

// Alternate Screen Buffer mode.
//
// Deprecated: use [ModeAltScreenSaveCursor] instead.
const (
	AltScreenBufferMode = DECMode(1049)

	SetAltScreenBufferMode     = "\x1b[?1049h"
	ResetAltScreenBufferMode   = "\x1b[?1049l"
	RequestAltScreenBufferMode = "\x1b[?1049$p"

	EnableAltScreenBuffer  = "\x1b[?1049h"
	DisableAltScreenBuffer = "\x1b[?1049l"
	RequestAltScreenBuffer = "\x1b[?1049$p"
)

// Bracketed Paste Mode determines whether pasted text is bracketed with escape sequences.
//
// Deprecated: use [ModeBracketedPaste] instead.
const (
	BracketedPasteMode = DECMode(2004)

	SetBracketedPasteMode     = "\x1b[?2004h"
	ResetBracketedPasteMode   = "\x1b[?2004l"
	RequestBracketedPasteMode = "\x1b[?2004$p"
)

// Deprecated: use [SetModeBracketedPaste], [ResetModeBracketedPaste], and
// [RequestModeBracketedPaste] instead.
const (
	EnableBracketedPaste  = "\x1b[?2004h" //nolint:revive
	DisableBracketedPaste = "\x1b[?2004l"
	RequestBracketedPaste = "\x1b[?2004$p"
)

// Synchronized Output Mode determines whether output is synchronized with the terminal.
//
// Deprecated: use [ModeSynchronizedOutput] instead.
const (
	SynchronizedOutputMode = DECMode(2026)

	SetSynchronizedOutputMode     = "\x1b[?2026h"
	ResetSynchronizedOutputMode   = "\x1b[?2026l"
	RequestSynchronizedOutputMode = "\x1b[?2026$p"
)

// Synchronized output mode.
//
// Deprecated: use [ModeSynchronizedOutput], [SetModeSynchronizedOutput],
// [ResetModeSynchronizedOutput], and [RequestModeSynchronizedOutput] instead.
const (
	SyncdOutputMode = DECMode(2026)

	EnableSyncdOutput  = "\x1b[?2026h"
	DisableSyncdOutput = "\x1b[?2026l"
	RequestSyncdOutput = "\x1b[?2026$p"
)

// Unicode Core Mode determines whether the terminal uses Unicode grapheme clustering.
//
// Deprecated: use [ModeUnicodeCore] instead.
const (
	UnicodeCoreMode = DECMode(2027)

	SetUnicodeCoreMode     = "\x1b[?2027h"
	ResetUnicodeCoreMode   = "\x1b[?2027l"
	RequestUnicodeCoreMode = "\x1b[?2027$p"
)

// Grapheme Clustering Mode determines whether the terminal looks for grapheme clusters.
//
// Deprecated: use [ModeUnicodeCore], [SetModeUnicodeCore],
// [ResetModeUnicodeCore], and [RequestModeUnicodeCore] instead.
const (
	GraphemeClusteringMode = DECMode(2027)

	SetGraphemeClusteringMode     = "\x1b[?2027h"
	ResetGraphemeClusteringMode   = "\x1b[?2027l"
	RequestGraphemeClusteringMode = "\x1b[?2027$p"
)

// Unicode Core mode.
//
// Deprecated: use [SetModeUnicodeCore], [ResetModeUnicodeCore], and
// [RequestModeUnicodeCore] instead.
const (
	EnableGraphemeClustering  = "\x1b[?2027h"
	DisableGraphemeClustering = "\x1b[?2027l"
	RequestGraphemeClustering = "\x1b[?2027$p"
)

// Light Dark Mode enables reporting the operating system's color scheme preference.
//
// Deprecated: use [ModeLightDark] instead.
const (
	LightDarkMode = DECMode(2031)

	SetLightDarkMode     = "\x1b[?2031h"
	ResetLightDarkMode   = "\x1b[?2031l"
	RequestLightDarkMode = "\x1b[?2031$p"
)

// In Band Resize Mode reports terminal resize events as escape sequences.
//
// Deprecated: use [ModeInBandResize] instead.
const (
	InBandResizeMode = DECMode(2048)

	SetInBandResizeMode     = "\x1b[?2048h"
	ResetInBandResizeMode   = "\x1b[?2048l"
	RequestInBandResizeMode = "\x1b[?2048$p"
)

// Win32Input determines whether input is processed by the Win32 console and Conpty.
//
// Deprecated: use [ModeWin32Input] instead.
const (
	Win32InputMode = DECMode(9001)

	SetWin32InputMode     = "\x1b[?9001h"
	ResetWin32InputMode   = "\x1b[?9001l"
	RequestWin32InputMode = "\x1b[?9001$p"
)

// Deprecated: use [SetModeWin32Input], [ResetModeWin32Input], and
// [RequestModeWin32Input] instead.
const (
	EnableWin32Input  = "\x1b[?9001h" //nolint:revive
	DisableWin32Input = "\x1b[?9001l"
	RequestWin32Input = "\x1b[?9001$p"
)
