// Copyright 2015 CoreOS, Inc.
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

package etcdhttp

import (
	"strconv"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/prometheus/client_golang/prometheus"
	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
)

var (
	incomingEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "etcd",
			Subsystem: "http",
			Name:      "events_received_total",
			Help:      "Counter of events (incoming requests successfully parsed and authd).",
		}, []string{"method"})

	successfulEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "etcd",
			Subsystem: "http",
			Name:      "events_successful_total",
			Help: "Counter of successful events (non-watches), by method (GET/PUT etc.) and action " +
				"(compareAndSwap, put etc.).",
		}, []string{"method", "action"})

	failedEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "etcd",
			Subsystem: "http",
			Name:      "events_failed_total",
			Help: "Counter of failed events (non-watches), by method (GET/PUT etc.) and code " +
				"(400, 500 etc.).",
		}, []string{"method", "code"})

	successfulEventsHandlingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "etcd",
			Subsystem: "http",
			Name:      "events_handling_time_seconds",
			Help: "Bucketed histogram of handling time (s) of successful events (non-watches), by method " +
				"(GET/PUT etc.).",
			Buckets: prometheus.ExponentialBuckets(0.0005, 2, 13),
		}, []string{"method"})
)

func init() {
	prometheus.MustRegister(incomingEvents)
	prometheus.MustRegister(successfulEvents)
	prometheus.MustRegister(failedEvents)
	prometheus.MustRegister(successfulEventsHandlingTime)
}

func ReportIncomingEvent(request etcdserverpb.Request) {
	incomingEvents.WithLabelValues(methodFromRequest(request)).Inc()
}

func ReportSuccessfulEvent(request etcdserverpb.Request, response etcdserver.Response, startTime time.Time) {
	action := response.Event.Action
	method := methodFromRequest(request)
	successfulEvents.WithLabelValues(method, action).Inc()
	successfulEventsHandlingTime.WithLabelValues(method).Observe(time.Since(startTime).Seconds())
}

func ReportFailedEvent(request etcdserverpb.Request, err error) {
	method := methodFromRequest(request)
	failedEvents.WithLabelValues(method, strconv.Itoa(codeFromError(err))).Inc()
}

func methodFromRequest(request etcdserverpb.Request) string {
	if request.Method == "GET" && request.Quorum {
		return "QGET"
	}
	return request.Method
}

func codeFromError(err error) int {
	if err == nil {
		return 500
	}
	switch e := err.(type) {
	case *etcdErr.Error:
		return (*etcdErr.Error)(e).ErrorCode
	case *httptypes.HTTPError:
		return (*httptypes.HTTPError)(e).Code
	default:
		return 500
	}
}
