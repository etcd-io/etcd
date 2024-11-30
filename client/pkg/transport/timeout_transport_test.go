// Copyright 2015 The etcd Authors
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
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNewTimeoutTransport tests that NewTimeoutTransport returns a transport
// that can dial out timeout connections.
func TestNewTimeoutTransport(t *testing.T) {
	tr, err := NewTimeoutTransport(TLSInfo{}, time.Hour, time.Hour, time.Hour)
	require.NoError(t, err)

	remoteAddr := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.RemoteAddr))
	}
	srv := httptest.NewServer(http.HandlerFunc(remoteAddr))

	defer srv.Close()
	conn, err := tr.Dial("tcp", srv.Listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	tconn, ok := conn.(*timeoutConn)
	require.Truef(t, ok, "failed to dial out *timeoutConn")
	if tconn.readTimeout != time.Hour {
		t.Errorf("read timeout = %s, want %s", tconn.readTimeout, time.Hour)
	}
	if tconn.writeTimeout != time.Hour {
		t.Errorf("write timeout = %s, want %s", tconn.writeTimeout, time.Hour)
	}

	// ensure not reuse timeout connection
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	addr0, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)

	resp, err = tr.RoundTrip(req)
	require.NoError(t, err)
	addr1, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)

	if bytes.Equal(addr0, addr1) {
		t.Errorf("addr0 = %s addr1= %s, want not equal", addr0, addr1)
	}
}
