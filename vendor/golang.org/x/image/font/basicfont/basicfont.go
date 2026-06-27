// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen.go

// Package basicfont provides fixed-size font faces.
package basicfont // import "golang.org/x/image/font/basicfont"

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Range maps a contiguous range of runes to vertically adjacent sub-images of
// a Face's Mask image. The rune range is inclusive on the low end and
// exclusive on the high end.
//
// If Low <= r && r < High, then the rune r is mapped to the sub-image of
// Face.Mask whose bounds are image.Rect(0, y*h, Face.Width, (y+1)*h),
// where y = (int(r-Low) + Offset) and h = (Face.Ascent + Face.Descent).
type Range struct {
	Low, High rune
	Offset    int
}

// Face7x13 is a Face derived from the public domain X11 misc-fixed font files.
//
// At the moment, it holds the printable characters in ASCII starting with
// space, and the Unicode replacement character U+FFFD.
//
// Its data is entirely self-contained and does not require loading from
// separate files.
var Face7x13 = &Face{
	Advance: 7,
	Width:   6,
	Height:  13,
	Ascent:  11,
	Descent: 2,
	Mask:    mask7x13,
	Ranges: []Range{
		{'\u0020', '\u007f', 0},
		{'\ufffd', '\ufffe', 95},
	},
}

// Face is a basic font face whose glyphs all have the same metrics.
//
// It is safe to use concurrently.
type Face struct {
	// Advance is the glyph advance, in pixels.
	Advance int
	// Width is the glyph width, in pixels.
	Width int
	// Height is the inter-line height, in pixels.
	Height int
	// Ascent is the glyph ascent, in pixels.
	Ascent int
	// Descent is the glyph descent, in pixels.
	Descent int
	// Left is the left side bearing, in pixels. A positive value means that
	// all of a glyph is to the right of the dot.
	Left int

	// Mask contains all of the glyph masks. Its width is typically the Face's
	// Width, and its height a multiple of the Face's Height.
	Mask image.Image
	// Ranges map runes to sub-images of Mask. The rune ranges must not
	// overlap, and must be in increasing rune order.
	Ranges []Range
}

func (f *Face) Close() error                   { return nil }
func (f *Face) Kern(r0, r1 rune) fixed.Int26_6 { return 0 }

func (f *Face) Metrics() font.Metrics {
	return font.Metrics{
		Height:     fixed.I(f.Height),
		Ascent:     fixed.I(f.Ascent),
		Descent:    fixed.I(f.Descent),
		XHeight:    fixed.I(f.Ascent),
		CapHeight:  fixed.I(f.Ascent),
		CaretSlope: image.Point{X: 0, Y: 1},
	}
}

func (f *Face) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {

	if found, rng := f.find(r); rng != nil {
		maskp.Y = (int(found-rng.Low) + rng.Offset) * (f.Ascent + f.Descent)
		x := int(dot.X+32)>>6 + f.Left
		y := int(dot.Y+32) >> 6
		dr = image.Rectangle{
			Min: image.Point{
				X: x,
				Y: y - f.Ascent,
			},
			Max: image.Point{
				X: x + f.Width,
				Y: y + f.Descent,
			},
		}

		return dr, f.Mask, maskp, fixed.I(f.Advance), r == found
	}
	return image.Rectangle{}, nil, image.Point{}, 0, false
}

func (f *Face) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	if found, rng := f.find(r); rng != nil {
		return fixed.R(0, -f.Ascent, f.Width, +f.Descent), fixed.I(f.Advance), r == found
	}
	return fixed.Rectangle26_6{}, 0, false
}

func (f *Face) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	if found, rng := f.find(r); rng != nil {
		return fixed.I(f.Advance), r == found
	}
	return 0, false
}

func (f *Face) find(r rune) (rune, *Range) {
	for {
		for i, rng := range f.Ranges {
			if (rng.Low <= r) && (r < rng.High) {
				return r, &f.Ranges[i]
			}
		}
		if r == '\ufffd' {
			return 0, nil
		}
		r = '\ufffd'
	}
}
