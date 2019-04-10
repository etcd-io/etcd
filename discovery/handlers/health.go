package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coreos/discovery.etcd.io/handlers/httperror"
	"github.com/prometheus/client_golang/prometheus"
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
	prometheus.MustRegister(healthCounter)
}

func HealthHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	st := ctx.Value(stateKey).(*State)

	token, err := st.setupToken(0)
	if err != nil || token == "" {
		log.Printf("health failed to setupToken %v", err)
		httperror.Error(w, r, "health failed to setupToken", 400, healthCounter)
		return
	}

	err = st.deleteToken(token)
	if err != nil {
		log.Printf("health failed to deleteToken %v", err)
		httperror.Error(w, r, "health failed to deleteToken", 400, healthCounter)
		return
	}

	fmt.Fprintf(w, "OK")
	healthCounter.WithLabelValues("200", r.Method).Add(1)
}
