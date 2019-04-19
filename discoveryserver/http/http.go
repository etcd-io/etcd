package http

import (
	"context"
	"net/http"
	"os"

	"go.etcd.io/etcd/discoveryserver/handlers"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Setup(ctx context.Context, etcdHost, discHost string) *handlers.State {
	handler, st := RegisterHandlers(ctx, etcdHost, discHost)
	logH := gorillaHandlers.LoggingHandler(os.Stdout, handler)

	http.Handle("/", logH)
	http.Handle("/metrics", promhttp.Handler())

	return st
}

func RegisterHandlers(ctx context.Context, etcdHost, discHost string) (http.Handler, *handlers.State) {
	st := handlers.Setup(etcdHost, discHost)
	r := mux.NewRouter()

	r.HandleFunc("/", handlers.HomeHandler)
	r.Handle("/new", &handlers.ContextAdapter{
		Ctx:     ctx,
		Handler: handlers.With(handlers.ContextHandlerFunc(handlers.NewTokenHandler), st),
	})
	r.Handle("/health", &handlers.ContextAdapter{
		Ctx:     ctx,
		Handler: handlers.With(handlers.ContextHandlerFunc(handlers.HealthHandler), st),
	})
	r.HandleFunc("/robots.txt", handlers.RobotsHandler)

	// Only allow exact tokens with GETs and PUTs
	r.Handle("/{token:[a-f0-9]{32}}", &handlers.ContextAdapter{
		Ctx:     ctx,
		Handler: handlers.With(handlers.ContextHandlerFunc(handlers.TokenHandler), st),
	}).Methods("GET", "PUT")
	r.Handle("/{token:[a-f0-9]{32}}/", &handlers.ContextAdapter{
		Ctx:     ctx,
		Handler: handlers.With(handlers.ContextHandlerFunc(handlers.TokenHandler), st),
	}).Methods("GET", "PUT")
	r.Handle("/{token:[a-f0-9]{32}}/{machine}", &handlers.ContextAdapter{
		Ctx:     ctx,
		Handler: handlers.With(handlers.ContextHandlerFunc(handlers.TokenHandler), st),
	}).Methods("GET", "PUT", "DELETE")
	r.Handle("/{token:[a-f0-9]{32}}/_config/size", &handlers.ContextAdapter{
		Ctx:     ctx,
		Handler: handlers.With(handlers.ContextHandlerFunc(handlers.TokenHandler), st),
	}).Methods("GET")

	return r, st
}
