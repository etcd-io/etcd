// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plot

import (
	"fmt"
	"math"

	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Align returns a two-dimensional row-major array of Canvases which will
// produce tiled plots with DataCanvases that are evenly sized and spaced.
// The arguments to the function are a two-dimensional row-major array
// of plots, a tile configuration, and the canvas to which the tiled
// plots are to be drawn.
func Align(plots [][]*Plot, t draw.Tiles, dc draw.Canvas) [][]draw.Canvas {
	o := make([][]draw.Canvas, len(plots))

	if len(plots) != t.Rows {
		panic(fmt.Errorf("plot: plots rows (%d) != tiles rows (%d)", len(plots), t.Rows))
	}

	// Create the initial tiles.
	for j := 0; j < t.Rows; j++ {
		if len(plots[j]) != t.Cols {
			panic(fmt.Errorf("plot: plots row %d columns (%d) != tiles columns (%d)", j, len(plots[j]), t.Rows))
		}

		o[j] = make([]draw.Canvas, len(plots[j]))
		for i := 0; i < t.Cols; i++ {
			o[j][i] = t.At(dc, i, j)
		}
	}

	type posNeg struct {
		p, n float64
	}
	xSpacing := make([]posNeg, t.Cols)
	ySpacing := make([]posNeg, t.Rows)

	// Calculate the maximum spacing between data canvases
	// for each row and column.
	for j, row := range plots {
		for i, p := range row {
			if p == nil {
				continue
			}
			c := o[j][i]
			dataC := p.DataCanvas(o[j][i])
			xSpacing[i].n = math.Max(float64(dataC.Min.X-c.Min.X), xSpacing[i].n)
			xSpacing[i].p = math.Max(float64(c.Max.X-dataC.Max.X), xSpacing[i].p)
			ySpacing[j].n = math.Max(float64(dataC.Min.Y-c.Min.Y), ySpacing[j].n)
			ySpacing[j].p = math.Max(float64(c.Max.Y-dataC.Max.Y), ySpacing[j].p)
		}
	}

	// Calculate the total row and column spacing.
	var xTotalSpace float64
	for _, s := range xSpacing {
		xTotalSpace += s.n + s.p
	}
	xTotalSpace += float64(t.PadX)*float64(len(xSpacing)-1) + float64(t.PadLeft+t.PadRight)
	var yTotalSpace float64
	for _, s := range ySpacing {
		yTotalSpace += s.n + s.p
	}
	yTotalSpace += float64(t.PadY)*float64(len(ySpacing)-1) + float64(t.PadTop+t.PadBottom)

	avgWidth := vg.Length((float64(dc.Max.X-dc.Min.X) - xTotalSpace) / float64(t.Cols))
	avgHeight := vg.Length((float64(dc.Max.Y-dc.Min.Y) - yTotalSpace) / float64(t.Rows))

	moveVertical := make([]vg.Length, t.Cols)
	for j := t.Rows - 1; j >= 0; j-- {
		row := plots[j]
		var moveHorizontal vg.Length
		for i, p := range row {
			c := o[j][i]

			if p != nil {
				dataC := p.DataCanvas(c)
				// Adjust the horizontal and vertical spacing between
				// canvases to match the maximum for each column and row,
				// respectively.
				c = draw.Crop(c,
					vg.Length(xSpacing[i].n)-(dataC.Min.X-c.Min.X),
					c.Max.X-dataC.Max.X-vg.Length(xSpacing[i].p),
					vg.Length(ySpacing[j].n)-(dataC.Min.Y-c.Min.Y),
					c.Max.Y-dataC.Max.Y-vg.Length(ySpacing[j].p),
				)
			}

			var width, height vg.Length
			if p == nil {
				width = c.Max.X - c.Min.X - vg.Length(xSpacing[i].p+xSpacing[i].n)
				height = c.Max.Y - c.Min.Y - vg.Length(ySpacing[j].p+ySpacing[j].n)
			} else {
				dataC := p.DataCanvas(c)
				width = dataC.Max.X - dataC.Min.X
				height = dataC.Max.Y - dataC.Min.Y
			}

			// Adjust the canvas so that the height and width of the
			// DataCanvas is the same for all plots.
			o[j][i] = draw.Crop(c,
				moveHorizontal,
				moveHorizontal+avgWidth-width,
				moveVertical[i],
				moveVertical[i]+avgHeight-height,
			)
			moveHorizontal += avgWidth - width
			moveVertical[i] += avgHeight - height
		}
	}
	return o
}
