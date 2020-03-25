// Copyright 2017 The etcd Authors
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
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.etcd.io/etcd/etcdserver"
	"go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/raft"
	"go.uber.org/zap"
)

const (
	PathMetrics = "/metrics"
	PathHealth  = "/health"
)

// HandleMetricsHealth registers metrics and health handlers.
func HandleMetricsHealth(lg *zap.Logger, mux *http.ServeMux, srv etcdserver.ServerV2) {
	mux.Handle(PathMetrics, promhttp.Handler())
	mux.Handle(PathHealth, NewHealthHandler(lg, func() Health { return checkHealth(lg, srv) }))
}

// HandlePrometheus registers prometheus handler on '/metrics'.
func HandlePrometheus(mux *http.ServeMux) {
	mux.Handle(PathMetrics, promhttp.Handler())
}

// NewHealthHandler handles '/health' requests.
func NewHealthHandler(lg *zap.Logger, hfunc func() Health) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			lg.Warn("/health error", zap.Int("status-code", http.StatusMethodNotAllowed))
			return
		}
		h := hfunc()
		d, _ := json.Marshal(h)
		if h.Health != "true" {
			http.Error(w, string(d), http.StatusServiceUnavailable)
			lg.Warn("/health error", zap.String("output", string(d)), zap.Int("status-code", http.StatusServiceUnavailable))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(d)
		lg.Info("/health OK", zap.Int("status-code", http.StatusOK))
	}
}

var (
	healthSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "server",
		Name:      "health_success",
		Help:      "The total number of successful health checks",
	})
	healthFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "server",
		Name:      "health_failures",
		Help:      "The total number of failed health checks",
	})
)

func init() {
	prometheus.MustRegister(healthSuccess)
	prometheus.MustRegister(healthFailed)
}

// Health defines etcd server health status.
// TODO: remove manual parsing in etcdctl cluster-health
type Health struct {
	Health string `json:"health"`
}

// TODO: server NOSPACE, etcdserver.ErrNoLeader in health API

func checkHealth(lg *zap.Logger, srv etcdserver.ServerV2) (h Health) {
	h.Health = "true"

	defer func() {
		if h.Health == "true" {
			healthSuccess.Inc()
		} else {
			healthFailed.Inc()
		}
	}()

	as := srv.Alarms()
	if len(as) > 0 {
		h.Health = "false"
		for _, v := range as {
			lg.Warn("serving /health false due to an alarm", zap.String("alarm", v.String()))
		}
		return
	}

	if uint64(srv.Leader()) == raft.None {
		h.Health = "false"
		lg.Warn("serving /health false; no leader")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err := srv.Do(ctx, etcdserverpb.Request{Method: "QGET"})
	cancel()
	if err != nil {
		h.Health = "false"
		lg.Warn("serving /health false; QGET fails", zap.Error(err))
	}

	lg.Info("serving /health true")
	return
}
