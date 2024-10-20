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

// TestNewTimeoutListener tests that NewTimeoutListener returns a
// rwTimeoutListener struct with timeouts set.
func TestNewTimeoutListener(t *testing.T) {
	l, err := NewTimeoutListener("127.0.0.1:0", "http", nil, time.Hour, time.Hour)
	require.NoErrorf(t, err, "unexpected NewTimeoutListener error")
	defer l.Close()
	tln := l.(*rwTimeoutListener)
	if tln.readTimeout != time.Hour {
		t.Errorf("read timeout = %s, want %s", tln.readTimeout, time.Hour)
	}
	if tln.writeTimeout != time.Hour {
		t.Errorf("write timeout = %s, want %s", tln.writeTimeout, time.Hour)
	}
}

func TestWriteReadTimeoutListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoErrorf(t, err, "unexpected listen error")
	wln := rwTimeoutListener{
		Listener:     ln,
		writeTimeout: 10 * time.Millisecond,
		readTimeout:  10 * time.Millisecond,
	}

	blocker := func(stopCh <-chan struct{}) {
		conn, derr := net.Dial("tcp", ln.Addr().String())
		if derr != nil {
			t.Errorf("unexpected dail error: %v", derr)
		}
		defer conn.Close()
		// block the receiver until the writer timeout
		<-stopCh
	}

	writerStopCh := make(chan struct{}, 1)
	go blocker(writerStopCh)

	conn, err := wln.Accept()
	if err != nil {
		writerStopCh <- struct{}{}
		t.Fatalf("unexpected accept error: %v", err)
	}
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
	// It waits 1s more to avoid delay in low-end system.
	case <-time.After(wln.writeTimeout*10 + time.Second):
		writerStopCh <- struct{}{}
		t.Fatal("wait timeout")
	}

	var operr *net.OpError
	if !errors.As(err, &operr) || operr.Op != "write" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}
	writerStopCh <- struct{}{}

	readerStopCh := make(chan struct{}, 1)
	go blocker(readerStopCh)

	conn, err = wln.Accept()
	if err != nil {
		readerStopCh <- struct{}{}
		t.Fatalf("unexpected accept error: %v", err)
	}
	buf := make([]byte, 10)

	go func() {
		_, err = conn.Read(buf)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(wln.readTimeout * 10):
		readerStopCh <- struct{}{}
		t.Fatal("wait timeout")
	}

	if !errors.As(err, &operr) || operr.Op != "read" || !operr.Timeout() {
		t.Errorf("err = %v, want read i/o timeout error", err)
	}
	readerStopCh <- struct{}{}
}
