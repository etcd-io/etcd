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

package dataset

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	_ = iota
	// fieldIndexRatio is the index of the ratio field in the CSV file.
	fieldIndexRatio
	// fieldIndexConnSize is the index of the connection size (connSize) field in the CSV file.
	fieldIndexConnSize
	// fieldIndexValueSize is the index of the value size (valueSize) field in the CSV file.
	fieldIndexValueSize
	// fieldIndexIterOffset is the index of the first iteration field in the CSV file.
	fieldIndexIterOffset
)

// DataSet holds the data for the heatmaps, including the parameter used for the run.
type DataSet struct {
	// FileName is the name of the file from which the data was loaded.
	FileName string
	// Records is a map from the ratio of read to write operations to the data for that ratio.
	Records map[float64][]DataRecord
	// Param is the parameter used for the run.
	Param string
}

// DataRecord holds the data for a single heatmap chart.
type DataRecord struct {
	ConnSize  int64
	ValueSize int64
	AvgRead   float64
	AvgWrite  float64
}

// GetSortedRatios returns the sorted ratios of read to write operations in the dataset.
func (d *DataSet) GetSortedRatios() []float64 {
	ratios := make([]float64, 0)
	for ratio := range d.Records {
		ratios = append(ratios, ratio)
	}
	sort.Float64s(ratios)
	return ratios
}

// LoadCSVData loads the data from a CSV file into a DataSet.
func LoadCSVData(inputFile string) (*DataSet, error) {
	file, err := os.Open(inputFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	records := make(map[float64][]DataRecord)

	// Count the number of iterations.
	iters := 0
	for _, header := range lines[0][fieldIndexIterOffset:] {
		if strings.HasPrefix(header, "iter") {
			iters++
		}
	}

	// Running parameters are stored in the first line after the header, after the iteration fields.
	param := lines[1][fieldIndexIterOffset+iters]

	for _, line := range lines[2:] { // Skip header line.
		ratio, _ := strconv.ParseFloat(line[fieldIndexRatio], 64)
		if _, ok := records[ratio]; !ok {
			records[ratio] = make([]DataRecord, 0)
		}
		connSize, _ := strconv.ParseInt(line[fieldIndexConnSize], 10, 64)
		valueSize, _ := strconv.ParseInt(line[fieldIndexValueSize], 10, 64)

		// Calculate the average read and write values for the iterations.
		var readSum, writeSum float64
		for _, v := range line[fieldIndexIterOffset : fieldIndexIterOffset+iters] {
			splitted := strings.Split(v, ":")

			readValue, _ := strconv.ParseFloat(splitted[0], 64)
			readSum += readValue

			writeValue, _ := strconv.ParseFloat(splitted[1], 64)
			writeSum += writeValue
		}

		records[ratio] = append(records[ratio], DataRecord{
			ConnSize:  connSize,
			ValueSize: valueSize,
			AvgRead:   readSum / float64(iters),
			AvgWrite:  writeSum / float64(iters),
		})
	}

	return &DataSet{FileName: filepath.Base(inputFile), Records: records, Param: param}, nil
}
