package lipgloss

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Canvas is a cell-buffer that can be used to compose and draw [uv.Drawable]s
// like [Layer]s.
//
// Composed drawables are drawn onto the canvas in the order they were
// composed, meaning later drawables will appear "on top" of earlier ones.
//
// A canvas can read, modify, and render its cell contents.
//
// It implements [uv.Screen] and [uv.Drawable].
type Canvas struct {
	scr uv.ScreenBuffer
}

var _ uv.Screen = (*Canvas)(nil)

// NewCanvas creates a new [Canvas] with the given size.
func NewCanvas(width, height int) *Canvas {
	c := new(Canvas)
	c.scr = uv.NewScreenBuffer(width, height)
	c.scr.Method = ansi.GraphemeWidth
	return c
}

// Resize resizes the canvas to the given width and height.
func (c *Canvas) Resize(width, height int) {
	c.scr.Resize(width, height)
}

// Clear clears the canvas.
func (c *Canvas) Clear() {
	c.scr.Clear()
}

// Bounds implements [uv.Screen].
func (c *Canvas) Bounds() uv.Rectangle {
	return c.scr.Bounds()
}

// Width returns the width of the canvas.
func (c *Canvas) Width() int {
	return c.scr.Width()
}

// Height returns the height of the canvas.
func (c *Canvas) Height() int {
	return c.scr.Height()
}

// CellAt implements [uv.Screen].
func (c *Canvas) CellAt(x int, y int) *uv.Cell {
	return c.scr.CellAt(x, y)
}

// SetCell implements [uv.Screen].
func (c *Canvas) SetCell(x int, y int, cell *uv.Cell) {
	c.scr.SetCell(x, y, cell)
}

// WidthMethod implements [uv.Screen].
func (c *Canvas) WidthMethod() uv.WidthMethod {
	return c.scr.WidthMethod()
}

// Compose composes a [Layer] or any [uv.Drawable] onto the [Canvas].
func (c *Canvas) Compose(drawer uv.Drawable) *Canvas {
	drawer.Draw(c, c.Bounds())
	return c
}

// Draw draws the [Canvas] onto the given [uv.Screen] within the specified
// area.
//
// It implements [uv.Drawable].
func (c *Canvas) Draw(scr uv.Screen, area uv.Rectangle) {
	c.scr.Draw(scr, area)
}

// Render renders the canvas into a styled string.
func (c *Canvas) Render() string {
	return c.scr.Render()
}
