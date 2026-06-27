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

var (
	// DefaultQuartMedianStyle is a fat dot.
	DefaultQuartMedianStyle = draw.GlyphStyle{
		Color:  color.Black,
		Radius: vg.Points(1.5),
		Shape:  draw.CircleGlyph{},
	}

	// DefaultQuartWhiskerStyle is a hairline.
	DefaultQuartWhiskerStyle = draw.LineStyle{
		Color:    color.Black,
		Width:    vg.Points(0.5),
		Dashes:   []vg.Length{},
		DashOffs: 0,
	}
)

// QuartPlot implements the Plotter interface, drawing
// a plot to represent the distribution of values.
//
// This style of the plot appears in Tufte's "The Visual
// Display of Quantitative Information".
type QuartPlot struct {
	fiveStatPlot

	// Offset is added to the x location of each plot.
	// When the Offset is zero, the plot is drawn
	// centered at its x location.
	Offset vg.Length

	// MedianStyle is the line style for the median point.
	MedianStyle draw.GlyphStyle

	// WhiskerStyle is the line style used to draw the
	// whiskers.
	WhiskerStyle draw.LineStyle

	// Horizontal dictates whether the QuartPlot should be in the vertical
	// (default) or horizontal direction.
	Horizontal bool
}

// NewQuartPlot returns a new QuartPlot that represents
// the distribution of the given values.
//
// An error is returned if the plot is created with
// no values.
//
// The fence values are 1.5x the interquartile before
// the first quartile and after the third quartile.  Any
// value that is outside of the fences are drawn as
// Outside points.  The adjacent values (to which the
// whiskers stretch) are the minimum and maximum
// values that are not outside the fences.
func NewQuartPlot(loc float64, values Valuer) (*QuartPlot, error) {
	b := new(QuartPlot)
	var err error
	if b.fiveStatPlot, err = newFiveStat(0, loc, values); err != nil {
		return nil, err
	}

	b.MedianStyle = DefaultQuartMedianStyle
	b.WhiskerStyle = DefaultQuartWhiskerStyle

	return b, err
}

// Plot draws the QuartPlot on Canvas c and Plot plt.
func (b *QuartPlot) Plot(c draw.Canvas, plt *plot.Plot) {
	if b.Horizontal {
		b := &horizQuartPlot{b}
		b.Plot(c, plt)
		return
	}

	trX, trY := plt.Transforms(&c)
	x := trX(b.Location)
	if !c.ContainsX(x) {
		return
	}
	x += b.Offset

	med := vg.Point{X: x, Y: trY(b.Median)}
	q1 := trY(b.Quartile1)
	q3 := trY(b.Quartile3)
	aLow := trY(b.AdjLow)
	aHigh := trY(b.AdjHigh)

	c.StrokeLine2(b.WhiskerStyle, x, aHigh, x, q3)
	if c.ContainsY(med.Y) {
		c.DrawGlyphNoClip(b.MedianStyle, med)
	}
	c.StrokeLine2(b.WhiskerStyle, x, aLow, x, q1)

	ostyle := b.MedianStyle
	ostyle.Radius = b.MedianStyle.Radius / 2
	for _, out := range b.Outside {
		y := trY(b.Value(out))
		if c.ContainsY(y) {
			c.DrawGlyphNoClip(ostyle, vg.Point{X: x, Y: y})
		}
	}
}

// DataRange returns the minimum and maximum x
// and y values, implementing the plot.DataRanger
// interface.
func (b *QuartPlot) DataRange() (float64, float64, float64, float64) {
	if b.Horizontal {
		b := &horizQuartPlot{b}
		return b.DataRange()
	}
	return b.Location, b.Location, b.Min, b.Max
}

// GlyphBoxes returns a slice of GlyphBoxes for the plot,
// implementing the plot.GlyphBoxer interface.
func (b *QuartPlot) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	if b.Horizontal {
		b := &horizQuartPlot{b}
		return b.GlyphBoxes(plt)
	}

	bs := make([]plot.GlyphBox, len(b.Outside)+1)

	ostyle := b.MedianStyle
	ostyle.Radius = b.MedianStyle.Radius / 2
	for i, out := range b.Outside {
		bs[i].X = plt.X.Norm(b.Location)
		bs[i].Y = plt.Y.Norm(b.Value(out))
		bs[i].Rectangle = ostyle.Rectangle()
		bs[i].Rectangle.Min.X += b.Offset
	}
	bs[len(bs)-1].X = plt.X.Norm(b.Location)
	bs[len(bs)-1].Y = plt.Y.Norm(b.Median)
	bs[len(bs)-1].Rectangle = b.MedianStyle.Rectangle()
	bs[len(bs)-1].Rectangle.Min.X += b.Offset
	return bs
}

// OutsideLabels returns a *Labels that will plot
// a label for each of the outside points.  The
// labels are assumed to correspond to the
// points used to create the plot.
func (b *QuartPlot) OutsideLabels(labels Labeller) (*Labels, error) {
	if b.Horizontal {
		b := &horizQuartPlot{b}
		return b.OutsideLabels(labels)
	}
	strs := make([]string, len(b.Outside))
	for i, out := range b.Outside {
		strs[i] = labels.Label(out)
	}
	o := quartPlotOutsideLabels{b, strs}
	ls, err := NewLabels(o)
	if err != nil {
		return nil, err
	}
	off := 0.5 * b.MedianStyle.Radius
	ls.Offset = ls.Offset.Add(vg.Point{X: off, Y: off})
	return ls, nil
}

type quartPlotOutsideLabels struct {
	qp     *QuartPlot
	labels []string
}

func (o quartPlotOutsideLabels) Len() int {
	return len(o.qp.Outside)
}

func (o quartPlotOutsideLabels) XY(i int) (float64, float64) {
	return o.qp.Location, o.qp.Value(o.qp.Outside[i])
}

func (o quartPlotOutsideLabels) Label(i int) string {
	return o.labels[i]
}

// horizQuartPlot is like a regular QuartPlot, however,
// it draws horizontally instead of Vertically.
type horizQuartPlot struct{ *QuartPlot }

func (b horizQuartPlot) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	y := trY(b.Location)
	if !c.ContainsY(y) {
		return
	}
	y += b.Offset

	med := vg.Point{X: trX(b.Median), Y: y}
	q1 := trX(b.Quartile1)
	q3 := trX(b.Quartile3)
	aLow := trX(b.AdjLow)
	aHigh := trX(b.AdjHigh)

	c.StrokeLine2(b.WhiskerStyle, aHigh, y, q3, y)
	if c.ContainsX(med.X) {
		c.DrawGlyphNoClip(b.MedianStyle, med)
	}
	c.StrokeLine2(b.WhiskerStyle, aLow, y, q1, y)

	ostyle := b.MedianStyle
	ostyle.Radius = b.MedianStyle.Radius / 2
	for _, out := range b.Outside {
		x := trX(b.Value(out))
		if c.ContainsX(x) {
			c.DrawGlyphNoClip(ostyle, vg.Point{X: x, Y: y})
		}
	}
}

// DataRange returns the minimum and maximum x
// and y values, implementing the plot.DataRanger
// interface.
func (b horizQuartPlot) DataRange() (float64, float64, float64, float64) {
	return b.Min, b.Max, b.Location, b.Location
}

// GlyphBoxes returns a slice of GlyphBoxes for the plot,
// implementing the plot.GlyphBoxer interface.
func (b horizQuartPlot) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	bs := make([]plot.GlyphBox, len(b.Outside)+1)

	ostyle := b.MedianStyle
	ostyle.Radius = b.MedianStyle.Radius / 2
	for i, out := range b.Outside {
		bs[i].X = plt.X.Norm(b.Value(out))
		bs[i].Y = plt.Y.Norm(b.Location)
		bs[i].Rectangle = ostyle.Rectangle()
		bs[i].Rectangle.Min.Y += b.Offset
	}
	bs[len(bs)-1].X = plt.X.Norm(b.Median)
	bs[len(bs)-1].Y = plt.Y.Norm(b.Location)
	bs[len(bs)-1].Rectangle = b.MedianStyle.Rectangle()
	bs[len(bs)-1].Rectangle.Min.Y += b.Offset
	return bs
}

// OutsideLabels returns a *Labels that will plot
// a label for each of the outside points.  The
// labels are assumed to correspond to the
// points used to create the plot.
func (b *horizQuartPlot) OutsideLabels(labels Labeller) (*Labels, error) {
	strs := make([]string, len(b.Outside))
	for i, out := range b.Outside {
		strs[i] = labels.Label(out)
	}
	o := horizQuartPlotOutsideLabels{
		quartPlotOutsideLabels{b.QuartPlot, strs},
	}
	ls, err := NewLabels(o)
	if err != nil {
		return nil, err
	}
	off := 0.5 * b.MedianStyle.Radius
	ls.Offset = ls.Offset.Add(vg.Point{X: off, Y: off})
	return ls, nil
}

type horizQuartPlotOutsideLabels struct {
	quartPlotOutsideLabels
}

func (o horizQuartPlotOutsideLabels) XY(i int) (float64, float64) {
	return o.qp.Value(o.qp.Outside[i]), o.qp.Location
}
