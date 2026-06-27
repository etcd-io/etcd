// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package plotutil contains a small number of utilites for creating plots.
//
// This package is under active development so portions of it may change.
package plotutil // import "gonum.org/v1/plot/plotutil"

import (
	"image/color"

	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// DefaultColors is a set of colors used by the Color function.
var DefaultColors = SoftColors

var DarkColors = []color.Color{
	rgb(238, 46, 47),
	rgb(0, 140, 72),
	rgb(24, 90, 169),
	rgb(244, 125, 35),
	rgb(102, 44, 145),
	rgb(162, 29, 33),
	rgb(180, 56, 148),
}

var SoftColors = []color.Color{
	rgb(241, 90, 96),
	rgb(122, 195, 106),
	rgb(90, 155, 212),
	rgb(250, 167, 91),
	rgb(158, 103, 171),
	rgb(206, 112, 88),
	rgb(215, 127, 180),
}

func rgb(r, g, b uint8) color.RGBA {
	return color.RGBA{r, g, b, 255}
}

// Color returns the ith default color, wrapping
// if i is less than zero or greater than the max
// number of colors in the DefaultColors slice.
func Color(i int) color.Color {
	n := len(DefaultColors)
	if i < 0 {
		return DefaultColors[i%n+n]
	}
	return DefaultColors[i%n]
}

// DefaultGlyphShapes is a set of GlyphDrawers used by
// the Shape function.
var DefaultGlyphShapes = []draw.GlyphDrawer{
	draw.RingGlyph{},
	draw.SquareGlyph{},
	draw.TriangleGlyph{},
	draw.CrossGlyph{},
	draw.PlusGlyph{},
	draw.CircleGlyph{},
	draw.BoxGlyph{},
	draw.PyramidGlyph{},
}

// Shape returns the ith default glyph shape,
// wrapping if i is less than zero or greater
// than the max number of GlyphDrawers
// in the DefaultGlyphShapes slice.
func Shape(i int) draw.GlyphDrawer {
	n := len(DefaultGlyphShapes)
	if i < 0 {
		return DefaultGlyphShapes[i%n+n]
	}
	return DefaultGlyphShapes[i%n]
}

// DefaultDashes is a set of dash patterns used by
// the Dashes function.
var DefaultDashes = [][]vg.Length{
	{},

	{vg.Points(6), vg.Points(2)},

	{vg.Points(2), vg.Points(2)},

	{vg.Points(1), vg.Points(1)},

	{vg.Points(5), vg.Points(2), vg.Points(1), vg.Points(2)},

	{vg.Points(10), vg.Points(2), vg.Points(2), vg.Points(2),
		vg.Points(2), vg.Points(2), vg.Points(2), vg.Points(2)},

	{vg.Points(10), vg.Points(2), vg.Points(2), vg.Points(2)},

	{vg.Points(5), vg.Points(2), vg.Points(5), vg.Points(2),
		vg.Points(2), vg.Points(2), vg.Points(2), vg.Points(2)},

	{vg.Points(4), vg.Points(2), vg.Points(4), vg.Points(1),
		vg.Points(1), vg.Points(1), vg.Points(1), vg.Points(1),
		vg.Points(1), vg.Points(1)},
}

// Dashes returns the ith default dash pattern,
// wrapping if i is less than zero or greater
// than the max number of dash patters
// in the DefaultDashes slice.
func Dashes(i int) []vg.Length {
	n := len(DefaultDashes)
	if i < 0 {
		return DefaultDashes[i%n+n]
	}
	return DefaultDashes[i%n]
}
