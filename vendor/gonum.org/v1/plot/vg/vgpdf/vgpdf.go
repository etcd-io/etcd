// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vgpdf implements the vg.Canvas interface
// using gofpdf (github.com/phpdave11/gofpdf).
package vgpdf // import "gonum.org/v1/plot/vg/vgpdf"

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"

	pdf "codeberg.org/go-pdf/fpdf"
	stdfnt "golang.org/x/image/font"

	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// codePageEncoding holds informations about the characters encoding of TrueType
// font files, needed by gofpdf to embed fonts in a PDF document.
// We use cp1252 (code page 1252, Windows Western) to encode characters.
// See:
//   - https://en.wikipedia.org/wiki/Windows-1252
//
// TODO: provide a Canvas-level func option to embed fonts with a user provided
// code page schema?
//
//go:embed cp1252.map
var codePageEncoding []byte

func init() {
	draw.RegisterFormat("pdf", func(w, h vg.Length) vg.CanvasWriterTo {
		return New(w, h)
	})
}

// DPI is the nominal resolution of drawing in PDF.
const DPI = 72

// Canvas implements the vg.Canvas interface,
// drawing to a PDF.
type Canvas struct {
	doc  *pdf.Fpdf
	w, h vg.Length

	dpi       int
	numImages int
	stack     []context
	fonts     map[font.Font]struct{}

	// Switch to embed fonts in PDF file.
	// The default is to embed fonts.
	// This makes the PDF file more portable but also larger.
	embed bool
}

type context struct {
	fill  color.Color
	line  color.Color
	width vg.Length
}

// New creates a new PDF Canvas.
func New(w, h vg.Length) *Canvas {
	cfg := pdf.InitType{
		UnitStr: "pt",
		Size:    pdf.SizeType{Wd: w.Points(), Ht: h.Points()},
	}
	c := &Canvas{
		doc:   pdf.NewCustom(&cfg),
		w:     w,
		h:     h,
		dpi:   DPI,
		stack: make([]context, 1),
		fonts: make(map[font.Font]struct{}),
		embed: true,
	}
	c.NextPage()
	vg.Initialize(c)
	return c
}

// EmbedFonts specifies whether the resulting PDF canvas should
// embed the fonts or not.
// EmbedFonts returns the previous value before modification.
func (c *Canvas) EmbedFonts(v bool) bool {
	prev := c.embed
	c.embed = v
	return prev
}

func (c *Canvas) DPI() float64 {
	return float64(c.dpi)
}

func (c *Canvas) context() *context {
	return &c.stack[len(c.stack)-1]
}

func (c *Canvas) Size() (w, h vg.Length) {
	return c.w, c.h
}

func (c *Canvas) SetLineWidth(w vg.Length) {
	c.context().width = w
	lw := c.unit(w)
	c.doc.SetLineWidth(lw)
}

func (c *Canvas) SetLineDash(dashes []vg.Length, offs vg.Length) {
	ds := make([]float64, len(dashes))
	for i, d := range dashes {
		ds[i] = c.unit(d)
	}
	c.doc.SetDashPattern(ds, c.unit(offs))
}

func (c *Canvas) SetColor(clr color.Color) {
	if clr == nil {
		clr = color.Black
	}
	c.context().line = clr
	c.context().fill = clr
	r, g, b, a := rgba(clr)
	c.doc.SetFillColor(r, g, b)
	c.doc.SetDrawColor(r, g, b)
	c.doc.SetTextColor(r, g, b)
	c.doc.SetAlpha(a, "Normal")
}

func (c *Canvas) Rotate(r float64) {
	c.doc.TransformRotate(-r*180/math.Pi, 0, 0)
}

func (c *Canvas) Translate(pt vg.Point) {
	xp, yp := c.pdfPoint(pt)
	c.doc.TransformTranslate(xp, yp)
}

func (c *Canvas) Scale(x float64, y float64) {
	c.doc.TransformScale(x*100, y*100, 0, 0)
}

func (c *Canvas) Push() {
	c.stack = append(c.stack, *c.context())
	c.doc.TransformBegin()
}

func (c *Canvas) Pop() {
	c.doc.TransformEnd()
	c.stack = c.stack[:len(c.stack)-1]
}

func (c *Canvas) Stroke(p vg.Path) {
	if c.context().width > 0 {
		c.pdfPath(p, "D")
	}
}

func (c *Canvas) Fill(p vg.Path) {
	c.pdfPath(p, "F")
}

func (c *Canvas) FillString(fnt font.Face, pt vg.Point, str string) {
	if fnt.Font.Size == 0 {
		return
	}

	c.font(fnt, pt)
	style := ""
	if fnt.Font.Weight == stdfnt.WeightBold {
		style += "B"
	}
	if fnt.Font.Style == stdfnt.StyleItalic {
		style += "I"
	}
	c.doc.SetFont(fnt.Name(), style, c.unit(fnt.Font.Size))

	c.Push()
	defer c.Pop()
	c.Translate(pt)
	// go-fpdf uses the top left corner as origin.
	c.Scale(1, -1)
	left, top, right, bottom := c.sbounds(fnt, str)
	w := right - left
	h := bottom - top
	margin := c.doc.GetCellMargin()

	c.doc.MoveTo(-left-margin, top)
	c.doc.CellFormat(w, h, str, "", 0, "BL", false, 0, "")
}

func (c *Canvas) sbounds(fnt font.Face, txt string) (left, top, right, bottom float64) {
	_, h := c.doc.GetFontSize()
	style := ""
	if fnt.Font.Weight == stdfnt.WeightBold {
		style += "B"
	}
	if fnt.Font.Style == stdfnt.StyleItalic {
		style += "I"
	}
	d := c.doc.GetFontDesc(fnt.Name(), style)
	if d.Ascent == 0 {
		// not defined (standard font?), use average of 81%
		top = 0.81 * h
	} else {
		top = -float64(d.Ascent) * h / float64(d.Ascent-d.Descent)
	}
	return 0, top, c.doc.GetStringWidth(txt), top + h
}

// DrawImage implements the vg.Canvas.DrawImage method.
func (c *Canvas) DrawImage(rect vg.Rectangle, img image.Image) {
	opts := pdf.ImageOptions{ImageType: "png", ReadDpi: true}
	name := c.imageName()

	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		log.Panicf("error encoding image to PNG: %v", err)
	}
	c.doc.RegisterImageOptionsReader(name, opts, buf)

	xp, yp := c.pdfPoint(rect.Min)
	wp, hp := c.pdfPoint(rect.Size())

	c.doc.ImageOptions(name, xp, yp, wp, hp, false, opts, 0, "")
}

// font registers a font and a size with the PDF canvas.
func (c *Canvas) font(fnt font.Face, pt vg.Point) {
	if _, ok := c.fonts[fnt.Font]; ok {
		return
	}
	name := fnt.Name()
	key := fontKey{font: fnt, embed: c.embed}
	raw := new(bytes.Buffer)
	_, err := fnt.Face.WriteSourceTo(nil, raw)
	if err != nil {
		log.Panicf("vgpdf: could not generate font %q data for PDF: %+v", name, err)
	}

	zdata, jdata, err := getFont(key, raw.Bytes(), codePageEncoding)
	if err != nil {
		log.Panicf("vgpdf: could not generate font data for PDF: %v", err)
	}

	c.fonts[fnt.Font] = struct{}{}
	c.doc.AddFontFromBytes(name, "", jdata, zdata)
}

// pdfPath processes a vg.Path and applies it to the canvas.
func (c *Canvas) pdfPath(path vg.Path, style string) {
	var (
		xp float64
		yp float64
	)
	for _, comp := range path {
		switch comp.Type {
		case vg.MoveComp:
			xp, yp = c.pdfPoint(comp.Pos)
			c.doc.MoveTo(xp, yp)
		case vg.LineComp:
			c.doc.LineTo(c.pdfPoint(comp.Pos))
		case vg.ArcComp:
			c.arc(comp, style)
		case vg.CurveComp:
			px, py := c.pdfPoint(comp.Pos)
			switch len(comp.Control) {
			case 1:
				cx, cy := c.pdfPoint(comp.Control[0])
				c.doc.CurveTo(cx, cy, px, py)
			case 2:
				cx, cy := c.pdfPoint(comp.Control[0])
				dx, dy := c.pdfPoint(comp.Control[1])
				c.doc.CurveBezierCubicTo(cx, cy, dx, dy, px, py)
			default:
				panic("vgpdf: invalid number of control points")
			}
		case vg.CloseComp:
			c.doc.LineTo(xp, yp)
			c.doc.ClosePath()
		default:
			panic(fmt.Sprintf("Unknown path component type: %d\n", comp.Type))
		}
	}
	c.doc.DrawPath(style)
}

func (c *Canvas) arc(comp vg.PathComp, style string) {
	x0 := comp.Pos.X + comp.Radius*vg.Length(math.Cos(comp.Start))
	y0 := comp.Pos.Y + comp.Radius*vg.Length(math.Sin(comp.Start))
	c.doc.LineTo(c.pdfPointXY(x0, y0))
	r := c.unit(comp.Radius)
	const deg = 180 / math.Pi
	angle := comp.Angle * deg
	beg := comp.Start * deg
	end := beg + angle
	x := c.unit(comp.Pos.X)
	y := c.unit(comp.Pos.Y)
	c.doc.Arc(x, y, r, r, angle, beg, end, style)
	x1 := comp.Pos.X + comp.Radius*vg.Length(math.Cos(comp.Start+comp.Angle))
	y1 := comp.Pos.Y + comp.Radius*vg.Length(math.Sin(comp.Start+comp.Angle))
	c.doc.MoveTo(c.pdfPointXY(x1, y1))
}

func (c *Canvas) pdfPointXY(x, y vg.Length) (float64, float64) {
	return c.unit(x), c.unit(y)
}

func (c *Canvas) pdfPoint(pt vg.Point) (float64, float64) {
	return c.unit(pt.X), c.unit(pt.Y)
}

// unit returns a fpdf.Unit, converted from a vg.Length.
func (c *Canvas) unit(l vg.Length) float64 {
	return l.Dots(c.DPI())
}

// imageName generates a unique image name for this PDF canvas
func (c *Canvas) imageName() string {
	c.numImages++
	return fmt.Sprintf("image_%03d.png", c.numImages)
}

// WriterCounter implements the io.Writer interface, and counts
// the total number of bytes written.
type writerCounter struct {
	io.Writer
	n int64
}

func (w *writerCounter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.n += int64(n)
	return n, err
}

// WriteTo writes the Canvas to an io.Writer.
// After calling Write, the canvas is closed
// and may no longer be used for drawing.
func (c *Canvas) WriteTo(w io.Writer) (int64, error) {
	c.Pop()
	c.doc.Close()
	wc := writerCounter{Writer: w}
	b := bufio.NewWriter(&wc)
	if err := c.doc.Output(b); err != nil {
		return wc.n, err
	}
	err := b.Flush()
	return wc.n, err
}

// rgba converts a Go color into a gofpdf 3-tuple int + 1 float64
func rgba(c color.Color) (int, int, int, float64) {
	if c == nil {
		c = color.Black
	}
	r, g, b, a := c.RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8), float64(a) / math.MaxUint16
}

type fontsCache struct {
	sync.RWMutex
	cache map[fontKey]fontVal
}

// fontKey represents a PDF font request.
// fontKey needs to know whether the font will be embedded or not,
// as gofpdf.MakeFont will generate different informations.
type fontKey struct {
	font  font.Face
	embed bool
}

type fontVal struct {
	z, j []byte
}

func (c *fontsCache) get(key fontKey) (fontVal, bool) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.cache[key]
	return v, ok
}

func (c *fontsCache) add(k fontKey, v fontVal) {
	c.Lock()
	defer c.Unlock()
	c.cache[k] = v
}

var pdfFonts = &fontsCache{
	cache: make(map[fontKey]fontVal),
}

func getFont(key fontKey, font, encoding []byte) (z, j []byte, err error) {
	if v, ok := pdfFonts.get(key); ok {
		return v.z, v.j, nil
	}

	v, err := makeFont(key, font, encoding)
	if err != nil {
		return nil, nil, err
	}
	return v.z, v.j, nil
}

func makeFont(key fontKey, font, encoding []byte) (val fontVal, err error) {
	tmpdir, err := os.MkdirTemp("", "gofpdf-makefont-")
	if err != nil {
		return val, err
	}
	defer os.RemoveAll(tmpdir)

	indir := filepath.Join(tmpdir, "input")
	err = os.Mkdir(indir, 0755)
	if err != nil {
		return val, err
	}

	outdir := filepath.Join(tmpdir, "output")
	err = os.Mkdir(outdir, 0755)
	if err != nil {
		return val, err
	}

	fname := filepath.Join(indir, "font.ttf")
	encname := filepath.Join(indir, "cp1252.map")

	err = os.WriteFile(fname, font, 0644)
	if err != nil {
		return val, err
	}

	err = os.WriteFile(encname, encoding, 0644)
	if err != nil {
		return val, err
	}

	err = pdf.MakeFont(fname, encname, outdir, io.Discard, key.embed)
	if err != nil {
		return val, err
	}

	if key.embed {
		z, err := os.ReadFile(filepath.Join(outdir, "font.z"))
		if err != nil {
			return val, err
		}
		val.z = z
	}

	j, err := os.ReadFile(filepath.Join(outdir, "font.json"))
	if err != nil {
		return val, err
	}
	val.j = j

	pdfFonts.add(key, val)

	return val, nil
}

// NextPage creates a new page in the final PDF document.
// The new page is the new current page.
// Modifications applied to the canvas will only be applied to that new page.
func (c *Canvas) NextPage() {
	if c.doc.PageNo() > 0 {
		c.Pop()
	}
	c.doc.SetMargins(0, 0, 0)
	c.doc.AddPage()
	c.Push()
	c.Translate(vg.Point{X: 0, Y: c.h})
	c.Scale(1, -1)
}
