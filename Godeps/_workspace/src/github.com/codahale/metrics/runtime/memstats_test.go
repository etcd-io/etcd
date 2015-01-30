package runtime

import (
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codahale/metrics"
)

func TestMemStats(t *testing.T) {
	counters, gauges := metrics.Snapshot()

	expectedCounters := []string{
		"Mem.NumGC",
		"Mem.PauseTotalNs",
	}

	expectedGauges := []string{
		"Mem.LastGC",
		"Mem.Alloc",
		"Mem.HeapObjects",
	}

	for _, name := range expectedCounters {
		if _, ok := counters[name]; !ok {
			t.Errorf("Missing counters %q", name)
		}
	}

	for _, name := range expectedGauges {
		if _, ok := gauges[name]; !ok {
			t.Errorf("Missing gauge %q", name)
		}
	}
}
