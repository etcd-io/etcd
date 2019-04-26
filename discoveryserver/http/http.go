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

package http

import (
	"context"
	"net/http"
	"os"

	"go.etcd.io/etcd/discoveryserver/handlers"
	"go.etcd.io/etcd/discoveryserver/metrics"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Setup(ctx context.Context, etcdHost, discHost string) *handlers.State {
	handler, st := RegisterHandlers(ctx, etcdHost, discHost)
	logH := gorillaHandlers.LoggingHandler(os.Stdout, handler)

	http.Handle("/", logH)
	http.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))

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
