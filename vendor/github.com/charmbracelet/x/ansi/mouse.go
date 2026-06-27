package ansi

import (
	"fmt"
)

// MouseButton represents the button that was pressed during a mouse message.
type MouseButton byte

// Mouse event buttons
//
// This is based on X11 mouse button codes.
//
//	1 = left button
//	2 = middle button (pressing the scroll wheel)
//	3 = right button
//	4 = turn scroll wheel up
//	5 = turn scroll wheel down
//	6 = push scroll wheel left
//	7 = push scroll wheel right
//	8 = 4th button (aka browser backward button)
//	9 = 5th button (aka browser forward button)
//	10
//	11
//
// Other buttons are not supported.
const (
	MouseNone MouseButton = iota
	MouseButton1
	MouseButton2
	MouseButton3
	MouseButton4
	MouseButton5
	MouseButton6
	MouseButton7
	MouseButton8
	MouseButton9
	MouseButton10
	MouseButton11

	MouseLeft       = MouseButton1
	MouseMiddle     = MouseButton2
	MouseRight      = MouseButton3
	MouseWheelUp    = MouseButton4
	MouseWheelDown  = MouseButton5
	MouseWheelLeft  = MouseButton6
	MouseWheelRight = MouseButton7
	MouseBackward   = MouseButton8
	MouseForward    = MouseButton9
	MouseRelease    = MouseNone
)

var mouseButtons = map[MouseButton]string{
	MouseNone:       "none",
	MouseLeft:       "left",
	MouseMiddle:     "middle",
	MouseRight:      "right",
	MouseWheelUp:    "wheelup",
	MouseWheelDown:  "wheeldown",
	MouseWheelLeft:  "wheelleft",
	MouseWheelRight: "wheelright",
	MouseBackward:   "backward",
	MouseForward:    "forward",
	MouseButton10:   "button10",
	MouseButton11:   "button11",
}

// String returns a string representation of the mouse button.
func (b MouseButton) String() string {
	return mouseButtons[b]
}

// EncodeMouseButton returns a byte representing a mouse button.
// The button is a bitmask of the following leftmost values:
//
//   - The first two bits are the button number:
//     0 = left button, wheel up, or button no. 8 aka (backwards)
//     1 = middle button, wheel down, or button no. 9 aka (forwards)
//     2 = right button, wheel left, or button no. 10
//     3 = release event, wheel right, or button no. 11
//
//   - The third bit indicates whether the shift key was pressed.
//
//   - The fourth bit indicates the alt key was pressed.
//
//   - The fifth bit indicates the control key was pressed.
//
//   - The sixth bit indicates motion events. Combined with button number 3, i.e.
//     release event, it represents a drag event.
//
//   - The seventh bit indicates a wheel event.
//
//   - The eighth bit indicates additional buttons.
//
// If button is [MouseNone], and motion is false, this returns a release event.
// If button is undefined, this function returns 0xff.
func EncodeMouseButton(b MouseButton, motion, shift, alt, ctrl bool) (m byte) {
	// mouse bit shifts
	const (
		bitShift  = 0b0000_0100
		bitAlt    = 0b0000_1000
		bitCtrl   = 0b0001_0000
		bitMotion = 0b0010_0000
		bitWheel  = 0b0100_0000
		bitAdd    = 0b1000_0000 // additional buttons 8-11

		bitsMask = 0b0000_0011
	)

	if b == MouseNone {
		m = bitsMask
	} else if b >= MouseLeft && b <= MouseRight {
		m = byte(b - MouseLeft)
	} else if b >= MouseWheelUp && b <= MouseWheelRight {
		m = byte(b - MouseWheelUp)
		m |= bitWheel
	} else if b >= MouseBackward && b <= MouseButton11 {
		m = byte(b - MouseBackward)
		m |= bitAdd
	} else {
		m = 0xff // invalid button
	}

	if shift {
		m |= bitShift
	}
	if alt {
		m |= bitAlt
	}
	if ctrl {
		m |= bitCtrl
	}
	if motion {
		m |= bitMotion
	}

	return m
}

// x10Offset is the offset for X10 mouse events.
// See https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#Mouse%20Tracking
const x10Offset = 32

// MouseX10 returns an escape sequence representing a mouse event in X10 mode.
// Note that this requires the terminal support X10 mouse modes.
//
//	CSI M Cb Cx Cy
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#Mouse%20Tracking
func MouseX10(b byte, x, y int) string {
	return "\x1b[M" + string(b+x10Offset) + string(byte(x)+x10Offset+1) + string(byte(y)+x10Offset+1)
}

// MouseSgr returns an escape sequence representing a mouse event in SGR mode.
//
//	CSI < Cb ; Cx ; Cy M
//	CSI < Cb ; Cx ; Cy m (release)
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#Mouse%20Tracking
func MouseSgr(b byte, x, y int, release bool) string {
	s := 'M'
	if release {
		s = 'm'
	}
	if x < 0 {
		x = -x
	}
	if y < 0 {
		y = -y
	}
	return fmt.Sprintf("\x1b[<%d;%d;%d%c", b, x+1, y+1, s)
}
