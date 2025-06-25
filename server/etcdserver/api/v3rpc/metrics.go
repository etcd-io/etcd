// Copyright 2016 The etcd Authors
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

package v3rpc

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	sentBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "network",
		Name:      "client_grpc_sent_bytes_total",
		Help:      "The total number of bytes sent to grpc clients.",
	})

	sentWatchResponseSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "etcd",
		Subsystem: "network",
		Name:      "watch_events_size_bytes",
		// 64 bytes to 16MB (default kv size is 1.5M)
		Buckets: []float64{64, 256, 512, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216},
		Help:    "The watch response size sent to watch clients in bytes.",
	})

	receivedBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "network",
		Name:      "client_grpc_received_bytes_total",
		Help:      "The total number of bytes received from grpc clients.",
	})

	streamFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "etcd",
			Subsystem: "network",
			Name:      "server_stream_failures_total",
			Help:      "The total number of stream failures from the local server.",
		},
		[]string{"Type", "API"},
	)

	clientRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "etcd",
			Subsystem: "server",
			Name:      "client_requests_total",
			Help:      "The total number of client requests per client version.",
		},
		[]string{"type", "client_api_version"},
	)
)

func init() {
	prometheus.MustRegister(sentBytes)
	prometheus.MustRegister(sentWatchResponseSize)
	prometheus.MustRegister(receivedBytes)
	prometheus.MustRegister(streamFailures)
	prometheus.MustRegister(clientRequests)
}
