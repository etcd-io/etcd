// Copyright 2015 CoreOS, Inc.
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

package etcdmain

import (
	"bytes"
	"fmt"
	"io/ioutil"
	defaultLog "log"
	"net"
	"net/http"
	"os"
	"time"
)

// serveHTTP accepts incoming HTTP connections on the listener l,
// creating a new service goroutine for each. The service goroutines
// read requests and then call handler to reply to them.
func serveHTTP(l net.Listener, handler http.Handler, readTimeout time.Duration, debug bool) error {
	srv := &http.Server{
		Handler:     handler,
		ReadTimeout: readTimeout,
	}
	var logger *defaultLog.Logger
	if debug {
		now := time.Now()
		pf := new(bytes.Buffer)
		pf.WriteString(now.Format("2006-01-02 15:04:05"))
		pf.WriteString(fmt.Sprintf(".%06d", now.Nanosecond()/1000))
		pf.WriteString(" D | etcdhttp: ")
		logger = defaultLog.New(os.Stderr, pf.String(), 0)
	} else {
		// do not log user error
		logger = defaultLog.New(ioutil.Discard, "etcdhttp", 0)
	}
	srv.ErrorLog = logger
	return srv.Serve(l)
}
