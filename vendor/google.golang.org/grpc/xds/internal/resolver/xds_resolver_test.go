/*
 *
 * Copyright 2019 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package resolver

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
	xdsinternal "google.golang.org/grpc/xds/internal"
)

// This is initialized at init time.
var fbb *fakeBalancerBuilder

// We register a fake balancer builder and the actual xds_resolver here. We use
// the fake balancer builder to verify the service config pushed by the
// resolver.
func init() {
	resolver.Register(NewBuilder())
	fbb = &fakeBalancerBuilder{
		wantLBConfig: &wrappedLBConfig{lbCfg: json.RawMessage(`{
      "childPolicy":[
        {
          "round_robin": {}
        }
      ]
    }`)},
		errCh: make(chan error),
	}
	balancer.Register(fbb)
}

// testClientConn is a fake implemetation of resolver.ClientConn. All is does
// is to store the state received from the resolver locally and close the
// provided done channel.
type testClientConn struct {
	done     chan struct{}
	gotState resolver.State
}

func (t *testClientConn) UpdateState(s resolver.State) {
	t.gotState = s
	close(t.done)
}

func (*testClientConn) NewAddress(addresses []resolver.Address) { panic("unimplemented") }
func (*testClientConn) NewServiceConfig(serviceConfig string)   { panic("unimplemented") }

// TestXDSRsolverSchemeAndAddresses creates a new xds resolver, verifies that
// it returns an empty address list and the appropriate xds-experimental
// scheme.
func TestXDSRsolverSchemeAndAddresses(t *testing.T) {
	b := NewBuilder()
	wantScheme := "xds-experimental"
	if b.Scheme() != wantScheme {
		t.Fatalf("got scheme %s, want %s", b.Scheme(), wantScheme)
	}

	tcc := &testClientConn{done: make(chan struct{})}
	r, err := b.Build(resolver.Target{}, tcc, resolver.BuildOption{})
	if err != nil {
		t.Fatalf("xdsBuilder.Build() failed with error: %v", err)
	}
	defer r.Close()

	<-tcc.done
	if len(tcc.gotState.Addresses) != 0 {
		t.Fatalf("got address list from resolver %v, want empty list", tcc.gotState.Addresses)
	}
}

// fakeBalancer is used to verify that the xds_resolver returns the expected
// serice config.
type fakeBalancer struct {
	wantLBConfig *wrappedLBConfig
	errCh        chan error
}

func (*fakeBalancer) HandleSubConnStateChange(_ balancer.SubConn, _ connectivity.State) {
	panic("unimplemented")
}
func (*fakeBalancer) HandleResolvedAddrs(_ []resolver.Address, _ error) {
	panic("unimplemented")
}

// UpdateClientConnState verifies that the received LBConfig matches the
// provided one, and if not, sends an error on the provided channel.
func (f *fakeBalancer) UpdateClientConnState(ccs balancer.ClientConnState) {
	gotLBConfig, ok := ccs.BalancerConfig.(*wrappedLBConfig)
	if !ok {
		f.errCh <- fmt.Errorf("in fakeBalancer got lbConfig of type %T, want %T", ccs.BalancerConfig, &wrappedLBConfig{})
		return
	}

	var gotCfg, wantCfg xdsinternal.LBConfig
	if err := wantCfg.UnmarshalJSON(f.wantLBConfig.lbCfg); err != nil {
		f.errCh <- fmt.Errorf("unable to unmarshal balancer config %s into xds config", string(f.wantLBConfig.lbCfg))
		return
	}
	if err := gotCfg.UnmarshalJSON(gotLBConfig.lbCfg); err != nil {
		f.errCh <- fmt.Errorf("unable to unmarshal balancer config %s into xds config", string(gotLBConfig.lbCfg))
		return
	}
	if !reflect.DeepEqual(gotCfg, wantCfg) {
		f.errCh <- fmt.Errorf("in fakeBalancer got lbConfig %v, want %v", gotCfg, wantCfg)
		return
	}

	f.errCh <- nil
}

func (*fakeBalancer) UpdateSubConnState(_ balancer.SubConn, _ balancer.SubConnState) {
	panic("unimplemented")
}

func (*fakeBalancer) Close() {}

// fakeBalancerBuilder builds a fake balancer and also provides a ParseConfig
// method (which doesn't really the parse config, but just stores it as is).
type fakeBalancerBuilder struct {
	wantLBConfig *wrappedLBConfig
	errCh        chan error
}

func (f *fakeBalancerBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	return &fakeBalancer{f.wantLBConfig, f.errCh}
}

func (f *fakeBalancerBuilder) Name() string {
	return "xds_experimental"
}

func (f *fakeBalancerBuilder) ParseConfig(c json.RawMessage) (serviceconfig.LoadBalancingConfig, error) {
	return &wrappedLBConfig{lbCfg: c}, nil
}

// wrappedLBConfig simply wraps the provided LB config with a
// serviceconfig.LoadBalancingConfig interface.
type wrappedLBConfig struct {
	serviceconfig.LoadBalancingConfig
	lbCfg json.RawMessage
}

// TestXDSRsolverServiceConfig verifies that the xds_resolver returns the
// expected service config.
//
// The following sequence of events happen in this test:
// * The xds_experimental balancer (fake) and resolver builders are initialized
//   at init time.
// * We dial a dummy address here with the xds-experimental scheme. This should
//   pick the xds_resolver, which should return the hard-coded service config,
//   which should reach the fake balancer that we registered (because the
//   service config asks for the xds balancer).
// * In the fake balancer, we verify that we receive the expected LB config.
func TestXDSRsolverServiceConfig(t *testing.T) {
	xdsAddr := fmt.Sprintf("%s:///dummy", xdsScheme)
	cc, err := grpc.Dial(xdsAddr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("grpc.Dial(%s) failed with error: %v", xdsAddr, err)
	}
	defer cc.Close()

	timer := time.NewTimer(5 * time.Second)
	select {
	case <-timer.C:
		t.Fatal("timed out waiting for service config to reach balancer")
	case err := <-fbb.errCh:
		if err != nil {
			t.Error(err)
		}
	}
}
