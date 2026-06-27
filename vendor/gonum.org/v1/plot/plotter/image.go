// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image"
	"math"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Image is a plotter that draws a scaled, raster image.
type Image struct {
	img            image.Image
	cols           int
	rows           int
	xmin, xmax, dx float64
	ymin, ymax, dy float64
}

// NewImage creates a new image plotter.
// Image will plot img inside the rectangle defined by the
// (xmin, ymin) and (xmax, ymax) points given in the data space.
// The img will be scaled to fit inside the rectangle.
func NewImage(img image.Image, xmin, ymin, xmax, ymax float64) *Image {
	if src, ok := img.(*image.Uniform); ok {
		img = uniform{
			src,
			image.Rect(0, 0, int(xmax-xmin+0.5), int(ymax-ymin+0.5)),
		}
	}
	bounds := img.Bounds()
	cols := bounds.Dx()
	rows := bounds.Dy()
	dx := math.Abs(xmax-xmin) / float64(cols)
	dy := math.Abs(ymax-ymin) / float64(rows)
	return &Image{
		img:  img,
		cols: cols,
		rows: rows,
		xmin: xmin,
		xmax: xmax,
		dx:   dx,
		ymin: ymin,
		ymax: ymax,
		dy:   dy,
	}
}

// Plot implements the Plot method of the plot.Plotter interface.
func (img *Image) Plot(c draw.Canvas, p *plot.Plot) {
	trX, trY := p.Transforms(&c)
	xmin := trX(img.xmin)
	ymin := trY(img.ymin)
	xmax := trX(img.xmax)
	ymax := trY(img.ymax)
	rect := vg.Rectangle{
		Min: vg.Point{X: xmin, Y: ymin},
		Max: vg.Point{X: xmax, Y: ymax},
	}
	c.DrawImage(rect, img.transformFor(p))
}

// DataRange implements the DataRange method
// of the plot.DataRanger interface.
func (img *Image) DataRange() (xmin, xmax, ymin, ymax float64) {
	return img.xmin, img.xmax, img.ymin, img.ymax
}

// GlyphBoxes implements the GlyphBoxes method
// of the plot.GlyphBoxer interface.
func (img *Image) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	return nil
}

// transform warps the image to align with non-linear axes.
func (img *Image) transformFor(p *plot.Plot) image.Image {
	_, xLinear := p.X.Scale.(plot.LinearScale)
	_, yLinear := p.Y.Scale.(plot.LinearScale)
	if xLinear && yLinear {
		return img.img
	}
	b := img.img.Bounds()
	o := image.NewNRGBA64(b)
	for c := 0; c < img.cols; c++ {
		// Find the equivalent image column after applying axis transforms.
		cTrans := int(p.X.Norm(img.x(c)) * float64(img.cols))
		// Find the equivalent column of the previous image column after applying
		// axis transforms.
		cPrevTrans := int(p.X.Norm(img.x(maxInt(c-1, 0))) * float64(img.cols))
		for r := 0; r < img.rows; r++ {
			// Find the equivalent image row after applying axis transforms.
			rTrans := int(p.Y.Norm(img.y(r)) * float64(img.rows))
			// Find the equivalent row of the previous image row after applying
			// axis transforms.
			rPrevTrans := int(p.Y.Norm(img.y(maxInt(r-1, 0))) * float64(img.rows))
			crColor := img.img.At(c, img.rows-r-1)
			// Set all the pixels in the new image between (cPrevTrans, rPrevTrans)
			// and (cTrans, rTrans) to the color at (c,r) in the original image.
			// TODO: Improve interpolation.
			for cPrime := cPrevTrans; cPrime <= cTrans; cPrime++ {
				for rPrime := rPrevTrans; rPrime <= rTrans; rPrime++ {
					o.Set(cPrime, img.rows-rPrime-1, crColor)
				}
			}
		}
	}
	return o
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (img *Image) x(c int) float64 {
	if c >= img.cols || c < 0 {
		panic("plotter/image: illegal range")
	}
	return img.xmin + float64(c)*img.dx
}

func (img *Image) y(r int) float64 {
	if r >= img.rows || r < 0 {
		panic("plotter/image: illegal range")
	}
	return img.ymin + float64(r)*img.dy
}

// uniform is a cropped uniform image.
type uniform struct {
	*image.Uniform
	rect image.Rectangle
}

func (img uniform) Bounds() image.Rectangle {
	return img.rect
}
