// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// the file is borrowed from github.com/rakyll/boom/boomer/print.go

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	barChar = "∎"
)

type result struct {
	errStr   string
	duration time.Duration
}

type report struct {
	avgTotal float64
	fastest  float64
	slowest  float64
	average  float64
	rps      float64

	results chan *result
	total   time.Duration

	errorDist map[string]int
	lats      []float64
}

func printReport(size int, results chan *result, total time.Duration) {
	r := &report{
		results:   results,
		total:     total,
		errorDist: make(map[string]int),
	}
	r.finalize()
}

func (r *report) finalize() {
	for {
		select {
		case res := <-r.results:
			if res.errStr != "" {
				r.errorDist[res.errStr]++
			} else {
				r.lats = append(r.lats, res.duration.Seconds())
				r.avgTotal += res.duration.Seconds()
			}
		default:
			r.rps = float64(len(r.lats)) / r.total.Seconds()
			r.average = r.avgTotal / float64(len(r.lats))
			r.print()
			return
		}
	}
}

func (r *report) print() {
	sort.Float64s(r.lats)

	if len(r.lats) > 0 {
		r.fastest = r.lats[0]
		r.slowest = r.lats[len(r.lats)-1]
		fmt.Printf("\nSummary:\n")
		fmt.Printf("  Total:\t%4.4f secs.\n", r.total.Seconds())
		fmt.Printf("  Slowest:\t%4.4f secs.\n", r.slowest)
		fmt.Printf("  Fastest:\t%4.4f secs.\n", r.fastest)
		fmt.Printf("  Average:\t%4.4f secs.\n", r.average)
		fmt.Printf("  Requests/sec:\t%4.4f\n", r.rps)
		r.printHistogram()
		r.printLatencies()
	}

	if len(r.errorDist) > 0 {
		r.printErrors()
	}
}

// Prints percentile latencies.
func (r *report) printLatencies() {
	pctls := []int{10, 25, 50, 75, 90, 95, 99}
	data := make([]float64, len(pctls))
	j := 0
	for i := 0; i < len(r.lats) && j < len(pctls); i++ {
		current := i * 100 / len(r.lats)
		if current >= pctls[j] {
			data[j] = r.lats[i]
			j++
		}
	}
	fmt.Printf("\nLatency distribution:\n")
	for i := 0; i < len(pctls); i++ {
		if data[i] > 0 {
			fmt.Printf("  %v%% in %4.4f secs.\n", pctls[i], data[i])
		}
	}
}

func (r *report) printHistogram() {
	bc := 10
	buckets := make([]float64, bc+1)
	counts := make([]int, bc+1)
	bs := (r.slowest - r.fastest) / float64(bc)
	for i := 0; i < bc; i++ {
		buckets[i] = r.fastest + bs*float64(i)
	}
	buckets[bc] = r.slowest
	var bi int
	var max int
	for i := 0; i < len(r.lats); {
		if r.lats[i] <= buckets[bi] {
			i++
			counts[bi]++
			if max < counts[bi] {
				max = counts[bi]
			}
		} else if bi < len(buckets)-1 {
			bi++
		}
	}
	fmt.Printf("\nResponse time histogram:\n")
	for i := 0; i < len(buckets); i++ {
		// Normalize bar lengths.
		var barLen int
		if max > 0 {
			barLen = counts[i] * 40 / max
		}
		fmt.Printf("  %4.3f [%v]\t|%v\n", buckets[i], counts[i], strings.Repeat(barChar, barLen))
	}
}

func (r *report) printErrors() {
	fmt.Printf("\nError distribution:\n")
	for err, num := range r.errorDist {
		fmt.Printf("  [%d]\t%s\n", num, err)
	}
}
