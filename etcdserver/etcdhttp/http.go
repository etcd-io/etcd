/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdhttp

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
)

const (
	// time to wait for response from EtcdServer requests
	defaultServerTimeout = 5 * time.Minute

	// time to wait for a Watch request
	defaultWatchTimeout = 5 * time.Minute
)

var errClosed = errors.New("etcdhttp: client closed connection")

// writeError logs and writes the given Error to the ResponseWriter
// If Error is an etcdErr, it is rendered to the ResponseWriter
// Otherwise, it is assumed to be an InternalServerError
func writeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	switch e := err.(type) {
	case *etcdErr.Error:
		e.WriteTo(w)
	case *httptypes.HTTPError:
		e.WriteTo(w)
	default:
		log.Printf("etcdhttp: unexpected error: %v", err)
		herr := httptypes.NewHTTPError(http.StatusInternalServerError, "Internal Server Error")
		herr.WriteTo(w)
	}
}

// allowMethod verifies that the given method is one of the allowed methods,
// and if not, it writes an error to w.  A boolean is returned indicating
// whether or not the method is allowed.
func allowMethod(w http.ResponseWriter, m string, ms ...string) bool {
	for _, meth := range ms {
		if m == meth {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(ms, ","))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}
