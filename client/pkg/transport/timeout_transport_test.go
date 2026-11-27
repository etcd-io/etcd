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
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
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
	conn, err := tr.DialContext(context.Background(), "tcp", srv.Listener.Addr().String())
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

func TestConnectionsDialedUseTLSInfoLocalAddrAsSourceAddress(t *testing.T) {
	const localAddr = "127.0.0.2"
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	d := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(localAddr)}}
	conn, err := d.Dial("tcp", probe.Addr().String())
	probe.Close()
	if err != nil {
		t.Skipf("%s is not assignable on this host (Linux assigns all of 127.0.0.0/8 to loopback): %v", localAddr, err)
	}
	conn.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.RemoteAddr))
	}))
	defer srv.Close()

	tr, err := NewTimeoutTransport(TLSInfo{LocalAddr: localAddr}, time.Second, time.Hour, time.Hour)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	remote, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)

	host, _, err := net.SplitHostPort(string(remote))
	require.NoError(t, err)
	require.Equal(t, localAddr, host)
}

// TestNewTimeoutTransportReadTimeout verifies that an established HTTP stream
// dialed via NewTimeoutTransport times out on reads when the peer stops sending
// while keeping the connection open (half-open connection, #22638).
func TestNewTimeoutTransportReadTimeout(t *testing.T) {
	for _, scheme := range []string{"http", "unix"} {
		t.Run(scheme, func(t *testing.T) {
			srvURL := startHalfOpenServer(t, scheme, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
				w.(http.Flusher).Flush()
			})

			tr, err := NewTimeoutTransport(TLSInfo{}, 5*time.Second, 200*time.Millisecond, 0)
			require.NoError(t, err)
			t.Cleanup(tr.CloseIdleConnections)

			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, nil)
			require.NoError(t, err)
			resp, err := tr.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			var initial [2]byte
			_, err = io.ReadFull(resp.Body, initial[:])
			require.NoError(t, err)
			require.Equal(t, "ok", string(initial[:]))

			_, err = io.ReadAll(resp.Body)
			require.NoErrorf(t, ctx.Err(), "expected the 200ms read timeout to fire; instead the read blocked until the test context deadline")
			var opErr *net.OpError
			require.ErrorAs(t, err, &opErr)
			require.Equal(t, "read", opErr.Op)
			require.True(t, opErr.Timeout())
		})
	}
}

// TestNewTimeoutTransportWriteTimeout verifies that an established HTTP stream
// dialed via NewTimeoutTransport times out on writes when the peer stops
// reading while keeping the connection open (half-open connection, #22638).
func TestNewTimeoutTransportWriteTimeout(t *testing.T) {
	for _, scheme := range []string{"http", "unix"} {
		t.Run(scheme, func(t *testing.T) {
			srvURL := startHalfOpenServer(t, scheme, func(_ http.ResponseWriter, r *http.Request) {
				// Accept the request, then stop reading so that the client's
				// writes eventually block on full socket buffers.
				var first [1]byte
				_, _ = io.ReadFull(r.Body, first[:])
			})

			tr, err := NewTimeoutTransport(TLSInfo{}, 5*time.Second, 0, 200*time.Millisecond)
			require.NoError(t, err)
			t.Cleanup(tr.CloseIdleConnections)

			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, srvURL, endlessReader{ctx: ctx})
			require.NoError(t, err)

			resp, err := tr.RoundTrip(req)
			if resp != nil {
				resp.Body.Close()
			}
			require.NoErrorf(t, ctx.Err(), "expected the 200ms write timeout to fire; instead the write blocked until the test context deadline")
			var opErr *net.OpError
			require.ErrorAs(t, err, &opErr)
			require.Equal(t, "write", opErr.Op)
			require.True(t, opErr.Timeout())
		})
	}
}

type endlessReader struct {
	ctx context.Context
}

func (r endlessReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return len(p), nil
}

// startHalfOpenServer starts an HTTP server whose handler runs setup and then
// holds the connection open without reading or writing further bytes (no FIN or
// RST) until test cleanup.
func startHalfOpenServer(t *testing.T, scheme string, setup http.HandlerFunc) string {
	t.Helper()
	addr := "127.0.0.1:0"
	if scheme == "unix" {
		if runtime.GOOS == "windows" {
			t.Skip("unix:// scheme with host:port socket name is not supported on Windows")
		}
		t.Chdir(t.TempDir())
		addr = "localhost:0"
	}
	ln, err := NewListener(addr, scheme, nil)
	require.NoError(t, err)
	srvURL := scheme + "://" + ln.Addr().String()

	unblock := make(chan struct{})
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setup(w, r)
			<-unblock
		}),
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
		close(unblock)
		<-done
	})
	return srvURL
}
