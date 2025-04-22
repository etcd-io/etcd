// Copyright 2025 The etcd Authors
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
	"cmp"
	"fmt"
	"image/color"
	"slices"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"

	"go.etcd.io/etcd/tools/rw-heatmaps/v3/pkg/dataset"
)

// PlotLineCharts creates a new line chart.
func PlotLineCharts(datasets []*dataset.DataSet, title, outputImageFile, outputFormat string) error {
	plot.DefaultFont = font.Font{
		Typeface: "Liberation",
		Variant:  "Sans",
	}

	canvas := plotLineChart(datasets, title)
	return saveCanvas(canvas, "readwrite", outputImageFile, outputFormat)
}

func plotLineChart(datasets []*dataset.DataSet, title string) *vgimg.Canvas {
	ratiosLength := func() int {
		max := slices.MaxFunc(datasets, func(a, b *dataset.DataSet) int {
			return cmp.Compare(len(a.GetSortedRatios()), len(b.GetSortedRatios()))
		})
		return len(max.GetSortedRatios())
	}()

	// Make a nx1 grid of line charts.
	const cols = 1
	rows := ratiosLength

	// Set the width and height of the canvas.
	width, height := 30*vg.Centimeter, 15*font.Length(ratiosLength)*vg.Centimeter

	canvas := vgimg.New(width, height)
	dc := draw.New(canvas)

	// Create a tiled layout for the plots.
	t := draw.Tiles{
		Rows:      rows,
		Cols:      cols,
		PadX:      vg.Millimeter * 4,
		PadY:      vg.Millimeter * 4,
		PadTop:    vg.Millimeter * 15,
		PadBottom: vg.Millimeter * 2,
		PadLeft:   vg.Millimeter * 2,
		PadRight:  vg.Millimeter * 2,
	}

	plots := make([][]*plot.Plot, rows)
	legends := make([]plot.Legend, rows)
	for i := range plots {
		plots[i] = make([]*plot.Plot, cols)
	}

	// Load records into the grid.
	ratios := slices.MaxFunc(datasets, func(a, b *dataset.DataSet) int {
		return cmp.Compare(len(a.GetSortedRatios()), len(b.GetSortedRatios()))
	}).GetSortedRatios()

	for row, ratio := range ratios {
		var records [][]dataset.DataRecord
		var fileNames []string
		for _, d := range datasets {
			records = append(records, d.Records[ratio])
			fileNames = append(fileNames, d.FileName)
		}
		p, l := plotIndividualLineChart(fmt.Sprintf("R/W Ratio %0.04f", ratio), records, fileNames)
		plots[row] = []*plot.Plot{p}
		legends[row] = l
	}

	// Fill the canvas with the plots and legends.
	canvases := plot.Align(plots, t, dc)
	for i := 0; i < rows; i++ {
		// Continue if there is no plot in the current cell (incomplete data).
		if plots[i][0] == nil {
			continue
		}

		l := legends[i]
		r := l.Rectangle(canvases[i][0])
		legendWidth := r.Max.X - r.Min.X
		// Adjust the legend down a little.
		l.YOffs = plots[i][0].Title.TextStyle.FontExtents().Height * 3
		l.Draw(canvases[i][0])

		c := draw.Crop(canvases[i][0], 0, -legendWidth-vg.Millimeter, 0, 0)
		plots[i][0].Draw(c)
	}

	// Add the title and parameter legend.
	l := plot.NewLegend()
	l.Add(title)
	for _, d := range datasets {
		l.Add(fmt.Sprintf("%s: %s", d.FileName, d.Param))
	}
	l.Top = true
	l.Left = true
	l.Draw(dc)

	return canvas
}

func plotIndividualLineChart(title string, records [][]dataset.DataRecord, fileNames []string) (*plot.Plot, plot.Legend) {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = "Connections Amount"
	p.X.Scale = plot.LogScale{}
	p.X.Tick.Marker = pow2Ticks{}
	p.Y.Label.Text = "QPS (Requests/sec)"
	p.Y.Scale = plot.LogScale{}
	p.Y.Tick.Marker = pow2Ticks{}

	legend := plot.NewLegend()

	values := getSortedValueSizes(records...)
	for i, rs := range records {
		rec := make(map[int64][]dataset.DataRecord)
		for _, r := range rs {
			rec[r.ValueSize] = append(rec[r.ValueSize], r)
		}
		if len(records) > 1 {
			addValues(p, &legend, values, rec, i, fileNames[i])
		} else {
			addValues(p, &legend, values, rec, i, "")
		}
	}

	return p, legend
}

func getSortedValueSizes(records ...[]dataset.DataRecord) []int {
	valueMap := make(map[int64]struct{})
	for _, rs := range records {
		for _, r := range rs {
			valueMap[r.ValueSize] = struct{}{}
		}
	}

	var values []int
	for v := range valueMap {
		values = append(values, int(v))
	}
	sort.Ints(values)

	return values
}

func addValues(p *plot.Plot, legend *plot.Legend, values []int, rec map[int64][]dataset.DataRecord, index int, fileName string) {
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

		readLine, s, err := plotter.NewLinePoints(readPts)
		if err != nil {
			panic(err)
		}
		if index == 0 {
			readLine.Color = plotutil.Color(0)
		} else {
			readLine.Color = plotutil.Color(2)
		}
		readLine.Width = vg.Length(vg.Millimeter * 0.15 * vg.Length(i+1))
		readLine.Dashes = []vg.Length{vg.Points(6), vg.Points(2)}
		s.Color = readLine.Color
		p.Add(readLine, s)

		writeLine, s, err := plotter.NewLinePoints(writePts)
		if err != nil {
			panic(err)
		}
		if index == 0 {
			writeLine.Color = plotutil.Color(0)
		} else {
			writeLine.Color = plotutil.Color(2)
		}
		writeLine.Width = vg.Length(vg.Millimeter * 0.15 * vg.Length(i+1))
		s.Color = writeLine.Color
		p.Add(writeLine, s)

		if index == 0 {
			l, _, _ := plotter.NewLinePoints(writePts)
			l.Color = color.RGBA{0, 0, 0, 255}
			l.Width = vg.Length(vg.Millimeter * 0.15 * vg.Length(i+1))
			legend.Add(fmt.Sprintf("%d", value), plot.Thumbnailer(l))
		}
		if i == len(values)-1 {
			legend.Add(fmt.Sprintf("read %s", fileName), plot.Thumbnailer(readLine))
			legend.Add(fmt.Sprintf("write %s", fileName), plot.Thumbnailer(writeLine))
		}
	}
}
