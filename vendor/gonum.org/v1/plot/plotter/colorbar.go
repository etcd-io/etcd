// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/vg/draw"
)

// ColorBar is a plot.Plotter that draws a color bar legend for a ColorMap.
type ColorBar struct {
	ColorMap palette.ColorMap

	// Vertical determines wether the legend will be
	// plotted vertically or horizontally.
	// The default is false (horizontal).
	Vertical bool

	// Colors specifies the number of colors to be
	// shown in the legend. If Colors is not specified,
	// a default will be used.
	Colors int
}

// colors returns the number of colors to be shown
// in the legend, substituting invalid values
// with the default of one color per point.
func (l *ColorBar) colors(c draw.Canvas) int {
	if l.Colors > 0 {
		return l.Colors
	}
	if l.Vertical {
		return int(c.Max.Y - c.Min.Y)
	}
	return int(c.Max.X - c.Min.X)
}

// check determines whether the ColorBar is
// valid in its current configuration.
func (l *ColorBar) check() {
	if l.ColorMap == nil {
		panic("plotter: nil ColorMap in ColorBar")
	}
	if l.ColorMap.Max() == l.ColorMap.Min() {
		panic("plotter: ColorMap Max==Min")
	}
}

// Plot implements the Plot method of the plot.Plotter interface.
func (l *ColorBar) Plot(c draw.Canvas, p *plot.Plot) {
	l.check()
	colors := l.colors(c)
	var pImg *Image
	delta := (l.ColorMap.Max() - l.ColorMap.Min()) / float64(colors)
	if l.Vertical {
		img := image.NewNRGBA64(image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 1, Y: colors},
		})
		for i := 0; i < colors; i++ {
			color, err := l.ColorMap.At(l.ColorMap.Min() + delta*float64(i))
			if err != nil {
				panic(err)
			}
			img.Set(0, colors-1-i, color)
		}
		pImg = NewImage(img, 0, l.ColorMap.Min(), 1, l.ColorMap.Max())
	} else {
		img := image.NewNRGBA64(image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: colors, Y: 1},
		})
		for i := 0; i < colors; i++ {
			color, err := l.ColorMap.At(l.ColorMap.Min() + delta*float64(i))
			if err != nil {
				panic(err)
			}
			img.Set(i, 0, color)
		}
		pImg = NewImage(img, l.ColorMap.Min(), 0, l.ColorMap.Max(), 1)
	}
	pImg.Plot(c, p)
}

// DataRange implements the DataRange method
// of the plot.DataRanger interface.
func (l *ColorBar) DataRange() (xmin, xmax, ymin, ymax float64) {
	l.check()
	if l.Vertical {
		return 0, 1, l.ColorMap.Min(), l.ColorMap.Max()
	}
	return l.ColorMap.Min(), l.ColorMap.Max(), 0, 1
}
