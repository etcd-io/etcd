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

package resolver

import (
	"sync"
	"testing"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// recordingClientConn is a test implementation of resolver.ClientConn that
// records every UpdateState call so tests can assert on the number and
// contents of the updates that gRPC would have observed.
type recordingClientConn struct {
	resolver.ClientConn

	mu     sync.Mutex
	states []resolver.State
}

func (c *recordingClientConn) UpdateState(s resolver.State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states = append(c.states, s)
	return nil
}

func (c *recordingClientConn) ReportError(error) {}

func (c *recordingClientConn) NewAddress([]resolver.Address) {}

func (c *recordingClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	// Return a non-nil ParseResult with no error. The resolver only checks
	// Err and passes the ParseResult pointer through; tests can observe its
	// presence on the forwarded resolver.State.
	return &serviceconfig.ParseResult{}
}

func (c *recordingClientConn) snapshot() []resolver.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]resolver.State, len(c.states))
	copy(out, c.states)
	return out
}

// TestBuildSendsSingleUpdateWithServiceConfig is a regression test for
// https://github.com/etcd-io/etcd/issues/21660. Build must emit exactly one
// resolver update, and that update must already carry the round_robin
// ServiceConfig alongside the configured endpoints. Before the fix, Build
// ran the embedded manual.Resolver.Build first and then pushed the endpoints
// and ServiceConfig via a follow-up UpdateState call, which made gRPC
// switch balancers mid-connection.
func TestBuildSendsSingleUpdateWithServiceConfig(t *testing.T) {
	r := New("127.0.0.1:2379", "127.0.0.2:2379")
	cc := &recordingClientConn{}

	if _, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{}); err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}

	states := cc.snapshot()
	if len(states) != 1 {
		t.Fatalf("expected exactly 1 UpdateState call, got %d", len(states))
	}

	got := states[0]
	if got.ServiceConfig == nil {
		t.Fatalf("expected ServiceConfig to be set on the initial update")
	}
	if got.ServiceConfig.Err != nil {
		t.Fatalf("unexpected ServiceConfig parse error: %v", got.ServiceConfig.Err)
	}
	if len(got.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints on the initial update, got %d", len(got.Endpoints))
	}
}

// TestSetEndpointsAfterBuild verifies SetEndpoints continues to propagate
// updates once the resolver has been built.
func TestSetEndpointsAfterBuild(t *testing.T) {
	r := New("127.0.0.1:2379")
	cc := &recordingClientConn{}

	if _, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{}); err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}

	r.SetEndpoints([]string{"127.0.0.1:2379", "127.0.0.2:2379", "127.0.0.3:2379"})

	states := cc.snapshot()
	if len(states) != 2 {
		t.Fatalf("expected 2 UpdateState calls (Build + SetEndpoints), got %d", len(states))
	}
	if n := len(states[1].Endpoints); n != 3 {
		t.Fatalf("expected 3 endpoints after SetEndpoints, got %d", n)
	}
	if states[1].ServiceConfig == nil {
		t.Fatalf("SetEndpoints update should carry the ServiceConfig")
	}
}
