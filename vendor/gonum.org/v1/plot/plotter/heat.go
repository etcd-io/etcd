// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image"
	"image/color"
	"math"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// GridXYZ describes three dimensional data where the X and Y
// coordinates are arranged on a rectangular grid.
type GridXYZ interface {
	// Dims returns the dimensions of the grid.
	Dims() (c, r int)

	// Z returns the value of a grid value at (c, r).
	// It will panic if c or r are out of bounds for the grid.
	Z(c, r int) float64

	// X returns the coordinate for the column at the index c.
	// It will panic if c is out of bounds for the grid.
	X(c int) float64

	// Y returns the coordinate for the row at the index r.
	// It will panic if r is out of bounds for the grid.
	Y(r int) float64
}

// HeatMap implements the Plotter interface, drawing
// a heat map of the values in the GridXYZ field.
type HeatMap struct {
	GridXYZ GridXYZ

	// Palette is the color palette used to render
	// the heat map. Palette must not be nil or
	// return a zero length []color.Color.
	Palette palette.Palette

	// Underflow and Overflow are colors used to fill
	// heat map elements outside the dynamic range
	// defined by Min and Max.
	Underflow color.Color
	Overflow  color.Color

	// NaN is the color used to fill heat map elements
	// that are NaN or do not map to a unique palette
	// color.
	NaN color.Color

	// Min and Max define the dynamic range of the
	// heat map.
	Min, Max float64

	// Rasterized indicates whether the heatmap
	// should be produced using raster-based drawing.
	Rasterized bool
}

// NewHeatMap creates as new heat map plotter for the given data,
// using the provided palette. If g has Min and Max methods that return
// a float, those returned values are used to set the respective HeatMap
// fields. If the returned HeatMap is used when Min is greater than Max,
// the Plot method will panic.
func NewHeatMap(g GridXYZ, p palette.Palette) *HeatMap {
	var min, max float64
	type minMaxer interface {
		Min() float64
		Max() float64
	}
	switch g := g.(type) {
	case minMaxer:
		min, max = g.Min(), g.Max()
	default:
		min, max = math.Inf(1), math.Inf(-1)
		c, r := g.Dims()
		for i := 0; i < c; i++ {
			for j := 0; j < r; j++ {
				v := g.Z(i, j)
				if math.IsNaN(v) {
					continue
				}
				min = math.Min(min, v)
				max = math.Max(max, v)
			}
		}
	}

	return &HeatMap{
		GridXYZ: g,
		Palette: p,
		Min:     min,
		Max:     max,
	}
}

// Plot implements the Plot method of the plot.Plotter interface.
func (h *HeatMap) Plot(c draw.Canvas, plt *plot.Plot) {
	if h.Rasterized {
		h.plotRasterized(c, plt)
	} else {
		h.plotVectorized(c, plt)
	}
}

// plotRasterized plots the heatmap using raster-based drawing.
func (h *HeatMap) plotRasterized(c draw.Canvas, plt *plot.Plot) {
	cols, rows := h.GridXYZ.Dims()
	img := image.NewRGBA64(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: cols, Y: rows},
	})

	pal := h.Palette.Colors()
	ps := float64(len(pal)-1) / (h.Max - h.Min)
	for i := 0; i < cols; i++ {
		for j := 0; j < rows; j++ {
			var col color.Color
			switch v := h.GridXYZ.Z(i, j); {
			case v < h.Min:
				col = h.Underflow
			case v > h.Max:
				col = h.Overflow
			case math.IsNaN(v), math.IsInf(ps, 0):
				col = h.NaN
			default:
				col = pal[int((v-h.Min)*ps+0.5)] // Apply palette scaling.
			}

			if col != nil {
				img.Set(i, rows-j-1, col)
			}
		}
	}

	xmin, xmax, ymin, ymax := h.DataRange()
	pImg := NewImage(img, xmin, ymin, xmax, ymax)
	pImg.Plot(c, plt)
}

// plotVectorized plots the heatmap using vector-based drawing.
func (h *HeatMap) plotVectorized(c draw.Canvas, plt *plot.Plot) {
	if h.Min > h.Max {
		panic("contour: invalid Z range: min greater than max")
	}
	pal := h.Palette.Colors()
	if len(pal) == 0 {
		panic("heatmap: empty palette")
	}
	// ps scales the palette uniformly across the data range.
	ps := float64(len(pal)-1) / (h.Max - h.Min)

	trX, trY := plt.Transforms(&c)

	var pa vg.Path
	cols, rows := h.GridXYZ.Dims()
	for i := 0; i < cols; i++ {
		var right, left float64
		switch i {
		case 0:
			if cols == 1 {
				right = 0.5
			} else {
				right = (h.GridXYZ.X(1) - h.GridXYZ.X(0)) / 2
			}
			left = -right
		case cols - 1:
			right = (h.GridXYZ.X(cols-1) - h.GridXYZ.X(cols-2)) / 2
			left = -right
		default:
			right = (h.GridXYZ.X(i+1) - h.GridXYZ.X(i)) / 2
			left = -(h.GridXYZ.X(i) - h.GridXYZ.X(i-1)) / 2
		}

		for j := 0; j < rows; j++ {
			var up, down float64
			switch j {
			case 0:
				if rows == 1 {
					up = 0.5
				} else {
					up = (h.GridXYZ.Y(1) - h.GridXYZ.Y(0)) / 2
				}
				down = -up
			case rows - 1:
				up = (h.GridXYZ.Y(rows-1) - h.GridXYZ.Y(rows-2)) / 2
				down = -up
			default:
				up = (h.GridXYZ.Y(j+1) - h.GridXYZ.Y(j)) / 2
				down = -(h.GridXYZ.Y(j) - h.GridXYZ.Y(j-1)) / 2
			}

			x, y := trX(h.GridXYZ.X(i)+left), trY(h.GridXYZ.Y(j)+down)
			dx, dy := trX(h.GridXYZ.X(i)+right), trY(h.GridXYZ.Y(j)+up)

			if !c.Contains(vg.Point{X: x, Y: y}) || !c.Contains(vg.Point{X: dx, Y: dy}) {
				continue
			}

			pa = pa[:0]
			pa.Move(vg.Point{X: x, Y: y})
			pa.Line(vg.Point{X: dx, Y: y})
			pa.Line(vg.Point{X: dx, Y: dy})
			pa.Line(vg.Point{X: x, Y: dy})
			pa.Close()

			var col color.Color
			switch v := h.GridXYZ.Z(i, j); {
			case v < h.Min:
				col = h.Underflow
			case v > h.Max:
				col = h.Overflow
			case math.IsNaN(v), math.IsInf(ps, 0):
				col = h.NaN
			default:
				col = pal[int((v-h.Min)*ps+0.5)] // Apply palette scaling.
			}
			if col != nil {
				c.SetColor(col)
				c.Fill(pa)
			}
		}
	}
}

// DataRange implements the DataRange method
// of the plot.DataRanger interface.
func (h *HeatMap) DataRange() (xmin, xmax, ymin, ymax float64) {
	c, r := h.GridXYZ.Dims()
	switch c {
	case 1: // Make a unit length when there is no neighbour.
		xmax = h.GridXYZ.X(0) + 0.5
		xmin = h.GridXYZ.X(0) - 0.5
	default:
		xmax = h.GridXYZ.X(c-1) + (h.GridXYZ.X(c-1)-h.GridXYZ.X(c-2))/2
		xmin = h.GridXYZ.X(0) - (h.GridXYZ.X(1)-h.GridXYZ.X(0))/2
	}
	switch r {
	case 1: // Make a unit length when there is no neighbour.
		ymax = h.GridXYZ.Y(0) + 0.5
		ymin = h.GridXYZ.Y(0) - 0.5
	default:
		ymax = h.GridXYZ.Y(r-1) + (h.GridXYZ.Y(r-1)-h.GridXYZ.Y(r-2))/2
		ymin = h.GridXYZ.Y(0) - (h.GridXYZ.Y(1)-h.GridXYZ.Y(0))/2
	}
	return xmin, xmax, ymin, ymax
}

// GlyphBoxes implements the GlyphBoxes method
// of the plot.GlyphBoxer interface.
func (h *HeatMap) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	c, r := h.GridXYZ.Dims()
	b := make([]plot.GlyphBox, 0, r*c)
	for i := 0; i < c; i++ {
		for j := 0; j < r; j++ {
			b = append(b, plot.GlyphBox{
				X: plt.X.Norm(h.GridXYZ.X(i)),
				Y: plt.Y.Norm(h.GridXYZ.Y(j)),
				Rectangle: vg.Rectangle{
					Min: vg.Point{X: -5, Y: -5},
					Max: vg.Point{X: +5, Y: +5},
				},
			})
		}
	}
	return b
}
