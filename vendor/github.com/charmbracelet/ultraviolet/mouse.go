package uv

import (
	"github.com/charmbracelet/x/ansi"
)

// MouseMode represents the mouse mode for the terminal. It is used to enable
// or disable mouse support on the terminal.
type MouseMode byte

// Mouse modes.
const (
	MouseModeNone MouseMode = iota
	MouseModeClick
	MouseModeDrag
	MouseModeMotion
)

// MouseButton represents the button that was pressed during a mouse message.
type MouseButton = ansi.MouseButton

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
	MouseNone       = ansi.MouseNone
	MouseLeft       = ansi.MouseLeft
	MouseMiddle     = ansi.MouseMiddle
	MouseRight      = ansi.MouseRight
	MouseWheelUp    = ansi.MouseWheelUp
	MouseWheelDown  = ansi.MouseWheelDown
	MouseWheelLeft  = ansi.MouseWheelLeft
	MouseWheelRight = ansi.MouseWheelRight
	MouseBackward   = ansi.MouseBackward
	MouseForward    = ansi.MouseForward
	MouseButton10   = ansi.MouseButton10
	MouseButton11   = ansi.MouseButton11
)

// Mouse represents a Mouse message. Use [MouseEvent] to represent all mouse
// messages.
//
// The X and Y coordinates are zero-based, with (0,0) being the upper left
// corner of the terminal.
//
//	// Catch all mouse events
//	switch Event := Event.(type) {
//	case MouseEvent:
//	    m := Event.Mouse()
//	    fmt.Println("Mouse event:", m.X, m.Y, m)
//	}
//
//	// Only catch mouse click events
//	switch Event := Event.(type) {
//	case MouseClickEvent:
//	    fmt.Println("Mouse click event:", Event.X, Event.Y, Event)
//	}
type Mouse struct {
	X, Y   int
	Button MouseButton
	Mod    KeyMod
}

// String returns a string representation of the mouse message.
func (m Mouse) String() (s string) {
	if m.Mod.Contains(ModCtrl) {
		s += "ctrl+"
	}
	if m.Mod.Contains(ModAlt) {
		s += "alt+"
	}
	if m.Mod.Contains(ModShift) {
		s += "shift+"
	}

	str := m.Button.String()
	if str == "" {
		s += "unknown"
	} else if str != "none" { // motion events don't have a button
		s += str
	}

	return s
}
