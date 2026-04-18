// Copyright 2015 The etcd Authors
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

package auth

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	currentAuthRevision = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "etcd_debugging",
			Subsystem: "auth",
			Name:      "revision",
			Help:      "The current revision of auth store.",
		},
		func() float64 {
			reportCurrentAuthRevMu.RLock()
			defer reportCurrentAuthRevMu.RUnlock()
			return reportCurrentAuthRev()
		},
	)
	// overridden by auth store initialization
	reportCurrentAuthRevMu sync.RWMutex
	reportCurrentAuthRev   = func() float64 { return 0 }

	clientCertExpirationSecs = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "etcd",
		Subsystem: "auth",
		Name:      "client_cert_expiration_seconds",
		Help:      "Distribution of remaining lifetime on client certificate for mTLS connections.",
		Buckets: []float64{
			0,
			1800,     // 30 minutes
			3600,     // 1 hour
			7200,     // 2 hours
			21600,    // 6 hours
			43200,    // 12 hours
			86400,    // 1 day
			172800,   // 2 days
			345600,   // 4 days
			604800,   // 1 week
			2592000,  // 1 month
			7776000,  // 3 months
			15552000, // 6 months
			31104000, // 1 year
		},
	})

	mtlsAuthSuccessTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "auth",
		Name:      "mtls_success_total",
		Help:      "Total number of successful mTLS client authentications.",
	})

	// reasons: "no_certificate", "missing_metadata", "gateway_proxy", "expired_certificate"
	mtlsAuthFailureTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "auth",
		Name:      "mtls_failure_total",
		Help:      "Total number of failed mTLS client authentications by reason.",
	}, []string{"reason"})
)

func init() {
	prometheus.MustRegister(currentAuthRevision)
	prometheus.MustRegister(clientCertExpirationSecs)
	prometheus.MustRegister(mtlsAuthSuccessTotal)
	prometheus.MustRegister(mtlsAuthFailureTotal)
}

const (
	FailReasonNoCertificate      = "no_certificate"
	FailReasonMissingMetadata    = "missing_metadata"
	FailReasonGatewayProxy       = "gateway_proxy"
	FailReasonExpiredCertificate = "expired_certificate"
)

// RecordMtlsFailure increments the mTLS authentication failure counter for the given reason.
func RecordMtlsFailure(reason string) {
	mtlsAuthFailureTotal.WithLabelValues(reason).Inc()
}
