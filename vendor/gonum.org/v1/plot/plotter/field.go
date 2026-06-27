// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"math"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// FieldXY describes a two dimensional vector field where the
// X and Y coordinates are arranged on a rectangular grid.
type FieldXY interface {
	// Dims returns the dimensions of the grid.
	Dims() (c, r int)

	// Vector returns the value of a vector field at (c, r).
	// It will panic if c or r are out of bounds for the field.
	Vector(c, r int) XY

	// X returns the coordinate for the column at the index c.
	// It will panic if c is out of bounds for the grid.
	X(c int) float64

	// Y returns the coordinate for the row at the index r.
	// It will panic if r is out of bounds for the grid.
	Y(r int) float64
}

// Field implements the Plotter interface, drawing
// a vector field of the values in the FieldXY field.
type Field struct {
	FieldXY FieldXY

	// DrawGlyph is the user hook to draw a field
	// vector glyph. The function should draw a unit
	// vector to (1, 0) on the vg.Canvas, c with the
	// sty LineStyle. The Field plotter will rotate
	// and scale the unit vector appropriately.
	// If the magnitude of v is zero, no scaling or
	// rotation is performed.
	//
	// The direction and magnitude of v can be used
	// to determine properties of the glyph drawing
	// but should not be used to determine size or
	// directions of the glyph.
	//
	// If DrawGlyph is nil, a simple arrow will be
	// drawn.
	DrawGlyph func(c vg.Canvas, sty draw.LineStyle, v XY)

	// LineStyle is the style of the line used to
	// render vectors when DrawGlyph is nil.
	// Otherwise it is passed to DrawGlyph.
	LineStyle draw.LineStyle

	// max define the dynamic range of the field.
	max float64
}

// NewField creates a new vector field plotter.
func NewField(f FieldXY) *Field {
	max := math.Inf(-1)
	c, r := f.Dims()
	for i := 0; i < c; i++ {
		for j := 0; j < r; j++ {
			v := f.Vector(i, j)
			d := math.Hypot(v.X, v.Y)
			if math.IsNaN(d) {
				continue
			}
			max = math.Max(max, d)
		}
	}

	return &Field{
		FieldXY:   f,
		LineStyle: DefaultLineStyle,
		max:       max,
	}
}

// Plot implements the Plot method of the plot.Plotter interface.
func (f *Field) Plot(c draw.Canvas, plt *plot.Plot) {
	c.Push()
	defer c.Pop()
	c.SetLineStyle(f.LineStyle)

	trX, trY := plt.Transforms(&c)

	cols, rows := f.FieldXY.Dims()
	for i := 0; i < cols; i++ {
		var right, left float64
		switch i {
		case 0:
			if cols == 1 {
				right = 0.5
			} else {
				right = (f.FieldXY.X(1) - f.FieldXY.X(0)) / 2
			}
			left = -right
		case cols - 1:
			right = (f.FieldXY.X(cols-1) - f.FieldXY.X(cols-2)) / 2
			left = -right
		default:
			right = (f.FieldXY.X(i+1) - f.FieldXY.X(i)) / 2
			left = -(f.FieldXY.X(i) - f.FieldXY.X(i-1)) / 2
		}

		for j := 0; j < rows; j++ {
			var up, down float64
			switch j {
			case 0:
				if rows == 1 {
					up = 0.5
				} else {
					up = (f.FieldXY.Y(1) - f.FieldXY.Y(0)) / 2
				}
				down = -up
			case rows - 1:
				up = (f.FieldXY.Y(rows-1) - f.FieldXY.Y(rows-2)) / 2
				down = -up
			default:
				up = (f.FieldXY.Y(j+1) - f.FieldXY.Y(j)) / 2
				down = -(f.FieldXY.Y(j) - f.FieldXY.Y(j-1)) / 2
			}

			x, y := trX(f.FieldXY.X(i)+left), trY(f.FieldXY.Y(j)+down)
			dx, dy := trX(f.FieldXY.X(i)+right), trY(f.FieldXY.Y(j)+up)

			if !c.Contains(vg.Point{X: x, Y: y}) || !c.Contains(vg.Point{X: dx, Y: dy}) {
				continue
			}

			c.Push()
			c.Translate(vg.Point{X: (x + dx) / 2, Y: (y + dy) / 2})

			v := f.FieldXY.Vector(i, j)
			s := math.Hypot(v.X, v.Y) / (2 * f.max)
			// Do not scale when the vector is zero, otherwise the
			// user cannot render special-case glyphs for that case.
			if s != 0 {
				c.Rotate(math.Atan2(v.Y, v.X))
				c.Scale(s*float64(dx-x), s*float64(dy-y))
			}
			v.X /= f.max
			v.Y /= f.max

			if f.DrawGlyph == nil {
				drawVector(c, v)
			} else {
				f.DrawGlyph(c, f.LineStyle, v)
			}
			c.Pop()
		}
	}
}

func drawVector(c vg.Canvas, v XY) {
	if math.Hypot(v.X, v.Y) == 0 {
		return
	}
	// TODO(kortschak): Improve this arrow.
	var pa vg.Path
	pa.Move(vg.Point{})
	pa.Line(vg.Point{X: 1, Y: 0})
	pa.Close()
	c.Stroke(pa)
}

// DataRange implements the DataRange method
// of the plot.DataRanger interface.
func (f *Field) DataRange() (xmin, xmax, ymin, ymax float64) {
	c, r := f.FieldXY.Dims()
	switch c {
	case 1: // Make a unit length when there is no neighbour.
		xmax = f.FieldXY.X(0) + 0.5
		xmin = f.FieldXY.X(0) - 0.5
	default:
		xmax = f.FieldXY.X(c-1) + (f.FieldXY.X(c-1)-f.FieldXY.X(c-2))/2
		xmin = f.FieldXY.X(0) - (f.FieldXY.X(1)-f.FieldXY.X(0))/2
	}
	switch r {
	case 1: // Make a unit length when there is no neighbour.
		ymax = f.FieldXY.Y(0) + 0.5
		ymin = f.FieldXY.Y(0) - 0.5
	default:
		ymax = f.FieldXY.Y(r-1) + (f.FieldXY.Y(r-1)-f.FieldXY.Y(r-2))/2
		ymin = f.FieldXY.Y(0) - (f.FieldXY.Y(1)-f.FieldXY.Y(0))/2
	}
	return xmin, xmax, ymin, ymax
}

// GlyphBoxes implements the GlyphBoxes method
// of the plot.GlyphBoxer interface.
func (f *Field) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	c, r := f.FieldXY.Dims()
	b := make([]plot.GlyphBox, 0, r*c)
	for i := 0; i < c; i++ {
		for j := 0; j < r; j++ {
			b = append(b, plot.GlyphBox{
				X: plt.X.Norm(f.FieldXY.X(i)),
				Y: plt.Y.Norm(f.FieldXY.Y(j)),
				Rectangle: vg.Rectangle{
					Min: vg.Point{X: -5, Y: -5},
					Max: vg.Point{X: +5, Y: +5},
				},
			})
		}
	}
	return b
}
