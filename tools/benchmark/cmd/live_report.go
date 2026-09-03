// Copyright 2026 The etcd Authors
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

package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// This file provides shared helpers for periodic live reporting of benchmark
// metrics, enabled when --report-interval is set to a positive value.
// Commands that support this feature and must emit a JSON-lines stream of
// in-progress stats to stdout (keeping the final summary on stderr) use the
// types and functions in this file.

// intervalTracker accumulates operation latencies for one reporting window.
// Call snapshot() at the end of each tick to drain the window and get a copy.
type intervalTracker struct {
	mu    sync.Mutex
	lats  []float64
	start time.Time
}

func newIntervalTracker() *intervalTracker {
	return &intervalTracker{start: time.Now()}
}

// add records the latency of a single completed operation.
func (t *intervalTracker) add(dur time.Duration) {
	t.mu.Lock()
	t.lats = append(t.lats, dur.Seconds())
	t.mu.Unlock()
}

// snapshot returns a copy of the latencies accumulated since the last snapshot
// (or since creation), together with the elapsed duration of that window in
// seconds, and then resets the window for the next interval.
func (t *intervalTracker) snapshot() (lats []float64, elapsed float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	lats = append([]float64(nil), t.lats...)
	elapsed = time.Since(t.start).Seconds()
	t.lats = nil
	t.start = time.Now()
	return lats, elapsed
}

// summarize computes descriptive statistics for a set of latency samples (in
// seconds) observed over an elapsed interval (in seconds).
func summarize(lats []float64, elapsed float64) (ops int, rps, avg, stddev, p50, p90, p99 float64) {
	ops = len(lats)
	if ops == 0 || elapsed == 0 {
		return ops, rps, avg, stddev, p50, p90, p99
	}

	cp := append([]float64(nil), lats...)
	sort.Float64s(cp)

	var sum float64
	for _, v := range cp {
		sum += v
	}
	avg = sum / float64(ops)
	rps = float64(ops) / elapsed

	var variance float64
	for _, v := range cp {
		d := v - avg
		variance += d * d
	}
	stddev = math.Sqrt(variance / float64(ops))

	idx := func(p float64) int {
		return int(p / 100.0 * float64(len(cp)-1))
	}
	p50 = cp[idx(50)]
	p90 = cp[idx(90)]
	p99 = cp[idx(99)]
	return ops, rps, avg, stddev, p50, p90, p99
}

// singleLiveSnapshot is the JSON record emitted to stdout on each tick by
// single-metric commands (put, range, txn-put, stm). Latency fields use
// seconds as the unit, matching the existing report package convention.
type singleLiveSnapshot struct {
	ID        uint64  `json:"id"`
	Timestamp string  `json:"ts"`
	Elapsed   float64 `json:"elapsed_sec"`
	Ops       int     `json:"ops"`
	RPS       float64 `json:"rps"`
	Avg       float64 `json:"avg_s"`
	StdDev    float64 `json:"stddev_s"`
	P50       float64 `json:"p50_s"`
	P90       float64 `json:"p90_s"`
	P99       float64 `json:"p99_s"`
}

// startSingleLiveReporter launches a background goroutine that emits one
// singleLiveSnapshot JSON line to stdout every intervalSec seconds.
// Close the returned channel to stop the reporter cleanly.
func startSingleLiveReporter(intervalSec int, tracker *intervalTracker) chan struct{} {
	stop := make(chan struct{})
	var id uint64
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				lats, elapsed := tracker.snapshot()
				if len(lats) == 0 {
					continue
				}
				ops, rps, avg, stddev, p50, p90, p99 := summarize(lats, elapsed)
				snap := singleLiveSnapshot{
					ID:        atomic.AddUint64(&id, 1),
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Elapsed:   elapsed,
					Ops:       ops,
					RPS:       rps,
					Avg:       avg,
					StdDev:    stddev,
					P50:       p50,
					P90:       p90,
					P99:       p99,
				}
				b, err := json.Marshal(snap)
				if err != nil {
					fmt.Fprintf(os.Stderr, "live report marshal error: %v\n", err)
					continue
				}
				fmt.Fprintln(os.Stdout, string(b))
			case <-stop:
				return
			}
		}
	}()
	return stop
}
