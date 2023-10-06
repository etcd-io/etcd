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
	"time"

	"go.etcd.io/etcd/auth"
	"go.etcd.io/etcd/etcdserver"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/types"
	"go.etcd.io/etcd/raft"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	PathHealth = "/health"
)

type ServerHealth interface {
	serverHealthV2V3
	Range(context.Context, *pb.RangeRequest) (*pb.RangeResponse, error)
	Config() etcdserver.ServerConfig
	AuthStore() auth.AuthStore
}

type serverHealthV2V3 interface {
	Alarms() []*pb.AlarmMember
	Leader() types.ID
}

// HandleHealthForV2 registers metrics and health handlers for v2.
func HandleHealthForV2(mux *http.ServeMux, srv etcdserver.ServerV2) {
	mux.Handle(PathHealth, NewHealthHandler(func(ctx context.Context, excludedAlarms AlarmSet, serializable bool) Health {
		if h := checkAlarms(srv, excludedAlarms); h.Health != "true" {
			return h
		}
		if h := checkLeader(srv, serializable); h.Health != "true" {
			return h
		}
		return checkV2API(ctx, srv)
	}))
}

// HandleHealth registers metrics and health handlers. it checks health by using v3 range request
// and its corresponding timeout.
func HandleHealth(mux *http.ServeMux, srv ServerHealth) {
	mux.Handle(PathHealth, NewHealthHandler(func(ctx context.Context, excludedAlarms AlarmSet, serializable bool) Health {
		if h := checkAlarms(srv, excludedAlarms); h.Health != "true" {
			return h
		}
		if h := checkLeader(srv, serializable); h.Health != "true" {
			return h
		}
		return checkAPI(ctx, srv, serializable)
	}))
}

// NewHealthHandler handles '/health' requests.
func NewHealthHandler(hfunc func(ctx context.Context, excludedAlarms AlarmSet, serializable bool) Health) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			plog.Warningf("/health error (status code %d)", http.StatusMethodNotAllowed)
			return
		}
		excludedAlarms := getExcludedAlarms(r)
		// Passing the query parameter "serializable=true" ensures that the
		// health of the local etcd is checked vs the health of the cluster.
		// This is useful for probes attempting to validate the liveness of
		// the etcd process vs readiness of the cluster to serve requests.
		serializableFlag := getSerializableFlag(r)
		h := hfunc(r.Context(), excludedAlarms, serializableFlag)
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
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(d)
		plog.Debugf("/health OK (status code %d)", http.StatusOK)
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
	Reason string `json:"-"`
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

func getSerializableFlag(r *http.Request) bool {
	return r.URL.Query().Get("serializable") == "true"
}

// TODO: etcdserver.ErrNoLeader in health API

func checkAlarms(srv serverHealthV2V3, excludedAlarms AlarmSet) Health {
	h := Health{Health: "true"}
	as := srv.Alarms()
	if len(as) > 0 {
		for _, v := range as {
			alarmName := v.Alarm.String()
			if _, found := excludedAlarms[alarmName]; found {
				plog.Debugf("/health excluded alarm %s", v.String())
				continue
			}

			h.Health = "false"
			switch v.Alarm {
			case pb.AlarmType_NOSPACE:
				h.Reason = "ALARM NOSPACE"
			case pb.AlarmType_CORRUPT:
				h.Reason = "ALARM CORRUPT"
			default:
				h.Reason = "ALARM UNKNOWN"
			}
			plog.Warningf("/health error due to %s", v.String())
			return h
		}
	}

	return h
}

func checkLeader(srv serverHealthV2V3, serializable bool) Health {
	h := Health{Health: "true"}
	if !serializable && (uint64(srv.Leader()) == raft.None) {
		h.Health = "false"
		h.Reason = "RAFT NO LEADER"
		plog.Warningf("/health error; no leader (status code %d)", http.StatusServiceUnavailable)
	}
	return h
}

func checkV2API(ctx context.Context, srv etcdserver.ServerV2) Health {
	h := Health{Health: "true"}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	_, err := srv.Do(ctx, pb.Request{Method: "QGET"})
	cancel()
	if err != nil {
		h.Health = "false"
		h.Reason = fmt.Sprintf("QGET ERROR:%s", err)
		plog.Warningf("/health error; QGET failed %v (status code %d)", err, http.StatusServiceUnavailable)
		return h
	}
	return h
}

func checkAPI(ctx context.Context, srv ServerHealth, serializable bool) Health {
	h := Health{Health: "true"}
	cfg := srv.Config()
	ctx = srv.AuthStore().WithRoot(ctx)
	cctx, cancel := context.WithTimeout(ctx, cfg.ReqTimeout())
	_, err := srv.Range(cctx, &pb.RangeRequest{KeysOnly: true, Limit: 1, Serializable: serializable})
	cancel()
	if err != nil {
		h.Health = "false"
		h.Reason = fmt.Sprintf("RANGE ERROR:%s", err)
		plog.Warningf("serving /health false; Range failed %v (status code %d)", err, http.StatusServiceUnavailable)
		return h
	}
	return h
}
