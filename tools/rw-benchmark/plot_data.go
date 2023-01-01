// Copyright 2022 The etcd Authors
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

package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/ahrtr/gocontainer/map/linkedmap"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

const (
	minWidth      = 400
	minHeight     = 200
	defaultWidth  = 600
	defaultHeight = 300
)

type params struct {
	csvFiles []string
	outFile  string

	width   int // width(pixel) of each line chart
	height  int // height(pixel) of each line chart
	legends []string
	layout  components.Layout
}

func usage() string {
	return strings.TrimLeft(`
rw-benchmark is a tool for visualize etcd read-write performance result.

Usage:
    rw-benchmark [options] result-file1.csv [result-file2.csv]

Additional options:
    -legend: Comma separated names of legends, such as "main,pr", defaults to "1" or "1,2" depending on the number of CSV files provided.
    -layout: The layout of the page, valid values: none, center and flex, defaults to "flex".
    -width: The width(pixel) of the each line chart, defaults to 600.
    -height: The height(pixel) of the each line chart, defaults to 300.
    -o: The HTML file name in which the benchmark data will be rendered, defaults to "rw_benchmark.html".
    -h: Print usage.
`, "\n")
}

// parseParams parses all the options and arguments
func parseParams(args ...string) (params, error) {
	var ps params
	// Require at least one argument.
	//
	// Usually when users use a tool in the first time, they don't know how
	// to use it, so usually they just execute the command without any
	// arguments. So it would be better to display the usage instead of an
	// error message in this case.
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage())
		os.Exit(2)
	}

	var (
		help bool

		legend string
		layout string
		width  int
		height int

		outFile string

		err error
	)
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.BoolVar(&help, "h", false, "Print the usage")
	fs.IntVar(&width, "width", defaultWidth, "The width(pixel) of the each line chart.")
	fs.IntVar(&height, "height", defaultHeight, "The height(pixel) of the each line chart.")
	fs.StringVar(&legend, "legend", "", "The comma separated names of legends.")
	fs.StringVar(&layout, "layout", "flex", "The layout of the page, valid values: none, center and flex.")
	fs.StringVar(&outFile, "o", "rw_benchmark.html", "The HTML file name in which the benchmark data will be rendered.")
	if err = fs.Parse(args); err != nil {
		return ps, err
	} else if help {
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}

	// At most two csv files can be provided.
	if fs.NArg() < 1 || fs.NArg() > 2 {
		return ps, fmt.Errorf("unexpected number of arguments: %d, only 1 or 2 csv files are expected", fs.NArg())
	}

	if ps.layout, err = parseLayout(layout); err != nil {
		return ps, err
	}

	ps.width = width
	if ps.width < minWidth {
		ps.width = minWidth
	}
	ps.height = height
	if ps.height < minHeight {
		ps.height = minHeight
	}
	ps.outFile = outFile

	csvFile1 := fs.Arg(0)
	ps.csvFiles = append(ps.csvFiles, csvFile1)
	csvFile2 := fs.Arg(1)
	if len(csvFile2) > 0 {
		ps.csvFiles = append(ps.csvFiles, csvFile2)
	}

	if len(legend) > 0 {
		ps.legends = strings.Split(legend, ",")
	} else {
		// If no legend is provided, defaults to "1" and "2".
		for i := range ps.csvFiles {
			ps.legends = append(ps.legends, fmt.Sprintf("%d", i+1))
		}
	}

	if len(ps.legends) != len(ps.csvFiles) {
		return ps, fmt.Errorf("the number of legends(%d) doesn't match the number of csv files(%d)", len(ps.legends), len(ps.csvFiles))
	}

	return ps, nil
}

func parseLayout(layout string) (components.Layout, error) {
	switch layout {
	case "none":
		return components.PageNoneLayout, nil
	case "center":
		return components.PageCenterLayout, nil
	case "flex":
		return components.PageFlexLayout, nil
	default:
		return components.PageNoneLayout, fmt.Errorf("invalid layout %q", layout)
	}
}

// loadCSV loads the data in a given csv file
//
// The return value is a map with three levels:
//
//	Level 1 (lmRatio):     Ratio --> lmValueSize
//	Level 2 (lmValueSize): ValueSize --> lmConns
//	Level 3 (lmConns):     ConnSize --> [2]uint64
//
// In the value(array) of the 3rd map, the first value is the
// read-qps, the second value is the write-qps.
func loadCSV(filename string) (linkedmap.Interface, error) {
	// Open the CSV file
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open csv file %q, error: %w", filename, err)
	}
	defer f.Close()

	// Read the CSV file
	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read csv file %q, error: %w", filename, err)
	}

	lmRatio := linkedmap.New()
	// Parse the data
	for i, rec := range records {
		// When `REPEAT_COUNT` is 1, then there are 6 fields in each record.
		// Example:
		//     DATA,0.007,32,16,245.3039:35907.7856,
		if len(rec) >= 6 && rec[0] == "DATA" {
			// 0.007 in above example
			ratio, err := strconv.ParseFloat(rec[1], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ratio %q at file %q:%d, error: %w", rec[1], filename, i, err)
			}

			// 32 in above example
			conns, err := strconv.ParseUint(rec[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse conns %q at file %q:%d, error: %w", rec[2], filename, i, err)
			}

			// 16 in above example
			valSize, err := strconv.ParseUint(rec[3], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse value_size %q at file %q:%d, error: %w", rec[3], filename, i, err)
			}

			// parse all the QPS values. Note: the last column is empty.
			var (
				cnt         = len(rec) - 5
				avgReadQPS  float64
				avgWriteQPS float64
				sumReadQPS  float64
				sumWriteQPS float64
			)
			for j := 4; j < len(rec)-1; j++ {
				// 245.3039:35907.7856 in above example
				qps := strings.Split(rec[j], ":")
				if len(qps) != 2 {
					return nil, fmt.Errorf("unexpected qps values %q at file %q:%d", rec[j], filename, i)
				}
				readQPS, err := strconv.ParseFloat(qps[0], 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse read qps %q at file %q:%d, error: %w", qps[0], filename, i, err)
				}
				sumReadQPS += readQPS
				writeQPS, err := strconv.ParseFloat(qps[1], 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse write qps %q at file %q:%d, error: %w", qps[1], filename, i, err)
				}
				sumWriteQPS += writeQPS
			}
			avgReadQPS, avgWriteQPS = sumReadQPS/float64(cnt), sumWriteQPS/float64(cnt)

			// Save the data into LinkedMap.
			// The first level map: lmRatio
			var (
				lmValueSize linkedmap.Interface
				lmConn      linkedmap.Interface
			)
			lm := lmRatio.Get(ratio)
			if lm == nil {
				lmValueSize = linkedmap.New()
				lmRatio.Put(ratio, lmValueSize)
			} else {
				lmValueSize = lm.(linkedmap.Interface)
			}

			// The second level map: lmValueSize
			lm = lmValueSize.Get(valSize)
			if lm == nil {
				lmConn = linkedmap.New()
				lmValueSize.Put(valSize, lmConn)
			} else {
				lmConn = lm.(linkedmap.Interface)
			}

			// The third level map: lmConns
			lmConn.Put(conns, [2]uint64{uint64(avgReadQPS), uint64(avgWriteQPS)})
		}
	}
	return lmRatio, nil
}

func loadData(files ...string) ([]linkedmap.Interface, error) {
	var dataMaps []linkedmap.Interface
	for _, f := range files {
		lm, err := loadCSV(f)
		if err != nil {
			return nil, err
		}
		dataMaps = append(dataMaps, lm)
	}

	return dataMaps, nil
}

// convertBenchmarkData converts the benchmark data to format
// which is suitable for the line chart.
func convertBenchmarkData(lmConn linkedmap.Interface) ([]uint64, []uint64, []uint64) {
	var (
		conns []uint64
		rQPS  []uint64
		wQPS  []uint64
	)
	it, hasNext := lmConn.Iterator()
	var k, v interface{}
	for hasNext {
		k, v, hasNext = it()
		connSize := k.(uint64)
		rwQPS := v.([2]uint64)
		conns = append(conns, connSize)
		rQPS = append(rQPS, rwQPS[0])
		wQPS = append(wQPS, rwQPS[1])
	}

	return conns, rQPS, wQPS
}

func generateLineData(qps []uint64) []opts.LineData {
	items := make([]opts.LineData, 0)
	for _, v := range qps {
		items = append(items, opts.LineData{Value: v})
	}
	return items
}

// renderChart visualizes the benchmark data in a line chart.
// Note:
//  1. Each line chart is related to a ratio and valueSize combination.
//  2. The data in both CSV files are rendered in one line chart if the second file is present.
func renderChart(page *components.Page, ratio float64, valueSize uint64, conns []uint64, rQPSs [][]uint64, wQPSs [][]uint64, ps params) {
	// create a new line instance
	line := charts.NewLine()

	width := fmt.Sprintf("%dpx", ps.width)
	height := fmt.Sprintf("%dpx", ps.height)
	// set some global options like Title/Legend/ToolTip
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:  width,
			Height: height,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: fmt.Sprintf("R/W benchmark (RW Ratio: %g, Value Size: %d)", ratio, valueSize),
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: "QPS (Request/sec)",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "Connections",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true}),
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Orient: "vertical",
			Left:   "right",
			Top:    "middle",
		}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:   true,
			Orient: "horizontal",
			Right:  "100",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: true, Title: "Save as image"},
				DataView: &opts.ToolBoxFeatureDataView{
					Show:  true,
					Title: "Show as table",
					Lang:  []string{"Data view", "Turn off", "Refresh"},
				},
			}}))

	// Set data for X axis
	line.SetXAxis(conns)

	// Render read QPS from the first CSV file
	line.AddSeries(fmt.Sprintf("%s R", ps.legends[0]), generateLineData(rQPSs[0]),
		charts.WithLineStyleOpts(opts.LineStyle{
			Color: "Blue",
			Type:  "solid",
		}))

	// Render read QPS from the second CSV file
	if len(rQPSs) > 1 {
		line.AddSeries(fmt.Sprintf("%s R", ps.legends[1]), generateLineData(rQPSs[1]),
			charts.WithLineStyleOpts(opts.LineStyle{
				Color: "Blue",
				Type:  "dashed",
			}))
	}

	// Render write QPS from the first CSV file
	line.AddSeries(fmt.Sprintf("%s W", ps.legends[0]), generateLineData(wQPSs[0]),
		charts.WithLineStyleOpts(opts.LineStyle{
			Color: "Red",
			Type:  "solid",
		}))

	// Render write QPS from the second CSV file
	if len(wQPSs) > 1 {
		line.AddSeries(fmt.Sprintf("%s W", ps.legends[1]), generateLineData(wQPSs[1]),
			charts.WithLineStyleOpts(opts.LineStyle{
				Color: "Red",
				Type:  "dashed",
			}))
	}

	page.AddCharts(line)
}

// renderPage renders all data in one HTML page, which may contain multiple
// line charts, each of which is related to a read/write ratio and valueSize
// combination.
//
// Each element in the `dataMap` is a map with three levels, please see
// comment for function `loadCSV`.
func renderPage(dataMap []linkedmap.Interface, ps params) error {
	page := components.NewPage()
	page.SetLayout(ps.layout)

	it1, hasNext1 := dataMap[0].Iterator()
	var k1, v1 interface{}
	// Loop the first level map (lmRatio)
	for hasNext1 {
		k1, v1, hasNext1 = it1()

		ratio := k1.(float64)
		lmValueSize := v1.(linkedmap.Interface)

		// Loop the second level map (lmValueSize)
		it2, hasNext2 := lmValueSize.Iterator()
		var k2, v2 interface{}
		for hasNext2 {
			k2, v2, hasNext2 = it2()
			valueSize := k2.(uint64)
			lmConn := v2.(linkedmap.Interface)

			var (
				conns []uint64
				rQPSs [][]uint64
				wQPSs [][]uint64
			)
			// Loop the third level map (lmConn) to convert the benchmark data
			conns, rQPS1, wQPS1 := convertBenchmarkData(lmConn)
			rQPSs = append(rQPSs, rQPS1)
			wQPSs = append(wQPSs, wQPS1)

			// Convert the related benchmark data in the second CSV file if present.
			if len(dataMap) > 1 {
				if lm1 := dataMap[1].Get(ratio); lm1 != nil {
					lmValueSize2 := lm1.(linkedmap.Interface)

					if lm2 := lmValueSize2.Get(valueSize); lm2 != nil {
						lmConn2 := lm2.(linkedmap.Interface)
						conn2, rQPS2, wQPS2 := convertBenchmarkData(lmConn2)
						if reflect.DeepEqual(conns, conn2) {
							rQPSs = append(rQPSs, rQPS2)
							wQPSs = append(wQPSs, wQPS2)
						} else {
							fmt.Fprintf(os.Stderr, "[Ratio: %g, ValueSize: %d] ignore the benchmark data in the second CSV file due to different conns, %v vs %v\n",
								ratio, valueSize, conns, conn2)
						}
					} else {
						fmt.Fprintf(os.Stderr, "[Ratio: %g, ValueSize: %d] ignore the benchmark data in the second CSV file due to valueSize not found\n",
							ratio, valueSize)
					}
				} else {
					fmt.Fprintf(os.Stderr, "[Ratio: %g, ValueSize: %d] ignore the benchmark data in the second CSV file due to ratio not found\n",
						ratio, valueSize)
				}
			}

			renderChart(page, ratio, valueSize, conns, rQPSs, wQPSs, ps)
		}
	}

	f, err := os.Create(ps.outFile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	page.Render(io.MultiWriter(f))

	return nil
}

func main() {
	// parse CLI flags and arguments
	//legends, csvFiles, outFile, err := parseParams(os.Args[1:]...)
	ps, err := parseParams(os.Args[1:]...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse the parameters: %v\n", err)
		exit()
	}

	// load data of CSV files (1 or 2 files are expected)
	dataMap, err := loadData(ps.csvFiles...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load data file(s): %v\n", err)
		exit()
	}

	// render all data in one HTML page
	if err = renderPage(dataMap, ps); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render data to HTML page: %v\n", err)
		exit()
	}
}

func exit() {
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Run `rw-benchmark -h` to print usage info.\n")
	os.Exit(1)
}
