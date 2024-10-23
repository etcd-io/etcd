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
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadWriteTimeoutDialer(t *testing.T) {
	stop := make(chan struct{})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoErrorf(t, err, "unexpected listen error")
	defer func() {
		stop <- struct{}{}
	}()
	ts := testBlockingServer{ln, 2, stop}
	go ts.Start(t)

	d := rwTimeoutDialer{
		wtimeoutd:  10 * time.Millisecond,
		rdtimeoutd: 10 * time.Millisecond,
	}
	conn, err := d.Dial("tcp", ln.Addr().String())
	require.NoErrorf(t, err, "unexpected dial error")
	defer conn.Close()

	// fill the socket buffer
	data := make([]byte, 5*1024*1024)
	done := make(chan struct{}, 1)
	go func() {
		_, err = conn.Write(data)
		done <- struct{}{}
	}()

	select {
	case <-done:
	// Wait 5s more than timeout to avoid delay in low-end systems;
	// the slack was 1s extra, but that wasn't enough for CI.
	case <-time.After(d.wtimeoutd*10 + 5*time.Second):
		t.Fatal("wait timeout")
	}

	var operr *net.OpError
	if !errors.As(err, &operr) || operr.Op != "write" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}

	conn, err = d.Dial("tcp", ln.Addr().String())
	require.NoErrorf(t, err, "unexpected dial error")
	defer conn.Close()

	buf := make([]byte, 10)
	go func() {
		_, err = conn.Read(buf)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(d.rdtimeoutd * 10):
		t.Fatal("wait timeout")
	}

	if !errors.As(err, &operr) || operr.Op != "read" || !operr.Timeout() {
		t.Errorf("err = %v, want read i/o timeout error", err)
	}
}

type testBlockingServer struct {
	ln   net.Listener
	n    int
	stop chan struct{}
}

func (ts *testBlockingServer) Start(t *testing.T) {
	t.Helper()
	for i := 0; i < ts.n; i++ {
		conn, err := ts.ln.Accept()
		if err != nil {
			t.Error(err)
		}
		defer conn.Close()
	}
	<-ts.stop
}
