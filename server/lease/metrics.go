// Copyright 2018 The etcd Authors
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

package lease

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	leaseGranted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "granted_total",
		Help:      "The total number of granted leases.",
	})

	leaseRevoked = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "revoked_total",
		Help:      "The total number of revoked leases.",
	})

	leaseRenewed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "renewed_total",
		Help:      "The number of renewed leases seen by the leader.",
	})

	leaseTotalTTLs = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "etcd_debugging",
			Subsystem: "lease",
			Name:      "ttl_total",
			Help:      "Bucketed histogram of lease TTLs.",
			// 1 second -> 3 months
			Buckets: prometheus.ExponentialBuckets(1, 2, 24),
		})

	leaseAttached = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "attach_total",
		Help:      "The number of leases that are attached to a lease item.",
	})

	leaseDetached = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "detach_total",
		Help:      "The number of leases that are detached from a lease item.",
	})

	initLeaseCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "initial_lease_count",
		Help:      "Reports an initial lease count.",
	})

	leaseGrantError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "grant_errors",
		Help:      "Error count by type to count for lease grants.",
	}, []string{"error"})

	leaseRevokeError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "revoke_errors",
		Help:      "Error count by type to count for lease revokes.",
	}, []string{"error"})

	leaseRenewError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "etcd_debugging",
		Subsystem: "lease",
		Name:      "renew_errors",
		Help:      "Error count by type to count for lease renewals.",
	}, []string{"error"})
)

func init() {
	prometheus.MustRegister(leaseGranted)
	prometheus.MustRegister(leaseRevoked)
	prometheus.MustRegister(leaseRenewed)
	prometheus.MustRegister(leaseTotalTTLs)
	prometheus.MustRegister(leaseAttached)
	prometheus.MustRegister(leaseDetached)
	prometheus.MustRegister(leaseGrantError)
	prometheus.MustRegister(leaseRevokeError)
	prometheus.MustRegister(leaseRenewError)
}
