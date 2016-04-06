// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.  See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Tyler Neely (t@jujit.su)

package loghisto

import (
	"fmt"
	"os"
	"runtime"
	"text/tabwriter"
	"time"
)

// PrintBenchmark will run the provided function at the specified
// concurrency, time the operation, and once per second write the
// following information to standard out:
//
// 2014-08-09 17:44:57 -0400 EDT
// raft_AppendLogEntries_count:        16488
// raft_AppendLogEntries_max:          3.982478339757623e+07
// raft_AppendLogEntries_99.99:        3.864778314316012e+07
// raft_AppendLogEntries_99.9:         3.4366224772310276e+06
// raft_AppendLogEntries_99:           2.0228126576114902e+06
// raft_AppendLogEntries_50:           469769.7083161708
// raft_AppendLogEntries_min:          129313.15075081984
// raft_AppendLogEntries_sum:          9.975892639594093e+09
// raft_AppendLogEntries_avg:          605039.5827022133
// raft_AppendLogEntries_agg_avg:      618937
// raft_AppendLogEntries_agg_count:    121095
// raft_AppendLogEntries_agg_sum:      7.4950269894e+10
// sys.Alloc:                          997328
// sys.NumGC:                          1115
// sys.PauseTotalNs:                   2.94946542e+08
// sys.NumGoroutine:                   26
func PrintBenchmark(name string, concurrency uint, op func()) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var ms = NewMetricSystem(time.Second, true)
	mc := make(chan *ProcessedMetricSet, 1)
	ms.SubscribeToProcessedMetrics(mc)
	ms.Start()
	defer ms.Stop()

	go receiver(name, mc)

	for i := uint(0); i < concurrency; i++ {
		go func() {
			for {
				timer := ms.StartTimer(name)
				op()
				timer.Stop()
			}
		}()
	}

	<-make(chan struct{})
}

func receiver(name string, mc chan *ProcessedMetricSet) {
	interesting := []string{
		fmt.Sprintf("%s_count", name),
		fmt.Sprintf("%s_max", name),
		fmt.Sprintf("%s_99.99", name),
		fmt.Sprintf("%s_99.9", name),
		fmt.Sprintf("%s_99", name),
		fmt.Sprintf("%s_95", name),
		fmt.Sprintf("%s_90", name),
		fmt.Sprintf("%s_75", name),
		fmt.Sprintf("%s_50", name),
		fmt.Sprintf("%s_min", name),
		fmt.Sprintf("%s_sum", name),
		fmt.Sprintf("%s_avg", name),
		fmt.Sprintf("%s_agg_avg", name),
		fmt.Sprintf("%s_agg_count", name),
		fmt.Sprintf("%s_agg_sum", name),
		"sys.Alloc",
		"sys.NumGC",
		"sys.PauseTotalNs",
		"sys.NumGoroutine",
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)

	for m := range mc {
		fmt.Fprintln(w, m.Time)
		for _, e := range interesting {
			fmt.Fprintln(w, fmt.Sprintf("%s:\t", e), m.Metrics[e])
		}
		fmt.Fprintln(w)
		w.Flush()
	}
}
