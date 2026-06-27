package uv

import (
	"image/color"
	"runtime"
)

var isWindows = runtime.GOOS == "windows"

// Drawable represents a drawable component on a [Screen].
type Drawable interface {
	// Draw renders the component on the screen for the given area.
	Draw(scr Screen, area Rectangle)
}

// WidthMethod determines how many columns a grapheme occupies on the screen.
type WidthMethod interface {
	StringWidth(s string) int
}

// Screen represents a screen that can be drawn to.
type Screen interface {
	// Bounds returns the bounds of the screen. This is the rectangle that
	// includes the start and end points of the screen.
	Bounds() Rectangle

	// CellAt returns the cell at the given position. If the position is out of
	// bounds, it returns nil. Otherwise, it always returns a cell, even if it
	// is empty (i.e., a cell with a space character and a width of 1).
	CellAt(x, y int) *Cell

	// SetCell sets the cell at the given position. A nil cell is treated as an
	// empty cell with a space character and a width of 1.
	SetCell(x, y int, c *Cell)

	// WidthMethod returns the width method used by the screen.
	WidthMethod() WidthMethod
}

// Cursor represents a cursor on the terminal screen.
type Cursor struct {
	// Position is a [Position] that determines the cursor's position on the
	// screen relative to the top left corner of the frame.
	Position

	// Color is a [color.Color] that determines the cursor's color.
	Color color.Color

	// Shape is a [CursorShape] that determines the cursor's shape.
	Shape CursorShape

	// Blink is a boolean that determines whether the cursor should blink.
	Blink bool
}

// NewCursor returns a new cursor with the default settings and the given
// position.
func NewCursor(x, y int) *Cursor {
	return &Cursor{
		Position: Position{X: x, Y: y},
		Color:    nil,
		Shape:    CursorBlock,
		Blink:    true,
	}
}

// ProgressBarState represents the state of the progress bar.
type ProgressBarState int

// Progress bar states.
const (
	ProgressBarNone ProgressBarState = iota
	ProgressBarDefault
	ProgressBarError
	ProgressBarIndeterminate
	ProgressBarWarning
)

// String return a human-readable value for the given [ProgressBarState].
func (s ProgressBarState) String() string {
	return [...]string{
		"None",
		"Default",
		"Error",
		"Indeterminate",
		"Warning",
	}[s]
}

// ProgressBar represents the terminal progress bar.
//
// Support depends on the terminal.
//
// See https://learn.microsoft.com/en-us/windows/terminal/tutorials/progress-bar-sequences
type ProgressBar struct {
	// State is the current state of the progress bar. It can be one of
	// [ProgressBarNone], [ProgressBarDefault], [ProgressBarError],
	// [ProgressBarIndeterminate], and [ProgressBarWarning].
	State ProgressBarState
	// Value is the current value of the progress bar. It should be between
	// 0 and 100.
	Value int
}

// NewProgressBar returns a new progress bar with the given state and value.
// The value is ignored if the state is [ProgressBarNone] or
// [ProgressBarIndeterminate].
func NewProgressBar(state ProgressBarState, value int) *ProgressBar {
	return &ProgressBar{
		State: state,
		Value: min(max(value, 0), 100),
	}
}
