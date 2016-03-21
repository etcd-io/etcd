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
	"io/ioutil"
	defaultLog "log"
	"net"
	"net/http"
	"time"

	"github.com/cockroachdb/cmux"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
)

// serve accepts incoming connections on the listener l,
// creating a new service goroutine for each. The service goroutines
// read requests and then call handler to reply to them.
func serve(l net.Listener, grpcS *grpc.Server, handler http.Handler, readTimeout time.Duration) error {
	// TODO: assert net.Listener type? Arbitrary listener might break HTTPS server which
	// expect a TLS Conn type.
	httpl := l
	if grpcS != nil {
		m := cmux.New(l)
		grpcl := m.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
		httpl = m.Match(cmux.Any())
		go plog.Fatal(m.Serve())
		go plog.Fatal(grpcS.Serve(grpcl))
	}

	logger := defaultLog.New(ioutil.Discard, "etcdhttp", 0)
	// TODO: add debug flag; enable logging when debug flag is set
	srv := &http.Server{
		Handler:     handler,
		ReadTimeout: readTimeout,
		ErrorLog:    logger, // do not log user error
	}
	return srv.Serve(httpl)
}
