// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

var (
	// DefaultFont is the default font for label text.
	DefaultFont = plot.DefaultFont

	// DefaultFontSize is the default font.
	DefaultFontSize = vg.Points(10)
)

// Labels implements the Plotter interface,
// drawing a set of labels at specified points.
type Labels struct {
	XYs

	// Labels is the set of labels corresponding
	// to each point.
	Labels []string

	// TextStyle is the style of the label text. Each label
	// can have a different text style.
	TextStyle []text.Style

	// Offset is added directly to the final label location.
	Offset vg.Point
}

// NewLabels returns a new Labels using the DefaultFont and
// the DefaultFontSize.
func NewLabels(d XYLabeller) (*Labels, error) {
	xys, err := CopyXYs(d)
	if err != nil {
		return nil, err
	}

	if d.Len() != len(xys) {
		return nil, errors.New("plotter: number of points does not match the number of labels")
	}

	strs := make([]string, d.Len())
	for i := range strs {
		strs[i] = d.Label(i)
	}

	styles := make([]text.Style, d.Len())
	for i := range styles {
		styles[i] = text.Style{
			Font:    font.From(DefaultFont, DefaultFontSize),
			Handler: plot.DefaultTextHandler,
		}
	}

	return &Labels{
		XYs:       xys,
		Labels:    strs,
		TextStyle: styles,
	}, nil
}

// Plot implements the Plotter interface, drawing labels.
func (l *Labels) Plot(c draw.Canvas, p *plot.Plot) {
	trX, trY := p.Transforms(&c)
	for i, label := range l.Labels {
		pt := vg.Point{X: trX(l.XYs[i].X), Y: trY(l.XYs[i].Y)}
		if !c.Contains(pt) {
			continue
		}
		pt = pt.Add(l.Offset)
		c.FillText(l.TextStyle[i], pt, label)
	}
}

// DataRange returns the minimum and maximum X and Y values
func (l *Labels) DataRange() (xmin, xmax, ymin, ymax float64) {
	return XYRange(l)
}

// GlyphBoxes returns a slice of GlyphBoxes,
// one for each of the labels, implementing the
// plot.GlyphBoxer interface.
func (l *Labels) GlyphBoxes(p *plot.Plot) []plot.GlyphBox {
	bs := make([]plot.GlyphBox, len(l.Labels))
	for i, label := range l.Labels {
		bs[i] = plot.GlyphBox{
			X:         p.X.Norm(l.XYs[i].X),
			Y:         p.Y.Norm(l.XYs[i].Y),
			Rectangle: l.TextStyle[i].Rectangle(label).Add(l.Offset),
		}
	}
	return bs
}

// XYLabeller combines the XYer and Labeller types.
type XYLabeller interface {
	XYer
	Labeller
}

// XYLabels holds XY data with labels.
// The ith label corresponds to the ith XY.
type XYLabels struct {
	XYs
	Labels []string
}

// Label returns the label for point index i.
func (l XYLabels) Label(i int) string {
	return l.Labels[i]
}

var _ XYLabeller = (*XYLabels)(nil)
