// Copyright 2016 The etcd Authors
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
	"net"
	"testing"

	"github.com/coreos/etcd/pkg/testutil"
)

func TestNewListenerStoppable(t *testing.T) {
	defer testutil.AfterTest(t)

	stopc := make(chan struct{})
	ln, err := NewListenerStoppable("127.0.0.1:0", "http", nil, stopc)
	if err != nil {
		t.Fatal(err)
	}

	connw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err = connw.Close(); err != nil {
		t.Fatal(err)
	}

	connl, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	bts := make([]byte, 5)
	if _, err = connl.Read(bts); err != nil {
		t.Fatal(err)
	}
	if err = connl.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bts, []byte("hello")) {
		t.Fatalf("expected %q, got %q", "hello", string(bts))
	}
}

func TestNewListenerStoppableStop(t *testing.T) {
	defer testutil.AfterTest(t)

	stopc := make(chan struct{})
	ln, err := NewListenerStoppable("127.0.0.1:0", "http", nil, stopc)
	if err != nil {
		t.Fatal(err)
	}

	connw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = connw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	close(stopc)

	if _, err = ln.Accept(); err != ErrListenerStopped {
		t.Fatalf("expected %v, got %v", ErrListenerStopped, err)
	}
}
