// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package font defines an interface for font faces, for drawing text on an
// image.
//
// Other packages provide font face implementations. For example, a truetype
// package would provide one based on .ttf font files.
package font // import "golang.org/x/image/font"

import (
	"image"
	"image/draw"
	"io"
	"unicode/utf8"

	"golang.org/x/image/math/fixed"
)

// TODO: who is responsible for caches (glyph images, glyph indices, kerns)?
// The Drawer or the Face?

// Face is a font face. Its glyphs are often derived from a font file, such as
// "Comic_Sans_MS.ttf", but a face has a specific size, style, weight and
// hinting. For example, the 12pt and 18pt versions of Comic Sans are two
// different faces, even if derived from the same font file.
//
// A Face is not safe for concurrent use by multiple goroutines, as its methods
// may re-use implementation-specific caches and mask image buffers.
//
// To create a Face, look to other packages that implement specific font file
// formats.
type Face interface {
	io.Closer

	// Glyph returns the draw.DrawMask parameters (dr, mask, maskp) to draw r's
	// glyph at the sub-pixel destination location dot, and that glyph's
	// advance width.
	//
	// It returns !ok if the face does not contain a glyph for r. This includes
	// returning !ok for a fallback glyph (such as substituting a U+FFFD glyph
	// or OpenType's .notdef glyph), in which case the other return values may
	// still be non-zero.
	//
	// The contents of the mask image returned by one Glyph call may change
	// after the next Glyph call. Callers that want to cache the mask must make
	// a copy.
	Glyph(dot fixed.Point26_6, r rune) (
		dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool)

	// GlyphBounds returns the bounding box of r's glyph, drawn at a dot equal
	// to the origin, and that glyph's advance width.
	//
	// It returns !ok if the face does not contain a glyph for r. This includes
	// returning !ok for a fallback glyph (such as substituting a U+FFFD glyph
	// or OpenType's .notdef glyph), in which case the other return values may
	// still be non-zero.
	//
	// The glyph's ascent and descent are equal to -bounds.Min.Y and
	// +bounds.Max.Y. The glyph's left-side and right-side bearings are equal
	// to bounds.Min.X and advance-bounds.Max.X. A visual depiction of what
	// these metrics are is at
	// https://developer.apple.com/library/archive/documentation/TextFonts/Conceptual/CocoaTextArchitecture/Art/glyphterms_2x.png
	GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool)

	// GlyphAdvance returns the advance width of r's glyph.
	//
	// It returns !ok if the face does not contain a glyph for r. This includes
	// returning !ok for a fallback glyph (such as substituting a U+FFFD glyph
	// or OpenType's .notdef glyph), in which case the other return values may
	// still be non-zero.
	GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool)

	// Kern returns the horizontal adjustment for the kerning pair (r0, r1). A
	// positive kern means to move the glyphs further apart.
	Kern(r0, r1 rune) fixed.Int26_6

	// Metrics returns the metrics for this Face.
	Metrics() Metrics

	// TODO: ColoredGlyph for various emoji?
	// TODO: Ligatures? Shaping?
}

// Metrics holds the metrics for a Face. A visual depiction is at
// https://developer.apple.com/library/mac/documentation/TextFonts/Conceptual/CocoaTextArchitecture/Art/glyph_metrics_2x.png
type Metrics struct {
	// Height is the recommended amount of vertical space between two lines of
	// text.
	Height fixed.Int26_6

	// Ascent is the distance from the top of a line to its baseline.
	Ascent fixed.Int26_6

	// Descent is the distance from the bottom of a line to its baseline. The
	// value is typically positive, even though a descender goes below the
	// baseline.
	Descent fixed.Int26_6

	// XHeight is the distance from the top of non-ascending lowercase letters
	// to the baseline.
	XHeight fixed.Int26_6

	// CapHeight is the distance from the top of uppercase letters to the
	// baseline.
	CapHeight fixed.Int26_6

	// CaretSlope is the slope of a caret as a vector with the Y axis pointing up.
	// The slope {0, 1} is the vertical caret.
	CaretSlope image.Point
}

// Drawer draws text on a destination image.
//
// A Drawer is not safe for concurrent use by multiple goroutines, since its
// Face is not.
type Drawer struct {
	// Dst is the destination image.
	Dst draw.Image
	// Src is the source image.
	Src image.Image
	// Face provides the glyph mask images.
	Face Face
	// Dot is the baseline location to draw the next glyph. The majority of the
	// affected pixels will be above and to the right of the dot, but some may
	// be below or to the left. For example, drawing a 'j' in an italic face
	// may affect pixels below and to the left of the dot.
	Dot fixed.Point26_6

	// TODO: Clip image.Image?
	// TODO: SrcP image.Point for Src images other than *image.Uniform? How
	// does it get updated during DrawString?
}

// TODO: should DrawString return the last rune drawn, so the next DrawString
// call can kern beforehand? Or should that be the responsibility of the caller
// if they really want to do that, since they have to explicitly shift d.Dot
// anyway? What if ligatures span more than two runes? What if grapheme
// clusters span multiple runes?
//
// TODO: do we assume that the input is in any particular Unicode Normalization
// Form?
//
// TODO: have DrawRunes(s []rune)? DrawRuneReader(io.RuneReader)?? If we take
// io.RuneReader, we can't assume that we can rewind the stream.
//
// TODO: how does this work with line breaking: drawing text up until a
// vertical line? Should DrawString return the number of runes drawn?

// DrawBytes draws s at the dot and advances the dot's location.
//
// It is equivalent to DrawString(string(s)) but may be more efficient.
func (d *Drawer) DrawBytes(s []byte) {
	prevC := rune(-1)
	for len(s) > 0 {
		c, size := utf8.DecodeRune(s)
		s = s[size:]
		if prevC >= 0 {
			d.Dot.X += d.Face.Kern(prevC, c)
		}
		dr, mask, maskp, advance, _ := d.Face.Glyph(d.Dot, c)
		if !dr.Empty() {
			draw.DrawMask(d.Dst, dr, d.Src, image.Point{}, mask, maskp, draw.Over)
		}
		d.Dot.X += advance
		prevC = c
	}
}

// DrawString draws s at the dot and advances the dot's location.
func (d *Drawer) DrawString(s string) {
	prevC := rune(-1)
	for _, c := range s {
		if prevC >= 0 {
			d.Dot.X += d.Face.Kern(prevC, c)
		}
		dr, mask, maskp, advance, _ := d.Face.Glyph(d.Dot, c)
		if !dr.Empty() {
			draw.DrawMask(d.Dst, dr, d.Src, image.Point{}, mask, maskp, draw.Over)
		}
		d.Dot.X += advance
		prevC = c
	}
}

// BoundBytes returns the bounding box of s, drawn at the drawer dot, as well as
// the advance.
//
// It is equivalent to BoundBytes(string(s)) but may be more efficient.
func (d *Drawer) BoundBytes(s []byte) (bounds fixed.Rectangle26_6, advance fixed.Int26_6) {
	bounds, advance = BoundBytes(d.Face, s)
	bounds.Min = bounds.Min.Add(d.Dot)
	bounds.Max = bounds.Max.Add(d.Dot)
	return
}

// BoundString returns the bounding box of s, drawn at the drawer dot, as well
// as the advance.
func (d *Drawer) BoundString(s string) (bounds fixed.Rectangle26_6, advance fixed.Int26_6) {
	bounds, advance = BoundString(d.Face, s)
	bounds.Min = bounds.Min.Add(d.Dot)
	bounds.Max = bounds.Max.Add(d.Dot)
	return
}

// MeasureBytes returns how far dot would advance by drawing s.
//
// It is equivalent to MeasureString(string(s)) but may be more efficient.
func (d *Drawer) MeasureBytes(s []byte) (advance fixed.Int26_6) {
	return MeasureBytes(d.Face, s)
}

// MeasureString returns how far dot would advance by drawing s.
func (d *Drawer) MeasureString(s string) (advance fixed.Int26_6) {
	return MeasureString(d.Face, s)
}

// BoundBytes returns the bounding box of s with f, drawn at a dot equal to the
// origin, as well as the advance.
//
// It is equivalent to BoundString(string(s)) but may be more efficient.
func BoundBytes(f Face, s []byte) (bounds fixed.Rectangle26_6, advance fixed.Int26_6) {
	prevC := rune(-1)
	for len(s) > 0 {
		c, size := utf8.DecodeRune(s)
		s = s[size:]
		if prevC >= 0 {
			advance += f.Kern(prevC, c)
		}
		b, a, _ := f.GlyphBounds(c)
		if !b.Empty() {
			b.Min.X += advance
			b.Max.X += advance
			bounds = bounds.Union(b)
		}
		advance += a
		prevC = c
	}
	return
}

// BoundString returns the bounding box of s with f, drawn at a dot equal to the
// origin, as well as the advance.
func BoundString(f Face, s string) (bounds fixed.Rectangle26_6, advance fixed.Int26_6) {
	prevC := rune(-1)
	for _, c := range s {
		if prevC >= 0 {
			advance += f.Kern(prevC, c)
		}
		b, a, _ := f.GlyphBounds(c)
		if !b.Empty() {
			b.Min.X += advance
			b.Max.X += advance
			bounds = bounds.Union(b)
		}
		advance += a
		prevC = c
	}
	return
}

// MeasureBytes returns how far dot would advance by drawing s with f.
//
// It is equivalent to MeasureString(string(s)) but may be more efficient.
func MeasureBytes(f Face, s []byte) (advance fixed.Int26_6) {
	prevC := rune(-1)
	for len(s) > 0 {
		c, size := utf8.DecodeRune(s)
		s = s[size:]
		if prevC >= 0 {
			advance += f.Kern(prevC, c)
		}
		a, _ := f.GlyphAdvance(c)
		advance += a
		prevC = c
	}
	return advance
}

// MeasureString returns how far dot would advance by drawing s with f.
func MeasureString(f Face, s string) (advance fixed.Int26_6) {
	prevC := rune(-1)
	for _, c := range s {
		if prevC >= 0 {
			advance += f.Kern(prevC, c)
		}
		a, _ := f.GlyphAdvance(c)
		advance += a
		prevC = c
	}
	return advance
}

// Hinting selects how to quantize a vector font's glyph nodes.
//
// Not all fonts support hinting.
type Hinting int

const (
	HintingNone Hinting = iota
	HintingVertical
	HintingFull
)

// Stretch selects a normal, condensed, or expanded face.
//
// Not all fonts support stretches.
type Stretch int

const (
	StretchUltraCondensed Stretch = -4
	StretchExtraCondensed Stretch = -3
	StretchCondensed      Stretch = -2
	StretchSemiCondensed  Stretch = -1
	StretchNormal         Stretch = +0
	StretchSemiExpanded   Stretch = +1
	StretchExpanded       Stretch = +2
	StretchExtraExpanded  Stretch = +3
	StretchUltraExpanded  Stretch = +4
)

// Style selects a normal, italic, or oblique face.
//
// Not all fonts support styles.
type Style int

const (
	StyleNormal Style = iota
	StyleItalic
	StyleOblique
)

// Weight selects a normal, light or bold face.
//
// Not all fonts support weights.
//
// The named Weight constants (e.g. WeightBold) correspond to CSS' common
// weight names (e.g. "Bold"), but the numerical values differ, so that in Go,
// the zero value means to use a normal weight. For the CSS names and values,
// see https://developer.mozilla.org/en/docs/Web/CSS/font-weight
type Weight int

const (
	WeightThin       Weight = -3 // CSS font-weight value 100.
	WeightExtraLight Weight = -2 // CSS font-weight value 200.
	WeightLight      Weight = -1 // CSS font-weight value 300.
	WeightNormal     Weight = +0 // CSS font-weight value 400.
	WeightMedium     Weight = +1 // CSS font-weight value 500.
	WeightSemiBold   Weight = +2 // CSS font-weight value 600.
	WeightBold       Weight = +3 // CSS font-weight value 700.
	WeightExtraBold  Weight = +4 // CSS font-weight value 800.
	WeightBlack      Weight = +5 // CSS font-weight value 900.
)
