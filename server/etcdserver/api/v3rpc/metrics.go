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
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

var (
	sentBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "network",
		Name:      "client_grpc_sent_bytes_total",
		Help:      "The total number of bytes sent to grpc clients.",
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
	prometheus.MustRegister(receivedBytes)
	prometheus.MustRegister(streamFailures)
	prometheus.MustRegister(clientRequests)
}

func splitMethodName(fullMethodName string) (string, string) {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/") // remove leading slash
	if i := strings.Index(fullMethodName, "/"); i >= 0 {
		return fullMethodName[:i], fullMethodName[i+1:]
	}
	return "unknown", "unknown"
}

// constructExtensiveMetricsInterceptors constructs unary and stream interceptors to record histogram metrics for gRPC requests
func constructExtensiveMetricsInterceptors() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	// Define a new histogram metric using default buckets
	serverHandledHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"grpc_type", "grpc_service", "grpc_method"},
	)
	prometheus.MustRegister(serverHandledHistogram)

	// method to record histogram metrics for both unary and stream requests
	recordHistogramMetrics := func(serverHandledHistogram *prometheus.HistogramVec, grpcType, fullMethodName string, startTime time.Time) {
		grpcService, grpcMethod := splitMethodName(fullMethodName)
		serverHandledHistogram.WithLabelValues(grpcType, grpcService, grpcMethod).Observe(time.Since(startTime).Seconds())
	}

	// Add a new interceptor to spit out histogram metrics for unary requests
	unaryInterceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		startTime := time.Now()
		resp, err = handler(ctx, req)
		recordHistogramMetrics(serverHandledHistogram, "unary", info.FullMethod, startTime)
		return resp, err
	}
	streamInterceptor := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		err := handler(srv, ss)
		recordHistogramMetrics(serverHandledHistogram, "stream", info.FullMethod, startTime)
		return err
	}
	return unaryInterceptor, streamInterceptor
}
