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

	"go.etcd.io/etcd/version"
)

func TestServeVersion(t *testing.T) {
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	rw := httptest.NewRecorder()
	serveVersion(rw, req, "2.1.0")
	if rw.Code != http.StatusOK {
		t.Errorf("code=%d, want %d", rw.Code, http.StatusOK)
	}
	vs := version.Versions{
		Server:  version.Version,
		Cluster: "2.1.0",
	}
	w, err := json.Marshal(&vs)
	if err != nil {
		t.Fatal(err)
	}
	if g := rw.Body.String(); g != string(w) {
		t.Fatalf("body = %q, want %q", g, string(w))
	}
	if ct := rw.HeaderMap.Get("Content-Type"); ct != "application/json" {
		t.Errorf("contet-type header = %s, want %s", ct, "application/json")
	}
}

func TestServeVersionFails(t *testing.T) {
	for _, m := range []string{
		"CONNECT", "TRACE", "PUT", "POST", "HEAD",
	} {
		req, err := http.NewRequest(m, "", nil)
		if err != nil {
			t.Fatalf("error creating request: %v", err)
		}
		rw := httptest.NewRecorder()
		serveVersion(rw, req, "2.1.0")
		if rw.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: code=%d, want %d", m, rw.Code, http.StatusMethodNotAllowed)
		}
	}
}
