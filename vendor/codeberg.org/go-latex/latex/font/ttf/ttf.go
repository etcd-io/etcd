// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ttf provides a truetype font Backend
package ttf // import "codeberg.org/go-latex/latex/font/ttf"

import (
	"errors"
	"fmt"
	"unicode"

	"codeberg.org/go-latex/latex/drawtex"
	"codeberg.org/go-latex/latex/font"
	"codeberg.org/go-latex/latex/internal/tex2unicode"
	stdfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type Fonts struct {
	Default *sfnt.Font

	Rm   *sfnt.Font
	It   *sfnt.Font
	Bf   *sfnt.Font
	BfIt *sfnt.Font
}

type Backend struct {
	canvas *drawtex.Canvas
	glyphs map[ttfKey]ttfVal
	fonts  map[string]*sfnt.Font
}

func New(cnv *drawtex.Canvas) *Backend {
	return NewFrom(cnv, &defaultFonts)
}

func NewFrom(cnv *drawtex.Canvas, fnts *Fonts) *Backend {
	be := &Backend{
		canvas: cnv,
		glyphs: make(map[ttfKey]ttfVal),
		fonts:  make(map[string]*sfnt.Font),
	}

	be.fonts["default"] = fnts.Default
	be.fonts["regular"] = fnts.Rm
	be.fonts["rm"] = fnts.Rm
	be.fonts["it"] = fnts.It
	be.fonts["bf"] = fnts.Bf

	return be
}

// RenderGlyphs renders the glyph g at the reference point (x,y).
func (be *Backend) RenderGlyph(x, y float64, font font.Font, symbol string, dpi float64) {
	glyph := be.getInfo(symbol, font, dpi, true)
	be.canvas.RenderGlyph(x, y, drawtex.Glyph{
		Font:       glyph.font,
		Size:       glyph.size,
		Postscript: glyph.postscript,
		Metrics:    glyph.metrics,
		Symbol:     string(glyph.rune),
		Num:        glyph.glyph,
		Offset:     glyph.offset,
	})
}

// RenderRectFilled draws a filled black rectangle from (x1,y1) to (x2,y2).
func (be *Backend) RenderRectFilled(x1, y1, x2, y2 float64) {
	be.canvas.RenderRectFilled(x1, y1, x2, y2)
}

// Metrics returns the metrics.
func (be *Backend) Metrics(symbol string, fnt font.Font, dpi float64, math bool) font.Metrics {
	return be.getInfo(symbol, fnt, dpi, math).metrics
}

func (be *Backend) getInfo(symbol string, fnt font.Font, dpi float64, math bool) ttfVal {
	key := ttfKey{symbol, fnt, dpi}
	val, ok := be.glyphs[key]
	if ok {
		return val
	}

	var (
		buf     sfnt.Buffer
		hinting = hintingNone
	)

	ft, rn, _ /*symbol*/, fontSize, slanted := be.getGlyph(symbol, fnt, math)

	postscript, err := ft.Name(&buf, sfnt.NameIDPostScript)
	if err != nil {
		panic(fmt.Errorf("could not retrieve postscript name of font: %+v", err))
	}

	idx, err := ft.GlyphIndex(&buf, rn)
	if err != nil {
		panic(fmt.Errorf("could not retrieve glyph index for %q: %+v", rn, err))
	}

	symName, err := ft.GlyphName(&buf, idx)
	if err != nil {
		panic(fmt.Errorf("could not retrieve glyph name of %q: %+v", rn, err))
	}

	var ppem = int(ft.UnitsPerEm() * 6)
	_, err = ft.LoadGlyph(&buf, idx, fixed.I(ppem), nil)
	if err != nil {
		panic(fmt.Errorf("could not load glyph %q: %+v", rn, err))
	}

	adv, err := ft.GlyphAdvance(&buf, idx, fixed.I(ppem), hinting)
	if err != nil {
		panic(fmt.Errorf("could not retrieve glyph advance for %q: %+v", rn, err))
	}

	fupe := fixed.Int26_6(ft.UnitsPerEm())
	_, err = ft.LoadGlyph(&buf, idx, fupe, nil)
	if err != nil {
		panic(fmt.Errorf("could not load glyph %q: %+v", rn, err))
	}

	bnds, _, err := ft.GlyphBounds(&buf, idx, fixed.I(12), hinting)
	if err != nil {
		panic(err)
	}

	var (
		scale  = fontSize / 12
		xmin   = scale * float64(bnds.Min.X) / 64
		xmax   = scale * float64(bnds.Max.X) / 64
		ymin   = scale * float64(-bnds.Max.Y) / 64 // FIXME
		ymax   = scale * float64(-bnds.Min.Y) / 64 // FIXME
		width  = xmax - xmin
		height = ymax - ymin
	)

	offset := 0.0
	if postscript == "Cmex10" {
		offset = height/2 + (fnt.Size / 3 * dpi / 72)
	}

	me := font.Metrics{
		Advance: float64(adv) / 65536 * fnt.Size / 12,
		Height:  height,
		Width:   width,
		XMin:    xmin,
		XMax:    xmax,
		YMin:    ymin + offset,
		YMax:    ymax + offset,
		Iceberg: ymax + offset,
		Slanted: slanted,
	}

	be.glyphs[key] = ttfVal{
		font:       ft,
		size:       fnt.Size,
		postscript: postscript,
		metrics:    me,
		symbolName: symName,
		rune:       rn,
		glyph:      idx,
		offset:     offset,
	}
	return be.glyphs[key]
}

// XHeight returns the xheight for the given font and dpi.
func (be *Backend) XHeight(fnt font.Font, dpi float64) float64 {
	ft := be.getFont(fnt.Type)
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{
		DPI:     dpi,
		Size:    fnt.Size,
		Hinting: stdfont.HintingNone,
	})
	if err != nil {
		panic(fmt.Errorf("could not open font face for font=%s,%g,%s: %+v",
			fnt.Name, fnt.Size, fnt.Type, err,
		))
	}
	defer face.Close()

	return float64(-face.Metrics().XHeight) / 64
}

const (
	hintingNone = stdfont.HintingNone
	//hintingFull = stdfont.HintingFull
)

func (be *Backend) getGlyph(symbol string, font font.Font, math bool) (*sfnt.Font, rune, string, float64, bool) {
	var (
		fontType = font.Type
		idx      = tex2unicode.Index(symbol, math)
	)

	// only characters in the "Letter" class should be italicized in "it" mode.
	// Greek capital letters should be roman.
	if font.Type == "it" && idx < 0x10000 {
		if !unicode.Is(unicode.L, idx) {
			fontType = "rm"
		}
	}
	slanted := (fontType == "it") || be.isSlanted(symbol)
	ft := be.getFont(fontType)
	if ft == nil {
		panic("could not find TTF font for [" + fontType + "]")
	}

	// FIXME(sbinet):
	// \sigma -> sigma, A->A, \infty->infinity, \nabla->gradient
	// etc...
	symbolName := symbol
	return ft, idx, symbolName, font.Size, slanted
}

func (*Backend) isSlanted(symbol string) bool {
	switch symbol {
	case `\int`, `\oint`:
		return true
	default:
		return false
	}
}

func (be *Backend) getFont(fontType string) *sfnt.Font {
	return be.fonts[fontType]
}

// UnderlineThickness returns the line thickness that matches the given font.
// It is used as a base unit for drawing lines such as in a fraction or radical.
func (*Backend) UnderlineThickness(font font.Font, dpi float64) float64 {
	// theoretically, we could grab the underline thickness from the font
	// metrics.
	// but that information is just too un-reliable.
	// so, it is hardcoded.
	return (0.75 / 12 * font.Size * dpi) / 72
}

// Kern returns the kerning distance between two symbols.
func (be *Backend) Kern(ft1 font.Font, sym1 string, ft2 font.Font, sym2 string, dpi float64) float64 {
	if ft1.Name == ft2.Name && ft1.Size == ft2.Size {
		const math = true
		info1 := be.getInfo(sym1, ft1, dpi, math)
		info2 := be.getInfo(sym2, ft2, dpi, math)
		scale := fixed.Int26_6(info1.font.UnitsPerEm())
		var buf sfnt.Buffer
		k, err := info1.font.Kern(&buf, info1.glyph, info2.glyph, scale, hintingNone)
		if err != nil {
			if errors.Is(err, sfnt.ErrNotFound) {
				return 0
			}
			panic(fmt.Errorf("could not compute kerning for %q/%q: %+v",
				sym1, sym2, err,
			))
		}
		return float64(k) / 64
	}
	return 0
}

type ttfKey struct {
	symbol string
	font   font.Font
	dpi    float64
}

type ttfVal struct {
	font       *sfnt.Font
	size       float64
	postscript string
	metrics    font.Metrics
	symbolName string
	rune       rune
	glyph      sfnt.GlyphIndex
	offset     float64
}

var defaultFonts = Fonts{
	Rm:   mustParseTTF(goregular.TTF),
	It:   mustParseTTF(goitalic.TTF),
	Bf:   mustParseTTF(gobold.TTF),
	BfIt: mustParseTTF(gobolditalic.TTF),
}

func mustParseTTF(raw []byte) *sfnt.Font {
	ft, err := sfnt.Parse(raw)
	if err != nil {
		panic(fmt.Errorf("could not parse raw TTF data: %+v", err))
	}
	return ft
}

func init() {
	defaultFonts.Default = defaultFonts.Rm
}

var (
	_ font.Backend = (*Backend)(nil)
)
