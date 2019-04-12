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
	prometheus.MustRegister(tokenCounter)
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

		pq := r.URL.Query().Get("prevIndex")
		prevIndex, err := strconv.ParseUint(pq, 10, 64)
		if err != nil {
			prevIndex = 0
		}

		wq := r.URL.Query().Get("watch")
		if wq != "" {
			watcher, err := st.v2.Watch(r.URL.Path, recursive, false, uint64(prevIndex))
			go func() {
				time.Sleep(time.Minute)
				watcher.Remove()
			}()
			<-watcher.EventChan()
			if err != nil {
				log.Printf("TokenHandler: %v", err)
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		ev, err = st.v2.Get(r.URL.Path, recursive, true)
		if err != nil {
			log.Printf("TokenHandler: %v", err)
			httperror.Error(w, r, "Unable to GET token", 400, tokenCounter)
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
			log.Printf("TokenHandler: %v", err)
			httperror.Error(w, r, "Unable to DELETE token", 400, tokenCounter)
			return
		}
	case http.MethodPut:
		ev, err = st.v2.Set(r.URL.Path, false, r.FormValue("value"), v2store.TTLOptionSet{})
		if err != nil {
			log.Printf("TokenHandler: %v", err)
			httperror.Error(w, r, "Unable to PUT token", 400, tokenCounter)
			return
		}
	default:
		log.Printf("TokenHandler: %v", err)
		httperror.Error(w, r, "Unable to GET token", 400, tokenCounter)
	}

	n, err := json.Marshal(ev)
	if err != nil {
		log.Printf("TokenHandler: %v", err)
		httperror.Error(w, r, "Unable to GET token", 400, tokenCounter)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Etcd-Index", fmt.Sprintf("%d", ev.EtcdIndex))
	w.WriteHeader(http.StatusOK)
	w.Write(n)
}
