// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plot

import (
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/font/liberation"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

var (
	// DefaultFont is the name of the default font for plot text.
	DefaultFont = font.Font{
		Typeface: "Liberation",
		Variant:  "Serif",
	}

	// DefaultTextHandler is the default text handler used for text processing.
	DefaultTextHandler text.Handler
)

// Plot is the basic type representing a plot.
type Plot struct {
	Title struct {
		// Text is the text of the plot title.  If
		// Text is the empty string then the plot
		// will not have a title.
		Text string

		// Padding is the amount of padding
		// between the bottom of the title and
		// the top of the plot.
		Padding vg.Length

		// TextStyle specifies how the plot title text should be displayed.
		TextStyle text.Style
	}

	// BackgroundColor is the background color of the plot.
	// The default is White.
	BackgroundColor color.Color

	// X and Y are the horizontal and vertical axes
	// of the plot respectively.
	X, Y Axis

	// Legend is the plot's legend.
	Legend Legend

	// TextHandler parses and formats text according to a given
	// dialect (Markdown, LaTeX, plain, ...)
	// The default is a plain text handler.
	TextHandler text.Handler

	// plotters are drawn by calling their Plot method
	// after the axes are drawn.
	plotters []Plotter
}

// Plotter is an interface that wraps the Plot method.
// Some standard implementations of Plotter can be
// found in the gonum.org/v1/plot/plotter
// package, documented here:
// https://godoc.org/gonum.org/v1/plot/plotter
type Plotter interface {
	// Plot draws the data to a draw.Canvas.
	Plot(draw.Canvas, *Plot)
}

// DataRanger wraps the DataRange method.
type DataRanger interface {
	// DataRange returns the range of X and Y values.
	DataRange() (xmin, xmax, ymin, ymax float64)
}

// orientation describes whether an axis is horizontal or vertical.
type orientation byte

const (
	horizontal orientation = iota
	vertical
)

// New returns a new plot with some reasonable default settings.
func New() *Plot {
	hdlr := DefaultTextHandler
	p := &Plot{
		BackgroundColor: color.White,
		X:               makeAxis(horizontal),
		Y:               makeAxis(vertical),
		Legend:          newLegend(hdlr),
		TextHandler:     hdlr,
	}
	p.Title.TextStyle = text.Style{
		Color:   color.Black,
		Font:    font.From(DefaultFont, 12),
		XAlign:  draw.XCenter,
		YAlign:  draw.YTop,
		Handler: hdlr,
	}
	return p
}

// Add adds a Plotters to the plot.
//
// If the plotters implements DataRanger then the
// minimum and maximum values of the X and Y
// axes are changed if necessary to fit the range of
// the data.
//
// When drawing the plot, Plotters are drawn in the
// order in which they were added to the plot.
func (p *Plot) Add(ps ...Plotter) {
	for _, d := range ps {
		if x, ok := d.(DataRanger); ok {
			xmin, xmax, ymin, ymax := x.DataRange()
			p.X.Min = math.Min(p.X.Min, xmin)
			p.X.Max = math.Max(p.X.Max, xmax)
			p.Y.Min = math.Min(p.Y.Min, ymin)
			p.Y.Max = math.Max(p.Y.Max, ymax)
		}
	}

	p.plotters = append(p.plotters, ps...)
}

// Draw draws a plot to a draw.Canvas.
//
// Plotters are drawn in the order in which they were
// added to the plot.  Plotters that  implement the
// GlyphBoxer interface will have their GlyphBoxes
// taken into account when padding the plot so that
// none of their glyphs are clipped.
func (p *Plot) Draw(c draw.Canvas) {
	if p.BackgroundColor != nil {
		c.SetColor(p.BackgroundColor)
		c.Fill(c.Rectangle.Path())
	}

	if p.Title.Text != "" {
		descent := p.Title.TextStyle.FontExtents().Descent
		c.FillText(p.Title.TextStyle, vg.Point{X: c.Center().X, Y: c.Max.Y + descent}, p.Title.Text)

		rect := p.Title.TextStyle.Rectangle(p.Title.Text)
		c.Max.Y -= rect.Size().Y
		c.Max.Y -= p.Title.Padding
	}

	p.X.sanitizeRange()
	x := horizontalAxis{p.X}
	p.Y.sanitizeRange()
	y := verticalAxis{p.Y}

	ywidth := y.size()

	xheight := x.size()
	x.draw(padX(p, draw.Crop(c, ywidth, 0, 0, 0)))
	y.draw(padY(p, draw.Crop(c, 0, 0, xheight, 0)))

	dataC := padY(p, padX(p, draw.Crop(c, ywidth, 0, xheight, 0)))
	for _, data := range p.plotters {
		data.Plot(dataC, p)
	}

	p.Legend.Draw(draw.Crop(c, ywidth, 0, xheight, 0))
}

// DataCanvas returns a new draw.Canvas that
// is the subset of the given draw area into which
// the plot data will be drawn.
func (p *Plot) DataCanvas(da draw.Canvas) draw.Canvas {
	if p.Title.Text != "" {
		rect := p.Title.TextStyle.Rectangle(p.Title.Text)
		da.Max.Y -= rect.Size().Y
		da.Max.Y -= p.Title.Padding
	}
	p.X.sanitizeRange()
	x := horizontalAxis{p.X}
	p.Y.sanitizeRange()
	y := verticalAxis{p.Y}
	return padY(p, padX(p, draw.Crop(da, y.size(), 0, x.size(), 0)))
}

// DrawGlyphBoxes draws red outlines around the plot's
// GlyphBoxes.  This is intended for debugging.
func (p *Plot) DrawGlyphBoxes(c draw.Canvas) {
	dac := p.DataCanvas(c)
	sty := draw.LineStyle{
		Color: color.RGBA{R: 255, A: 255},
		Width: vg.Points(0.5),
	}

	drawBox := func(c draw.Canvas, b GlyphBox) {
		x := c.X(b.X) + b.Rectangle.Min.X
		y := c.Y(b.Y) + b.Rectangle.Min.Y
		c.StrokeLines(sty, []vg.Point{
			{X: x, Y: y},
			{X: x + b.Rectangle.Size().X, Y: y},
			{X: x + b.Rectangle.Size().X, Y: y + b.Rectangle.Size().Y},
			{X: x, Y: y + b.Rectangle.Size().Y},
			{X: x, Y: y},
		})
	}

	var title vg.Length
	if p.Title.Text != "" {
		rect := p.Title.TextStyle.Rectangle(p.Title.Text)
		title += rect.Size().Y
		title += p.Title.Padding
		box := GlyphBox{
			Rectangle: rect.Add(vg.Point{
				X: c.Center().X,
				Y: c.Max.Y,
			}),
		}
		drawBox(c, box)
	}

	for _, b := range p.GlyphBoxes(p) {
		drawBox(dac, b)
	}

	p.X.sanitizeRange()
	p.Y.sanitizeRange()

	x := horizontalAxis{p.X}
	y := verticalAxis{p.Y}

	ywidth := y.size()
	xheight := x.size()

	cx := padX(p, draw.Crop(c, ywidth, 0, 0, 0))
	for _, b := range x.GlyphBoxes(p) {
		drawBox(cx, b)
	}

	cy := padY(p, draw.Crop(c, 0, 0, xheight, 0))
	cy.Max.Y -= title
	for _, b := range y.GlyphBoxes(p) {
		drawBox(cy, b)
	}
}

// padX returns a draw.Canvas that is padded horizontally
// so that glyphs will no be clipped.
func padX(p *Plot, c draw.Canvas) draw.Canvas {
	glyphs := p.GlyphBoxes(p)
	l := leftMost(&c, glyphs)
	xAxis := horizontalAxis{p.X}
	glyphs = append(glyphs, xAxis.GlyphBoxes(p)...)
	r := rightMost(&c, glyphs)

	minx := c.Min.X - l.Min.X
	maxx := c.Max.X - (r.Min.X + r.Size().X)
	lx := vg.Length(l.X)
	rx := vg.Length(r.X)
	n := (lx*maxx - rx*minx) / (lx - rx)
	m := ((lx-1)*maxx - rx*minx + minx) / (lx - rx)
	return draw.Canvas{
		Canvas: vg.Canvas(c),
		Rectangle: vg.Rectangle{
			Min: vg.Point{X: n, Y: c.Min.Y},
			Max: vg.Point{X: m, Y: c.Max.Y},
		},
	}
}

// rightMost returns the right-most GlyphBox.
func rightMost(c *draw.Canvas, boxes []GlyphBox) GlyphBox {
	maxx := c.Max.X
	r := GlyphBox{X: 1}
	for _, b := range boxes {
		if b.Size().X <= 0 {
			continue
		}
		if x := c.X(b.X) + b.Min.X + b.Size().X; x > maxx && b.X <= 1 {
			maxx = x
			r = b
		}
	}
	return r
}

// leftMost returns the left-most GlyphBox.
func leftMost(c *draw.Canvas, boxes []GlyphBox) GlyphBox {
	minx := c.Min.X
	l := GlyphBox{}
	for _, b := range boxes {
		if b.Size().X <= 0 {
			continue
		}
		if x := c.X(b.X) + b.Min.X; x < minx && b.X >= 0 {
			minx = x
			l = b
		}
	}
	return l
}

// padY returns a draw.Canvas that is padded vertically
// so that glyphs will no be clipped.
func padY(p *Plot, c draw.Canvas) draw.Canvas {
	glyphs := p.GlyphBoxes(p)
	b := bottomMost(&c, glyphs)
	yAxis := verticalAxis{p.Y}
	glyphs = append(glyphs, yAxis.GlyphBoxes(p)...)
	t := topMost(&c, glyphs)

	miny := c.Min.Y - b.Min.Y
	maxy := c.Max.Y - (t.Min.Y + t.Size().Y)
	by := vg.Length(b.Y)
	ty := vg.Length(t.Y)
	n := (by*maxy - ty*miny) / (by - ty)
	m := ((by-1)*maxy - ty*miny + miny) / (by - ty)
	return draw.Canvas{
		Canvas: vg.Canvas(c),
		Rectangle: vg.Rectangle{
			Min: vg.Point{Y: n, X: c.Min.X},
			Max: vg.Point{Y: m, X: c.Max.X},
		},
	}
}

// topMost returns the top-most GlyphBox.
func topMost(c *draw.Canvas, boxes []GlyphBox) GlyphBox {
	maxy := c.Max.Y
	t := GlyphBox{Y: 1}
	for _, b := range boxes {
		if b.Size().Y <= 0 {
			continue
		}
		if y := c.Y(b.Y) + b.Min.Y + b.Size().Y; y > maxy && b.Y <= 1 {
			maxy = y
			t = b
		}
	}
	return t
}

// bottomMost returns the bottom-most GlyphBox.
func bottomMost(c *draw.Canvas, boxes []GlyphBox) GlyphBox {
	miny := c.Min.Y
	l := GlyphBox{}
	for _, b := range boxes {
		if b.Size().Y <= 0 {
			continue
		}
		if y := c.Y(b.Y) + b.Min.Y; y < miny && b.Y >= 0 {
			miny = y
			l = b
		}
	}
	return l
}

// Transforms returns functions to transfrom
// from the x and y data coordinate system to
// the draw coordinate system of the given
// draw area.
func (p *Plot) Transforms(c *draw.Canvas) (x, y func(float64) vg.Length) {
	x = func(x float64) vg.Length { return c.X(p.X.Norm(x)) }
	y = func(y float64) vg.Length { return c.Y(p.Y.Norm(y)) }
	return
}

// GlyphBoxer wraps the GlyphBoxes method.
// It should be implemented by things that meet
// the Plotter interface that draw glyphs so that
// their glyphs are not clipped if drawn near the
// edge of the draw.Canvas.
//
// When computing padding, the plot ignores
// GlyphBoxes as follows:
// If the Size.X > 0 and the X value is not in range
// of the X axis then the box is ignored.
// If Size.Y > 0 and the Y value is not in range of
// the Y axis then the box is ignored.
//
// Also, GlyphBoxes with Size.X <= 0 are ignored
// when computing horizontal padding and
// GlyphBoxes with Size.Y <= 0 are ignored when
// computing vertical padding.  This is useful
// for things like box plots and bar charts where
// the boxes and bars are considered to be glyphs
// in the X direction (and thus need padding), but
// may be clipped in the Y direction (and do not
// need padding).
type GlyphBoxer interface {
	GlyphBoxes(*Plot) []GlyphBox
}

// A GlyphBox describes the location of a glyph
// and the offset/size of its bounding box.
//
// If the Rectangle.Size().X is non-positive (<= 0) then
// the GlyphBox is ignored when computing the
// horizontal padding, and likewise with
// Rectangle.Size().Y and the vertical padding.
type GlyphBox struct {
	// The glyph location in normalized coordinates.
	X, Y float64

	// Rectangle is the offset of the glyph's minimum drawing
	// point relative to the glyph location and its size.
	vg.Rectangle
}

// GlyphBoxes returns the GlyphBoxes for all plot
// data that meet the GlyphBoxer interface.
func (p *Plot) GlyphBoxes(*Plot) (boxes []GlyphBox) {
	for _, d := range p.plotters {
		gb, ok := d.(GlyphBoxer)
		if !ok {
			continue
		}
		for _, b := range gb.GlyphBoxes(p) {
			if b.Size().X > 0 && (b.X < 0 || b.X > 1) {
				continue
			}
			if b.Size().Y > 0 && (b.Y < 0 || b.Y > 1) {
				continue
			}
			boxes = append(boxes, b)
		}
	}
	return
}

// NominalX configures the plot to have a nominal X
// axis—an X axis with names instead of numbers.  The
// X location corresponding to each name are the integers,
// e.g., the x value 0 is centered above the first name and
// 1 is above the second name, etc.  Labels for x values
// that do not end up in range of the X axis will not have
// tick marks.
func (p *Plot) NominalX(names ...string) {
	p.X.Tick.Width = 0
	p.X.Tick.Length = 0
	p.X.Width = 0
	p.Y.Padding = p.X.Tick.Label.Width(names[0]) / 2
	ticks := make([]Tick, len(names))
	for i, name := range names {
		ticks[i] = Tick{float64(i), name}
	}
	p.X.Tick.Marker = ConstantTicks(ticks)
}

// HideX configures the X axis so that it will not be drawn.
func (p *Plot) HideX() {
	p.X.Tick.Length = 0
	p.X.Width = 0
	p.X.Tick.Marker = ConstantTicks([]Tick{})
}

// HideY configures the Y axis so that it will not be drawn.
func (p *Plot) HideY() {
	p.Y.Tick.Length = 0
	p.Y.Width = 0
	p.Y.Tick.Marker = ConstantTicks([]Tick{})
}

// HideAxes hides the X and Y axes.
func (p *Plot) HideAxes() {
	p.HideX()
	p.HideY()
}

// NominalY is like NominalX, but for the Y axis.
func (p *Plot) NominalY(names ...string) {
	p.Y.Tick.Width = 0
	p.Y.Tick.Length = 0
	p.Y.Width = 0
	p.X.Padding = p.Y.Tick.Label.Height(names[0]) / 2
	ticks := make([]Tick, len(names))
	for i, name := range names {
		ticks[i] = Tick{float64(i), name}
	}
	p.Y.Tick.Marker = ConstantTicks(ticks)
}

// WriterTo returns an io.WriterTo that will write the plot as
// the specified image format.
//
// Supported formats are:
//
//   - .eps
//   - .jpg|.jpeg
//   - .pdf
//   - .png
//   - .svg
//   - .tex
//   - .tif|.tiff
func (p *Plot) WriterTo(w, h vg.Length, format string) (io.WriterTo, error) {
	c, err := draw.NewFormattedCanvas(w, h, format)
	if err != nil {
		return nil, err
	}
	p.Draw(draw.New(c))
	return c, nil
}

// Save saves the plot to an image file.  The file format is determined
// by the extension.
//
// Supported extensions are:
//
//   - .eps
//   - .jpg|.jpeg
//   - .pdf
//   - .png
//   - .svg
//   - .tex
//   - .tif|.tiff
func (p *Plot) Save(w, h vg.Length, file string) (err error) {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer func() {
		e := f.Close()
		if err == nil {
			err = e
		}
	}()

	format := strings.ToLower(filepath.Ext(file))
	if len(format) != 0 {
		format = format[1:]
	}
	c, err := p.WriterTo(w, h, format)
	if err != nil {
		return err
	}

	_, err = c.WriteTo(f)
	return err
}

func init() {
	font.DefaultCache.Add(liberation.Collection())
	DefaultTextHandler = text.Plain{
		Fonts: font.DefaultCache,
	}
}
