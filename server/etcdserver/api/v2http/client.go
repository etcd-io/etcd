// Copyright 2015 The etcd Authors
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

// Package v2http provides etcd client and server implementations.
package v2http

import (
	"net/http"
	"time"

	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/etcdhttp"
	"go.uber.org/zap"
)

// NewClientHandler generates a muxed http.Handler with the given parameters to serve etcd client requests.
func NewClientHandler(lg *zap.Logger, server etcdserver.ServerPeer, timeout time.Duration) http.Handler {
	if lg == nil {
		lg = zap.NewNop()
	}
	mux := http.NewServeMux()
	etcdhttp.HandleBasic(lg, mux, server)
	etcdhttp.HandleMetricsHealth(lg, mux, server)
	return requestLogger(lg, mux)
}

func requestLogger(lg *zap.Logger, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lg != nil {
			lg.Debug(
				"handling HTTP request",
				zap.String("method", r.Method),
				zap.String("request-uri", r.RequestURI),
				zap.String("remote-addr", r.RemoteAddr),
			)
		}
		handler.ServeHTTP(w, r)
	})
}
