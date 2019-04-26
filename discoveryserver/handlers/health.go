// Copyright 2019 The etcd Authors
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

package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"go.etcd.io/etcd/discoveryserver/handlers/httperror"
	"go.etcd.io/etcd/discoveryserver/metrics"
)

var healthCounter *prometheus.CounterVec

func init() {
	healthCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "endpoint_health_requests_total",
			Help: "How many /health requests processed, partitioned by status code and HTTP method.",
		},
		[]string{"code", "method"},
	)
	metrics.Registry.MustRegister(healthCounter)
}

func HealthHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	st := ctx.Value(stateKey).(*State)

	token, gckey, err := st.setupToken(0)
	if err != nil || token == "" {
		log.Printf("health failed to setupToken %v", err)
		httperror.Error(w, r, "health failed to setupToken", 400, healthCounter)
		return
	}

	err = st.deleteToken(token, gckey)
	if err != nil {
		log.Printf("health failed to deleteToken %v", err)
		httperror.Error(w, r, "health failed to deleteToken", 400, healthCounter)
		return
	}

	fmt.Fprintf(w, "OK")
	healthCounter.WithLabelValues("200", r.Method).Add(1)
}
