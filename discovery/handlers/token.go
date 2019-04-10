package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"go.etcd.io/etcd/discovery/handlers/httperror"

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

	ev, err := st.v2.Get(r.URL.Path[1:], true, true)
	if err != nil {
		log.Printf("TokenHandler: %v", err)
		httperror.Error(w, r, "Unable to GET token", 404, tokenCounter)
		return
	}

	n, err := json.Marshal(ev.Node)
	if err != nil {
		log.Printf("TokenHandler: %v", err)
		httperror.Error(w, r, "Unable to GET token", 400, tokenCounter)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(n)
}
