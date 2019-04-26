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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/etcd/discoveryserver/handlers/httperror"
	"go.etcd.io/etcd/discoveryserver/metrics"
	"go.etcd.io/etcd/etcdserver/api/v2store"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	tokenCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "endpoint_token_requests_total",
			Help: "How many /token requests processed, partitioned by status code and HTTP method.",
		},
		[]string{"code", "method"},
	)
	metrics.Registry.MustRegister(tokenCounter)
}

var tokenCounter *prometheus.CounterVec

func TokenHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	st := ctx.Value(stateKey).(*State)

	var ev *v2store.Event
	var err error
	switch r.Method {
	case http.MethodGet:
		var recursive bool
		rq := r.URL.Query().Get("recursive")
		recursive, err := strconv.ParseBool(rq)
		if err != nil {
			recursive = false
		}

		pq := r.URL.Query().Get("waitIndex")
		waitIndex, err := strconv.ParseUint(pq, 10, 64)
		if err != nil {
			waitIndex = 0
		}

		wq := r.URL.Query().Get("wait")
		if wq != "" {
			watcher, err := st.v2.Watch(r.URL.Path, recursive, false, waitIndex+1)
			go func() {
				time.Sleep(time.Minute)
				watcher.Remove()
			}()
			ev = <-watcher.EventChan()
			watcher.Remove()
			if err != nil || ev == nil {
				log.Printf("TokenHandler: GET+wait: %v", err)
				w.WriteHeader(http.StatusOK)
				return
			}

			break
		}

		ev, err = st.v2.Get(r.URL.Path, recursive, true)
		if err != nil {
			log.Printf("TokenHandler: GET: %v", err)
			httperror.Error(w, r, "unable to GET token", 400, tokenCounter)
			return
		}

		// Filter out hidden nodes NOTE: there is only 1 level of
		// dirs in discovery so don't do anything fancy here
		if ev.Node.Dir {
			nodes := ev.Node.Nodes[:0]
			for _, n := range ev.Node.Nodes {
				if !strings.HasPrefix(path.Base(n.Key), "_") {
					nodes = append(nodes, n)
				}
			}
			ev.Node.Nodes = nodes
		}
	case http.MethodDelete:
		ev, err = st.v2.Delete(r.URL.Path, true, true)
		if err != nil {
			log.Printf("TokenHandler: DELETE: %v", err)
			httperror.Error(w, r, "unable to DELETE token", 400, tokenCounter)
			return
		}
	case http.MethodPut:
		ev, err = st.v2.Set(r.URL.Path, false, r.FormValue("value"), v2store.TTLOptionSet{})
		if err != nil {
			log.Printf("TokenHandler: PUT: %v", err)
			httperror.Error(w, r, "unable to PUT token", 400, tokenCounter)
			return
		}
	default:
		log.Printf("TokenHandler bad HTTP method: %v", r.Method)
		httperror.Error(w, r, "unable to GET token", 400, tokenCounter)
		return
	}

	n, err := json.Marshal(ev)
	if err != nil {
		log.Printf("TokenHandler: marshal: %v", err)
		httperror.Error(w, r, "unable to serialize token", 400, tokenCounter)
		return
	}

	tokenCounter.WithLabelValues("200", r.Method).Add(1)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Etcd-Index", fmt.Sprintf("%d", ev.EtcdIndex))
	w.WriteHeader(http.StatusOK)
	w.Write(n)
}
