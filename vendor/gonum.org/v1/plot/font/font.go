// Copyright ©2021 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package font

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// DefaultCache is the global cache for fonts.
var DefaultCache *Cache = NewCache(nil)

// Font represents a font face.
type Font struct {
	// Typeface identifies the Font.
	Typeface Typeface

	// TODO(sbinet): Gio@v0.2.0 has dropped font.Font.Variant
	// we should probably follow suit.

	// Variant is the variant of a font, such as "Mono" or "Smallcaps".
	Variant Variant

	// Style is the style of a font, such as Regular or Italic.
	Style font.Style

	// Weight is the weight of a font, such as Normal or Bold.
	Weight font.Weight

	// Size is the size of the font.
	Size Length
}

// Name returns a fully qualified name for the given font.
func (f *Font) Name() string {
	v := f.Variant
	w := weightName(f.Weight)
	s := styleName(f.Style)

	switch f.Style {
	case font.StyleNormal:
		s = ""
		if f.Weight == font.WeightNormal {
			w = "Regular"
		}
	default:
		if f.Weight == font.WeightNormal {
			w = ""
		}
	}

	return fmt.Sprintf("%s%s-%s%s", f.Typeface, v, w, s)
}

// From returns a copy of the provided font with its size set.
func From(fnt Font, size Length) Font {
	o := fnt
	o.Size = size
	return o
}

// Typeface identifies a particular typeface design.
// The empty string denotes the default typeface.
type Typeface string

// Variant denotes a typeface variant, such as "Mono", "Smallcaps" or "Math".
type Variant string

// Extents contains font metric information.
type Extents struct {
	// Ascent is the distance that the text
	// extends above the baseline.
	Ascent Length

	// Descent is the distance that the text
	// extends below the baseline. The descent
	// is given as a positive value.
	Descent Length

	// Height is the distance from the lowest
	// descending point to the highest ascending
	// point.
	Height Length
}

// Face holds a font descriptor and the associated font face.
type Face struct {
	Font Font
	Face *opentype.Font
}

// Name returns a fully qualified name for the given font.
func (f *Face) Name() string {
	return f.Font.Name()
}

// FontFace returns the opentype font face for the requested
// dots-per-inch resolution.
func (f *Face) FontFace(dpi float64) font.Face {
	face, err := opentype.NewFace(f.Face, &opentype.FaceOptions{
		Size: f.Font.Size.Points(),
		DPI:  dpi,
	})
	if err != nil {
		panic(err)
	}
	return face
}

// default hinting for OpenType fonts
const defaultHinting = font.HintingNone

// Extents returns the FontExtents for a font.
func (f *Face) Extents() Extents {
	var (
		// TODO(sbinet): re-use a Font-level sfnt.Buffer instead?
		buf  sfnt.Buffer
		ppem = fixed.Int26_6(f.Face.UnitsPerEm())
	)

	met, err := f.Face.Metrics(&buf, ppem, defaultHinting)
	if err != nil {
		panic(fmt.Errorf("could not extract font extents: %v", err))
	}
	scale := f.Font.Size / Points(float64(ppem))
	return Extents{
		Ascent:  Points(float64(met.Ascent)) * scale,
		Descent: Points(float64(met.Descent)) * scale,
		Height:  Points(float64(met.Height)) * scale,
	}
}

// Width returns width of a string when drawn using the font.
func (f *Face) Width(s string) Length {
	var (
		pixelsPerEm = fixed.Int26_6(f.Face.UnitsPerEm())

		// scale converts sfnt.Unit to float64
		scale = f.Font.Size / Points(float64(pixelsPerEm))

		width     = 0
		hasPrev   = false
		buf       sfnt.Buffer
		prev, idx sfnt.GlyphIndex
		hinting   = defaultHinting
	)
	for _, rune := range s {
		var err error
		idx, err = f.Face.GlyphIndex(&buf, rune)
		if err != nil {
			panic(fmt.Errorf("could not get glyph index: %v", err))
		}
		if hasPrev {
			kern, err := f.Face.Kern(&buf, prev, idx, pixelsPerEm, hinting)
			switch {
			case err == nil:
				width += int(kern)
			case errors.Is(err, sfnt.ErrNotFound):
				// no-op
			default:
				panic(fmt.Errorf("could not get kerning: %v", err))
			}
		}
		adv, err := f.Face.GlyphAdvance(&buf, idx, pixelsPerEm, hinting)
		if err != nil {
			panic(fmt.Errorf("could not retrieve glyph's advance: %v", err))
		}
		width += int(adv)
		prev, hasPrev = idx, true
	}
	return Points(float64(width)) * scale
}

// Collection is a collection of fonts, regrouped under a common typeface.
type Collection []Face

// Cache collects font faces.
type Cache struct {
	mu    sync.RWMutex
	def   Typeface
	faces map[Font]*opentype.Font
}

// We make Cache implement dummy GobDecoder and GobEncoder interfaces
// to allow plot.Plot (or any other type holding a Cache) to be (de)serialized
// with encoding/gob.
// As Cache holds opentype.Font, the reflect-based gob (de)serialization can not
// work: gob isn't happy with opentype.Font having no exported field:
//
//   error: gob: type font.Cache has no exported fields
//
// FIXME(sbinet): perhaps encode/decode Cache.def typeface?

func (c *Cache) GobEncode() ([]byte, error) { return nil, nil }
func (c *Cache) GobDecode([]byte) error {
	if c.faces == nil {
		c.faces = make(map[Font]*opentype.Font)
	}
	return nil
}

// NewCache creates a new cache of fonts from the provided collection of
// font Faces.
// The first font Face in the collection is set to be the default one.
func NewCache(coll Collection) *Cache {
	cache := &Cache{
		faces: make(map[Font]*opentype.Font, len(coll)),
	}
	cache.Add(coll)
	return cache
}

// Add adds a whole collection of font Faces to the font cache.
// If the cache is empty, the first font Face in the collection is set
// to be the default one.
func (c *Cache) Add(coll Collection) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.faces == nil {
		c.faces = make(map[Font]*opentype.Font, len(coll))
	}
	for i, f := range coll {
		if i == 0 && c.def == "" {
			c.def = f.Font.Typeface
		}
		fnt := f.Font
		fnt.Size = 0 // store all font descriptors with the same size.
		c.faces[fnt] = f.Face
	}
}

// Lookup returns the font Face corresponding to the provided Font descriptor,
// with the provided font size set.
//
// If no matching font Face could be found, the one corresponding to
// the default typeface is selected and returned.
func (c *Cache) Lookup(fnt Font, size Length) Face {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.faces) == 0 {
		return Face{}
	}

	face := c.lookup(fnt)
	if face == nil {
		fnt.Typeface = c.def
		face = c.lookup(fnt)
	}

	ff := Face{
		Font: fnt,
		Face: face,
	}
	ff.Font.Size = size
	return ff
}

// Has returns whether the cache contains the exact font descriptor.
func (c *Cache) Has(fnt Font) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	face := c.lookup(fnt)
	return face != nil
}

func (c *Cache) lookup(key Font) *opentype.Font {
	key.Size = 0

	tf := c.faces[key]
	if tf == nil {
		key := key
		key.Weight = font.WeightNormal
		tf = c.faces[key]
	}
	if tf == nil {
		key := key
		key.Style = font.StyleNormal
		tf = c.faces[key]
	}
	if tf == nil {
		key := key
		key.Style = font.StyleNormal
		key.Weight = font.WeightNormal
		tf = c.faces[key]
	}

	return tf
}

func weightName(w font.Weight) string {
	switch w {
	case font.WeightThin:
		return "Thin"
	case font.WeightExtraLight:
		return "ExtraLight"
	case font.WeightLight:
		return "Light"
	case font.WeightNormal:
		return "Regular"
	case font.WeightMedium:
		return "Medium"
	case font.WeightSemiBold:
		return "SemiBold"
	case font.WeightBold:
		return "Bold"
	case font.WeightExtraBold:
		return "ExtraBold"
	case font.WeightBlack:
		return "Black"
	}
	return fmt.Sprintf("weight(%d)", w)
}

func styleName(sty font.Style) string {
	switch sty {
	case font.StyleNormal:
		return "Normal"
	case font.StyleItalic:
		return "Italic"
	case font.StyleOblique:
		return "Oblique"
	}
	return fmt.Sprintf("style(%d)", sty)
}
