// Copyright ©2020 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text // import "gonum.org/v1/plot/text"

import (
	"math"
	"strings"

	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/vg"
)

// Plain is a text/plain handler.
type Plain struct {
	Fonts *font.Cache
}

var _ Handler = (*Plain)(nil)

// Cache returns the cache of fonts used by the text handler.
func (hdlr Plain) Cache() *font.Cache {
	return hdlr.Fonts
}

// Extents returns the Extents of a font.
func (hdlr Plain) Extents(fnt font.Font) font.Extents {
	face := hdlr.Fonts.Lookup(fnt, fnt.Size)
	return face.Extents()
}

// Lines splits a given block of text into separate lines.
func (hdlr Plain) Lines(txt string) []string {
	txt = strings.TrimRight(txt, "\n")
	return strings.Split(txt, "\n")
}

// Box returns the bounding box of the given non-multiline text where:
//   - width is the horizontal space from the origin.
//   - height is the vertical space above the baseline.
//   - depth is the vertical space below the baseline, a positive number.
func (hdlr Plain) Box(txt string, fnt font.Font) (width, height, depth vg.Length) {
	face := hdlr.Fonts.Lookup(fnt, fnt.Size)
	ext := face.Extents()
	width = face.Width(txt)
	height = ext.Ascent
	depth = ext.Descent

	return width, height, depth
}

// Draw renders the given text with the provided style and position
// on the canvas.
func (hdlr Plain) Draw(c vg.Canvas, txt string, sty Style, pt vg.Point) {
	txt = strings.TrimRight(txt, "\n")
	if len(txt) == 0 {
		return
	}

	fnt := hdlr.Fonts.Lookup(sty.Font, sty.Font.Size)
	c.SetColor(sty.Color)

	if sty.Rotation != 0 {
		c.Push()
		c.Rotate(sty.Rotation)
	}

	sin64, cos64 := math.Sincos(sty.Rotation)
	cos := vg.Length(cos64)
	sin := vg.Length(sin64)
	pt.X, pt.Y = pt.Y*sin+pt.X*cos, pt.Y*cos-pt.X*sin

	lines := hdlr.Lines(txt)
	ht := sty.Height(txt)
	pt.Y += ht*vg.Length(sty.YAlign) - fnt.Extents().Ascent
	for i, line := range lines {
		xoffs := vg.Length(sty.XAlign) * fnt.Width(line)
		n := vg.Length(len(lines) - i)
		c.FillString(fnt, pt.Add(vg.Point{X: xoffs, Y: n * sty.Font.Size}), line)
	}

	if sty.Rotation != 0 {
		c.Pop()
	}
}
