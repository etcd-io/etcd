// Copyright ©2020 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"codeberg.org/go-latex/latex/drawtex"
	"codeberg.org/go-latex/latex/font/ttf"
	"codeberg.org/go-latex/latex/mtex"
	"codeberg.org/go-latex/latex/tex"
	stdfnt "golang.org/x/image/font"

	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/vg"
)

// Latex parses, formats and renders LaTeX.
type Latex struct {
	// Fonts is the cache of font faces used by this text handler.
	Fonts *font.Cache

	// DPI is the dot-per-inch controlling the font resolution used by LaTeX.
	// If zero, the resolution defaults to 72.
	DPI float64
}

var _ Handler = (*Latex)(nil)

// Cache returns the cache of fonts used by the text handler.
func (hdlr Latex) Cache() *font.Cache {
	return hdlr.Fonts
}

// Extents returns the Extents of a font.
func (hdlr Latex) Extents(fnt font.Font) font.Extents {
	face := hdlr.Fonts.Lookup(fnt, fnt.Size)
	return face.Extents()
}

// Lines splits a given block of text into separate lines.
func (hdlr Latex) Lines(txt string) []string {
	txt = strings.TrimRight(txt, "\n")
	return strings.Split(txt, "\n")
}

// Box returns the bounding box of the given non-multiline text where:
//   - width is the horizontal space from the origin.
//   - height is the vertical space above the baseline.
//   - depth is the vertical space below the baseline, a positive number.
func (hdlr Latex) Box(txt string, fnt font.Font) (width, height, depth vg.Length) {
	cnv := drawtex.New()
	face := hdlr.Fonts.Lookup(fnt, fnt.Size)
	fnts := hdlr.fontsFor(fnt)
	box, err := mtex.Parse(txt, face.Font.Size.Points(), latexDPI, ttf.NewFrom(cnv, fnts))
	if err != nil {
		panic(fmt.Errorf("could not parse math expression: %w", err))
	}

	var sh tex.Ship
	sh.Call(0, 0, box.(tex.Tree))

	width = vg.Length(box.Width())
	height = vg.Length(box.Height())
	depth = vg.Length(box.Depth())

	// Add a bit of space, with a linegap as mtex.Box is returning
	// a very tight bounding box.
	// See gonum/plot#661.
	if depth != 0 {
		var (
			e       = face.Extents()
			linegap = e.Height - (e.Ascent + e.Descent)
		)
		depth += linegap
	}

	dpi := vg.Length(hdlr.dpi() / latexDPI)
	return width * dpi, height * dpi, depth * dpi
}

// Draw renders the given text with the provided style and position
// on the canvas.
func (hdlr Latex) Draw(c vg.Canvas, txt string, sty Style, pt vg.Point) {
	cnv := drawtex.New()
	face := hdlr.Fonts.Lookup(sty.Font, sty.Font.Size)
	fnts := hdlr.fontsFor(sty.Font)
	box, err := mtex.Parse(txt, face.Font.Size.Points(), latexDPI, ttf.NewFrom(cnv, fnts))
	if err != nil {
		panic(fmt.Errorf("could not parse math expression: %w", err))
	}

	var sh tex.Ship
	sh.Call(0, 0, box.(tex.Tree))

	w := box.Width()
	h := box.Height()
	d := box.Depth()

	dpi := hdlr.dpi() / latexDPI
	o := latex{
		cnv:   c,
		fonts: hdlr.Fonts,
		sty:   sty,
		pt:    pt,
		w:     vg.Length(w * dpi),
		h:     vg.Length((h + d) * dpi),
		cos:   1,
		sin:   0,
	}
	e := face.Extents()
	o.xoff = vg.Length(sty.XAlign) * o.w
	o.yoff = o.h + o.h*vg.Length(sty.YAlign) - (e.Height - e.Ascent)

	if sty.Rotation != 0 {
		sin64, cos64 := math.Sincos(sty.Rotation)
		o.cos = vg.Length(cos64)
		o.sin = vg.Length(sin64)

		o.cnv.Push()
		defer o.cnv.Pop()
		o.cnv.Rotate(sty.Rotation)
	}

	err = o.Render(w/latexDPI, (h+d)/latexDPI, dpi, cnv)
	if err != nil {
		panic(fmt.Errorf("could not render math expression: %w", err))
	}
}

func (hdlr *Latex) fontsFor(fnt font.Font) *ttf.Fonts {
	rm := fnt
	rm.Variant = "Serif"
	rm.Weight = stdfnt.WeightNormal
	rm.Style = stdfnt.StyleNormal

	it := rm
	it.Style = stdfnt.StyleItalic

	bf := rm
	bf.Style = stdfnt.StyleNormal
	bf.Weight = stdfnt.WeightBold

	bfit := bf
	bfit.Style = stdfnt.StyleItalic

	return &ttf.Fonts{
		Rm:      hdlr.Fonts.Lookup(rm, fnt.Size).Face,
		Default: hdlr.Fonts.Lookup(rm, fnt.Size).Face,
		It:      hdlr.Fonts.Lookup(it, fnt.Size).Face,
		Bf:      hdlr.Fonts.Lookup(bf, fnt.Size).Face,
		BfIt:    hdlr.Fonts.Lookup(bfit, fnt.Size).Face,
	}
}

// latexDPI is the default LaTeX resolution used for computing the LaTeX
// layout of equations and regular text.
// Dimensions are then rescaled to the desired resolution.
const latexDPI = 72.0

func (hdlr Latex) dpi() float64 {
	if hdlr.DPI == 0 {
		return latexDPI
	}
	return hdlr.DPI
}

type latex struct {
	cnv   vg.Canvas
	fonts *font.Cache
	sty   Style
	pt    vg.Point

	w vg.Length
	h vg.Length

	cos vg.Length
	sin vg.Length

	xoff vg.Length
	yoff vg.Length
}

var _ mtex.Renderer = (*latex)(nil)

func (r *latex) Render(width, height, dpi float64, c *drawtex.Canvas) error {
	r.cnv.SetColor(r.sty.Color)

	for _, op := range c.Ops() {
		switch op := op.(type) {
		case drawtex.GlyphOp:
			r.drawGlyph(dpi, op)
		case drawtex.RectOp:
			r.drawRect(dpi, op)
		default:
			panic(fmt.Errorf("unknown drawtex op %T", op))
		}
	}

	return nil
}

func (r *latex) drawGlyph(dpi float64, op drawtex.GlyphOp) {
	pt := r.pt
	if r.sty.Rotation != 0 {
		pt.X, pt.Y = r.rotate(pt.X, pt.Y)
	}

	pt = pt.Add(vg.Point{
		X: r.xoff + vg.Length(op.X*dpi),
		Y: r.yoff - vg.Length(op.Y*dpi),
	})

	fnt := font.Face{
		Font: font.From(r.sty.Font, vg.Length(op.Glyph.Size)),
		Face: op.Glyph.Font,
	}
	r.cnv.FillString(fnt, pt, op.Glyph.Symbol)
}

func (r *latex) drawRect(dpi float64, op drawtex.RectOp) {
	x1 := r.xoff + vg.Length(op.X1*dpi)
	x2 := r.xoff + vg.Length(op.X2*dpi)
	y1 := r.yoff - vg.Length(op.Y1*dpi)
	y2 := r.yoff - vg.Length(op.Y2*dpi)

	pt := r.pt
	if r.sty.Rotation != 0 {
		pt.X, pt.Y = r.rotate(pt.X, pt.Y)
	}

	pts := []vg.Point{
		vg.Point{X: x1, Y: y1}.Add(pt),
		vg.Point{X: x2, Y: y1}.Add(pt),
		vg.Point{X: x2, Y: y2}.Add(pt),
		vg.Point{X: x1, Y: y2}.Add(pt),
		vg.Point{X: x1, Y: y1}.Add(pt),
	}

	fillPolygon(r.cnv, r.sty.Color, pts)
}

func (r *latex) rotate(x, y vg.Length) (vg.Length, vg.Length) {
	u := x*r.cos + y*r.sin
	v := y*r.cos - x*r.sin
	return u, v
}

// FillPolygon fills a polygon with the given color.
func fillPolygon(c vg.Canvas, clr color.Color, pts []vg.Point) {
	if len(pts) == 0 {
		return
	}

	c.SetColor(clr)
	p := make(vg.Path, 0, len(pts)+1)
	p.Move(pts[0])
	for _, pt := range pts[1:] {
		p.Line(pt)
	}
	p.Close()
	c.Fill(p)
}
