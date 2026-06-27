// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Scatter implements the Plotter interface, drawing
// a glyph for each of a set of points.
type Scatter struct {
	// XYs is a copy of the points for this scatter.
	XYs

	// GlyphStyleFunc, if not nil, specifies GlyphStyles
	// for individual points
	GlyphStyleFunc func(int) draw.GlyphStyle

	// GlyphStyle is the style of the glyphs drawn
	// at each point.
	draw.GlyphStyle
}

// NewScatter returns a Scatter that uses the
// default glyph style.
func NewScatter(xys XYer) (*Scatter, error) {
	data, err := CopyXYs(xys)
	if err != nil {
		return nil, err
	}
	return &Scatter{
		XYs:        data,
		GlyphStyle: DefaultGlyphStyle,
	}, err
}

// Plot draws the Scatter, implementing the plot.Plotter
// interface.
func (pts *Scatter) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	glyph := func(i int) draw.GlyphStyle { return pts.GlyphStyle }
	if pts.GlyphStyleFunc != nil {
		glyph = pts.GlyphStyleFunc
	}
	for i, p := range pts.XYs {
		c.DrawGlyph(glyph(i), vg.Point{X: trX(p.X), Y: trY(p.Y)})
	}
}

// DataRange returns the minimum and maximum
// x and y values, implementing the plot.DataRanger
// interface.
func (pts *Scatter) DataRange() (xmin, xmax, ymin, ymax float64) {
	return XYRange(pts)
}

// GlyphBoxes returns a slice of plot.GlyphBoxes,
// implementing the plot.GlyphBoxer interface.
func (pts *Scatter) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	glyph := func(i int) draw.GlyphStyle { return pts.GlyphStyle }
	if pts.GlyphStyleFunc != nil {
		glyph = pts.GlyphStyleFunc
	}
	bs := make([]plot.GlyphBox, len(pts.XYs))
	for i, p := range pts.XYs {
		bs[i].X = plt.X.Norm(p.X)
		bs[i].Y = plt.Y.Norm(p.Y)
		r := glyph(i).Radius
		bs[i].Rectangle = vg.Rectangle{
			Min: vg.Point{X: -r, Y: -r},
			Max: vg.Point{X: +r, Y: +r},
		}
	}
	return bs
}

// Thumbnail the thumbnail for the Scatter,
// implementing the plot.Thumbnailer interface.
func (pts *Scatter) Thumbnail(c *draw.Canvas) {
	c.DrawGlyph(pts.GlyphStyle, c.Center())
}
