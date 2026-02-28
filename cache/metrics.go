// Copyright 2025 The etcd Authors
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

package cache

import "github.com/prometheus/client_golang/prometheus"

var (
	// Cache size: number of keys stored in the cache's B-tree.
	storeKeysTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "store_keys_total",
		Help:      "Total number of keys in the cache store.",
	})
	// Cache size: latest revision tracked by the store.
	storeLatestRevision = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "store_latest_revision",
		Help:      "Latest revision observed by the cache store.",
	})

	// Latency tracking: Get operation duration.
	getDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "get_duration_seconds",
		Help:      "Latency distribution of cache Get operations in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 16),
	})
	// Latency tracking: Get operation totals by result.
	getTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "get_total",
		Help:      "Total number of cache Get operations.",
	}, []string{"result"})
	// Latency tracking: Watch registration duration.
	watchDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "watch_register_duration_seconds",
		Help:      "Latency distribution of cache Watch registration in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 16),
	})
	// Latency tracking: Watch operation totals by result.
	watchTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "watch_total",
		Help:      "Total number of cache Watch operations.",
	}, []string{"result"})

	// Buffer monitoring: number of active (caught-up) watchers in the demux.
	demuxActiveWatchers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "demux_active_watchers",
		Help:      "Number of active (caught-up) watchers in the demux.",
	})
	// Buffer monitoring: number of lagging watchers pending resync.
	demuxLaggingWatchers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "demux_lagging_watchers",
		Help:      "Number of lagging watchers waiting for resync.",
	})
	// Buffer monitoring: number of entries in the demux event history ring buffer.
	demuxHistorySize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "cache",
		Name:      "demux_history_size",
		Help:      "Number of entries currently stored in the demux history ring buffer.",
	})
)

func init() {
	prometheus.MustRegister(storeKeysTotal)
	prometheus.MustRegister(storeLatestRevision)
	prometheus.MustRegister(getDurationSeconds)
	prometheus.MustRegister(getTotal)
	prometheus.MustRegister(watchDurationSeconds)
	prometheus.MustRegister(watchTotal)
	prometheus.MustRegister(demuxActiveWatchers)
	prometheus.MustRegister(demuxLaggingWatchers)
	prometheus.MustRegister(demuxHistorySize)
}
