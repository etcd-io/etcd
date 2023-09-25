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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/etcdserver"
)

const (
	PathHealth      = "/health"
	PathProxyHealth = "/proxy/health"
)

type ServerHealth interface {
	serverHealthV2V3
	Range(context.Context, *pb.RangeRequest) (*pb.RangeResponse, error)
	Config() config.ServerConfig
	AuthStore() auth.AuthStore
}

type serverHealthV2V3 interface {
	Alarms() []*pb.AlarmMember
	Leader() types.ID
}

// HandleHealth registers metrics and health handlers for v2.
func HandleHealthForV2(lg *zap.Logger, mux *http.ServeMux, srv etcdserver.ServerV2) {
	mux.Handle(PathHealth, NewHealthHandler(lg, func(ctx context.Context, excludedAlarms StringSet, serializable bool) Health {
		if h := checkAlarms(lg, srv, excludedAlarms); h.Health != "true" {
			return h
		}
		if h := checkLeader(lg, srv, serializable); h.Health != "true" {
			return h
		}
		return checkV2API(ctx, lg, srv)
	}))
}

// HandleHealth registers metrics and health handlers. it checks health by using v3 range request
// and its corresponding timeout.
func HandleHealth(lg *zap.Logger, mux *http.ServeMux, srv ServerHealth) {
	mux.Handle(PathHealth, NewHealthHandler(lg, func(ctx context.Context, excludedAlarms StringSet, serializable bool) Health {
		if h := checkAlarms(lg, srv, excludedAlarms); h.Health != "true" {
			return h
		}
		if h := checkLeader(lg, srv, serializable); h.Health != "true" {
			return h
		}
		return checkAPI(ctx, lg, srv, serializable)
	}))

	installLivezEndpoints(lg, mux, srv)
	installReadyzEndpoints(lg, mux, srv)
}

// NewHealthHandler handles '/health' requests.
func NewHealthHandler(lg *zap.Logger, hfunc func(ctx context.Context, excludedAlarms StringSet, Serializable bool) Health) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			lg.Warn("/health error", zap.Int("status-code", http.StatusMethodNotAllowed))
			return
		}
		excludedAlarms := getQuerySet(r, "exclude")
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

func getQuerySet(r *http.Request, query string) StringSet {
	querySet := make(map[string]struct{})
	qs, found := r.URL.Query()[query]
	if found {
		for _, q := range qs {
			if len(q) == 0 {
				continue
			}
			querySet[q] = struct{}{}
		}
	}
	return querySet
}

func getSerializableFlag(r *http.Request) bool {
	return r.URL.Query().Get("serializable") == "true"
}

// TODO: etcdserver.ErrNoLeader in health API

func checkAlarms(lg *zap.Logger, srv serverHealthV2V3, excludedAlarms StringSet) Health {
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
			case pb.AlarmType_NOSPACE:
				h.Reason = "ALARM NOSPACE"
			case pb.AlarmType_CORRUPT:
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

func checkLeader(lg *zap.Logger, srv serverHealthV2V3, serializable bool) Health {
	h := Health{Health: "true"}
	if !serializable && (uint64(srv.Leader()) == raft.None) {
		h.Health = "false"
		h.Reason = "RAFT NO LEADER"
		lg.Warn("serving /health false; no leader")
	}
	return h
}

func checkV2API(ctx context.Context, lg *zap.Logger, srv etcdserver.ServerV2) Health {
	h := Health{Health: "true"}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	_, err := srv.Do(ctx, pb.Request{Method: "QGET"})
	cancel()
	if err != nil {
		h.Health = "false"
		h.Reason = fmt.Sprintf("QGET ERROR:%s", err)
		lg.Warn("serving /health false; QGET fails", zap.Error(err))
		return h
	}
	lg.Debug("serving /health true")
	return h
}

func checkAPI(ctx context.Context, lg *zap.Logger, srv ServerHealth, serializable bool) Health {
	h := Health{Health: "true"}
	cfg := srv.Config()
	ctx = srv.AuthStore().WithRoot(ctx)
	cctx, cancel := context.WithTimeout(ctx, cfg.ReqTimeout())
	_, err := srv.Range(cctx, &pb.RangeRequest{KeysOnly: true, Limit: 1, Serializable: serializable})
	cancel()
	if err != nil {
		h.Health = "false"
		h.Reason = fmt.Sprintf("RANGE ERROR:%s", err)
		lg.Warn("serving /health false; Range fails", zap.Error(err))
		return h
	}
	lg.Debug("serving /health true")
	return h
}

type HealthCheck func(ctx context.Context) error

type CheckRegistry struct {
	path   string
	checks map[string]HealthCheck
}

func installLivezEndpoints(lg *zap.Logger, mux *http.ServeMux, server ServerHealth) {
	reg := CheckRegistry{path: "/livez", checks: make(map[string]HealthCheck)}
	reg.Register("serializable_read", serializableReadCheck(server))
	reg.InstallHttpEndpoints(lg, mux)
}

func installReadyzEndpoints(lg *zap.Logger, mux *http.ServeMux, server ServerHealth) {
	reg := CheckRegistry{path: "/readyz", checks: make(map[string]HealthCheck)}
	reg.Register("data_corruption", activeAlarmCheck(server, pb.AlarmType_CORRUPT))
	reg.Register("serializable_read", serializableReadCheck(server))
	reg.InstallHttpEndpoints(lg, mux)
}

func (reg *CheckRegistry) Register(name string, check HealthCheck) {
	reg.checks[name] = check
}

func (reg *CheckRegistry) InstallHttpEndpoints(lg *zap.Logger, mux *http.ServeMux) {
	checkNames := make([]string, 0, len(reg.checks))
	for k := range reg.checks {
		checkNames = append(checkNames, k)
	}

	// installs the http handler for the root path.
	reg.installRootHttpEndpoint(lg, mux, reg.path, checkNames...)
	for _, checkName := range checkNames {
		// installs the http handler for the individual check sub path.
		subpath := path.Join(reg.path, checkName)
		check := checkName
		mux.Handle(subpath, newHealthHandler(subpath, lg, func(r *http.Request) Health {
			return reg.runHealthChecks(r.Context(), check)
		}))
	}
}

func (reg *CheckRegistry) runHealthChecks(ctx context.Context, checkNames ...string) Health {
	h := Health{Health: "true"}
	var individualCheckOutput bytes.Buffer
	for _, checkName := range checkNames {
		check, found := reg.checks[checkName]
		if !found {
			panic(fmt.Errorf("Health check: %s not registered", checkName))
		}
		if err := check(ctx); err != nil {
			fmt.Fprintf(&individualCheckOutput, "[-]%s failed: %v\n", checkName, err)
			h.Health = "false"
		} else {
			fmt.Fprintf(&individualCheckOutput, "[+]%s ok\n", checkName)
		}
	}
	h.Reason = individualCheckOutput.String()
	return h
}

// installRootHttpEndpoint installs the http handler for the root path.
func (reg *CheckRegistry) installRootHttpEndpoint(lg *zap.Logger, mux *http.ServeMux, path string, checks ...string) {
	hfunc := func(r *http.Request) Health {
		// extracts the health check names to be excludeList from the query param
		excluded := getQuerySet(r, "exclude")

		filteredCheckNames := filterCheckList(lg, listToStringSet(checks), excluded)
		return reg.runHealthChecks(r.Context(), filteredCheckNames...)
	}
	mux.Handle(path, newHealthHandler(path, lg, hfunc))
}

// newHealthHandler generates a http HandlerFunc for a health check function hfunc.
func newHealthHandler(path string, lg *zap.Logger, hfunc func(*http.Request) Health) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			lg.Warn("Health request error", zap.String("path", path), zap.Int("status-code", http.StatusMethodNotAllowed))
			return
		}
		h := hfunc(r)
		// Always returns detailed reason for failed checks.
		if h.Health != "true" {
			http.Error(w, h.Reason, http.StatusServiceUnavailable)
			lg.Error("Health check error", zap.String("path", path), zap.String("reason", h.Reason), zap.Int("status-code", http.StatusServiceUnavailable))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Only writes detailed reason for verbose requests.
		if _, found := r.URL.Query()["verbose"]; found {
			fmt.Fprint(w, h.Reason)
		}
		fmt.Fprint(w, "ok\n")
		lg.Debug("Health check OK", zap.String("path", path), zap.String("reason", h.Reason), zap.Int("status-code", http.StatusOK))
	}
}

func filterCheckList(lg *zap.Logger, checks StringSet, excluded StringSet) []string {
	filteredList := []string{}
	for chk := range checks {
		if _, found := excluded[chk]; found {
			delete(excluded, chk)
			continue
		}
		filteredList = append(filteredList, chk)
	}
	if len(excluded) > 0 {
		// For version compatibility, excluding non-exist checks would not fail the request.
		lg.Warn("some health checks cannot be excluded", zap.String("missing-health-checks", formatQuoted(excluded.List()...)))
	}
	return filteredList
}

// formatQuoted returns a formatted string of the health check names,
// preserving the order passed in.
func formatQuoted(names ...string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	return strings.Join(quoted, ",")
}

type StringSet map[string]struct{}

func (s StringSet) List() []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

func listToStringSet(list []string) StringSet {
	set := make(map[string]struct{})
	for _, s := range list {
		set[s] = struct{}{}
	}
	return set
}

// activeAlarmCheck checks if a specific alarm type is active in the server.
func activeAlarmCheck(srv ServerHealth, at pb.AlarmType) func(context.Context) error {
	return func(ctx context.Context) error {
		as := srv.Alarms()
		for _, v := range as {
			if v.Alarm == at {
				return fmt.Errorf("alarm activated: %s", at.String())
			}
		}
		return nil
	}
}

func serializableReadCheck(srv ServerHealth) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		ctx = srv.AuthStore().WithRoot(ctx)
		_, err := srv.Range(ctx, &pb.RangeRequest{KeysOnly: true, Limit: 1, Serializable: true})
		if err != nil {
			return fmt.Errorf("range error: %w", err)
		}
		return nil
	}
}
