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

package transport

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coreos/etcd/pkg/runtime"
)

func TestLimitedConnListenerAccept(t *testing.T) {
	if _, err := runtime.FDUsage(); err != nil {
		t.Skip("skip test due to unsupported runtime.FDUsage")
	}

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	fdNum, err := runtime.FDUsage()
	if err != nil {
		t.Fatal(err)
	}
	srv := &httptest.Server{
		Listener: &LimitedConnListener{
			Listener:       ln,
			RuntimeFDLimit: fdNum + 100,
		},
		Config: &http.Server{},
	}
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	defer resp.Body.Close()
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
}

func TestLimitedConnListenerLimit(t *testing.T) {
	if _, err := runtime.FDUsage(); err != nil {
		t.Skip("skip test due to unsupported runtime.FDUsage")
	}

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &httptest.Server{
		Listener: &LimitedConnListener{
			Listener:       ln,
			RuntimeFDLimit: 0,
		},
		Config: &http.Server{},
	}
	srv.Start()
	defer srv.Close()

	_, err = http.Get(srv.URL)
	if err == nil {
		t.Fatalf("unexpected nil Get error")
	}
}
