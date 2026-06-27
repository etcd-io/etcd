// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image/color"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// StepKind specifies a form of a connection of two consecutive points.
type StepKind int

const (
	// NoStep connects two points by simple line
	NoStep StepKind = iota

	// PreStep connects two points by following lines: vertical, horizontal.
	PreStep

	// MidStep connects two points by following lines: horizontal, vertical, horizontal.
	// Vertical line is placed in the middle of the interval.
	MidStep

	// PostStep connects two points by following lines: horizontal, vertical.
	PostStep
)

// Line implements the Plotter interface, drawing a line.
type Line struct {
	// XYs is a copy of the points for this line.
	XYs

	// StepStyle is the kind of the step line.
	StepStyle StepKind

	// LineStyle is the style of the line connecting the points.
	// Use zero width to disable lines.
	draw.LineStyle

	// FillColor is the color to fill the area below the plot.
	// Use nil to disable the filling. This is the default.
	FillColor color.Color
}

// NewLine returns a Line that uses the default line style and
// does not draw glyphs.
func NewLine(xys XYer) (*Line, error) {
	data, err := CopyXYs(xys)
	if err != nil {
		return nil, err
	}
	return &Line{
		XYs:       data,
		LineStyle: DefaultLineStyle,
	}, nil
}

// Plot draws the Line, implementing the plot.Plotter interface.
func (pts *Line) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	ps := make([]vg.Point, len(pts.XYs))

	for i, p := range pts.XYs {
		ps[i].X = trX(p.X)
		ps[i].Y = trY(p.Y)
	}

	if pts.FillColor != nil && len(ps) > 0 {
		minY := trY(plt.Y.Min)
		fillPoly := []vg.Point{{X: ps[0].X, Y: minY}}
		switch pts.StepStyle {
		case PreStep:
			fillPoly = append(fillPoly, ps[1:]...)
		case PostStep:
			fillPoly = append(fillPoly, ps[:len(ps)-1]...)
		default:
			fillPoly = append(fillPoly, ps...)
		}
		fillPoly = append(fillPoly, vg.Point{X: ps[len(ps)-1].X, Y: minY})
		fillPoly = c.ClipPolygonXY(fillPoly)
		if len(fillPoly) > 0 {
			c.SetColor(pts.FillColor)
			var pa vg.Path
			prev := fillPoly[0]
			pa.Move(prev)
			for _, pt := range fillPoly[1:] {
				switch pts.StepStyle {
				case NoStep:
					pa.Line(pt)
				case PreStep:
					pa.Line(vg.Point{X: prev.X, Y: pt.Y})
					pa.Line(pt)
				case MidStep:
					pa.Line(vg.Point{X: (prev.X + pt.X) / 2, Y: prev.Y})
					pa.Line(vg.Point{X: (prev.X + pt.X) / 2, Y: pt.Y})
					pa.Line(pt)
				case PostStep:
					pa.Line(vg.Point{X: pt.X, Y: prev.Y})
					pa.Line(pt)
				}
				prev = pt
			}
			pa.Close()
			c.Fill(pa)
		}
	}

	lines := c.ClipLinesXY(ps)
	if pts.LineStyle.Width != 0 && len(lines) != 0 {
		c.SetLineStyle(pts.LineStyle)
		for _, l := range lines {
			if len(l) == 0 {
				continue
			}
			var p vg.Path
			prev := l[0]
			p.Move(prev)
			for _, pt := range l[1:] {
				switch pts.StepStyle {
				case PreStep:
					p.Line(vg.Point{X: prev.X, Y: pt.Y})
				case MidStep:
					p.Line(vg.Point{X: (prev.X + pt.X) / 2, Y: prev.Y})
					p.Line(vg.Point{X: (prev.X + pt.X) / 2, Y: pt.Y})
				case PostStep:
					p.Line(vg.Point{X: pt.X, Y: prev.Y})
				}
				p.Line(pt)
				prev = pt
			}
			c.Stroke(p)
		}
	}
}

// DataRange returns the minimum and maximum
// x and y values, implementing the plot.DataRanger interface.
func (pts *Line) DataRange() (xmin, xmax, ymin, ymax float64) {
	return XYRange(pts)
}

// GlyphBoxes implements the plot.GlyphBoxer interface.
func (pts *Line) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	r := 0.5 * pts.LineStyle.Width
	rect := vg.Rectangle{
		Min: vg.Point{
			X: -r,
			Y: -r,
		},
		Max: vg.Point{
			X: +r,
			Y: +r,
		},
	}

	bs := make([]plot.GlyphBox, pts.XYs.Len())
	for i := range bs {
		x, y := pts.XY(i)
		bs[i] = plot.GlyphBox{
			X:         plt.X.Norm(x),
			Y:         plt.Y.Norm(y),
			Rectangle: rect,
		}
	}
	return bs
}

// Thumbnail returns the thumbnail for the Line, implementing the plot.Thumbnailer interface.
func (pts *Line) Thumbnail(c *draw.Canvas) {
	if pts.FillColor != nil {
		var topY vg.Length
		if pts.LineStyle.Width == 0 {
			topY = c.Max.Y
		} else {
			topY = (c.Min.Y + c.Max.Y) / 2
		}
		points := []vg.Point{
			{X: c.Min.X, Y: c.Min.Y},
			{X: c.Min.X, Y: topY},
			{X: c.Max.X, Y: topY},
			{X: c.Max.X, Y: c.Min.Y},
		}
		poly := c.ClipPolygonY(points)
		c.FillPolygon(pts.FillColor, poly)
	}

	if pts.LineStyle.Width != 0 {
		y := c.Center().Y
		c.StrokeLine2(pts.LineStyle, c.Min.X, y, c.Max.X, y)
	}
}

// NewLinePoints returns both a Line and a
// Points for the given point data.
func NewLinePoints(xys XYer) (*Line, *Scatter, error) {
	s, err := NewScatter(xys)
	if err != nil {
		return nil, nil, err
	}
	l := &Line{
		XYs:       s.XYs,
		LineStyle: DefaultLineStyle,
	}
	return l, s, nil
}
