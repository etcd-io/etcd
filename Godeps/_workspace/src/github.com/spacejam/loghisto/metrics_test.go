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
	"math"
	"runtime"
	"testing"
	"time"
)

func ExampleMetricSystem() {
	ms := NewMetricSystem(time.Microsecond, true)
	ms.Start()
	myMetricStream := make(chan *ProcessedMetricSet, 2)
	ms.SubscribeToProcessedMetrics(myMetricStream)

	timeToken := ms.StartTimer("submit_metrics")
	ms.Counter("range_splits", 1)
	ms.Histogram("some_ipc_latency", 123)
	timeToken.Stop()

	processedMetricSet := <-myMetricStream
	ms.UnsubscribeFromProcessedMetrics(myMetricStream)

	m := processedMetricSet.Metrics

	example := []struct {
		Name  string
		Value float64
	}{
		{
			"total range splits during the process lifetime",
			m["range_splits"],
		}, {
			"range splits in this period",
			m["range_splits_rate"],
		}, {
			"some_ipc 99.9th percentile",
			m["some_ipc_latency_99.9"],
		}, {
			"some_ipc max",
			m["some_ipc_latency_max"],
		}, {
			"some_ipc calls this period",
			m["some_ipc_latency_count"],
		}, {
			"some_ipc calls during the process lifetime",
			m["some_ipc_latency_agg_count"],
		}, {
			"some_ipc total latency this period",
			m["some_ipc_latency_sum"],
		}, {
			"some_ipc mean this period",
			m["some_ipc_latency_avg"],
		}, {
			"some_ipc aggregate man",
			m["some_ipc_latency_agg_avg"],
		}, {
			"time spent submitting metrics this period",
			m["submit_metrics_sum"],
		}, {
			"number of goroutines",
			m["sys.NumGoroutine"],
		}, {
			"time spent in GC",
			m["sys.PauseTotalNs"],
		},
	}
	for _, nameValue := range example {
		var result string
		if nameValue.Value == float64(0) {
			result = "NOT present"
		} else {
			result = "present"
		}
		fmt.Println(nameValue.Name, result)
	}
	ms.Stop()
	// Output:
	// total range splits during the process lifetime present
	// range splits in this period present
	// some_ipc 99.9th percentile present
	// some_ipc max present
	// some_ipc calls this period present
	// some_ipc calls during the process lifetime present
	// some_ipc total latency this period present
	// some_ipc mean this period present
	// some_ipc aggregate man present
	// time spent submitting metrics this period present
	// number of goroutines present
	// time spent in GC present
}

func TestPercentile(t *testing.T) {
	metrics := map[float64]uint64{
		10:  9000,
		25:  900,
		33:  90,
		47:  9,
		500: 1,
	}

	percentileToExpected := map[float64]float64{
		0:     10,
		.99:   25,
		.999:  33,
		.9991: 47,
		.9999: 47,
		1:     500,
	}

	totalcount := uint64(0)
	proportions := make([]proportion, 0, len(metrics))
	for value, count := range metrics {
		totalcount += count
		proportions = append(proportions, proportion{Value: value, Count: count})
	}

	for p, expected := range percentileToExpected {
		result, err := percentile(totalcount, proportions, p)
		if err != nil {
			t.Error("error:", err)
		}

		// results must be within 1% of their expected values.
		diff := math.Abs(expected/result - 1)
		if diff > .01 {
			t.Errorf("percentile: %.04f, expected: %.04f, actual: %.04f, %% off: %.04f\n",
				p, expected, result, diff*100)
		}
	}
}

func TestCompress(t *testing.T) {
	toTest := []float64{
		-421408208120481,
		-1,
		0,
		1,
		214141241241241,
	}
	for _, f := range toTest {
		result := decompress(compress(f))
		var diff float64
		if result == 0 {
			diff = math.Abs(f - result)
		} else {
			diff = math.Abs(f/result - 1)
		}
		if diff > .01 {
			t.Errorf("expected: %f, actual: %f, %% off: %.04f\n",
				f, result, diff*100)
		}
	}
}

func TestSysStats(t *testing.T) {
	metricSystem := NewMetricSystem(time.Microsecond, true)
	gauges := metricSystem.collectRawMetrics().Gauges
	v, present := gauges["sys.Alloc"]
	if v <= 0 || !present {
		t.Errorf("expected positive reported allocated bytes, got %f\n", v)
	}
}

func TestTimer(t *testing.T) {
	metricSystem := NewMetricSystem(time.Microsecond, false)
	token1 := metricSystem.StartTimer("timer1")
	token2 := metricSystem.StartTimer("timer1")
	time.Sleep(50 & time.Microsecond)
	token1.Stop()
	time.Sleep(5 * time.Microsecond)
	token2.Stop()
	token3 := metricSystem.StartTimer("timer1")
	time.Sleep(10 * time.Microsecond)
	token3.Stop()
	result := metricSystem.processMetrics(metricSystem.collectRawMetrics()).Metrics

	if result["timer1_min"] > result["timer1_50"] ||
		result["timer1_50"] > result["timer1_max"] {
		t.Error("bad result map:", result)
	}
}

func TestRate(t *testing.T) {
	metricSystem := NewMetricSystem(time.Microsecond, false)
	metricSystem.Counter("rate1", 777)
	time.Sleep(20 * time.Millisecond)
	metrics := metricSystem.processMetrics(metricSystem.collectRawMetrics()).Metrics
	if metrics["rate1_rate"] != 777 {
		t.Error("count one value")
	}
	metricSystem.Counter("rate1", 1223)
	time.Sleep(20 * time.Millisecond)
	metrics = metricSystem.processMetrics(metricSystem.collectRawMetrics()).Metrics
	if metrics["rate1_rate"] != 1223 {
		t.Errorf("expected rate: 1223, actual: %f", metrics["rate1_rate"])
	}
	metricSystem.Counter("rate1", 1223)
	metricSystem.Counter("rate1", 1223)
	time.Sleep(20 * time.Millisecond)
	metrics = metricSystem.processMetrics(metricSystem.collectRawMetrics()).Metrics
	if metrics["rate1_rate"] != 2446 {
		t.Errorf("expected rate: 2446, actual: %f", metrics["rate1_rate"])
	}
}

func TestCounter(t *testing.T) {
	metricSystem := NewMetricSystem(time.Microsecond, false)
	metricSystem.Counter("counter1", 3290)
	time.Sleep(20 * time.Millisecond)
	metrics := metricSystem.processMetrics(metricSystem.collectRawMetrics()).Metrics
	if metrics["counter1"] != 3290 {
		t.Error("count one value", metrics)
	}
	metricSystem.Counter("counter1", 10000)
	time.Sleep(20 * time.Millisecond)
	metrics = metricSystem.processMetrics(metricSystem.collectRawMetrics()).Metrics
	if metrics["counter1"] != 13290 {
		t.Error("accumulate counts across broadcasts")
	}

}

func TestUpdateSubscribers(t *testing.T) {
	rawMetricStream := make(chan *RawMetricSet)
	processedMetricStream := make(chan *ProcessedMetricSet)

	metricSystem := NewMetricSystem(2*time.Microsecond, false)
	metricSystem.SubscribeToRawMetrics(rawMetricStream)
	metricSystem.SubscribeToProcessedMetrics(processedMetricStream)

	metricSystem.Counter("counter5", 33)

	go func() {
		select {
		case <-rawMetricStream:
		case <-time.After(20 * time.Millisecond):
			t.Error("received no raw metrics from the MetricSystem after 2 milliseconds.")
		}
		metricSystem.UnsubscribeFromRawMetrics(rawMetricStream)
	}()
	go func() {
		select {
		case <-processedMetricStream:
		case <-time.After(20 * time.Millisecond):
			t.Error("received no processed metrics from the MetricSystem after 2 milliseconds.")
		}
		metricSystem.UnsubscribeFromProcessedMetrics(processedMetricStream)
	}()

	metricSystem.Start()
	time.Sleep(20 * time.Millisecond)

	go func() {
		select {
		case <-rawMetricStream:
			t.Error("received raw metrics from the MetricSystem after unsubscribing.")
		default:
		}
	}()
	go func() {
		select {
		case <-processedMetricStream:
			t.Error("received processed metrics from the MetricSystem after unsubscribing.")
		default:
		}
	}()
	time.Sleep(20 * time.Millisecond)
}

func TestProcessedBroadcast(t *testing.T) {
	processedMetricStream := make(chan *ProcessedMetricSet, 128)
	metricSystem := NewMetricSystem(time.Microsecond, false)
	metricSystem.SubscribeToProcessedMetrics(processedMetricStream)

	metricSystem.Histogram("histogram1", 33)
	metricSystem.Histogram("histogram1", 59)
	metricSystem.Histogram("histogram1", 330000)
	metricSystem.Start()

	select {
	case processedMetrics := <-processedMetricStream:
		if int(processedMetrics.Metrics["histogram1_sum"]) != 331132 {
			t.Error("expected histogram1_sum to be 331132, instead was",
				processedMetrics.Metrics["histogram1_sum"])
		}
		if int(processedMetrics.Metrics["histogram1_agg_avg"]) != 110377 {
			t.Error("expected histogram1_agg_avg to be 110377, instead was",
				processedMetrics.Metrics["histogram1_agg_avg"])
		}
		if int(processedMetrics.Metrics["histogram1_count"]) != 3 {
			t.Error("expected histogram1_count to be 3, instead was",
				processedMetrics.Metrics["histogram1_count"])
		}
	case <-time.After(20 * time.Millisecond):
		t.Error("received no metrics from the MetricSystem after 2 milliseconds.")
	}

	metricSystem.UnsubscribeFromProcessedMetrics(processedMetricStream)
	metricSystem.Stop()
}

func TestRawBroadcast(t *testing.T) {
	rawMetricStream := make(chan *RawMetricSet, 128)
	metricSystem := NewMetricSystem(time.Microsecond, false)
	metricSystem.SubscribeToRawMetrics(rawMetricStream)

	metricSystem.Counter("counter2", 10)
	metricSystem.Counter("counter2", 111)
	metricSystem.Start()

	select {
	case rawMetrics := <-rawMetricStream:
		if rawMetrics.Counters["counter2"] != 121 {
			t.Error("expected counter2 to be 121, instead was",
				rawMetrics.Counters["counter2"])
		}
		if rawMetrics.Rates["counter2"] != 121 {
			t.Error("expected counter2 rate to be 121, instead was",
				rawMetrics.Counters["counter2"])
		}
	case <-time.After(20 * time.Millisecond):
		t.Error("received no metrics from the MetricSystem after 2 milliseconds.")
	}

	metricSystem.UnsubscribeFromRawMetrics(rawMetricStream)
	metricSystem.Stop()
}

func TestMetricSystemStop(t *testing.T) {
	metricSystem := NewMetricSystem(time.Microsecond, false)

	startingRoutines := runtime.NumGoroutine()

	metricSystem.Start()
	metricSystem.Stop()

	time.Sleep(20 * time.Millisecond)

	endRoutines := runtime.NumGoroutine()
	if startingRoutines < endRoutines {
		t.Errorf("lingering goroutines have not been cleaned up: "+
			"before: %d, after: %d\n", startingRoutines, endRoutines)
	}
}
