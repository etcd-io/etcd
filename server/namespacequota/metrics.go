// Copyright 2021 The etcd Authors
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

package namespacequota

import "github.com/prometheus/client_golang/prometheus"

var (
	namespaceQuotaCreatedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "created_total",
		Help:      "The total number of namespace quotas created.",
	})
	namespaceQuotaDeletedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "deleted_total",
		Help:      "The total number of namespace quotas deleted.",
	})
	namespaceQuotaByteUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "byte_usage_total",
		Help:      "The total number of namespace quota byte usage.",
	}, []string{"key"})
	namespaceQuotaKeyUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "key_usage",
		Help:      "The total number of namespace quota key usage.",
	}, []string{"key"})
	namespaceQuotaByteQuotaExceededTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "byte_quota_exceeded_total",
		Help:      "The total number of times namespace quota tried to exceed the set byte quota.",
	})
	namespaceQuotaKeyQuotaExceededTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "key_quota_exceeded_total",
		Help:      "The total number of times namespace quota tried to exceed the set key quota.",
	})
	namespaceQuotaEnforcementStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "quota_enforcement_status",
		Help:      "The current status of quota enforcement.",
	})
	namespaceQuotaTreeUpdateDurationSec = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "tree_update_duration_seconds",
		Help:      "The duration to perform namespace quota updates to tree.",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 20),
	})
	namespaceQuotaGetDiffValueDurationSec = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "etcd_debugging",
		Subsystem: "namespacequota",
		Name:      "get_diff_value_duration_seconds",
		Help:      "The duration to diff value size.",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 20),
	})
)

func init() {
	prometheus.MustRegister(namespaceQuotaCreatedCounter)
	prometheus.MustRegister(namespaceQuotaDeletedCounter)
	prometheus.MustRegister(namespaceQuotaByteUsage)
	prometheus.MustRegister(namespaceQuotaKeyUsage)
	prometheus.MustRegister(namespaceQuotaByteQuotaExceededTotal)
	prometheus.MustRegister(namespaceQuotaKeyQuotaExceededTotal)
	prometheus.MustRegister(namespaceQuotaEnforcementStatus)
	prometheus.MustRegister(namespaceQuotaTreeUpdateDurationSec)
	prometheus.MustRegister(namespaceQuotaGetDiffValueDurationSec)
}
