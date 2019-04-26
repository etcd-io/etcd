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

package grpcproxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.etcd.io/etcd/etcdserver/api/etcdhttp"
)

// HandleHealth registers health handler on '/health'.
func HandleHealth(mux *http.ServeMux, c *http.Client, eps []string) {
	pathHealth := etcdhttp.PathHealth
	mux.HandleFunc(pathHealth, func(w http.ResponseWriter, r *http.Request) {
		target := createTarget(eps, pathHealth, r.TLS, true)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp, err := c.Get(target)
		if err != nil {
			body, _ := json.Marshal(etcdhttp.Health{Health: "false"})
			fmt.Fprintf(w, "%s", body)
		} else {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			fmt.Fprintf(w, "%s", body)
		}
	})
}
