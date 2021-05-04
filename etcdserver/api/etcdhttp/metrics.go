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

	"go.etcd.io/etcd/etcdserver"
	"go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/raft"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	PathMetrics = "/metrics"
	PathHealth  = "/health"
)

// HandleMetricsHealth registers metrics and health handlers.
func HandleMetricsHealth(mux *http.ServeMux, srv etcdserver.ServerV2) {
	mux.Handle(PathMetrics, promhttp.Handler())
	mux.Handle(PathHealth, NewHealthHandler(func(excludedAlarms AlarmSet) Health { return checkHealth(srv, excludedAlarms) }))
}

// HandlePrometheus registers prometheus handler on '/metrics'.
func HandlePrometheus(mux *http.ServeMux) {
	mux.Handle(PathMetrics, promhttp.Handler())
}

// NewHealthHandler handles '/health' requests.
func NewHealthHandler(hfunc func(excludedAlarms AlarmSet) Health) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			plog.Warningf("/health error (status code %d)", http.StatusMethodNotAllowed)
			return
		}
		excludedAlarms := getExcludedAlarms(r)
		h := hfunc(excludedAlarms)
		d, _ := json.Marshal(h)
		if h.Health != "true" {
			http.Error(w, string(d), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(d)
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

type AlarmSet map[string]struct{}

func getExcludedAlarms(r *http.Request) (alarms AlarmSet) {
	alarms = make(map[string]struct{}, 2)
	alms, found := r.URL.Query()["exclude"]
	if found {
		for _, alm := range alms {
			if len(alms) == 0 {
				continue
			}
			alarms[alm] = struct{}{}
		}
	}
	return alarms
}

// TODO: server NOSPACE, etcdserver.ErrNoLeader in health API

func checkHealth(srv etcdserver.ServerV2, excludedAlarms AlarmSet) Health {
	h := Health{Health: "true"}

	as := srv.Alarms()
	if len(as) > 0 {
		for _, v := range as {
			alarmName := v.Alarm.String()
			if _, found := excludedAlarms[alarmName]; found {
				plog.Debugf("/health excluded alarm %s", alarmName)
				delete(excludedAlarms, alarmName)
				continue
			}
			h.Health = "false"
			plog.Warningf("/health error due to %s", v.String())
			return h
		}
	}

	if len(excludedAlarms) > 0 {
		plog.Warningf("fail exclude alarms from health check, exclude alarms %+v", excludedAlarms)
	}

	if h.Health == "true" {
		if uint64(srv.Leader()) == raft.None {
			h.Health = "false"
			plog.Warningf("/health error; no leader (status code %d)", http.StatusServiceUnavailable)
		}
	}

	if h.Health == "true" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := srv.Do(ctx, etcdserverpb.Request{Method: "QGET"})
		cancel()
		if err != nil {
			h.Health = "false"
			plog.Warningf("/health error; QGET failed %v (status code %d)", err, http.StatusServiceUnavailable)
		}
	}

	if h.Health == "true" {
		healthSuccess.Inc()
		plog.Debugf("/health OK (status code %d)", http.StatusOK)
	} else {
		healthFailed.Inc()
	}
	return h
}
