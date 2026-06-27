// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"image/color"
	"math"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// fiveStatPlot contains the shared fields for quartile
// and box-whisker plots.
type fiveStatPlot struct {
	// Values is a copy of the values of the values used to
	// create this box plot.
	Values

	// Location is the location of the box along its axis.
	Location float64

	// Median is the median value of the data.
	Median float64

	// Quartile1 and Quartile3 are the first and
	// third quartiles of the data respectively.
	Quartile1, Quartile3 float64

	// AdjLow and AdjHigh are the `adjacent' values
	// on the low and high ends of the data.  The
	// adjacent values are the points to which the
	// whiskers are drawn.
	AdjLow, AdjHigh float64

	// Min and Max are the extreme values of the data.
	Min, Max float64

	// Outside are the indices of Vs for the outside points.
	Outside []int
}

// BoxPlot implements the Plotter interface, drawing
// a boxplot to represent the distribution of values.
type BoxPlot struct {
	fiveStatPlot

	// Offset is added to the x location of each box.
	// When the Offset is zero, the boxes are drawn
	// centered at their x location.
	Offset vg.Length

	// Width is the width used to draw the box.
	Width vg.Length

	// CapWidth is the width of the cap used to top
	// off a whisker.
	CapWidth vg.Length

	// GlyphStyle is the style of the outside point glyphs.
	GlyphStyle draw.GlyphStyle

	// FillColor is the color used to fill the box.
	// The default is no fill.
	FillColor color.Color

	// BoxStyle is the line style for the box.
	BoxStyle draw.LineStyle

	// MedianStyle is the line style for the median line.
	MedianStyle draw.LineStyle

	// WhiskerStyle is the line style used to draw the
	// whiskers.
	WhiskerStyle draw.LineStyle

	// Horizontal dictates whether the BoxPlot should be in the vertical
	// (default) or horizontal direction.
	Horizontal bool
}

// NewBoxPlot returns a new BoxPlot that represents
// the distribution of the given values.  The style of
// the box plot is that used for Tukey's schematic
// plots in “Exploratory Data Analysis.”
//
// An error is returned if the boxplot is created with
// no values.
//
// The fence values are 1.5x the interquartile before
// the first quartile and after the third quartile.  Any
// value that is outside of the fences are drawn as
// Outside points.  The adjacent values (to which the
// whiskers stretch) are the minimum and maximum
// values that are not outside the fences.
func NewBoxPlot(w vg.Length, loc float64, values Valuer) (*BoxPlot, error) {
	if w < 0 {
		return nil, errors.New("plotter: negative boxplot width")
	}

	b := new(BoxPlot)
	var err error
	if b.fiveStatPlot, err = newFiveStat(w, loc, values); err != nil {
		return nil, err
	}

	b.Width = w
	b.CapWidth = 3 * w / 4

	b.GlyphStyle = DefaultGlyphStyle
	b.BoxStyle = DefaultLineStyle
	b.MedianStyle = DefaultLineStyle
	b.WhiskerStyle = draw.LineStyle{
		Width:  vg.Points(0.5),
		Dashes: []vg.Length{vg.Points(4), vg.Points(2)},
	}

	if len(b.Values) == 0 {
		b.Width = 0
		b.GlyphStyle.Radius = 0
		b.BoxStyle.Width = 0
		b.MedianStyle.Width = 0
		b.WhiskerStyle.Width = 0
	}

	return b, nil
}

func newFiveStat(w vg.Length, loc float64, values Valuer) (fiveStatPlot, error) {
	var b fiveStatPlot
	b.Location = loc

	var err error
	if b.Values, err = CopyValues(values); err != nil {
		return fiveStatPlot{}, err
	}

	sorted := make(Values, len(b.Values))
	copy(sorted, b.Values)
	sort.Float64s(sorted)

	if len(sorted) == 1 {
		b.Median = sorted[0]
		b.Quartile1 = sorted[0]
		b.Quartile3 = sorted[0]
	} else {
		b.Median = median(sorted)
		b.Quartile1 = median(sorted[:len(sorted)/2])
		b.Quartile3 = median(sorted[len(sorted)/2:])
	}
	b.Min = sorted[0]
	b.Max = sorted[len(sorted)-1]

	low := b.Quartile1 - 1.5*(b.Quartile3-b.Quartile1)
	high := b.Quartile3 + 1.5*(b.Quartile3-b.Quartile1)
	b.AdjLow = math.Inf(1)
	b.AdjHigh = math.Inf(-1)
	for i, v := range b.Values {
		if v > high || v < low {
			b.Outside = append(b.Outside, i)
			continue
		}
		if v < b.AdjLow {
			b.AdjLow = v
		}
		if v > b.AdjHigh {
			b.AdjHigh = v
		}
	}

	return b, nil
}

// median returns the median value from a
// sorted Values.
func median(vs Values) float64 {
	if len(vs) == 1 {
		return vs[0]
	}
	med := vs[len(vs)/2]
	if len(vs)%2 == 0 {
		med += vs[len(vs)/2-1]
		med /= 2
	}
	return med
}

// Plot draws the BoxPlot on Canvas c and Plot plt.
func (b *BoxPlot) Plot(c draw.Canvas, plt *plot.Plot) {
	if b.Horizontal {
		b := &horizBoxPlot{b}
		b.Plot(c, plt)
		return
	}

	trX, trY := plt.Transforms(&c)
	x := trX(b.Location)
	if !c.ContainsX(x) {
		return
	}
	x += b.Offset

	med := trY(b.Median)
	q1 := trY(b.Quartile1)
	q3 := trY(b.Quartile3)
	aLow := trY(b.AdjLow)
	aHigh := trY(b.AdjHigh)

	pts := []vg.Point{
		{X: x - b.Width/2, Y: q1},
		{X: x - b.Width/2, Y: q3},
		{X: x + b.Width/2, Y: q3},
		{X: x + b.Width/2, Y: q1},
		{X: x - b.Width/2 - b.BoxStyle.Width/2, Y: q1},
	}
	box := c.ClipLinesY(pts)
	if b.FillColor != nil {
		c.FillPolygon(b.FillColor, c.ClipPolygonY(pts))
	}
	c.StrokeLines(b.BoxStyle, box...)

	medLine := c.ClipLinesY([]vg.Point{
		{X: x - b.Width/2, Y: med},
		{X: x + b.Width/2, Y: med},
	})
	c.StrokeLines(b.MedianStyle, medLine...)

	cap := b.CapWidth / 2
	whisks := c.ClipLinesY(
		[]vg.Point{{X: x, Y: q3}, {X: x, Y: aHigh}},
		[]vg.Point{{X: x - cap, Y: aHigh}, {X: x + cap, Y: aHigh}},
		[]vg.Point{{X: x, Y: q1}, {X: x, Y: aLow}},
		[]vg.Point{{X: x - cap, Y: aLow}, {X: x + cap, Y: aLow}},
	)
	c.StrokeLines(b.WhiskerStyle, whisks...)

	for _, out := range b.Outside {
		y := trY(b.Value(out))
		if c.ContainsY(y) {
			c.DrawGlyphNoClip(b.GlyphStyle, vg.Point{X: x, Y: y})
		}
	}
}

// DataRange returns the minimum and maximum x
// and y values, implementing the plot.DataRanger
// interface.
func (b *BoxPlot) DataRange() (float64, float64, float64, float64) {
	if b.Horizontal {
		b := &horizBoxPlot{b}
		return b.DataRange()
	}
	return b.Location, b.Location, b.Min, b.Max
}

// GlyphBoxes returns a slice of GlyphBoxes for the
// points and for the median line of the boxplot,
// implementing the plot.GlyphBoxer interface
func (b *BoxPlot) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	if b.Horizontal {
		b := &horizBoxPlot{b}
		return b.GlyphBoxes(plt)
	}

	bs := make([]plot.GlyphBox, len(b.Outside)+1)
	for i, out := range b.Outside {
		bs[i].X = plt.X.Norm(b.Location)
		bs[i].Y = plt.Y.Norm(b.Value(out))
		bs[i].Rectangle = b.GlyphStyle.Rectangle()
	}
	bs[len(bs)-1].X = plt.X.Norm(b.Location)
	bs[len(bs)-1].Y = plt.Y.Norm(b.Median)
	bs[len(bs)-1].Rectangle = vg.Rectangle{
		Min: vg.Point{X: b.Offset - (b.Width/2 + b.BoxStyle.Width/2)},
		Max: vg.Point{X: b.Offset + (b.Width/2 + b.BoxStyle.Width/2)},
	}
	return bs
}

// OutsideLabels returns a *Labels that will plot
// a label for each of the outside points.  The
// labels are assumed to correspond to the
// points used to create the box plot.
func (b *BoxPlot) OutsideLabels(labels Labeller) (*Labels, error) {
	if b.Horizontal {
		b := &horizBoxPlot{b}
		return b.OutsideLabels(labels)
	}

	strs := make([]string, len(b.Outside))
	for i, out := range b.Outside {
		strs[i] = labels.Label(out)
	}
	o := boxPlotOutsideLabels{b, strs}
	ls, err := NewLabels(o)
	if err != nil {
		return nil, err
	}
	off := 0.5 * b.GlyphStyle.Radius
	ls.Offset = ls.Offset.Add(vg.Point{X: off, Y: off})
	return ls, nil
}

type boxPlotOutsideLabels struct {
	box    *BoxPlot
	labels []string
}

func (o boxPlotOutsideLabels) Len() int {
	return len(o.box.Outside)
}

func (o boxPlotOutsideLabels) XY(i int) (float64, float64) {
	return o.box.Location, o.box.Value(o.box.Outside[i])
}

func (o boxPlotOutsideLabels) Label(i int) string {
	return o.labels[i]
}

// horizBoxPlot is like a regular BoxPlot, however,
// it draws horizontally instead of Vertically.
// TODO: Merge code for horizontal and vertical box plots as has been done for
// bar charts.
type horizBoxPlot struct{ *BoxPlot }

func (b horizBoxPlot) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	y := trY(b.Location)
	if !c.ContainsY(y) {
		return
	}
	y += b.Offset

	med := trX(b.Median)
	q1 := trX(b.Quartile1)
	q3 := trX(b.Quartile3)
	aLow := trX(b.AdjLow)
	aHigh := trX(b.AdjHigh)

	pts := []vg.Point{
		{X: q1, Y: y - b.Width/2},
		{X: q3, Y: y - b.Width/2},
		{X: q3, Y: y + b.Width/2},
		{X: q1, Y: y + b.Width/2},
		{X: q1, Y: y - b.Width/2 - b.BoxStyle.Width/2},
	}
	box := c.ClipLinesX(pts)
	if b.FillColor != nil {
		c.FillPolygon(b.FillColor, c.ClipPolygonX(pts))
	}
	c.StrokeLines(b.BoxStyle, box...)

	medLine := c.ClipLinesX([]vg.Point{
		{X: med, Y: y - b.Width/2},
		{X: med, Y: y + b.Width/2},
	})
	c.StrokeLines(b.MedianStyle, medLine...)

	cap := b.CapWidth / 2
	whisks := c.ClipLinesX(
		[]vg.Point{{X: q3, Y: y}, {X: aHigh, Y: y}},
		[]vg.Point{{X: aHigh, Y: y - cap}, {X: aHigh, Y: y + cap}},
		[]vg.Point{{X: q1, Y: y}, {X: aLow, Y: y}},
		[]vg.Point{{X: aLow, Y: y - cap}, {X: aLow, Y: y + cap}},
	)
	c.StrokeLines(b.WhiskerStyle, whisks...)

	for _, out := range b.Outside {
		x := trX(b.Value(out))
		if c.ContainsX(x) {
			c.DrawGlyphNoClip(b.GlyphStyle, vg.Point{X: x, Y: y})
		}
	}
}

// DataRange returns the minimum and maximum x
// and y values, implementing the plot.DataRanger
// interface.
func (b horizBoxPlot) DataRange() (float64, float64, float64, float64) {
	return b.Min, b.Max, b.Location, b.Location
}

// GlyphBoxes returns a slice of GlyphBoxes for the
// points and for the median line of the boxplot,
// implementing the plot.GlyphBoxer interface
func (b horizBoxPlot) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	bs := make([]plot.GlyphBox, len(b.Outside)+1)
	for i, out := range b.Outside {
		bs[i].X = plt.X.Norm(b.Value(out))
		bs[i].Y = plt.Y.Norm(b.Location)
		bs[i].Rectangle = b.GlyphStyle.Rectangle()
	}
	bs[len(bs)-1].X = plt.X.Norm(b.Median)
	bs[len(bs)-1].Y = plt.Y.Norm(b.Location)
	bs[len(bs)-1].Rectangle = vg.Rectangle{
		Min: vg.Point{Y: b.Offset - (b.Width/2 + b.BoxStyle.Width/2)},
		Max: vg.Point{Y: b.Offset + (b.Width/2 + b.BoxStyle.Width/2)},
	}
	return bs
}

// OutsideLabels returns a *Labels that will plot
// a label for each of the outside points.  The
// labels are assumed to correspond to the
// points used to create the box plot.
func (b *horizBoxPlot) OutsideLabels(labels Labeller) (*Labels, error) {
	strs := make([]string, len(b.Outside))
	for i, out := range b.Outside {
		strs[i] = labels.Label(out)
	}
	o := horizBoxPlotOutsideLabels{
		boxPlotOutsideLabels{b.BoxPlot, strs},
	}
	ls, err := NewLabels(o)
	if err != nil {
		return nil, err
	}
	off := 0.5 * b.GlyphStyle.Radius
	ls.Offset = ls.Offset.Add(vg.Point{X: off, Y: off})
	return ls, nil
}

type horizBoxPlotOutsideLabels struct {
	boxPlotOutsideLabels
}

func (o horizBoxPlotOutsideLabels) XY(i int) (float64, float64) {
	return o.box.Value(o.box.Outside[i]), o.box.Location
}

// ValueLabels implements both the Valuer
// and Labeller interfaces.
type ValueLabels []struct {
	Value float64
	Label string
}

// Len returns the number of items.
func (vs ValueLabels) Len() int {
	return len(vs)
}

// Value returns the value of item i.
func (vs ValueLabels) Value(i int) float64 {
	return vs[i].Value
}

// Label returns the label of item i.
func (vs ValueLabels) Label(i int) string {
	return vs[i].Label
}
