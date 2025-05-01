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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/version"
)

func TestServeVersion(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoErrorf(t, err, "error creating request: %v", err)
	rw := httptest.NewRecorder()
	serveVersion(rw, req, "3.6.0", "3.5.2")
	assert.Equalf(t, http.StatusOK, rw.Code, "code=%d, want %d", rw.Code, http.StatusOK)
	vs := version.Versions{
		Server:  version.Version,
		Cluster: "3.6.0",
		Storage: "3.5.2",
	}
	w, err := json.Marshal(&vs)
	require.NoError(t, err)
	g := rw.Body.String()
	require.Equalf(t, g, string(w), "body = %q, want %q", g, string(w))
	ct := rw.HeaderMap.Get("Content-Type")
	assert.Equalf(t, "application/json", ct, "contet-type header = %s, want %s", ct, "application/json")
}

func TestServeVersionFails(t *testing.T) {
	for _, m := range []string{
		"CONNECT", "TRACE", "PUT", "POST", "HEAD",
	} {
		t.Run(m, func(t *testing.T) {
			req, err := http.NewRequest(m, "", nil)
			require.NoErrorf(t, err, "error creating request: %v", err)
			rw := httptest.NewRecorder()
			serveVersion(rw, req, "3.6.0", "3.5.2")
			assert.Equalf(t, http.StatusMethodNotAllowed, rw.Code, "method %s: code=%d, want %d", m, rw.Code, http.StatusMethodNotAllowed)
		})
	}
}
