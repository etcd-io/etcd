// Copyright 2022 The etcd Authors
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
	"net/http"

	"go.uber.org/zap"

	httptypes "go.etcd.io/etcd/server/v3/etcdserver/api/etcdhttp/types"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2error"
	"go.etcd.io/etcd/server/v3/etcdserver/errors"
)

func allowMethod(w http.ResponseWriter, r *http.Request, m string) bool {
	if m == r.Method {
		return true
	}
	w.Header().Set("Allow", m)
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}

// writeError logs and writes the given Error to the ResponseWriter
// If Error is an etcdErr, it is rendered to the ResponseWriter
// Otherwise, it is assumed to be a StatusInternalServerError
func writeError(lg *zap.Logger, w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	switch e := err.(type) {
	case *v2error.Error:
		e.WriteTo(w)

	case *httptypes.HTTPError:
		if et := e.WriteTo(w); et != nil {
			if lg != nil {
				lg.Debug(
					"failed to write v2 HTTP error",
					zap.String("remote-addr", r.RemoteAddr),
					zap.String("internal-server-error", e.Error()),
					zap.Error(et),
				)
			}
		}

	default:
		switch err {
		case errors.ErrTimeoutDueToLeaderFail, errors.ErrTimeoutDueToConnectionLost, errors.ErrNotEnoughStartedMembers,
			errors.ErrUnhealthy:
			if lg != nil {
				lg.Warn(
					"v2 response error",
					zap.String("remote-addr", r.RemoteAddr),
					zap.String("internal-server-error", err.Error()),
				)
			}

		default:
			if lg != nil {
				lg.Warn(
					"unexpected v2 response error",
					zap.String("remote-addr", r.RemoteAddr),
					zap.String("internal-server-error", err.Error()),
				)
			}
		}

		herr := httptypes.NewHTTPError(http.StatusInternalServerError, "Internal Server Error")
		if et := herr.WriteTo(w); et != nil {
			if lg != nil {
				lg.Debug(
					"failed to write v2 HTTP error",
					zap.String("remote-addr", r.RemoteAddr),
					zap.String("internal-server-error", err.Error()),
					zap.Error(et),
				)
			}
		}
	}
}
