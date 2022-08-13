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
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/config"
	"go.uber.org/zap"
)

const (
	PathHealth      = "/health"
	PathProxyHealth = "/proxy/health"
)

type ServerHealth interface {
	Alarms() []*pb.AlarmMember
	Leader() types.ID
	Range(context.Context, *pb.RangeRequest) (*pb.RangeResponse, error)
	Config() config.ServerConfig
}

// HandleHealth registers metrics and health handlers. it checks health by using v3 range request
// and its corresponding timeout.
func HandleHealth(lg *zap.Logger, mux *http.ServeMux, srv ServerHealth) {
	mux.Handle(PathHealth, NewHealthHandler(lg, func(excludedAlarms AlarmSet, serializable bool) Health {
		if h := checkAlarms(lg, srv, excludedAlarms); h.Health != "true" {
			return h
		}
		if h := checkLeader(lg, srv, serializable); h.Health != "true" {
			return h
		}
		return checkAPI(lg, srv, serializable)
	}))
}

// NewHealthHandler handles '/health' requests.
func NewHealthHandler(lg *zap.Logger, hfunc func(excludedAlarms AlarmSet, Serializable bool) Health) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			lg.Warn("/health error", zap.Int("status-code", http.StatusMethodNotAllowed))
			return
		}
		excludedAlarms := getExcludedAlarms(r)
		// Passing the query parameter "serializable=true" ensures that the
		// health of the local etcd is checked vs the health of the cluster.
		// This is useful for probes attempting to validate the liveness of
		// the etcd process vs readiness of the cluster to serve requests.
		serializableFlag := getSerializableFlag(r)
		h := hfunc(excludedAlarms, serializableFlag)
		defer func() {
			if h.Health == "true" {
				healthSuccess.Inc()
			} else {
				healthFailed.Inc()
			}
		}()
		d, _ := json.Marshal(h)
		if h.Health != "true" {
			http.Error(w, string(d), http.StatusServiceUnavailable)
			lg.Warn("/health error", zap.String("output", string(d)), zap.Int("status-code", http.StatusServiceUnavailable))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(d)
		lg.Debug("/health OK", zap.Int("status-code", http.StatusOK))
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
	Reason string `json:"reason"`
}

type AlarmSet map[string]struct{}

func getExcludedAlarms(r *http.Request) (alarms AlarmSet) {
	alarms = make(map[string]struct{}, 2)
	alms, found := r.URL.Query()["exclude"]
	if found {
		for _, alm := range alms {
			if len(alm) == 0 {
				continue
			}
			alarms[alm] = struct{}{}
		}
	}
	return alarms
}

func getSerializableFlag(r *http.Request) bool {
	return r.URL.Query().Get("serializable") == "true"
}

// TODO: etcdserver.ErrNoLeader in health API

func checkAlarms(lg *zap.Logger, srv ServerHealth, excludedAlarms AlarmSet) Health {
	h := Health{Health: "true"}
	as := srv.Alarms()
	if len(as) > 0 {
		for _, v := range as {
			alarmName := v.Alarm.String()
			if _, found := excludedAlarms[alarmName]; found {
				lg.Debug("/health excluded alarm", zap.String("alarm", v.String()))
				continue
			}

			h.Health = "false"
			switch v.Alarm {
			case etcdserverpb.AlarmType_NOSPACE:
				h.Reason = "ALARM NOSPACE"
			case etcdserverpb.AlarmType_CORRUPT:
				h.Reason = "ALARM CORRUPT"
			default:
				h.Reason = "ALARM UNKNOWN"
			}
			lg.Warn("serving /health false due to an alarm", zap.String("alarm", v.String()))
			return h
		}
	}

	return h
}

func checkLeader(lg *zap.Logger, srv ServerHealth, serializable bool) Health {
	h := Health{Health: "true"}
	if !serializable && (uint64(srv.Leader()) == raft.None) {
		h.Health = "false"
		h.Reason = "RAFT NO LEADER"
		lg.Warn("serving /health false; no leader")
	}
	return h
}

func checkAPI(lg *zap.Logger, srv ServerHealth, serializable bool) Health {
	h := Health{Health: "true"}
	cfg := srv.Config()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ReqTimeout())
	_, err := srv.Range(ctx, &etcdserverpb.RangeRequest{KeysOnly: true, Limit: 1, Serializable: serializable})
	cancel()
	if err != nil && err != auth.ErrUserEmpty && err != auth.ErrPermissionDenied {
		h.Health = "false"
		h.Reason = fmt.Sprintf("RANGE ERROR:%s", err)
		lg.Warn("serving /health false; Range fails", zap.Error(err))
		return h
	}
	lg.Debug("serving /health true")
	return h
}
