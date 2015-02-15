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

package etcdserver

import "github.com/coreos/etcd/Godeps/_workspace/src/github.com/prometheus/client_golang/prometheus"

var (
	// TODO: with label in v3?
	proposeDurations = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "etcdserver_proposal_durations_milliseconds",
		Help: "The latency distributions of committing proposal.",
	})
	proposePending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "etcdserver_pending_proposal_total",
		Help: "The total number of pending proposals.",
	})
	// This is number of proposal failed in client's view.
	// The proposal might be later got committed in raft.
	proposeFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "etcdserver_proposal_failed_total",
		Help: "The total number of failed proposals.",
	})
)

func init() {
	prometheus.MustRegister(proposeDurations)
	prometheus.MustRegister(proposePending)
	prometheus.MustRegister(proposeFailed)
}
