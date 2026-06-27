// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Function implements the Plotter interface,
// drawing a line for the given function.
type Function struct {
	F func(x float64) (y float64)

	// XMin and XMax specify the range
	// of x values to pass to F.
	XMin, XMax float64

	Samples int

	draw.LineStyle
}

// NewFunction returns a Function that plots F using
// the default line style with 50 samples.
func NewFunction(f func(float64) float64) *Function {
	return &Function{
		F:         f,
		Samples:   50,
		LineStyle: DefaultLineStyle,
	}
}

// Plot implements the Plotter interface, drawing a line
// that connects each point in the Line.
func (f *Function) Plot(c draw.Canvas, p *plot.Plot) {
	trX, trY := p.Transforms(&c)

	min, max := f.XMin, f.XMax
	if min == 0 && max == 0 {
		min = p.X.Min
		max = p.X.Max
	}
	d := (max - min) / float64(f.Samples-1)
	line := make([]vg.Point, f.Samples)
	for i := range line {
		x := min + float64(i)*d
		line[i].X = trX(x)
		line[i].Y = trY(f.F(x))
	}
	c.StrokeLines(f.LineStyle, c.ClipLinesXY(line)...)
}

// Thumbnail draws a line in the given style down the
// center of a DrawArea as a thumbnail representation
// of the LineStyle of the function.
func (f Function) Thumbnail(c *draw.Canvas) {
	y := c.Center().Y
	c.StrokeLine2(f.LineStyle, c.Min.X, y, c.Max.X, y)
}
