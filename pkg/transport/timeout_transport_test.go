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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewTimeoutTransport tests that NewTimeoutTransport returns a transport
// that can dial out timeout connections.
func TestNewTimeoutTransport(t *testing.T) {
	tr, err := NewTimeoutTransport(TLSInfo{}, time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("unexpected NewTimeoutTransport error: %v", err)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	conn, err := tr.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer conn.Close()

	tconn, ok := conn.(*timeoutConn)
	if !ok {
		t.Fatalf("failed to dial out *timeoutConn")
	}
	if tconn.rdtimeoutd != time.Hour {
		t.Errorf("read timeout = %s, want %s", tconn.rdtimeoutd, time.Hour)
	}
	if tconn.wtimeoutd != time.Hour {
		t.Errorf("write timeout = %s, want %s", tconn.wtimeoutd, time.Hour)
	}
}
