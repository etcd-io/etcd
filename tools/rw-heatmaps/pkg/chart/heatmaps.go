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
	"io"
	"math"
	"os"
	"strings"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/palette/brewer"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"

	"go.etcd.io/etcd/tools/rw-heatmaps/v3/pkg/dataset"
)

// pow2Ticks is a type that implements the plot.Ticker interface for log2 scale.
type pow2Ticks struct{}

// Ticks returns the ticks for the log2 scale.
// It implements the plot.Ticker interface.
func (pow2Ticks) Ticks(min, max float64) []plot.Tick {
	var t []plot.Tick
	for i := math.Log2(min); math.Pow(2, i) <= max; i++ {
		t = append(t, plot.Tick{
			Value: math.Pow(2, i),
			Label: fmt.Sprintf("2^%d", int(i)),
		})
	}
	return t
}

// invertedPalette takes an existing palette and inverts it.
type invertedPalette struct {
	base palette.Palette
}

// Colors returns the sequence of colors in reverse order from the base palette.
// It implements the palette.Palette interface.
func (p invertedPalette) Colors() []color.Color {
	baseColors := p.base.Colors()
	invertedColors := make([]color.Color, len(baseColors))
	for i, c := range baseColors {
		invertedColors[len(baseColors)-i-1] = c
	}
	return invertedColors
}

// PlotHeatMaps plots, and saves the heatmaps for the given dataset.
func PlotHeatMaps(datasets []*dataset.DataSet, title, outputImageFile, outputFormat string, zeroCentered bool) error {
	plot.DefaultFont = font.Font{
		Typeface: "Liberation",
		Variant:  "Sans",
	}

	for _, plotType := range []string{"read", "write"} {
		var canvas *vgimg.Canvas
		if len(datasets) == 1 {
			canvas = plotHeatMapGrid(datasets[0], title, plotType)
		} else {
			canvas = plotComparisonHeatMapGrid(datasets, title, plotType, zeroCentered)
		}
		if err := saveCanvas(canvas, plotType, outputImageFile, outputFormat); err != nil {
			return err
		}
	}
	return nil
}

// plotHeatMapGrid plots a grid of heatmaps for the given dataset.
func plotHeatMapGrid(dataset *dataset.DataSet, title, plotType string) *vgimg.Canvas {
	// Make a 4x2 grid of heatmaps.
	const rows, cols = 4, 2

	// Set the width and height of the canvas.
	const width, height = 30 * vg.Centimeter, 40 * vg.Centimeter

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

	// Store the plots and legends (scale label) in a grid.
	plots := make([][]*plot.Plot, rows)
	legends := make([][]plot.Legend, rows)
	for i := range plots {
		plots[i] = make([]*plot.Plot, cols)
		legends[i] = make([]plot.Legend, cols)
	}

	// Load records into the grid.
	ratios := dataset.GetSortedRatios()
	row, col := 0, 0
	for _, ratio := range ratios {
		records := dataset.Records[ratio]
		p, l := plotIndividualHeatMap(fmt.Sprintf("R/W Ratio %0.04f", ratio), plotType, records)
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
			l.YOffs = -plots[i][j].Title.TextStyle.FontExtents().Height
			l.Draw(canvases[i][j])

			// Crop the plot to make space for the legend.
			c := draw.Crop(canvases[i][j], 0, -legendWidth-vg.Millimeter, 0, 0)
			plots[i][j].Draw(c)
		}
	}

	// Add the title and parameter legend.
	l := plot.NewLegend()
	l.Add(fmt.Sprintf("%s [%s]", title, strings.ToUpper(plotType)))
	l.Add(dataset.Param)
	l.Top = true
	l.Left = true
	l.Draw(dc)

	return canvas
}

// plotComparisonHeatMapGrid plots a grid of heatmaps for the given datasets.
func plotComparisonHeatMapGrid(datasets []*dataset.DataSet, title, plotType string, zeroCentered bool) *vgimg.Canvas {
	// Make a 8x3 grid of heatmaps.
	const rows, cols = 8, 3
	// Set the width and height of the canvas.
	const width, height = 40 * vg.Centimeter, 66 * vg.Centimeter

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

	// Store the plots and legends (scale label) in a grid.
	plots := make([][]*plot.Plot, rows)
	legends := make([][]plot.Legend, rows)
	for i := range plots {
		plots[i] = make([]*plot.Plot, cols)
		legends[i] = make([]plot.Legend, cols)
	}

	// Load records into the grid.
	ratios := datasets[0].GetSortedRatios()
	for row, ratio := range ratios {
		records := make([][]dataset.DataRecord, len(datasets))
		for col, dataset := range datasets {
			r := dataset.Records[ratio]
			p, l := plotIndividualHeatMap(fmt.Sprintf("R/W Ratio %0.04f", ratio), plotType, r)
			// Add the title to the first row.
			if row == 0 {
				p.Title.Text = fmt.Sprintf("%s\n%s", dataset.FileName, p.Title.Text)
			}

			plots[row][col] = p
			legends[row][col] = l
			records[col] = r
		}
		plots[row][2], legends[row][2] = plotDeltaHeatMap(fmt.Sprintf("R/W Ratio %0.04f", ratio), plotType, records, zeroCentered)
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
			l.YOffs = -plots[i][j].Title.TextStyle.FontExtents().Height
			l.Draw(canvases[i][j])

			// Crop the plot to make space for the legend.
			c := draw.Crop(canvases[i][j], 0, -legendWidth-vg.Millimeter, 0, 0)
			plots[i][j].Draw(c)
		}
	}

	// Add the title and parameter legend.
	l := plot.NewLegend()
	l.Add(fmt.Sprintf("%s [%s]", title, strings.ToUpper(plotType)))
	for _, dataset := range datasets {
		l.Add(fmt.Sprintf("%s: %s", dataset.FileName, dataset.Param))
	}
	l.Top = true
	l.Left = true
	l.Draw(dc)

	return canvas
}

// saveCanvas saves the canvas to a file.
func saveCanvas(canvas *vgimg.Canvas, plotType, outputImageFile, outputFormat string) error {
	f, err := os.Create(fmt.Sprintf("%s_%s.%s", outputImageFile, plotType, outputFormat))
	if err != nil {
		return err
	}
	defer f.Close()

	var w io.WriterTo
	switch outputFormat {
	case "png":
		w = vgimg.PngCanvas{Canvas: canvas}
	case "jpeg", "jpg":
		w = vgimg.PngCanvas{Canvas: canvas}
	case "tiff":
		w = vgimg.TiffCanvas{Canvas: canvas}
	}

	_, err = w.WriteTo(f)
	return err
}

// plotIndividualHeatMap plots a heatmap for a given set of records.
func plotIndividualHeatMap(title, plotType string, records []dataset.DataRecord) (*plot.Plot, plot.Legend) {
	p := plot.New()
	p.X.Scale = plot.LogScale{}
	p.X.Tick.Marker = pow2Ticks{}
	p.X.Label.Text = "Connections Amount"
	p.Y.Scale = plot.LogScale{}
	p.Y.Tick.Marker = pow2Ticks{}
	p.Y.Label.Text = "Value Size"

	gridData := newHeatMapGrid(plotType, records)

	// Use the YlGnBu color palette from ColorBrewer to match the original implementation.
	colors, _ := brewer.GetPalette(brewer.TypeAny, "YlGnBu", 9)
	pal := invertedPalette{colors}
	h := plotter.NewHeatMap(gridData, pal)

	p.Title.Text = fmt.Sprintf("%s [%.2f, %.2f]", title, h.Min, h.Max)
	p.Add(h)

	// Create a legend with the scale.
	legend := generateScaleLegend(h.Min, h.Max, pal)

	return p, legend
}

// plotDeltaHeatMap plots a heatmap for the delta between two sets of records.
func plotDeltaHeatMap(title, plotType string, records [][]dataset.DataRecord, zeroCentered bool) (*plot.Plot, plot.Legend) {
	p := plot.New()
	p.X.Scale = plot.LogScale{}
	p.X.Tick.Marker = pow2Ticks{}
	p.X.Label.Text = "Connections Amount"
	p.Y.Scale = plot.LogScale{}
	p.Y.Tick.Marker = pow2Ticks{}
	p.Y.Label.Text = "Value Size"

	gridData := newDeltaHeatMapGrid(plotType, records)

	// Use the RdBu color palette from ColorBrewer to match the original implementation.
	colors, _ := brewer.GetPalette(brewer.TypeAny, "RdBu", 11)
	pal := invertedPalette{colors}
	h := plotter.NewHeatMap(gridData, pal)
	p.Title.Text = fmt.Sprintf("%s [%.2f%%, %.2f%%]", title, h.Min, h.Max)

	if zeroCentered {
		if h.Min < 0 && math.Abs(h.Min) > h.Max {
			h.Max = math.Abs(h.Min)
		} else {
			h.Min = h.Max * -1
		}
	}

	p.Add(h)

	// Create a legend with the scale.
	legend := generateScaleLegend(h.Min, h.Max, pal)

	return p, legend
}

// generateScaleLegend generates legends for the heatmap.
func generateScaleLegend(min, max float64, pal palette.Palette) plot.Legend {
	legend := plot.NewLegend()
	thumbs := plotter.PaletteThumbnailers(pal)
	step := (max - min) / float64(len(thumbs)-1)
	for i := len(thumbs) - 1; i >= 0; i-- {
		legend.Add(fmt.Sprintf("%.0f", min+step*float64(i)), thumbs[i])
	}
	legend.Top = true

	return legend
}
