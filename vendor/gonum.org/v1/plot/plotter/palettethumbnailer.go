// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image/color"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// PaletteThumbnailers creates a slice of plot.Thumbnailers that can be used to
// add legend entries for the colors in a color palette.
func PaletteThumbnailers(p palette.Palette) []plot.Thumbnailer {
	colors := p.Colors()
	thumbnailers := make([]plot.Thumbnailer, len(colors))
	for i, c := range colors {
		thumbnailers[i] = paletteThumbnailer{color: c}
	}
	return thumbnailers
}

// paletteThumbnailer implements the Thumbnailer interface
// for color palettes.
type paletteThumbnailer struct {
	color color.Color
}

// Thumbnail satisfies the plot.Thumbnailer interface.
func (t paletteThumbnailer) Thumbnail(c *draw.Canvas) {
	pts := []vg.Point{
		{X: c.Min.X, Y: c.Min.Y},
		{X: c.Min.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Min.Y},
	}
	poly := c.ClipPolygonY(pts)
	c.FillPolygon(t.color, poly)
}
