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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
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
	// Ensures the server(runtime) doesn't panic due to the execution of
	// panic failpoints during processing of the HTTP request, as the
	// sender of the HTTP request should not be affected by the execution
	// of the panic failpoints and crash as a side effect
	panicMu.Lock()
	defer panicMu.Unlock()

	// flush before unlocking so a panic failpoint won't
	// take down the http server before it sends the response
	defer flush(w)

	key := r.RequestURI
	if len(key) == 0 || key[0] != '/' {
		http.Error(w, "malformed request URI", http.StatusBadRequest)
		return
	}
	key = key[1:]

	switch {
	// sets the failpoint
	case r.Method == "PUT":
		v, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed ReadAll in PUT", http.StatusBadRequest)
			return
		}

		fpMap := map[string]string{key: string(v)}
		if strings.EqualFold(key, "failpoints") {
			fpMap, err = parseFailpoints(string(v))
			if err != nil {
				http.Error(w, fmt.Sprintf("fail to parse failpoint: %v", err), http.StatusBadRequest)
				return
			}
		}

		for k, v := range fpMap {
			if err := Enable(k, v); err != nil {
				http.Error(w, fmt.Sprintf("fail to set failpoint: %v", err), http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)

	// gets status of the failpoint
	case r.Method == "GET":
		if len(key) == 0 {
			fps := list()
			sort.Strings(fps)
			lines := make([]string, len(fps))
			for i := range lines {
				s, _, _ := Status(fps[i])
				lines[i] = fps[i] + "=" + s
			}
			w.Write([]byte(strings.Join(lines, "\n") + "\n"))
		} else if strings.HasSuffix(key, "/count") {
			fp := key[:len(key)-len("/count")]
			_, count, err := Status(fp)
			if err != nil {
				if errors.Is(err, ErrNoExist) {
					http.Error(w, "failed to GET: "+err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, "failed to GET: "+err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.Write([]byte(strconv.Itoa(count)))
		} else {
			status, _, err := Status(key)
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

func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
