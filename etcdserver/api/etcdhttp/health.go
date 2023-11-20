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

// This file defines the http endpoints for etcd health checks.
// The endpoints include /livez, /readyz and /health.

package etcdhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"go.etcd.io/etcd/auth"
	"go.etcd.io/etcd/etcdserver"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/types"
	"go.etcd.io/etcd/raft"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	PathHealth                 = "/health"
	HealthStatusSuccess string = "success"
	HealthStatusError   string = "error"
	checkTypeLivez             = "livez"
	checkTypeReadyz            = "readyz"
	checkTypeHealth            = "health"
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
	mux.Handle(PathHealth, NewHealthHandler(func(ctx context.Context, excludedAlarms StringSet, serializable bool) Health {
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
	mux.Handle(PathHealth, NewHealthHandler(func(ctx context.Context, excludedAlarms StringSet, serializable bool) Health {
		if h := checkAlarms(srv, excludedAlarms); h.Health != "true" {
			return h
		}
		if h := checkLeader(srv, serializable); h.Health != "true" {
			return h
		}
		return checkAPI(ctx, srv, serializable)
	}))

	installLivezEndpoints(mux, srv)
	installReadyzEndpoints(mux, srv)
}

// NewHealthHandler handles '/health' requests.
func NewHealthHandler(hfunc func(ctx context.Context, excludedAlarms StringSet, serializable bool) Health) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			plog.Warningf("/health error (status code %d)", http.StatusMethodNotAllowed)
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
	healthCheckGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "etcd",
		Subsystem: "server",
		Name:      "healthcheck",
		Help:      "The result of each kind of healthcheck.",
	},
		[]string{"type", "name"},
	)
	healthCheckCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "server",
		Name:      "healthchecks_total",
		Help:      "The total number of each kind of healthcheck.",
	},
		[]string{"type", "name", "status"},
	)
)

func init() {
	prometheus.MustRegister(healthSuccess)
	prometheus.MustRegister(healthFailed)
	prometheus.MustRegister(healthCheckGauge)
	prometheus.MustRegister(healthCheckCounter)
}

// Health defines etcd server health status.
// TODO: remove manual parsing in etcdctl cluster-health
type Health struct {
	Health string `json:"health"`
	Reason string `json:"-"`
}

// HealthStatus is used in new /readyz or /livez health checks instead of the Health struct.
type HealthStatus struct {
	Reason string `json:"reason"`
	Status string `json:"status"`
}

func getQuerySet(r *http.Request, query string) StringSet {
	querySet := make(map[string]struct{})
	qs, found := r.URL.Query()[query]
	if found {
		for _, q := range qs {
			if len(qs) == 0 {
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

func checkAlarms(srv serverHealthV2V3, excludedAlarms StringSet) Health {
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
	plog.Debug("serving /health true")
	return h
}

type HealthCheck func(ctx context.Context) error

type CheckRegistry struct {
	checkType string
	checks    map[string]HealthCheck
}

func installLivezEndpoints(mux *http.ServeMux, server ServerHealth) {
	reg := CheckRegistry{checkType: checkTypeLivez, checks: make(map[string]HealthCheck)}
	reg.Register("serializable_read", readCheck(server, true /* serializable */))
	reg.InstallHttpEndpoints(mux)
}

func installReadyzEndpoints(mux *http.ServeMux, server ServerHealth) {
	reg := CheckRegistry{checkType: checkTypeReadyz, checks: make(map[string]HealthCheck)}
	reg.Register("data_corruption", activeAlarmCheck(server, pb.AlarmType_CORRUPT))
	// serializable_read checks if local read is ok.
	// linearizable_read checks if there is consensus in the cluster.
	// Having both serializable_read and linearizable_read helps isolate the cause of problems if there is a read failure.
	reg.Register("serializable_read", readCheck(server, true))
	// linearizable_read check would be replaced by read_index check in 3.6
	reg.Register("linearizable_read", readCheck(server, false))
	reg.InstallHttpEndpoints(mux)
}

func (reg *CheckRegistry) Register(name string, check HealthCheck) {
	reg.checks[name] = check
}

func (reg *CheckRegistry) RootPath() string {
	return "/" + reg.checkType
}

func (reg *CheckRegistry) InstallHttpEndpoints(mux *http.ServeMux) {
	checkNames := make([]string, 0, len(reg.checks))
	for k := range reg.checks {
		checkNames = append(checkNames, k)
	}

	// installs the http handler for the root path.
	reg.installRootHttpEndpoint(mux, checkNames...)
	for _, checkName := range checkNames {
		// installs the http handler for the individual check sub path.
		subpath := path.Join(reg.RootPath(), checkName)
		check := checkName
		mux.Handle(subpath, newHealthHandler(subpath, func(r *http.Request) HealthStatus {
			return reg.runHealthChecks(r.Context(), check)
		}))
	}
}

func (reg *CheckRegistry) runHealthChecks(ctx context.Context, checkNames ...string) HealthStatus {
	h := HealthStatus{Status: HealthStatusSuccess}
	var individualCheckOutput bytes.Buffer
	for _, checkName := range checkNames {
		check, found := reg.checks[checkName]
		if !found {
			panic(fmt.Errorf("Health check: %s not registered", checkName))
		}
		if err := check(ctx); err != nil {
			fmt.Fprintf(&individualCheckOutput, "[-]%s failed: %v\n", checkName, err)
			h.Status = HealthStatusError
			recordMetrics(reg.checkType, checkName, HealthStatusError)
		} else {
			fmt.Fprintf(&individualCheckOutput, "[+]%s ok\n", checkName)
			recordMetrics(reg.checkType, checkName, HealthStatusSuccess)
		}
	}
	h.Reason = individualCheckOutput.String()
	return h
}

// installRootHttpEndpoint installs the http handler for the root path.
func (reg *CheckRegistry) installRootHttpEndpoint(mux *http.ServeMux, checks ...string) {
	hfunc := func(r *http.Request) HealthStatus {
		// extracts the health check names to be excludeList from the query param
		excluded := getQuerySet(r, "exclude")

		filteredCheckNames := filterCheckList(listToStringSet(checks), excluded)
		h := reg.runHealthChecks(r.Context(), filteredCheckNames...)
		return h
	}
	mux.Handle(reg.RootPath(), newHealthHandler(reg.RootPath(), hfunc))
}

// newHealthHandler generates a http HandlerFunc for a health check function hfunc.
func newHealthHandler(path string, hfunc func(*http.Request) HealthStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			plog.Warningf("Health request error path=%s (status code %d)", path, http.StatusMethodNotAllowed)
			return
		}
		h := hfunc(r)
		// Always returns detailed reason for failed checks.
		if h.Status == HealthStatusError {
			http.Error(w, h.Reason, http.StatusServiceUnavailable)
			plog.Errorf("Health check error path=%s, reason=%s (status code %d)", path, h.Reason, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Only writes detailed reason for verbose requests.
		if _, found := r.URL.Query()["verbose"]; found {
			fmt.Fprint(w, h.Reason)
		}
		fmt.Fprint(w, "ok\n")
		plog.Debugf("Health OK path=%s, reason=%s (status code %d)", path, h.Reason, http.StatusOK)
	}
}

func filterCheckList(checks StringSet, excluded StringSet) []string {
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
		plog.Warningf("some health checks cannot be excluded, missing-health-checks=%s", formatQuoted(excluded.List()...))
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

func recordMetrics(checkType, name string, status string) {
	val := 0.0
	if status == HealthStatusSuccess {
		val = 1.0
	}
	healthCheckGauge.With(prometheus.Labels{
		"type": checkType,
		"name": name,
	}).Set(val)
	healthCheckCounter.With(prometheus.Labels{
		"type":   checkType,
		"name":   name,
		"status": status,
	}).Inc()
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

func readCheck(srv ServerHealth, serializable bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		ctx = srv.AuthStore().WithRoot(ctx)
		_, err := srv.Range(ctx, &pb.RangeRequest{KeysOnly: true, Limit: 1, Serializable: serializable})
		return err
	}
}
