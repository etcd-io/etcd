// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var (
	libraryNameToMarkdownName = map[string]string{
		"Zap":                   ":zap: zap",
		"Zap.Sugar":             ":zap: zap (sugared)",
		"stdlib.Println":        "standard library",
		"sirupsen/logrus":       "logrus",
		"go-kit/kit/log":        "go-kit",
		"inconshreveable/log15": "log15",
		"apex/log":              "apex/log",
		"go.pedge.io/lion":      "lion",
		"rs/zerolog":            "zerolog",
	}
)

func main() {
	flag.Parse()
	if err := do(); err != nil {
		log.Fatal(err)
	}
}

func do() error {
	tmplData, err := getTmplData()
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	t, err := template.New("tmpl").Parse(string(data))
	if err != nil {
		return err
	}
	return t.Execute(os.Stdout, tmplData)
}

func getTmplData() (*tmplData, error) {
	tmplData := &tmplData{}
	rows, err := getBenchmarkRows("BenchmarkAddingFields")
	if err != nil {
		return nil, err
	}
	tmplData.BenchmarkAddingFields = rows
	rows, err = getBenchmarkRows("BenchmarkAccumulatedContext")
	if err != nil {
		return nil, err
	}
	tmplData.BenchmarkAccumulatedContext = rows
	rows, err = getBenchmarkRows("BenchmarkWithoutFields")
	if err != nil {
		return nil, err
	}
	tmplData.BenchmarkWithoutFields = rows
	return tmplData, nil
}

func getBenchmarkRows(benchmarkName string) (string, error) {
	benchmarkOutput, err := getBenchmarkOutput(benchmarkName)
	if err != nil {
		return "", err
	}
	var benchmarkRows []*benchmarkRow
	for libraryName := range libraryNameToMarkdownName {
		benchmarkRow, err := getBenchmarkRow(benchmarkOutput, benchmarkName, libraryName)
		if err != nil {
			return "", err
		}
		if benchmarkRow == nil {
			continue
		}
		benchmarkRows = append(benchmarkRows, benchmarkRow)
	}
	sort.Sort(benchmarkRowsByTime(benchmarkRows))
	rows := []string{
		"| Package | Time | Objects Allocated |",
		"| :--- | :---: | :---: |",
	}
	for _, benchmarkRow := range benchmarkRows {
		rows = append(rows, benchmarkRow.String())
	}
	return strings.Join(rows, "\n"), nil
}

func getBenchmarkRow(input []string, benchmarkName string, libraryName string) (*benchmarkRow, error) {
	line, err := findUniqueSubstring(input, fmt.Sprintf("%s/%s-", benchmarkName, libraryName))
	if err != nil {
		return nil, err
	}
	if line == "" {
		return nil, nil
	}
	split := strings.Split(line, "\t")
	if len(split) < 5 {
		return nil, fmt.Errorf("unknown benchmark line: %s", line)
	}
	duration, err := time.ParseDuration(strings.Replace(strings.TrimSuffix(strings.TrimSpace(split[2]), "/op"), " ", "", -1))
	if err != nil {
		return nil, err
	}
	allocatedBytes, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(split[3]), " B/op"))
	if err != nil {
		return nil, err
	}
	allocatedObjects, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(split[4]), " allocs/op"))
	if err != nil {
		return nil, err
	}
	return &benchmarkRow{
		libraryNameToMarkdownName[libraryName],
		duration,
		allocatedBytes,
		allocatedObjects,
	}, nil
}

func findUniqueSubstring(input []string, substring string) (string, error) {
	var output string
	for _, line := range input {
		if strings.Contains(line, substring) {
			if output != "" {
				return "", fmt.Errorf("input has duplicate substring %s", substring)
			}
			output = line
		}
	}
	return output, nil
}

func getBenchmarkOutput(benchmarkName string) ([]string, error) {
	return getOutput("go", "test", fmt.Sprintf("-bench=%s", benchmarkName), "-benchmem", "./benchmarks")
}

func getOutput(name string, arg ...string) ([]string, error) {
	output, err := exec.Command(name, arg...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running %s %s: %v\n%s", name, strings.Join(arg, " "), err, string(output))
	}
	return strings.Split(string(output), "\n"), nil
}

type tmplData struct {
	BenchmarkAddingFields       string
	BenchmarkAccumulatedContext string
	BenchmarkWithoutFields      string
}

type benchmarkRow struct {
	Name             string
	Time             time.Duration
	AllocatedBytes   int
	AllocatedObjects int
}

func (b *benchmarkRow) String() string {
	return fmt.Sprintf("| %s | %d ns/op | %d allocs/op |", b.Name, b.Time.Nanoseconds(), b.AllocatedObjects)
}

type benchmarkRowsByTime []*benchmarkRow

func (b benchmarkRowsByTime) Len() int      { return len(b) }
func (b benchmarkRowsByTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b benchmarkRowsByTime) Less(i, j int) bool {
	left, right := b[i], b[j]
	leftZap, rightZap := strings.Contains(left.Name, "zap"), strings.Contains(right.Name, "zap")

	// If neither benchmark is for zap or both are, sort by time.
	if !(leftZap || rightZap) || (leftZap && rightZap) {
		return left.Time.Nanoseconds() < right.Time.Nanoseconds()
	}
	// Sort zap benchmark first.
	return leftZap
}
