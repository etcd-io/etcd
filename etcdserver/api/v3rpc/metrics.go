// Copyright 2016 CoreOS, Inc.
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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	receivedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "etcd",
			Subsystem: "grpc",
			Name:      "requests_total",
			Help:      "Counter of received requests.",
		}, []string{"grpc_service", "grpc_method"})

	failedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "etcd",
			Subsystem: "grpc",
			Name:      "requests_failed_total",
			Help:      "Counter of failed requests.",
		}, []string{"grpc_service", "grpc_method", "grpc_code"})

	handlingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "etcd",
			Subsystem: "grpc",
			Name:      "unary_requests_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of handled unary (non-stream) requests.",
			Buckets:   prometheus.ExponentialBuckets(0.0005, 2, 13),
		}, []string{"grpc_service", "grpc_method"})
)

func metricsUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	service, method := splitMethodName(info.FullMethod)
	receivedCounter.WithLabelValues(service, method).Inc()

	start := time.Now()
	resp, err = handler(ctx, req)
	if err != nil {
		failedCounter.WithLabelValues(service, method, grpc.Code(err).String()).Inc()
	}
	handlingDuration.WithLabelValues(service, method).Observe(time.Since(start).Seconds())

	return resp, err
}

func metricsStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	service, method := splitMethodName(info.FullMethod)
	receivedCounter.WithLabelValues(service, method).Inc()

	err := handler(srv, ss)
	if err != nil {
		failedCounter.WithLabelValues(service, method, grpc.Code(err).String()).Inc()
	}

	return err
}

func splitMethodName(fullMethodName string) (string, string) {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/") // remove leading slash
	if i := strings.Index(fullMethodName, "/"); i >= 0 {
		return fullMethodName[:i], fullMethodName[i+1:]
	}
	return "unknown", "unknown"
}

func init() {
	prometheus.MustRegister(receivedCounter)
	prometheus.MustRegister(failedCounter)
	prometheus.MustRegister(handlingDuration)
}
