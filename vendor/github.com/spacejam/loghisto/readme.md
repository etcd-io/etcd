loghisto
============
[![Build Status](https://travis-ci.org/spacejam/loghisto.svg)](https://travis-ci.org/spacejam/loghisto)

A metric system for high performance counters and histograms.  Unlike popular metric systems today, this does not destroy the accuracy of histograms by sampling.  Instead, a logarithmic bucketing function compresses values, generally within 1% of their true value (although between 0 and 1 the precision loss may not be within this boundary).  This allows for extreme compression, which allows us to calculate arbitrarily high percentiles with no loss of accuracy - just a small amount of precision.  This is particularly useful for highly-clustered events that are tolerant of a small precision loss, but for which you REALLY care about what the tail looks like, such as measuring latency across a distributed system.

Copied out of my work for the CockroachDB metrics system.  Based on an algorithm created by Keith Frost.


### running a print benchmark for quick analysis
```go
package main

import (
  "runtime"
  "github.com/spacejam/loghisto"
)

func benchmark() {
  // do some stuff
}

func main() {
  numCPU := runtime.NumCPU()
  runtime.GOMAXPROCS(numCPU)

  desiredConcurrency := uint(100)
  loghisto.PrintBenchmark("benchmark1234", desiredConcurrency, benchmark)
}
```
results in something like this printed to stdout each second:
```
2014-12-11 21:41:45 -0500 EST
benchmark1234_count:     2.0171025e+07
benchmark1234_max:       2.4642914167480484e+07
benchmark1234_99.99:     4913.768840299134
benchmark1234_99.9:      1001.2472422902518
benchmark1234_99:        71.24044000732538
benchmark1234_95:        67.03348428941965
benchmark1234_90:        65.68633104092515
benchmark1234_75:        63.07152259993664
benchmark1234_50:        58.739891704145194
benchmark1234_min:       -657.5233632152207           // Corollary: time.Since(time.Now()) is often < 0
benchmark1234_sum:       1.648051169322668e+09
benchmark1234_avg:       81.70388809307748
benchmark1234_agg_avg:   89
benchmark1234_agg_count: 6.0962226e+07
benchmark1234_agg_sum:   5.454779078e+09
sys.Alloc:               1.132672e+06
sys.NumGC:               5741
sys.PauseTotalNs:        1.569390954e+09
sys.NumGoroutine:        113
```
### adding an embedded metric system to your code
```go
import (
  "time"
  "fmt"
  "github.com/spacejam/loghisto"
)
func ExampleMetricSystem() {
  // Create metric system that reports once a minute, and includes stats
  // about goroutines, memory usage and GC.
  includeGoProcessStats := true
  ms := loghisto.NewMetricSystem(time.Minute, includeGoProcessStats)
  ms.Start()

  // create a channel that subscribes to metrics as they are produced once
  // per minute.
  // NOTE: if you allow this channel to fill up, the metric system will NOT
  // block, and  will FORGET about your channel if you fail to unblock the
  // channel after 3 configured intervals (in this case 3 minutes) rather
  // than causing a memory leak.
  myMetricStream := make(chan *loghisto.ProcessedMetricSet, 2)
  ms.SubscribeToProcessedMetrics(myMetricStream)

  // create some metrics
  timeToken := ms.StartTimer("time for creating a counter and histo")
  ms.Counter("some event", 1)
  ms.Histogram("some measured thing", 123)
  timeToken.Stop()

  for m := range myMetricStream {
    fmt.Printf("number of goroutines: %f\n", m.Metrics["sys.NumGoroutine"])
  }

  // if you want to manually unsubscribe from the metric stream
  ms.UnsubscribeFromProcessedMetrics(myMetricStream)

  // to stop and clean up your metric system
  ms.Stop()
}
```
### automatically sending your metrics to OpenTSDB, KairosDB or Graphite
```go
func ExampleExternalSubmitter() {
  includeGoProcessStats := true
  ms := NewMetricSystem(time.Minute, includeGoProcessStats)
  ms.Start()
  // graphite
  s := NewSubmitter(ms, GraphiteProtocol, "tcp", "localhost:7777")
  s.Start()

  // opentsdb / kairosdb
  s := NewSubmitter(ms, OpenTSDBProtocol, "tcp", "localhost:7777")
  s.Start()

  // to tear down:
  s.Shutdown()
}
```

See code for the Graphite/OpenTSDB protocols for adding your own output plugins, it's pretty simple.
