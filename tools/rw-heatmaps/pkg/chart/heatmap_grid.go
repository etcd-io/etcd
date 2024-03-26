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
	"sort"

	"go.etcd.io/etcd/tools/rw-heatmaps/v3/pkg/dataset"
)

// heatMapGrid holds X, Y, Z values for a heatmap.
type heatMapGrid struct {
	x, y []float64
	z    [][]float64 // The Z values should be arranged in a 2D slice.
}

// newHeatMapGrid returns a new heatMapGrid.
func newHeatMapGrid(plotType string, records []dataset.DataRecord) *heatMapGrid {
	x, y := populateGridAxes(records)

	// Create a 2D slice to hold the Z values.
	z := make([][]float64, len(y))
	for i := range z {
		z[i] = make([]float64, len(x))
		for j := range z[i] {
			recordIndex := i*len(x) + j
			// If the recordIndex is out of range (incomplete data), break the loop.
			if recordIndex >= len(records) {
				break
			}
			record := records[recordIndex]
			if plotType == "read" {
				z[i][j] = record.AvgRead
			} else {
				z[i][j] = record.AvgWrite
			}
		}
	}

	return &heatMapGrid{x, y, z}
}

// newDeltaHeatMapGrid returns a new heatMapGrid for the delta heatmap.
func newDeltaHeatMapGrid(plotType string, records [][]dataset.DataRecord) *heatMapGrid {
	delta := make([]dataset.DataRecord, len(records[0]))
	for i := range records[0] {
		delta[i] = dataset.DataRecord{
			ConnSize:  records[0][i].ConnSize,
			ValueSize: records[0][i].ValueSize,
			AvgRead:   ((records[1][i].AvgRead - records[0][i].AvgRead) / records[0][i].AvgRead) * 100,
			AvgWrite:  ((records[1][i].AvgWrite - records[0][i].AvgWrite) / records[0][i].AvgWrite) * 100,
		}
	}

	return newHeatMapGrid(plotType, delta)
}

// Dims returns the number of elements in the grid.
// It implements the plotter.GridXYZ interface.
func (h *heatMapGrid) Dims() (int, int) {
	return len(h.x), len(h.y)
}

// Z returns the value of a grid cell at (c, r).
// It implements the plotter.GridXYZ interface.
func (h *heatMapGrid) Z(c, r int) float64 {
	return h.z[r][c]
}

// X returns the coordinate for the column at index c.
// It implements the plotter.GridXYZ interface.
func (h *heatMapGrid) X(c int) float64 {
	if c >= len(h.x) {
		panic("index out of range")
	}
	return h.x[c]
}

// Y returns the coordinate for the row at index r.
// It implements the plotter.GridXYZ interface.
func (h *heatMapGrid) Y(r int) float64 {
	if r >= len(h.y) {
		panic("index out of range")
	}
	return h.y[r]
}

// populateGridAxes populates the X and Y axes for the heatmap grid.
func populateGridAxes(records []dataset.DataRecord) ([]float64, []float64) {
	var xslice, yslice []float64

	for _, record := range records {
		xslice = append(xslice, float64(record.ConnSize))
		yslice = append(yslice, float64(record.ValueSize))
	}

	// Sort and deduplicate the slices
	xUnique := uniqueSortedFloats(xslice)
	yUnique := uniqueSortedFloats(yslice)

	return xUnique, yUnique
}

// uniqueSortedFloats returns a sorted slice of unique float64 values.
func uniqueSortedFloats(input []float64) []float64 {
	unique := make([]float64, 0)
	seen := make(map[float64]bool)

	for _, value := range input {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}

	sort.Float64s(unique)
	return unique
}
