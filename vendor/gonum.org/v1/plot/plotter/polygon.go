// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image/color"
	"math"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Polygon implements the Plotter interface, drawing a polygon.
type Polygon struct {
	// XYs is a copy of the vertices of this polygon.
	// Each item in the array holds one ring in the
	// Polygon.
	XYs []XYs

	// LineStyle is the style of the line around the edge
	// of the polygon.
	draw.LineStyle

	// Color is the fill color of the polygon.
	Color color.Color
}

// NewPolygon returns a polygon that uses the default line style and
// no fill color, where xys are the rings of the polygon.
// Different backends may render overlapping rings and self-intersections
// differently, but all built-in backends treat inner rings
// with the opposite winding order from the outer ring as
// holes.
func NewPolygon(xys ...XYer) (*Polygon, error) {
	data := make([]XYs, len(xys))
	for i, d := range xys {
		var err error
		data[i], err = CopyXYs(d)
		if err != nil {
			return nil, err
		}
	}
	return &Polygon{
		XYs:       data,
		LineStyle: DefaultLineStyle,
	}, nil
}

// Plot draws the polygon, implementing the plot.Plotter
// interface.
func (pts *Polygon) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	ps := make([][]vg.Point, len(pts.XYs))

	for i, ring := range pts.XYs {
		ps[i] = make([]vg.Point, len(ring))
		for j, p := range ring {
			ps[i][j].X = trX(p.X)
			ps[i][j].Y = trY(p.Y)
		}
		ps[i] = c.ClipPolygonXY(ps[i])
	}
	if pts.Color != nil && len(ps) > 0 {
		c.SetColor(pts.Color)
		// allocate enough space for at least 4 path components per ring.
		// 3 is the minimum but 4 is more common.
		pa := make(vg.Path, 0, 4*len(ps))
		for _, ring := range ps {
			if len(ring) == 0 {
				continue
			}
			pa.Move(ring[0])
			for _, p := range ring[1:] {
				pa.Line(p)
			}
			pa.Close()
		}
		c.Fill(pa)
	}

	for _, ring := range ps {
		if len(ring) > 0 && ring[len(ring)-1] != ring[0] {
			ring = append(ring, ring[0])
		}
		c.StrokeLines(pts.LineStyle, c.ClipLinesXY(ring)...)
	}
}

// DataRange returns the minimum and maximum
// x and y values, implementing the plot.DataRanger
// interface.
func (pts *Polygon) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin = math.Inf(1)
	xmax = math.Inf(-1)
	ymin = math.Inf(1)
	ymax = math.Inf(-1)
	for _, ring := range pts.XYs {
		xmini, xmaxi := Range(XValues{ring})
		ymini, ymaxi := Range(YValues{ring})
		xmin = math.Min(xmin, xmini)
		xmax = math.Max(xmax, xmaxi)
		ymin = math.Min(ymin, ymini)
		ymax = math.Max(ymax, ymaxi)
	}
	return
}

// Thumbnail creates the thumbnail for the Polygon,
// implementing the plot.Thumbnailer interface.
func (pts *Polygon) Thumbnail(c *draw.Canvas) {
	if pts.Color != nil {
		points := []vg.Point{
			{X: c.Min.X, Y: c.Min.Y},
			{X: c.Min.X, Y: c.Max.Y},
			{X: c.Max.X, Y: c.Max.Y},
			{X: c.Max.X, Y: c.Min.Y},
		}
		poly := c.ClipPolygonY(points)
		c.FillPolygon(pts.Color, poly)

		points = append(points, vg.Point{X: c.Min.X, Y: c.Min.Y})
		c.StrokeLines(pts.LineStyle, points)
	} else {
		y := c.Center().Y
		c.StrokeLine2(pts.LineStyle, c.Min.X, y, c.Max.X, y)
	}
}
