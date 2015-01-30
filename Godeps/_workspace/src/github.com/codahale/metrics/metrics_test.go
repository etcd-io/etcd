package metrics_test

import (
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codahale/metrics"
)

func TestCounter(t *testing.T) {
	metrics.Reset()

	metrics.Counter("whee").Add()
	metrics.Counter("whee").AddN(10)

	counters, _ := metrics.Snapshot()
	if v, want := counters["whee"], uint64(11); v != want {
		t.Errorf("Counter was %v, but expected %v", v, want)
	}
}

func TestCounterFunc(t *testing.T) {
	metrics.Reset()

	metrics.Counter("whee").SetFunc(func() uint64 {
		return 100
	})

	counters, _ := metrics.Snapshot()
	if v, want := counters["whee"], uint64(100); v != want {
		t.Errorf("Counter was %v, but expected %v", v, want)
	}
}

func TestCounterBatchFunc(t *testing.T) {
	metrics.Reset()

	var a, b uint64

	metrics.Counter("whee").SetBatchFunc(
		"yay",
		func() {
			a, b = 1, 2
		},
		func() uint64 {
			return a
		},
	)

	metrics.Counter("woo").SetBatchFunc(
		"yay",
		func() {
			a, b = 1, 2
		},
		func() uint64 {
			return b
		},
	)

	counters, _ := metrics.Snapshot()
	if v, want := counters["whee"], uint64(1); v != want {
		t.Errorf("Counter was %v, but expected %v", v, want)
	}

	if v, want := counters["woo"], uint64(2); v != want {
		t.Errorf("Counter was %v, but expected %v", v, want)
	}
}

func TestCounterRemove(t *testing.T) {
	metrics.Reset()

	metrics.Counter("whee").Add()
	metrics.Counter("whee").Remove()

	counters, _ := metrics.Snapshot()
	if v, ok := counters["whee"]; ok {
		t.Errorf("Counter was %v, but expected nothing", v)
	}
}

func TestGaugeValue(t *testing.T) {
	metrics.Reset()

	metrics.Gauge("whee").Set(-100)

	_, gauges := metrics.Snapshot()
	if v, want := gauges["whee"], int64(-100); v != want {
		t.Errorf("Gauge was %v, but expected %v", v, want)
	}
}

func TestGaugeFunc(t *testing.T) {
	metrics.Reset()

	metrics.Gauge("whee").SetFunc(func() int64 {
		return -100
	})

	_, gauges := metrics.Snapshot()
	if v, want := gauges["whee"], int64(-100); v != want {
		t.Errorf("Gauge was %v, but expected %v", v, want)
	}
}

func TestGaugeRemove(t *testing.T) {
	metrics.Reset()

	metrics.Gauge("whee").Set(1)
	metrics.Gauge("whee").Remove()

	_, gauges := metrics.Snapshot()
	if v, ok := gauges["whee"]; ok {
		t.Errorf("Gauge was %v, but expected nothing", v)
	}
}

func TestHistogram(t *testing.T) {
	metrics.Reset()

	h := metrics.NewHistogram("heyo", 1, 1000, 3)
	for i := 100; i > 0; i-- {
		for j := 0; j < i; j++ {
			h.RecordValue(int64(i))
		}
	}

	_, gauges := metrics.Snapshot()

	if v, want := gauges["heyo.P50"], int64(71); v != want {
		t.Errorf("P50 was %v, but expected %v", v, want)
	}

	if v, want := gauges["heyo.P75"], int64(87); v != want {
		t.Errorf("P75 was %v, but expected %v", v, want)
	}

	if v, want := gauges["heyo.P90"], int64(95); v != want {
		t.Errorf("P90 was %v, but expected %v", v, want)
	}

	if v, want := gauges["heyo.P95"], int64(98); v != want {
		t.Errorf("P95 was %v, but expected %v", v, want)
	}

	if v, want := gauges["heyo.P99"], int64(100); v != want {
		t.Errorf("P99 was %v, but expected %v", v, want)
	}

	if v, want := gauges["heyo.P999"], int64(100); v != want {
		t.Errorf("P999 was %v, but expected %v", v, want)
	}
}

func TestHistogramRemove(t *testing.T) {
	metrics.Reset()

	h := metrics.NewHistogram("heyo", 1, 1000, 3)
	h.Remove()

	_, gauges := metrics.Snapshot()
	if v, ok := gauges["heyo.P50"]; ok {
		t.Errorf("Gauge was %v, but expected nothing", v)
	}
}

func BenchmarkCounterAdd(b *testing.B) {
	metrics.Reset()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.Counter("test1").Add()
		}
	})
}

func BenchmarkCounterAddN(b *testing.B) {
	metrics.Reset()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.Counter("test2").AddN(100)
		}
	})
}

func BenchmarkGaugeSet(b *testing.B) {
	metrics.Reset()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.Gauge("test2").Set(100)
		}
	})
}

func BenchmarkHistogramRecordValue(b *testing.B) {
	metrics.Reset()
	h := metrics.NewHistogram("hist", 1, 1000, 3)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h.RecordValue(100)
		}
	})
}
