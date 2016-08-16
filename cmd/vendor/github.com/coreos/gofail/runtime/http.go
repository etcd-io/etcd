// Copyright 2016 CoreOS, Inc.
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

package runtime

import (
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"strings"
)

type httpHandler struct{}

func serve(host string) error {
	ln, err := net.Listen("tcp", host)
	if err != nil {
		return err
	}
	go http.Serve(ln, &httpHandler{})
	return nil
}

func (*httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.RequestURI
	if len(key) == 0 || key[0] != '/' {
		http.Error(w, "malformed request URI", http.StatusBadRequest)
		return
	}
	key = key[1:]

	switch {
	// sets the failpoint
	case r.Method == "PUT":
		v, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed ReadAll in PUT", http.StatusBadRequest)
			return
		}
		unlock, eerr := enableAndLock(key, string(v))
		if eerr != nil {
			http.Error(w, "failed to set failpoint "+string(key), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		if f, ok := w.(http.Flusher); ok {
			// flush before unlocking so a panic failpoint won't
			// take down the http server before it sends the response
			f.Flush()
		}
		unlock()
	// gets status of the failpoint
	case r.Method == "GET":
		if len(key) == 0 {
			fps := List()
			sort.Strings(fps)
			lines := make([]string, len(fps))
			for i := range lines {
				s, _ := Status(fps[i])
				lines[i] = fps[i] + "=" + s
			}
			w.Write([]byte(strings.Join(lines, "\n") + "\n"))
		} else {
			status, err := Status(key)
			if err != nil {
				http.Error(w, "failed to GET: "+err.Error(), http.StatusNotFound)
			}
			w.Write([]byte(status + "\n"))
		}
	// deactivates a failpoint
	case r.Method == "DELETE":
		if err := Disable(key); err != nil {
			http.Error(w, "failed to delete failpoint "+err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Add("Allow", "DELETE")
		w.Header().Add("Allow", "GET")
		w.Header().Set("Allow", "PUT")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
