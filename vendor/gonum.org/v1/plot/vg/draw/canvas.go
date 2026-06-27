// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package draw // import "gonum.org/v1/plot/vg/draw"

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"sync"

	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
)

// formats holds the registered canvas image formats
var formats = struct {
	sync.RWMutex
	m map[string]func(w, h vg.Length) vg.CanvasWriterTo
}{
	m: make(map[string]func(w, h vg.Length) vg.CanvasWriterTo),
}

// Formats returns the sorted list of registered vg formats.
func Formats() []string {
	formats.RLock()
	defer formats.RUnlock()

	list := make([]string, 0, len(formats.m))
	for name := range formats.m {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

// RegisterFormat registers an image format for use by NewFormattedCanvas.
// name is the name of the format, like "jpeg" or "png".
// fn is the construction function to call for the format.
//
// RegisterFormat panics if fn is nil.
func RegisterFormat(name string, fn func(w, h vg.Length) vg.CanvasWriterTo) {
	formats.Lock()
	defer formats.Unlock()

	if fn == nil {
		panic("draw: RegisterFormat with nil function")
	}
	formats.m[name] = fn
}

// A Canvas is a vector graphics canvas along with
// an associated Rectangle defining a section of the canvas
// to which drawing should take place.
type Canvas struct {
	vg.Canvas
	vg.Rectangle
}

// XAlignment specifies text alignment in the X direction. Three preset
// options are available, but an arbitrary alignment
// can also be specified using XAlignment(desired number).
type XAlignment = text.XAlignment

const (
	// XLeft aligns the left edge of the text with the specified location.
	XLeft = text.XLeft
	// XCenter aligns the horizontal center of the text with the specified location.
	XCenter = text.XCenter
	// XRight aligns the right edge of the text with the specified location.
	XRight = text.XRight
)

// YAlignment specifies text alignment in the Y direction. Three preset
// options are available, but an arbitrary alignment
// can also be specified using YAlignment(desired number).
type YAlignment = text.YAlignment

const (
	// YTop aligns the top of of the text with the specified location.
	YTop = text.YTop
	// YCenter aligns the vertical center of the text with the specified location.
	YCenter = text.YCenter
	// YBottom aligns the bottom of the text with the specified location.
	YBottom = text.YBottom
)

// Position specifies the text position.
const (
	PosLeft   = text.PosLeft
	PosBottom = text.PosBottom
	PosCenter = text.PosCenter
	PosTop    = text.PosTop
	PosRight  = text.PosRight
)

// LineStyle describes what a line will look like.
type LineStyle struct {
	// Color is the color of the line.
	Color color.Color

	// Width is the width of the line.
	Width vg.Length

	Dashes   []vg.Length
	DashOffs vg.Length
}

// A GlyphStyle specifies the look of a glyph used to draw
// a point on a plot.
type GlyphStyle struct {
	// Color is the color used to draw the glyph.
	color.Color

	// Radius specifies the size of the glyph's radius.
	Radius vg.Length

	// Shape draws the shape of the glyph.
	Shape GlyphDrawer
}

// A GlyphDrawer wraps the DrawGlyph function.
type GlyphDrawer interface {
	// DrawGlyph draws the glyph at the given
	// point, with the given color and radius.
	DrawGlyph(*Canvas, GlyphStyle, vg.Point)
}

// DrawGlyph draws the given glyph to the draw
// area.  If the point is not within the Canvas
// or the sty.Shape is nil then nothing is drawn.
func (c *Canvas) DrawGlyph(sty GlyphStyle, pt vg.Point) {
	if sty.Shape == nil || !c.Contains(pt) {
		return
	}
	c.SetColor(sty.Color)
	sty.Shape.DrawGlyph(c, sty, pt)
}

// DrawGlyphNoClip draws the given glyph to the draw
// area.  If the sty.Shape is nil then nothing is drawn.
func (c *Canvas) DrawGlyphNoClip(sty GlyphStyle, pt vg.Point) {
	if sty.Shape == nil {
		return
	}
	c.SetColor(sty.Color)
	sty.Shape.DrawGlyph(c, sty, pt)
}

// Rectangle returns the rectangle surrounding this glyph,
// assuming that it is drawn centered at 0,0
func (g GlyphStyle) Rectangle() vg.Rectangle {
	return vg.Rectangle{
		Min: vg.Point{X: -g.Radius, Y: -g.Radius},
		Max: vg.Point{X: +g.Radius, Y: +g.Radius},
	}
}

// CircleGlyph is a glyph that draws a solid circle.
type CircleGlyph struct{}

// DrawGlyph implements the GlyphDrawer interface.
func (CircleGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	p := make(vg.Path, 0, 3)
	p.Move(vg.Point{X: pt.X + sty.Radius, Y: pt.Y})
	p.Arc(pt, sty.Radius, 0, 2*math.Pi)
	p.Close()
	c.Fill(p)
}

// RingGlyph is a glyph that draws the outline of a circle.
type RingGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (RingGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	c.SetLineStyle(LineStyle{Color: sty.Color, Width: vg.Points(0.5)})
	p := make(vg.Path, 0, 3)
	p.Move(vg.Point{X: pt.X + sty.Radius, Y: pt.Y})
	p.Arc(pt, sty.Radius, 0, 2*math.Pi)
	p.Close()
	c.Stroke(p)
}

const (
	cosπover4 = vg.Length(.707106781202420)
	sinπover6 = vg.Length(.500000000025921)
	cosπover6 = vg.Length(.866025403769473)
)

// SquareGlyph is a glyph that draws the outline of a square.
type SquareGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (SquareGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	c.SetLineStyle(LineStyle{Color: sty.Color, Width: vg.Points(0.5)})
	x := (sty.Radius-sty.Radius*cosπover4)/2 + sty.Radius*cosπover4
	p := make(vg.Path, 0, 5)
	p.Move(vg.Point{X: pt.X - x, Y: pt.Y - x})
	p.Line(vg.Point{X: pt.X + x, Y: pt.Y - x})
	p.Line(vg.Point{X: pt.X + x, Y: pt.Y + x})
	p.Line(vg.Point{X: pt.X - x, Y: pt.Y + x})
	p.Close()
	c.Stroke(p)
}

// BoxGlyph is a glyph that draws a filled square.
type BoxGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (BoxGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	x := (sty.Radius-sty.Radius*cosπover4)/2 + sty.Radius*cosπover4
	p := make(vg.Path, 0, 5)
	p.Move(vg.Point{X: pt.X - x, Y: pt.Y - x})
	p.Line(vg.Point{X: pt.X + x, Y: pt.Y - x})
	p.Line(vg.Point{X: pt.X + x, Y: pt.Y + x})
	p.Line(vg.Point{X: pt.X - x, Y: pt.Y + x})
	p.Close()
	c.Fill(p)
}

// TriangleGlyph is a glyph that draws the outline of a triangle.
type TriangleGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (TriangleGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	c.SetLineStyle(LineStyle{Color: sty.Color, Width: vg.Points(0.5)})
	r := sty.Radius + (sty.Radius-sty.Radius*sinπover6)/2
	p := make(vg.Path, 0, 4)
	p.Move(vg.Point{X: pt.X, Y: pt.Y + r})
	p.Line(vg.Point{X: pt.X - r*cosπover6, Y: pt.Y - r*sinπover6})
	p.Line(vg.Point{X: pt.X + r*cosπover6, Y: pt.Y - r*sinπover6})
	p.Close()
	c.Stroke(p)
}

// PyramidGlyph is a glyph that draws a filled triangle.
type PyramidGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (PyramidGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	r := sty.Radius + (sty.Radius-sty.Radius*sinπover6)/2
	p := make(vg.Path, 0, 4)
	p.Move(vg.Point{X: pt.X, Y: pt.Y + r})
	p.Line(vg.Point{X: pt.X - r*cosπover6, Y: pt.Y - r*sinπover6})
	p.Line(vg.Point{X: pt.X + r*cosπover6, Y: pt.Y - r*sinπover6})
	p.Close()
	c.Fill(p)
}

// PlusGlyph is a glyph that draws a plus sign
type PlusGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (PlusGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	c.SetLineStyle(LineStyle{Color: sty.Color, Width: vg.Points(0.5)})
	r := sty.Radius
	p := make(vg.Path, 0, 2)
	p.Move(vg.Point{X: pt.X, Y: pt.Y + r})
	p.Line(vg.Point{X: pt.X, Y: pt.Y - r})
	c.Stroke(p)
	p = p[:0]
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y})
	p.Line(vg.Point{X: pt.X + r, Y: pt.Y})
	c.Stroke(p)
}

// CrossGlyph is a glyph that draws a big X.
type CrossGlyph struct{}

// DrawGlyph implements the Glyph interface.
func (CrossGlyph) DrawGlyph(c *Canvas, sty GlyphStyle, pt vg.Point) {
	c.SetLineStyle(LineStyle{Color: sty.Color, Width: vg.Points(0.5)})
	r := sty.Radius * cosπover4
	p := make(vg.Path, 0, 2)
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y - r})
	p.Line(vg.Point{X: pt.X + r, Y: pt.Y + r})
	c.Stroke(p)
	p = p[:0]
	p.Move(vg.Point{X: pt.X - r, Y: pt.Y + r})
	p.Line(vg.Point{X: pt.X + r, Y: pt.Y - r})
	c.Stroke(p)
}

// New returns a new (bounded) draw.Canvas.
func New(c vg.CanvasSizer) Canvas {
	w, h := c.Size()
	return NewCanvas(c, w, h)
}

// NewFormattedCanvas creates a new vg.CanvasWriterTo with the specified
// image format. Supported formats need to be registered by importing one or
// more of the following packages:
//
//   - gonum.org/v1/plot/vg/vgeps: provides eps
//   - gonum.org/v1/plot/vg/vgimg: provides png, jpg|jpeg, tif|tiff
//   - gonum.org/v1/plot/vg/vgpdf: provides pdf
//   - gonum.org/v1/plot/vg/vgsvg: provides svg
//   - gonum.org/v1/plot/vg/vgtex: provides tex
func NewFormattedCanvas(w, h vg.Length, format string) (vg.CanvasWriterTo, error) {
	formats.RLock()
	defer formats.RUnlock()

	for name, fn := range formats.m {
		if format != name {
			continue
		}
		return fn(w, h), nil
	}
	return nil, fmt.Errorf("unsupported format: %q", format)
}

// NewCanvas returns a new (bounded) draw.Canvas of the given size.
func NewCanvas(c vg.Canvas, w, h vg.Length) Canvas {
	return Canvas{
		Canvas: c,
		Rectangle: vg.Rectangle{
			Min: vg.Point{X: 0, Y: 0},
			Max: vg.Point{X: w, Y: h},
		},
	}
}

// Center returns the center point of the area
func (c *Canvas) Center() vg.Point {
	return vg.Point{
		X: (c.Max.X-c.Min.X)/2 + c.Min.X,
		Y: (c.Max.Y-c.Min.Y)/2 + c.Min.Y,
	}
}

// Contains returns true if the Canvas contains the point.
func (c *Canvas) Contains(p vg.Point) bool {
	return c.ContainsX(p.X) && c.ContainsY(p.Y)
}

// ContainsX returns true if the Canvas contains the
// x coordinate.
func (c *Canvas) ContainsX(x vg.Length) bool {
	return x <= c.Max.X+slop && x >= c.Min.X-slop
}

// ContainsY returns true if the Canvas contains the
// y coordinate.
func (c *Canvas) ContainsY(y vg.Length) bool {
	return y <= c.Max.Y+slop && y >= c.Min.Y-slop
}

// X returns the value of x, given in the unit range,
// in the drawing coordinates of this draw area.
// A value of 0, for example, will return the minimum
// x value of the draw area and a value of 1 will
// return the maximum.
func (c *Canvas) X(x float64) vg.Length {
	return vg.Length(x)*(c.Max.X-c.Min.X) + c.Min.X
}

// Y returns the value of x, given in the unit range,
// in the drawing coordinates of this draw area.
// A value of 0, for example, will return the minimum
// y value of the draw area and a value of 1 will
// return the maximum.
func (c *Canvas) Y(y float64) vg.Length {
	return vg.Length(y)*(c.Max.Y-c.Min.Y) + c.Min.Y
}

// Crop returns a new Canvas corresponding to the Canvas
// c with the given lengths added to the minimum
// and maximum x and y values of the Canvas's Rectangle.
// Note that cropping the right and top sides of the canvas
// requires specifying negative values of right and top.
func Crop(c Canvas, left, right, bottom, top vg.Length) Canvas {
	minpt := vg.Point{
		X: c.Min.X + left,
		Y: c.Min.Y + bottom,
	}
	maxpt := vg.Point{
		X: c.Max.X + right,
		Y: c.Max.Y + top,
	}
	return Canvas{
		Canvas:    c,
		Rectangle: vg.Rectangle{Min: minpt, Max: maxpt},
	}
}

// Tiles creates regular subcanvases from a Canvas.
type Tiles struct {
	// Cols and Rows specify the number of rows and columns of tiles.
	Cols, Rows int
	// PadTop, PadBottom, PadRight, and PadLeft specify the padding
	// on the corresponding side of each tile.
	PadTop, PadBottom, PadRight, PadLeft vg.Length
	// PadX and PadY specify the padding between columns and rows
	// of tiles respectively..
	PadX, PadY vg.Length
}

// At returns the subcanvas within c that corresponds to the
// tile at column x, row y.
func (ts Tiles) At(c Canvas, x, y int) Canvas {
	tileH := (c.Max.Y - c.Min.Y - ts.PadTop - ts.PadBottom -
		vg.Length(ts.Rows-1)*ts.PadY) / vg.Length(ts.Rows)
	tileW := (c.Max.X - c.Min.X - ts.PadLeft - ts.PadRight -
		vg.Length(ts.Cols-1)*ts.PadX) / vg.Length(ts.Cols)

	ymax := c.Max.Y - ts.PadTop - vg.Length(y)*(ts.PadY+tileH)
	ymin := ymax - tileH
	xmin := c.Min.X + ts.PadLeft + vg.Length(x)*(ts.PadX+tileW)
	xmax := xmin + tileW

	return Canvas{
		Canvas: vg.Canvas(c),
		Rectangle: vg.Rectangle{
			Min: vg.Point{X: xmin, Y: ymin},
			Max: vg.Point{X: xmax, Y: ymax},
		},
	}
}

// SetLineStyle sets the current line style
func (c *Canvas) SetLineStyle(sty LineStyle) {
	c.SetColor(sty.Color)
	c.SetLineWidth(sty.Width)
	c.SetLineDash(sty.Dashes, sty.DashOffs)
}

// StrokeLines draws a line connecting a set of points
// in the given Canvas.
func (c *Canvas) StrokeLines(sty LineStyle, lines ...[]vg.Point) {
	if len(lines) == 0 {
		return
	}

	c.SetLineStyle(sty)

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		p := make(vg.Path, 0, len(l))
		p.Move(l[0])
		for _, pt := range l[1:] {
			p.Line(pt)
		}
		c.Stroke(p)
	}
}

// StrokeLine2 draws a line between two points in the given
// Canvas.
func (c *Canvas) StrokeLine2(sty LineStyle, x0, y0, x1, y1 vg.Length) {
	c.StrokeLines(sty, []vg.Point{{X: x0, Y: y0}, {X: x1, Y: y1}})
}

// ClipLinesXY returns a slice of lines that
// represent the given line clipped in both
// X and Y directions.
func (c *Canvas) ClipLinesXY(lines ...[]vg.Point) [][]vg.Point {
	return c.ClipLinesY(c.ClipLinesX(lines...)...)
}

// ClipLinesX returns a slice of lines that
// represent the given line clipped in the
// X direction.
func (c *Canvas) ClipLinesX(lines ...[]vg.Point) (clipped [][]vg.Point) {
	lines1 := make([][]vg.Point, 0, len(lines))
	for _, line := range lines {
		ls := clipLine(isLeft, vg.Point{X: c.Max.X, Y: c.Min.Y}, vg.Point{X: -1, Y: 0}, line)
		lines1 = append(lines1, ls...)
	}
	clipped = make([][]vg.Point, 0, len(lines1))
	for _, line := range lines1 {
		ls := clipLine(isRight, vg.Point{X: c.Min.X, Y: c.Min.Y}, vg.Point{X: 1, Y: 0}, line)
		clipped = append(clipped, ls...)
	}
	return
}

// ClipLinesY returns a slice of lines that
// represent the given line clipped in the
// Y direction.
func (c *Canvas) ClipLinesY(lines ...[]vg.Point) (clipped [][]vg.Point) {
	lines1 := make([][]vg.Point, 0, len(lines))
	for _, line := range lines {
		ls := clipLine(isAbove, vg.Point{X: c.Min.X, Y: c.Min.Y}, vg.Point{X: 0, Y: -1}, line)
		lines1 = append(lines1, ls...)
	}
	clipped = make([][]vg.Point, 0, len(lines1))
	for _, line := range lines1 {
		ls := clipLine(isBelow, vg.Point{X: c.Min.X, Y: c.Max.Y}, vg.Point{X: 0, Y: 1}, line)
		clipped = append(clipped, ls...)
	}
	return
}

// clipLine performs clipping of a line by a single
// clipping line specified by the norm, clip point,
// and in function.
func clipLine(in func(vg.Point, vg.Point) bool, clip, norm vg.Point, pts []vg.Point) (lines [][]vg.Point) {
	l := make([]vg.Point, 0, len(pts))
	for i := 1; i < len(pts); i++ {
		cur, next := pts[i-1], pts[i]
		curIn, nextIn := in(cur, clip), in(next, clip)
		switch {
		case curIn && nextIn:
			l = append(l, cur)

		case curIn && !nextIn:
			l = append(l, cur, isect(cur, next, clip, norm))
			lines = append(lines, l)
			l = []vg.Point{}

		case !curIn && !nextIn:
			// do nothing

		default: // !curIn && nextIn
			l = append(l, isect(cur, next, clip, norm))
		}
		if nextIn && i == len(pts)-1 {
			l = append(l, next)
		}
	}
	if len(l) > 1 {
		lines = append(lines, l)
	}
	return
}

// FillPolygon fills a polygon with the given color.
func (c *Canvas) FillPolygon(clr color.Color, pts []vg.Point) {
	if len(pts) == 0 {
		return
	}

	c.SetColor(clr)
	p := make(vg.Path, 0, len(pts)+1)
	p.Move(pts[0])
	for _, pt := range pts[1:] {
		p.Line(pt)
	}
	p.Close()
	c.Fill(p)
}

// ClipPolygonXY returns a slice of lines that
// represent the given polygon clipped in both
// X and Y directions.
func (c *Canvas) ClipPolygonXY(pts []vg.Point) []vg.Point {
	return c.ClipPolygonY(c.ClipPolygonX(pts))
}

// ClipPolygonX returns a slice of lines that
// represent the given polygon clipped in the
// X direction.
func (c *Canvas) ClipPolygonX(pts []vg.Point) []vg.Point {
	return clipPoly(isLeft, vg.Point{X: c.Max.X, Y: c.Min.Y}, vg.Point{X: -1, Y: 0},
		clipPoly(isRight, vg.Point{X: c.Min.X, Y: c.Min.Y}, vg.Point{X: 1, Y: 0}, pts))
}

// ClipPolygonY returns a slice of lines that
// represent the given polygon clipped in the
// Y direction.
func (c *Canvas) ClipPolygonY(pts []vg.Point) []vg.Point {
	return clipPoly(isBelow, vg.Point{X: c.Min.X, Y: c.Max.Y}, vg.Point{X: 0, Y: 1},
		clipPoly(isAbove, vg.Point{X: c.Min.X, Y: c.Min.Y}, vg.Point{X: 0, Y: -1}, pts))
}

// clipPoly performs clipping of a polygon by a single
// clipping line specified by the norm, clip point,
// and in function.
func clipPoly(in func(vg.Point, vg.Point) bool, clip, norm vg.Point, pts []vg.Point) (clipped []vg.Point) {
	clipped = make([]vg.Point, 0, len(pts))
	for i := 0; i < len(pts); i++ {
		j := i + 1
		if i == len(pts)-1 {
			j = 0
		}
		cur, next := pts[i], pts[j]
		curIn, nextIn := in(cur, clip), in(next, clip)
		switch {
		case curIn && nextIn:
			clipped = append(clipped, cur)

		case curIn && !nextIn:
			clipped = append(clipped, cur, isect(cur, next, clip, norm))

		case !curIn && !nextIn:
			// do nothing

		default: // !curIn && nextIn
			clipped = append(clipped, isect(cur, next, clip, norm))
		}
	}
	n := len(clipped)
	return clipped[:n:n]
}

// slop is some slop for floating point equality
const slop = 3e-8 // ≈ √1⁻¹⁵

func isLeft(p, clip vg.Point) bool {
	return p.X <= clip.X+slop
}

func isRight(p, clip vg.Point) bool {
	return p.X >= clip.X-slop
}

func isBelow(p, clip vg.Point) bool {
	return p.Y <= clip.Y+slop
}

func isAbove(p, clip vg.Point) bool {
	return p.Y >= clip.Y-slop
}

// isect returns the intersection of a line p0→p1 with the
// clipping line specified by the clip point and normal.
func isect(p0, p1, clip, norm vg.Point) vg.Point {
	// t = (norm · (p0 - clip)) / (norm · (p0 - p1))
	t := p0.Sub(clip).Dot(norm) / p0.Sub(p1).Dot(norm)

	// p = p0 + t*(p1 - p0)
	return p1.Sub(p0).Scale(t).Add(p0)
}

// FillText fills lines of text in the draw area.
// pt specifies the location where the text is to be drawn.
func (c *Canvas) FillText(sty TextStyle, pt vg.Point, txt string) {
	sty.Handler.Draw(c, txt, sty, pt)
}
