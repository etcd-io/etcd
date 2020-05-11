// Copyright 2016 The CMux Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package cmux

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/http2"
)

var (
	benchHTTP1Payload = make([]byte, 4096)
	benchHTTP2Payload = make([]byte, 4096)
)

func init() {
	copy(benchHTTP1Payload, []byte("GET http://www.w3.org/ HTTP/1.1"))
	copy(benchHTTP2Payload, http2.ClientPreface)
}

type mockConn struct {
	net.Conn
	r io.Reader
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	return c.r.Read(b)
}

func (c *mockConn) SetReadDeadline(time.Time) error {
	return nil
}

func discard(l net.Listener) {
	for {
		if _, err := l.Accept(); err != nil {
			return
		}
	}
}

func BenchmarkCMuxConnHTTP1(b *testing.B) {
	m := New(nil).(*cMux)
	l := m.Match(HTTP1Fast())

	go discard(l)

	donec := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(1)
			m.serve(&mockConn{
				r: bytes.NewReader(benchHTTP1Payload),
			}, donec, &wg)
		}
	})
}

func BenchmarkCMuxConnHTTP2(b *testing.B) {
	m := New(nil).(*cMux)
	l := m.Match(HTTP2())
	go discard(l)

	donec := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(1)
			m.serve(&mockConn{
				r: bytes.NewReader(benchHTTP2Payload),
			}, donec, &wg)
		}
	})
}

func BenchmarkCMuxConnHTTP1n2(b *testing.B) {
	m := New(nil).(*cMux)
	l1 := m.Match(HTTP1Fast())
	l2 := m.Match(HTTP2())

	go discard(l1)
	go discard(l2)

	donec := make(chan struct{})
	var wg sync.WaitGroup

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(1)
			m.serve(&mockConn{
				r: bytes.NewReader(benchHTTP2Payload),
			}, donec, &wg)
		}
	})
}

func BenchmarkCMuxConnHTTP2n1(b *testing.B) {
	m := New(nil).(*cMux)
	l2 := m.Match(HTTP2())
	l1 := m.Match(HTTP1Fast())

	go discard(l1)
	go discard(l2)

	donec := make(chan struct{})
	var wg sync.WaitGroup

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(1)
			m.serve(&mockConn{
				r: bytes.NewReader(benchHTTP1Payload),
			}, donec, &wg)
		}
	})
}
