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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// fakeClientConn records every state passed to UpdateState so tests can assert
// how many resolver updates were emitted and what they carried. It also lets a
// test control the value returned by ParseServiceConfig.
//
// Assertions compare whole resolver.State values built with composite literals
// rather than reading individual grpc resolver fields (state.Endpoints, .Addr,
// ...). Field reads would be flagged by tools/check-grpc-experimental as
// experimental gRPC API usage; composite-literal keys are not.
type fakeClientConn struct {
	resolver.ClientConn
	parsed        *serviceconfig.ParseResult
	states        []resolver.State
	parseCalls    int
	lastParsedArg string
}

func (f *fakeClientConn) UpdateState(s resolver.State) error {
	f.states = append(f.states, s)
	return nil
}

func (f *fakeClientConn) ParseServiceConfig(cfg string) *serviceconfig.ParseResult {
	f.parseCalls++
	f.lastParsedArg = cfg
	return f.parsed
}

func (f *fakeClientConn) ReportError(error) {}

func newFakeClientConn() *fakeClientConn {
	return &fakeClientConn{parsed: &serviceconfig.ParseResult{}}
}

// wantState builds the resolver.State we expect the resolver to emit for the
// given addresses and service config, using composite literals only.
func wantState(sc *serviceconfig.ParseResult, addrs ...struct{ addr, serverName string }) resolver.State {
	eps := make([]resolver.Endpoint, len(addrs))
	for i, a := range addrs {
		eps[i] = resolver.Endpoint{Addresses: []resolver.Address{
			{Addr: a.addr, ServerName: a.serverName},
		}}
	}
	return resolver.State{Endpoints: eps, ServiceConfig: sc}
}

func addr(a, serverName string) struct{ addr, serverName string } {
	return struct{ addr, serverName string }{a, serverName}
}

func TestNew(t *testing.T) {
	t.Run("stores endpoints", func(t *testing.T) {
		r := New("127.0.0.1:2379", "127.0.0.1:22379")
		require.NotNil(t, r)
		require.NotNil(t, r.Resolver)
		assert.Equal(t, []string{"127.0.0.1:2379", "127.0.0.1:22379"}, r.endpoints)
		// ServiceConfig is only populated once Build parses it.
		assert.Nil(t, r.serviceConfig)
	})

	t.Run("no endpoints", func(t *testing.T) {
		r := New()
		require.NotNil(t, r)
		assert.Empty(t, r.endpoints)
	})

	t.Run("uses the etcd-endpoints scheme", func(t *testing.T) {
		r := New()
		assert.Equal(t, Schema, r.Scheme())
		assert.Equal(t, "etcd-endpoints", r.Scheme())
	})
}

// TestBuildEmitsSingleResolverUpdate is a regression test for the double
// resolver update that Build used to produce. Build must emit exactly one
// update to the ClientConn, and that update must already carry both the
// endpoints and the round_robin ServiceConfig. A second update carrying the
// ServiceConfig separately would force gRPC to switch balancers mid-connection
// and tear down an in-flight SubConn.
func TestBuildEmitsSingleResolverUpdate(t *testing.T) {
	sc := &serviceconfig.ParseResult{}
	cc := &fakeClientConn{parsed: sc}

	r := New("127.0.0.1:2379", "127.0.0.1:22379")

	res, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)

	// Build must request the round_robin load balancing policy.
	assert.Equal(t, 1, cc.parseCalls)
	assert.JSONEq(t, `{"loadBalancingPolicy": "round_robin"}`, cc.lastParsedArg)

	// Exactly one resolver update, already carrying endpoints + ServiceConfig.
	require.Len(t, cc.states, 1)
	want := wantState(sc,
		addr("127.0.0.1:2379", "127.0.0.1:2379"),
		addr("127.0.0.1:22379", "127.0.0.1:22379"),
	)
	assert.Equal(t, want, cc.states[0])

	// The resolver's ClientConn is available after a successful Build.
	assert.Same(t, cc, r.CC())
	assert.Same(t, cc, getCC(*r))
}

func TestBuildReturnsServiceConfigParseError(t *testing.T) {
	parseErr := errors.New("bad service config")
	cc := &fakeClientConn{parsed: &serviceconfig.ParseResult{Err: parseErr}}

	r := New("127.0.0.1:2379")

	res, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	require.Error(t, err)
	assert.Equal(t, parseErr, err)
	assert.Nil(t, res)

	// No resolver update should be emitted when the service config is invalid.
	assert.Empty(t, cc.states)
}

func TestBuildWithNoEndpoints(t *testing.T) {
	cc := newFakeClientConn()

	r := New()

	_, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	require.NoError(t, err)

	// A single update is still emitted, with an empty (non-nil) endpoint slice
	// and the parsed ServiceConfig.
	require.Len(t, cc.states, 1)
	assert.Equal(t, wantState(cc.parsed), cc.states[0])
}

func TestBuildInterpretsEndpointSchemes(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		wantAddr       string
		wantServerName string
	}{
		{
			name:           "plain host:port",
			endpoint:       "127.0.0.1:2379",
			wantAddr:       "127.0.0.1:2379",
			wantServerName: "127.0.0.1:2379",
		},
		{
			name:           "http scheme",
			endpoint:       "http://127.0.0.1:2379",
			wantAddr:       "127.0.0.1:2379",
			wantServerName: "127.0.0.1:2379",
		},
		{
			name:           "https scheme",
			endpoint:       "https://etcd.local:2379",
			wantAddr:       "etcd.local:2379",
			wantServerName: "etcd.local:2379",
		},
		{
			name:           "unix absolute path",
			endpoint:       "unix:///tmp/etcd.sock",
			wantAddr:       "unix:///tmp/etcd.sock",
			wantServerName: "etcd.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := newFakeClientConn()
			r := New(tt.endpoint)

			_, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{})
			require.NoError(t, err)

			require.Len(t, cc.states, 1)
			want := wantState(cc.parsed, addr(tt.wantAddr, tt.wantServerName))
			assert.Equal(t, want, cc.states[0])
		})
	}
}

// TestSetEndpointsEmitsUpdateAfterBuild verifies that endpoint changes made
// after Build still propagate to the ClientConn as additional updates that
// keep carrying the ServiceConfig.
func TestSetEndpointsEmitsUpdateAfterBuild(t *testing.T) {
	sc := &serviceconfig.ParseResult{}
	cc := &fakeClientConn{parsed: sc}

	r := New("127.0.0.1:2379")

	_, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	require.Len(t, cc.states, 1)

	r.SetEndpoints([]string{"127.0.0.1:3379", "127.0.0.1:4379"})
	require.Len(t, cc.states, 2)

	want := wantState(sc,
		addr("127.0.0.1:3379", "127.0.0.1:3379"),
		addr("127.0.0.1:4379", "127.0.0.1:4379"),
	)
	assert.Equal(t, want, cc.states[1])
}

// TestSetEndpointsBeforeBuildDoesNotEmit verifies that SetEndpoints called
// before Build stores the endpoints without touching a (not-yet-assigned)
// ClientConn, and that the subsequent Build emits a single update carrying the
// updated endpoints.
func TestSetEndpointsBeforeBuildDoesNotEmit(t *testing.T) {
	cc := newFakeClientConn()

	r := New("127.0.0.1:2379")

	// No ClientConn yet: this must not panic and must not emit an update.
	r.SetEndpoints([]string{"127.0.0.1:9379"})
	assert.Empty(t, cc.states)
	assert.Equal(t, []string{"127.0.0.1:9379"}, r.endpoints)

	_, err := r.Build(resolver.Target{}, cc, resolver.BuildOptions{})
	require.NoError(t, err)

	require.Len(t, cc.states, 1)
	assert.Equal(t, wantState(cc.parsed, addr("127.0.0.1:9379", "127.0.0.1:9379")), cc.states[0])
}

func TestGetCCBeforeBuildReturnsNil(t *testing.T) {
	r := New("127.0.0.1:2379")
	// CC() panics before Build; getCC must recover and return nil.
	assert.Nil(t, getCC(*r))
}

func TestState(t *testing.T) {
	sc := &serviceconfig.ParseResult{}
	r := New("http://127.0.0.1:2379", "unix:///tmp/etcd.sock")
	r.serviceConfig = sc

	want := wantState(sc,
		addr("127.0.0.1:2379", "127.0.0.1:2379"),
		addr("unix:///tmp/etcd.sock", "etcd.sock"),
	)
	assert.Equal(t, want, r.state())
}
