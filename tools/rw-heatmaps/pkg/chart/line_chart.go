// Copyright 2024 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chart

import (
	"fmt"
	"image/color"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"

	"go.etcd.io/etcd/tools/rw-heatmaps/v3/pkg/dataset"
)

/*
type lineChart struct {
}
*/

// PlotLineCharts creates a new line chart.
func PlotLineCharts(datasets []*dataset.DataSet, title, outputImageFile, outputFormat string) error {
	plot.DefaultFont = font.Font{
		Typeface: "Liberation",
		Variant:  "Sans",
	}

	var canvas *vgimg.Canvas
	canvas = plotLineChart(datasets, title)
	if err := saveCanvas(canvas, "readwrite", outputImageFile, outputFormat); err != nil {
		return err
	}

	return nil
}

func plotLineChart(datasets []*dataset.DataSet, title string) *vgimg.Canvas {
	// Make a 4x2 grid of heatmaps.
	const rows, cols = 8, 1

	// Set the width and height of the canvas.
	const width, height = 30 * vg.Centimeter, 100 * vg.Centimeter

	canvas := vgimg.New(width, height)
	dc := draw.New(canvas)

	// Create a tiled layout for the plots.
	t := draw.Tiles{
		Rows:      rows,
		Cols:      cols,
		PadX:      vg.Millimeter * 4,
		PadY:      vg.Millimeter * 4,
		PadTop:    vg.Millimeter * 10,
		PadBottom: vg.Millimeter * 2,
		PadLeft:   vg.Millimeter * 2,
		PadRight:  vg.Millimeter * 2,
	}

	plots := make([][]*plot.Plot, rows)
	legends := make([][]plot.Legend, rows)
	for i := range plots {
		plots[i] = make([]*plot.Plot, cols)
		legends[i] = make([]plot.Legend, cols)
	}

	// Load records into the grid.
	ratios := datasets[0].GetSortedRatios()
	row, col := 0, 0
	for _, ratio := range ratios {
		records := datasets[0].Records[ratio]
		p, l := plotIndividualLineChart(fmt.Sprintf("R/W Ratio %0.04f", ratio), records)
		plots[row][col] = p
		legends[row][col] = l

		if col++; col == cols {
			col = 0
			row++
		}
	}

	// Fill the canvas with the plots and legends.
	canvases := plot.Align(plots, t, dc)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			// Continue if there is no plot in the current cell (incomplete data).
			if plots[i][j] == nil {
				continue
			}

			l := legends[i][j]
			r := l.Rectangle(canvases[i][j])
			legendWidth := r.Max.X - r.Min.X
			// Adjust the legend down a little.
			l.YOffs = plots[i][j].Title.TextStyle.FontExtents().Height * 3
			l.Draw(canvases[i][j])

			c := draw.Crop(canvases[i][j], 0, -legendWidth-vg.Millimeter, 0, 0)
			plots[i][j].Draw(c)
		}
	}

	// Add the title and parameter legend.
	l := plot.NewLegend()
	l.Add(title)
	l.Add(datasets[0].Param)
	l.Top = true
	l.Left = true
	l.Draw(dc)

	return canvas
}

func plotIndividualLineChart(title string, records []dataset.DataRecord) (*plot.Plot, plot.Legend) {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = "Connections Amount"
	p.X.Scale = plot.LogScale{}
	p.X.Tick.Marker = pow2Ticks{}
	p.Y.Label.Text = "QPS (Requests/sec)"
	p.Y.Scale = plot.LogScale{}
	p.Y.Tick.Marker = pow2Ticks{}

	legend := plot.NewLegend()

	var values []int
	rec := make(map[int64][]dataset.DataRecord)
	for _, r := range records {
		if _, ok := rec[r.ValueSize]; !ok {
			values = append(values, int(r.ValueSize))
		}
		rec[r.ValueSize] = append(rec[r.ValueSize], r)
	}
	sort.Ints(values)

	var DefaultGlyphShapes = []draw.GlyphDrawer{
		draw.RingGlyph{},
		draw.SquareGlyph{},
		draw.TriangleGlyph{},
		draw.CrossGlyph{},
		draw.PlusGlyph{},
		draw.CircleGlyph{},
		draw.BoxGlyph{},
		draw.PyramidGlyph{},
	}

	for i, value := range values {
		r := rec[int64(value)]
		readPts := make(plotter.XYs, len(r))
		writePts := make(plotter.XYs, len(r))
		for i, record := range r {
			writePts[i].X = float64(record.ConnSize)
			readPts[i].X = writePts[i].X
			readPts[i].Y = record.AvgRead
			writePts[i].Y = record.AvgWrite
		}

		l, s, err := plotter.NewLinePoints(readPts)
		if err != nil {
			panic(err)
		}
		l.Color = color.RGBA{241, 90, 96, 255}
		s.Color = l.Color
		s.Shape = DefaultGlyphShapes[i%len(DefaultGlyphShapes)]
		p.Add(l, s)
		if i == 0 {
			legend.Add("read", plot.Thumbnailer(l))
		}

		l, s, err = plotter.NewLinePoints(writePts)
		if err != nil {
			panic(err)
		}
		l.Color = color.RGBA{90, 155, 212, 255}
		//l.Dashes = []vg.Length{vg.Points(6), vg.Points(2)}
		s.Color = l.Color
		s.Shape = DefaultGlyphShapes[i%len(DefaultGlyphShapes)]
		p.Add(l, s)
		if i == 0 {
			legend.Add("write", plot.Thumbnailer(l))
		}

		sc, _ := plotter.NewScatter(writePts)
		sc.Color = color.RGBA{0, 0, 0, 255}
		sc.Shape = s.Shape
		sc.XYs = s.XYs
		legend.Add(fmt.Sprintf("%d", value), plot.Thumbnailer(sc))
	}

	return p, legend
}
