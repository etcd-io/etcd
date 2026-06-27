// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package font holds types to handle and abstract away font management.
package font

// Font represents a font.
type Font struct {
	Name string  // Name is the LaTeX name of the font (regular, default, it, ...)
	Type string  // Type is the LaTeX class of the font (it, rm, ...)
	Size float64 // Size is the font size in points.
}

// Backend is the interface that allows to render math expressions.
type Backend interface {
	// RenderGlyphs renders the glyph g at the reference point (x,y).
	RenderGlyph(x, y float64, font Font, symbol string, dpi float64)

	// RenderRectFilled draws a filled black rectangle from (x1,y1) to (x2,y2).
	RenderRectFilled(x1, y1, x2, y2 float64)

	// Kern returns the kerning distance between two symbols.
	Kern(ft1 Font, sym1 string, ft2 Font, sym2 string, dpi float64) float64

	// Metrics returns the metrics.
	Metrics(symbol string, font Font, dpi float64, math bool) Metrics

	// XHeight returns the xheight for the given font and dpi.
	XHeight(font Font, dpi float64) float64

	// UnderlineThickness returns the line thickness that matches the given font.
	// It is used as a base unit for drawing lines such as in a fraction or radical.
	UnderlineThickness(font Font, dpi float64) float64
}

// Metrics represents the metrics of a glyph in a given font.
type Metrics struct {
	Advance float64 // Advance distance of the glyph, in points.
	Height  float64 // Height of the glyph in points.
	Width   float64 // Width of the glyph in points.

	// Ink rectangle of the glyph.
	XMin, XMax, YMin, YMax float64

	// Iceberg is the distance from the baseline to the top of the glyph.
	// Iceberg corresponds to TeX's definition of "height".
	Iceberg float64

	// Slanted indicates whether the glyph is slanted.
	Slanted bool
}
