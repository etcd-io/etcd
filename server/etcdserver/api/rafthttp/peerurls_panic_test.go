// Copyright 2026 The etcd Authors
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

package rafthttp

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"go.etcd.io/etcd/client/pkg/v3/types"
)

func TestAddRemotePanicOnPeerURLsHeader(t *testing.T) {
	observed, logs := observer.New(zap.WarnLevel)
	tr := &Transport{
		Logger:    zap.New(observed),
		ID:        types.ID(1),
		ClusterID: types.ID(0x1000),
		Raft:      &fakeRaft{},
		ErrorC:    make(chan error, 16),
	}
	if err := tr.Start(); err != nil {
		t.Fatal(err)
	}
	defer tr.Stop()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: tr.Handler()}
	go srv.Serve(ln)
	defer srv.Close()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	req := "POST /raft/snapshot HTTP/1.1\r\n" +
		"Host: victim\r\n" +
		"X-Etcd-Cluster-ID: 1000\r\n" +
		"X-Server-Version: 3.5.13\r\n" +
		"X-Server-From: 2\r\n" +
		"X-PeerURLs: ftp://evil\r\n" +
		"Content-Length: 0\r\n\r\n"
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("expected an HTTP response, got no response line (err: %v); the handler likely panicked on the malformed X-PeerURLs header", err)
	}
	if !strings.HasPrefix(line, "HTTP/1.1 400") {
		t.Fatalf("expected a 400 response after the malformed header is skipped, got %q", line)
	}
	for _, e := range logs.All() {
		if e.Message == "failed NewURLs" {
			t.Fatalf("unexpected panic log for malformed X-PeerURLs header: %v", e)
		}
	}
}

func TestAddRemoteSkipsInvalidURLs(t *testing.T) {
	observed, logs := observer.New(zap.WarnLevel)
	tr := &Transport{
		Logger: zap.New(observed),
		ID:     types.ID(1),
		Raft:   &fakeRaft{},
		ErrorC: make(chan error, 16),
	}
	if err := tr.Start(); err != nil {
		t.Fatal(err)
	}
	defer tr.Stop()

	tr.AddRemote(types.ID(2), []string{"ftp://evil"})
	if _, ok := tr.remotes[types.ID(2)]; ok {
		t.Fatalf("AddRemote added a remote with an invalid URL")
	}
	found := false
	for _, e := range logs.All() {
		if e.Message == "failed to parse peer urls, skipping remote" {
			found = true
		}
		if e.Message == "failed NewURLs" {
			t.Fatalf("AddRemote panicked on invalid URLs: %v", e)
		}
	}
	if !found {
		t.Fatalf("expected a warning log for the skipped remote, got %v", logs.All())
	}
}
