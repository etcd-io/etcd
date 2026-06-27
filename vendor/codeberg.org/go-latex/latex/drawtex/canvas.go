// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package drawtex describes the graphics interface for drawing LaTeX.
package drawtex // import "codeberg.org/go-latex/latex/drawtex"

import (
	"codeberg.org/go-latex/latex/font"
	"golang.org/x/image/font/sfnt"
)

type Canvas struct {
	ops []Op
}

func New() *Canvas {
	return &Canvas{}
}

func (c *Canvas) RenderGlyph(x, y float64, infos Glyph) {
	c.ops = append(c.ops, GlyphOp{x, y, infos})
}

func (c *Canvas) RenderRectFilled(x1, y1, x2, y2 float64) {
	c.ops = append(c.ops, RectOp{x1, y1, x2, y2})
}

func (c *Canvas) Ops() []Op { return c.ops }

type Op interface {
	isOp()
}

type GlyphOp struct {
	X, Y  float64
	Glyph Glyph
}

func (GlyphOp) isOp() {}

type RectOp struct {
	X1, Y1 float64
	X2, Y2 float64
}

func (RectOp) isOp() {}

type Glyph struct {
	Font       *sfnt.Font
	Size       float64
	Postscript string
	Metrics    font.Metrics
	Symbol     string
	Num        sfnt.GlyphIndex
	Offset     float64
}

var (
	_ Op = (*GlyphOp)(nil)
	_ Op = (*RectOp)(nil)
)
