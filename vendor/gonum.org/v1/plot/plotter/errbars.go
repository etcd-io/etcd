// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"math"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// DefaultCapWidth is the default width of error bar caps.
var DefaultCapWidth = vg.Points(5)

// YErrorBars implements the plot.Plotter, plot.DataRanger,
// and plot.GlyphBoxer interfaces, drawing vertical error
// bars, denoting error in Y values.
type YErrorBars struct {
	XYs

	// YErrors is a copy of the Y errors for each point.
	YErrors

	// LineStyle is the style used to draw the error bars.
	draw.LineStyle

	// CapWidth is the width of the caps drawn at the top
	// of each error bar.
	CapWidth vg.Length
}

// NewYErrorBars returns a new YErrorBars plotter, or an error on failure.
// The error values from the YErrorer interface are interpreted as relative
// to the corresponding Y value. The errors for a given Y value are computed
// by taking the absolute value of the error returned by the YErrorer
// and subtracting the first and adding the second to the Y value.
func NewYErrorBars(yerrs interface {
	XYer
	YErrorer
}) (*YErrorBars, error) {

	errors := make(YErrors, yerrs.Len())
	for i := range errors {
		errors[i].Low, errors[i].High = yerrs.YError(i)
		if err := CheckFloats(errors[i].Low, errors[i].High); err != nil {
			return nil, err
		}
	}
	xys, err := CopyXYs(yerrs)
	if err != nil {
		return nil, err
	}

	return &YErrorBars{
		XYs:       xys,
		YErrors:   errors,
		LineStyle: DefaultLineStyle,
		CapWidth:  DefaultCapWidth,
	}, nil
}

// Plot implements the Plotter interface, drawing labels.
func (e *YErrorBars) Plot(c draw.Canvas, p *plot.Plot) {
	trX, trY := p.Transforms(&c)
	for i, err := range e.YErrors {
		x := trX(e.XYs[i].X)
		ylow := trY(e.XYs[i].Y - math.Abs(err.Low))
		yhigh := trY(e.XYs[i].Y + math.Abs(err.High))

		bar := c.ClipLinesY([]vg.Point{{X: x, Y: ylow}, {X: x, Y: yhigh}})
		c.StrokeLines(e.LineStyle, bar...)
		e.drawCap(&c, x, ylow)
		e.drawCap(&c, x, yhigh)
	}
}

// drawCap draws the cap if it is not clipped.
func (e *YErrorBars) drawCap(c *draw.Canvas, x, y vg.Length) {
	if !c.Contains(vg.Point{X: x, Y: y}) {
		return
	}
	c.StrokeLine2(e.LineStyle, x-e.CapWidth/2, y, x+e.CapWidth/2, y)
}

// DataRange implements the plot.DataRanger interface.
func (e *YErrorBars) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin, xmax = Range(XValues{e})
	ymin = math.Inf(1)
	ymax = math.Inf(-1)
	for i, err := range e.YErrors {
		y := e.XYs[i].Y
		ylow := y - math.Abs(err.Low)
		yhigh := y + math.Abs(err.High)
		ymin = math.Min(math.Min(math.Min(ymin, y), ylow), yhigh)
		ymax = math.Max(math.Max(math.Max(ymax, y), ylow), yhigh)
	}
	return
}

// GlyphBoxes implements the plot.GlyphBoxer interface.
func (e *YErrorBars) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	rect := vg.Rectangle{
		Min: vg.Point{
			X: -e.CapWidth / 2,
			Y: -e.LineStyle.Width / 2,
		},
		Max: vg.Point{
			X: +e.CapWidth / 2,
			Y: +e.LineStyle.Width / 2,
		},
	}
	var bs []plot.GlyphBox
	for i, err := range e.YErrors {
		x := plt.X.Norm(e.XYs[i].X)
		y := e.XYs[i].Y
		bs = append(bs,
			plot.GlyphBox{X: x, Y: plt.Y.Norm(y - err.Low), Rectangle: rect},
			plot.GlyphBox{X: x, Y: plt.Y.Norm(y + err.High), Rectangle: rect})
	}
	return bs
}

// XErrorBars implements the plot.Plotter, plot.DataRanger,
// and plot.GlyphBoxer interfaces, drawing horizontal error
// bars, denoting error in Y values.
type XErrorBars struct {
	XYs

	// XErrors is a copy of the X errors for each point.
	XErrors

	// LineStyle is the style used to draw the error bars.
	draw.LineStyle

	// CapWidth is the width of the caps drawn at the top
	// of each error bar.
	CapWidth vg.Length
}

// Returns a new XErrorBars plotter, or an error on failure. The error values
// from the XErrorer interface are interpreted as relative to the corresponding
// X value. The errors for a given X value are computed by taking the absolute
// value of the error returned by the XErrorer and subtracting the first and
// adding the second to the X value.
func NewXErrorBars(xerrs interface {
	XYer
	XErrorer
}) (*XErrorBars, error) {

	errors := make(XErrors, xerrs.Len())
	for i := range errors {
		errors[i].Low, errors[i].High = xerrs.XError(i)
		if err := CheckFloats(errors[i].Low, errors[i].High); err != nil {
			return nil, err
		}
	}
	xys, err := CopyXYs(xerrs)
	if err != nil {
		return nil, err
	}

	return &XErrorBars{
		XYs:       xys,
		XErrors:   errors,
		LineStyle: DefaultLineStyle,
		CapWidth:  DefaultCapWidth,
	}, nil
}

// Plot implements the Plotter interface, drawing labels.
func (e *XErrorBars) Plot(c draw.Canvas, p *plot.Plot) {
	trX, trY := p.Transforms(&c)
	for i, err := range e.XErrors {
		y := trY(e.XYs[i].Y)
		xlow := trX(e.XYs[i].X - math.Abs(err.Low))
		xhigh := trX(e.XYs[i].X + math.Abs(err.High))

		bar := c.ClipLinesX([]vg.Point{{X: xlow, Y: y}, {X: xhigh, Y: y}})
		c.StrokeLines(e.LineStyle, bar...)
		e.drawCap(&c, xlow, y)
		e.drawCap(&c, xhigh, y)
	}
}

// drawCap draws the cap if it is not clipped.
func (e *XErrorBars) drawCap(c *draw.Canvas, x, y vg.Length) {
	if !c.Contains(vg.Point{X: x, Y: y}) {
		return
	}
	c.StrokeLine2(e.LineStyle, x, y-e.CapWidth/2, x, y+e.CapWidth/2)
}

// DataRange implements the plot.DataRanger interface.
func (e *XErrorBars) DataRange() (xmin, xmax, ymin, ymax float64) {
	ymin, ymax = Range(YValues{e})
	xmin = math.Inf(1)
	xmax = math.Inf(-1)
	for i, err := range e.XErrors {
		x := e.XYs[i].X
		xlow := x - math.Abs(err.Low)
		xhigh := x + math.Abs(err.High)
		xmin = math.Min(math.Min(math.Min(xmin, x), xlow), xhigh)
		xmax = math.Max(math.Max(math.Max(xmax, x), xlow), xhigh)
	}
	return
}

// GlyphBoxes implements the plot.GlyphBoxer interface.
func (e *XErrorBars) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	rect := vg.Rectangle{
		Min: vg.Point{
			X: -e.LineStyle.Width / 2,
			Y: -e.CapWidth / 2,
		},
		Max: vg.Point{
			X: +e.LineStyle.Width / 2,
			Y: +e.CapWidth / 2,
		},
	}
	var bs []plot.GlyphBox
	for i, err := range e.XErrors {
		x := e.XYs[i].X
		y := plt.Y.Norm(e.XYs[i].Y)
		bs = append(bs,
			plot.GlyphBox{X: plt.X.Norm(x - err.Low), Y: y, Rectangle: rect},
			plot.GlyphBox{X: plt.X.Norm(x + err.High), Y: y, Rectangle: rect})
	}
	return bs
}
