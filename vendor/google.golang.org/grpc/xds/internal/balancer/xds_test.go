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

package balancer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	corepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/golang/protobuf/jsonpb"
	wrapperspb "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/internal/grpctest"
	"google.golang.org/grpc/internal/leakcheck"
	scpb "google.golang.org/grpc/internal/proto/grpc_service_config"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
	"google.golang.org/grpc/xds/internal/balancer/lrs"
	xdsclient "google.golang.org/grpc/xds/internal/client"
	"google.golang.org/grpc/xds/internal/client/bootstrap"
	"google.golang.org/grpc/xds/internal/testutils"
	"google.golang.org/grpc/xds/internal/testutils/fakexds"
)

var lbABuilder = &balancerABuilder{}

func init() {
	balancer.Register(&edsBalancerBuilder{})
	balancer.Register(lbABuilder)
	balancer.Register(&balancerBBuilder{})

	bootstrapConfigNew = func() *bootstrap.Config {
		return &bootstrap.Config{
			BalancerName: "",
			Creds:        grpc.WithInsecure(),
			NodeProto:    &corepb.Node{},
		}
	}
}

type s struct{}

func (s) Teardown(t *testing.T) {
	leakcheck.Check(t)
}

func Test(t *testing.T) {
	grpctest.RunSubTests(t, s{})
}

const (
	fakeBalancerA = "fake_balancer_A"
	fakeBalancerB = "fake_balancer_B"
)

var (
	testBalancerNameFooBar = "foo.bar"
	testLBConfigFooBar     = &XDSConfig{
		BalancerName:   testBalancerNameFooBar,
		ChildPolicy:    &loadBalancingConfig{Name: fakeBalancerB},
		FallBackPolicy: &loadBalancingConfig{Name: fakeBalancerA},
		EDSServiceName: testEDSClusterName,
	}

	specialAddrForBalancerA = resolver.Address{Addr: "this.is.balancer.A"}
	specialAddrForBalancerB = resolver.Address{Addr: "this.is.balancer.B"}
)

type balancerABuilder struct {
	mu           sync.Mutex
	lastBalancer *balancerA
}

func (b *balancerABuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	b.mu.Lock()
	b.lastBalancer = &balancerA{cc: cc, subconnStateChange: testutils.NewChannelWithSize(10)}
	b.mu.Unlock()
	return b.lastBalancer
}

func (b *balancerABuilder) Name() string {
	return string(fakeBalancerA)
}

func (b *balancerABuilder) getLastBalancer() *balancerA {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastBalancer
}

func (b *balancerABuilder) clearLastBalancer() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastBalancer = nil
}

type balancerBBuilder struct{}

func (b *balancerBBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	return &balancerB{cc: cc}
}

func (*balancerBBuilder) Name() string {
	return string(fakeBalancerB)
}

// A fake balancer implementation which does two things:
// * Appends a unique address to the list of resolved addresses received before
//   attempting to create a SubConn.
// * Makes the received subConn state changes available through a channel, for
//   the test to inspect.
type balancerA struct {
	cc                 balancer.ClientConn
	subconnStateChange *testutils.Channel
}

func (b *balancerA) HandleSubConnStateChange(sc balancer.SubConn, state connectivity.State) {
	b.subconnStateChange.Send(&scStateChange{sc: sc, state: state})
}

func (b *balancerA) HandleResolvedAddrs(addrs []resolver.Address, err error) {
	_, _ = b.cc.NewSubConn(append(addrs, specialAddrForBalancerA), balancer.NewSubConnOptions{})
}

func (b *balancerA) Close() {}

func (b *balancerA) waitForSubConnStateChange(wantState *scStateChange) error {
	return waitForSubConnStateChange(b.subconnStateChange, wantState)
}

// A fake balancer implementation which appends a unique address to the list of
// resolved addresses received before attempting to create a SubConn.
type balancerB struct {
	cc balancer.ClientConn
}

func (b *balancerB) HandleResolvedAddrs(addrs []resolver.Address, err error) {
	_, _ = b.cc.NewSubConn(append(addrs, specialAddrForBalancerB), balancer.NewSubConnOptions{})
}

func (balancerB) HandleSubConnStateChange(sc balancer.SubConn, state connectivity.State) {
	panic("implement me")
}
func (balancerB) Close() {}

func newTestClientConn() *testClientConn {
	return &testClientConn{newSubConns: testutils.NewChannelWithSize(10)}
}

type testClientConn struct {
	newSubConns *testutils.Channel
}

func (t *testClientConn) NewSubConn(addrs []resolver.Address, opts balancer.NewSubConnOptions) (balancer.SubConn, error) {
	t.newSubConns.Send(addrs)
	return nil, nil
}

func (t *testClientConn) waitForNewSubConns(wantAddrs []resolver.Address) error {
	val, err := t.newSubConns.Receive()
	if err != nil {
		return fmt.Errorf("error waiting for subconns: %v", err)
	}
	gotAddrs := val.([]resolver.Address)
	if !reflect.DeepEqual(gotAddrs, wantAddrs) {
		return fmt.Errorf("got subconn address %v, want %v", gotAddrs, wantAddrs)
	}
	return nil
}

func (testClientConn) RemoveSubConn(balancer.SubConn)                          {}
func (testClientConn) UpdateBalancerState(connectivity.State, balancer.Picker) {}
func (testClientConn) UpdateState(balancer.State)                              {}
func (testClientConn) ResolveNow(resolver.ResolveNowOptions)                   {}
func (testClientConn) Target() string                                          { return testServiceName }

type scStateChange struct {
	sc    balancer.SubConn
	state connectivity.State
}

type fakeEDSBalancer struct {
	cc                 balancer.ClientConn
	childPolicy        *testutils.Channel
	subconnStateChange *testutils.Channel
	loadStore          lrs.Store
}

func (f *fakeEDSBalancer) HandleSubConnStateChange(sc balancer.SubConn, state connectivity.State) {
	f.subconnStateChange.Send(&scStateChange{sc: sc, state: state})
}

func (f *fakeEDSBalancer) HandleChildPolicy(name string, config json.RawMessage) {
	f.childPolicy.Send(&loadBalancingConfig{Name: name, Config: config})
}

func (f *fakeEDSBalancer) Close()                                         {}
func (f *fakeEDSBalancer) HandleEDSResponse(edsResp *xdsclient.EDSUpdate) {}

func (f *fakeEDSBalancer) waitForChildPolicy(wantPolicy *loadBalancingConfig) error {
	val, err := f.childPolicy.Receive()
	if err != nil {
		return fmt.Errorf("error waiting for childPolicy: %v", err)
	}
	gotPolicy := val.(*loadBalancingConfig)
	if !reflect.DeepEqual(gotPolicy, wantPolicy) {
		return fmt.Errorf("got childPolicy %v, want %v", gotPolicy, wantPolicy)
	}
	return nil
}

func (f *fakeEDSBalancer) waitForSubConnStateChange(wantState *scStateChange) error {
	return waitForSubConnStateChange(f.subconnStateChange, wantState)
}

func waitForSubConnStateChange(ch *testutils.Channel, wantState *scStateChange) error {
	val, err := ch.Receive()
	if err != nil {
		return fmt.Errorf("error waiting for subconnStateChange: %v", err)
	}
	gotState := val.(*scStateChange)
	if !reflect.DeepEqual(gotState, wantState) {
		return fmt.Errorf("got subconnStateChange %v, want %v", gotState, wantState)
	}
	return nil
}

func newFakeEDSBalancer(cc balancer.ClientConn, loadStore lrs.Store) edsBalancerInterface {
	return &fakeEDSBalancer{
		cc:                 cc,
		childPolicy:        testutils.NewChannelWithSize(10),
		subconnStateChange: testutils.NewChannelWithSize(10),
		loadStore:          loadStore,
	}
}

type fakeSubConn struct{}

func (*fakeSubConn) UpdateAddresses([]resolver.Address) { panic("implement me") }
func (*fakeSubConn) Connect()                           { panic("implement me") }

// TestXDSFallbackResolvedAddrs verifies that the fallback balancer specified
// in the provided lbconfig is initialized, and that it receives the addresses
// pushed by the resolver.
//
// The test does the following:
// * Builds a new xds balancer.
// * Since there is no xDS server to respond to requests from the xds client
//   (created as part of the xds balancer), we expect the fallback policy to
//   kick in.
// * Repeatedly pushes new ClientConnState which specifies the same fallback
//   policy, but a different set of resolved addresses.
// * The fallback policy is implemented by a fake balancer, which appends a
//   unique address to the list of addresses it uses to create the SubConn.
// * We also have a fake ClientConn which verifies that it receives the
//   expected address list.
func (s) TestXDSFallbackResolvedAddrs(t *testing.T) {
	startupTimeout = 500 * time.Millisecond
	defer func() { startupTimeout = defaultTimeout }()

	builder := balancer.Get(edsName)
	cc := newTestClientConn()
	edsB, ok := builder.Build(cc, balancer.BuildOptions{Target: resolver.Target{Endpoint: testServiceName}}).(*edsBalancer)
	if !ok {
		t.Fatalf("builder.Build(%s) returned type {%T}, want {*edsBalancer}", edsName, edsB)
	}
	defer edsB.Close()

	tests := []struct {
		resolvedAddrs []resolver.Address
		wantAddrs     []resolver.Address
	}{
		{
			resolvedAddrs: []resolver.Address{{Addr: "1.1.1.1:10001"}, {Addr: "2.2.2.2:10002"}},
			wantAddrs:     []resolver.Address{{Addr: "1.1.1.1:10001"}, {Addr: "2.2.2.2:10002"}, specialAddrForBalancerA},
		},
		{
			resolvedAddrs: []resolver.Address{{Addr: "1.1.1.1:10001"}},
			wantAddrs:     []resolver.Address{{Addr: "1.1.1.1:10001"}, specialAddrForBalancerA},
		},
	}
	for _, test := range tests {
		edsB.UpdateClientConnState(balancer.ClientConnState{
			ResolverState:  resolver.State{Addresses: test.resolvedAddrs},
			BalancerConfig: testLBConfigFooBar,
		})
		if err := cc.waitForNewSubConns(test.wantAddrs); err != nil {
			t.Fatal(err)
		}
	}
}

// waitForNewXDSClientWithEDSWatch makes sure that a new xdsClient is created
// with the provided name. It also make sure that the newly created client
// registers an eds watcher.
func waitForNewXDSClientWithEDSWatch(t *testing.T, ch *testutils.Channel, wantName string) *fakexds.Client {
	t.Helper()

	val, err := ch.Receive()
	if err != nil {
		t.Fatalf("error when waiting for a new xds client: %v", err)
		return nil
	}
	xdsC := val.(*fakexds.Client)
	if xdsC.Name() != wantName {
		t.Fatalf("xdsClient created to balancer: %v, want %v", xdsC.Name(), wantName)
		return nil
	}
	_, err = xdsC.WaitForWatchEDS()
	if err != nil {
		t.Fatalf("xdsClient.WatchEDS failed with error: %v", err)
		return nil
	}
	return xdsC
}

// waitForNewEDSLB makes sure that a new edsLB is created by the top-level
// edsBalancer.
func waitForNewEDSLB(t *testing.T, ch *testutils.Channel) *fakeEDSBalancer {
	t.Helper()

	val, err := ch.Receive()
	if err != nil {
		t.Fatalf("error when waiting for a new edsLB: %v", err)
		return nil
	}
	return val.(*fakeEDSBalancer)
}

// setup overrides the functions which are used to create the xdsClient and the
// edsLB, creates fake version of them and makes them available on the provided
// channels. The returned cancel function should be called by the test for
// cleanup.
func setup(edsLBCh *testutils.Channel, xdsClientCh *testutils.Channel) func() {
	origNewEDSBalancer := newEDSBalancer
	newEDSBalancer = func(cc balancer.ClientConn, loadStore lrs.Store) edsBalancerInterface {
		edsLB := newFakeEDSBalancer(cc, loadStore)
		defer func() { edsLBCh.Send(edsLB) }()
		return edsLB
	}

	origXdsClientNew := xdsclientNew
	xdsclientNew = func(opts xdsclient.Options) (xdsClientInterface, error) {
		xdsC := fakexds.NewClientWithName(opts.Config.BalancerName)
		defer func() { xdsClientCh.Send(xdsC) }()
		return xdsC, nil
	}
	return func() {
		newEDSBalancer = origNewEDSBalancer
		xdsclientNew = origXdsClientNew
	}
}

// setupForFallback performs everything that setup does and in addition
// overrides the fallback startupTimeout to a small value to trigger fallback
// in tests.
func setupForFallback(edsLBCh *testutils.Channel, xdsClientCh *testutils.Channel) func() {
	cancel := setup(edsLBCh, xdsClientCh)
	startupTimeout = 500 * time.Millisecond
	return func() {
		cancel()
		startupTimeout = defaultTimeout
	}
}

// TestXDSConfigBalancerNameUpdate verifies different scenarios where the
// balancer name in the lbConfig is updated.
//
// The test does the following:
// * Builds a new xds balancer.
// * Since there is no xDS server to respond to requests from the xds client
//   (created as part of the xds balancer), we expect the fallback policy to
//   kick in.
// * Repeatedly pushes new ClientConnState which specifies different
//   balancerName in the lbConfig. We expect xdsClient objects to created
//   whenever the balancerName changes. We also expect a new edsLB to created
//   the first time the client receives an edsUpdate.
func (s) TestXDSConfigBalancerNameUpdate(t *testing.T) {
	edsLBCh := testutils.NewChannel()
	xdsClientCh := testutils.NewChannel()
	cancel := setupForFallback(edsLBCh, xdsClientCh)
	defer cancel()

	builder := balancer.Get(edsName)
	cc := newTestClientConn()
	edsB, ok := builder.Build(cc, balancer.BuildOptions{Target: resolver.Target{Endpoint: testEDSClusterName}}).(*edsBalancer)
	if !ok {
		t.Fatalf("builder.Build(%s) returned type {%T}, want {*edsBalancer}", edsName, edsB)
	}
	defer edsB.Close()

	addrs := []resolver.Address{{Addr: "1.1.1.1:10001"}, {Addr: "2.2.2.2:10002"}, {Addr: "3.3.3.3:10003"}}
	edsB.UpdateClientConnState(balancer.ClientConnState{
		ResolverState:  resolver.State{Addresses: addrs},
		BalancerConfig: testLBConfigFooBar,
	})

	waitForNewXDSClientWithEDSWatch(t, xdsClientCh, testBalancerNameFooBar)
	// Verify that fallbackLB (fakeBalancerA) takes over, since the xdsClient
	// receives no edsUpdate.
	if err := cc.waitForNewSubConns(append(addrs, specialAddrForBalancerA)); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		balancerName := fmt.Sprintf("balancer-%d", i)
		edsB.UpdateClientConnState(balancer.ClientConnState{
			ResolverState: resolver.State{Addresses: addrs},
			BalancerConfig: &XDSConfig{
				BalancerName:   balancerName,
				ChildPolicy:    &loadBalancingConfig{Name: fakeBalancerA},
				FallBackPolicy: &loadBalancingConfig{Name: fakeBalancerA},
				EDSServiceName: testEDSClusterName,
			},
		})

		xdsC := waitForNewXDSClientWithEDSWatch(t, xdsClientCh, balancerName)
		xdsC.InvokeWatchEDSCallback(&xdsclient.EDSUpdate{}, nil)

		// In the first iteration, an edsLB takes over from the fallbackLB. In the
		// second iteration, a new xds client is created, but the same edsLB is used.
		if i == 0 {
			if _, err := edsLBCh.Receive(); err != nil {
				t.Fatalf("edsBalancer did not create edsLB after receiveing EDS update: %v, %d", err, i)
			}
		} else {
			if _, err := edsLBCh.Receive(); err == nil {
				t.Fatal("edsBalancer created new edsLB when it was not expected to")
			}
		}
	}
}

// TestXDSConnfigChildPolicyUpdate verifies scenarios where the childPolicy
// section of the lbConfig is updated.
//
// The test does the following:
// * Builds a new xds balancer.
// * Pushes a new ClientConnState with a childPolicy set to fakeBalancerA.
//   Verifies that a new xdsClient is created. It then pushes a new edsUpdate
//   through the fakexds client. Verifies that a new edsLB is created and it
//   receives the expected childPolicy.
// * Pushes a new ClientConnState with a childPolicy set to fakeBalancerB.
//   This time around, we expect no new xdsClient or edsLB to be created.
//   Instead, we expect the existing edsLB to receive the new child policy.
func (s) TestXDSConnfigChildPolicyUpdate(t *testing.T) {
	edsLBCh := testutils.NewChannel()
	xdsClientCh := testutils.NewChannel()
	cancel := setup(edsLBCh, xdsClientCh)
	defer cancel()

	builder := balancer.Get(edsName)
	cc := newTestClientConn()
	edsB, ok := builder.Build(cc, balancer.BuildOptions{Target: resolver.Target{Endpoint: testServiceName}}).(*edsBalancer)
	if !ok {
		t.Fatalf("builder.Build(%s) returned type {%T}, want {*edsBalancer}", edsName, edsB)
	}
	defer edsB.Close()

	edsB.UpdateClientConnState(balancer.ClientConnState{
		BalancerConfig: &XDSConfig{
			BalancerName: testBalancerNameFooBar,
			ChildPolicy: &loadBalancingConfig{
				Name:   fakeBalancerA,
				Config: json.RawMessage("{}"),
			},
			EDSServiceName: testEDSClusterName,
		},
	})
	xdsC := waitForNewXDSClientWithEDSWatch(t, xdsClientCh, testBalancerNameFooBar)
	xdsC.InvokeWatchEDSCallback(&xdsclient.EDSUpdate{}, nil)
	edsLB := waitForNewEDSLB(t, edsLBCh)
	edsLB.waitForChildPolicy(&loadBalancingConfig{
		Name:   string(fakeBalancerA),
		Config: json.RawMessage(`{}`),
	})

	edsB.UpdateClientConnState(balancer.ClientConnState{
		BalancerConfig: &XDSConfig{
			BalancerName: testBalancerNameFooBar,
			ChildPolicy: &loadBalancingConfig{
				Name:   fakeBalancerB,
				Config: json.RawMessage("{}"),
			},
			EDSServiceName: testEDSClusterName,
		},
	})
	edsLB.waitForChildPolicy(&loadBalancingConfig{
		Name:   string(fakeBalancerA),
		Config: json.RawMessage(`{}`),
	})
}

// TestXDSConfigFallBackUpdate verifies different scenarios where the fallback
// config part of the lbConfig is updated.
//
// The test does the following:
// * Builds a top-level edsBalancer
// * Fakes the xdsClient and the underlying edsLB implementations.
// * Sends a ClientConn update to the edsBalancer with a bogus balancerName.
//   This will get the balancer into fallback monitoring, but since the
//   startupTimeout package variable is not overridden to a small value, fallback
//   will not kick-in as yet.
// * Sends another ClientConn update with fallback addresses. Still fallback
//   would not have kicked in because the startupTimeout hasn't expired.
// * Sends an EDSUpdate through the fakexds.Client object. This will trigger
//   the creation of an edsLB object. This is verified.
// * Trigger fallback by directly calling the loseContact method on the
//   top-level edsBalancer. This should instantiate the fallbackLB and should
//   send the appropriate subConns.
// * Update the fallback policy to specify and different fallback LB and make
//   sure the new LB receives appropriate subConns.
func (s) TestXDSConfigFallBackUpdate(t *testing.T) {
	edsLBCh := testutils.NewChannel()
	xdsClientCh := testutils.NewChannel()
	cancel := setup(edsLBCh, xdsClientCh)
	defer cancel()

	builder := balancer.Get(edsName)
	cc := newTestClientConn()
	edsB, ok := builder.Build(cc, balancer.BuildOptions{Target: resolver.Target{Endpoint: testEDSClusterName}}).(*edsBalancer)
	if !ok {
		t.Fatalf("builder.Build(%s) returned type {%T}, want {*edsBalancer}", edsName, edsB)
	}
	defer edsB.Close()

	bogusBalancerName := "wrong-balancer-name"
	edsB.UpdateClientConnState(balancer.ClientConnState{
		BalancerConfig: &XDSConfig{
			BalancerName:   bogusBalancerName,
			FallBackPolicy: &loadBalancingConfig{Name: fakeBalancerA},
		},
	})
	xdsC := waitForNewXDSClientWithEDSWatch(t, xdsClientCh, bogusBalancerName)

	addrs := []resolver.Address{{Addr: "1.1.1.1:10001"}, {Addr: "2.2.2.2:10002"}, {Addr: "3.3.3.3:10003"}}
	edsB.UpdateClientConnState(balancer.ClientConnState{
		ResolverState: resolver.State{Addresses: addrs},
		BalancerConfig: &XDSConfig{
			BalancerName:   bogusBalancerName,
			FallBackPolicy: &loadBalancingConfig{Name: fakeBalancerB},
		},
	})

	xdsC.InvokeWatchEDSCallback(&xdsclient.EDSUpdate{}, nil)
	if _, err := edsLBCh.Receive(); err != nil {
		t.Fatalf("edsBalancer did not create edsLB after receiveing EDS update: %v", err)
	}

	// Call loseContact explicitly, error in EDS callback is not handled.
	// Eventually, this should call EDS ballback with an error that indicates
	// "lost contact".
	edsB.loseContact()

	// Verify that fallback (fakeBalancerB) takes over.
	if err := cc.waitForNewSubConns(append(addrs, specialAddrForBalancerB)); err != nil {
		t.Fatal(err)
	}

	edsB.UpdateClientConnState(balancer.ClientConnState{
		ResolverState: resolver.State{Addresses: addrs},
		BalancerConfig: &XDSConfig{
			BalancerName:   bogusBalancerName,
			FallBackPolicy: &loadBalancingConfig{Name: fakeBalancerA},
		},
	})

	// Verify that fallbackLB (fakeBalancerA) takes over.
	if err := cc.waitForNewSubConns(append(addrs, specialAddrForBalancerA)); err != nil {
		t.Fatal(err)
	}
}

// TestXDSSubConnStateChange verifies if the top-level edsBalancer passes on
// the subConnStateChange to appropriate child balancers (it tests for edsLB
// and a fallbackLB).
func (s) TestXDSSubConnStateChange(t *testing.T) {
	edsLBCh := testutils.NewChannel()
	xdsClientCh := testutils.NewChannel()
	cancel := setup(edsLBCh, xdsClientCh)
	defer cancel()

	builder := balancer.Get(edsName)
	cc := newTestClientConn()
	edsB, ok := builder.Build(cc, balancer.BuildOptions{Target: resolver.Target{Endpoint: testEDSClusterName}}).(*edsBalancer)
	if !ok {
		t.Fatalf("builder.Build(%s) returned type {%T}, want {*edsBalancer}", edsName, edsB)
	}
	defer edsB.Close()

	addrs := []resolver.Address{{Addr: "1.1.1.1:10001"}, {Addr: "2.2.2.2:10002"}, {Addr: "3.3.3.3:10003"}}
	edsB.UpdateClientConnState(balancer.ClientConnState{
		ResolverState: resolver.State{Addresses: addrs},
		BalancerConfig: &XDSConfig{
			BalancerName:   testBalancerNameFooBar,
			ChildPolicy:    &loadBalancingConfig{Name: fakeBalancerA},
			FallBackPolicy: &loadBalancingConfig{Name: fakeBalancerA},
			EDSServiceName: testEDSClusterName,
		},
	})

	xdsC := waitForNewXDSClientWithEDSWatch(t, xdsClientCh, testBalancerNameFooBar)
	xdsC.InvokeWatchEDSCallback(&xdsclient.EDSUpdate{}, nil)
	edsLB := waitForNewEDSLB(t, edsLBCh)

	fsc := &fakeSubConn{}
	state := connectivity.Ready
	edsB.UpdateSubConnState(fsc, balancer.SubConnState{ConnectivityState: state})
	edsLB.waitForSubConnStateChange(&scStateChange{sc: fsc, state: state})

	// lbABuilder maintains a pointer to the last balancerA that it created. We
	// need to clear that to make sure a new one is created when we attempt to
	// fallback in the next line.
	lbABuilder.clearLastBalancer()
	// Call loseContact explicitly, error in EDS callback is not handled.
	// Eventually, this should call EDS ballback with an error that indicates
	// "lost contact".
	edsB.loseContact()
	// Verify that fallback (fakeBalancerA) takes over.
	if err := cc.waitForNewSubConns(append(addrs, specialAddrForBalancerA)); err != nil {
		t.Fatal(err)
	}
	fblb := lbABuilder.getLastBalancer()
	if fblb == nil {
		t.Fatal("expected fallback balancerA to be built on fallback")
	}
	edsB.UpdateSubConnState(fsc, balancer.SubConnState{ConnectivityState: state})
	fblb.waitForSubConnStateChange(&scStateChange{sc: fsc, state: state})
}

func TestXdsBalancerConfigParsing(t *testing.T) {
	const testEDSName = "eds.service"
	var testLRSName = "lrs.server"
	b := bytes.NewBuffer(nil)
	if err := (&jsonpb.Marshaler{}).Marshal(b, &scpb.XdsConfig{
		ChildPolicy: []*scpb.LoadBalancingConfig{
			{Policy: &scpb.LoadBalancingConfig_Xds{}},
			{Policy: &scpb.LoadBalancingConfig_RoundRobin{
				RoundRobin: &scpb.RoundRobinConfig{},
			}},
		},
		FallbackPolicy: []*scpb.LoadBalancingConfig{
			{Policy: &scpb.LoadBalancingConfig_Xds{}},
			{Policy: &scpb.LoadBalancingConfig_PickFirst{
				PickFirst: &scpb.PickFirstConfig{},
			}},
		},
		EdsServiceName:             testEDSName,
		LrsLoadReportingServerName: &wrapperspb.StringValue{Value: testLRSName},
	}); err != nil {
		t.Fatalf("%v", err)
	}

	tests := []struct {
		name    string
		js      json.RawMessage
		want    serviceconfig.LoadBalancingConfig
		wantErr bool
	}{
		{
			name: "jsonpb-generated",
			js:   b.Bytes(),
			want: &XDSConfig{
				ChildPolicy: &loadBalancingConfig{
					Name:   "round_robin",
					Config: json.RawMessage("{}"),
				},
				FallBackPolicy: &loadBalancingConfig{
					Name:   "pick_first",
					Config: json.RawMessage("{}"),
				},
				EDSServiceName:             testEDSName,
				LrsLoadReportingServerName: &testLRSName,
			},
			wantErr: false,
		},
		{
			// json with random balancers, and the first is not registered.
			name: "manually-generated",
			js: json.RawMessage(`
{
  "balancerName": "fake.foo.bar",
  "childPolicy": [
    {"fake_balancer_C": {}},
    {"fake_balancer_A": {}},
    {"fake_balancer_B": {}}
  ],
  "fallbackPolicy": [
    {"fake_balancer_C": {}},
    {"fake_balancer_B": {}},
    {"fake_balancer_A": {}}
  ],
  "edsServiceName": "eds.service",
  "lrsLoadReportingServerName": "lrs.server"
}`),
			want: &XDSConfig{
				BalancerName: "fake.foo.bar",
				ChildPolicy: &loadBalancingConfig{
					Name:   "fake_balancer_A",
					Config: json.RawMessage("{}"),
				},
				FallBackPolicy: &loadBalancingConfig{
					Name:   "fake_balancer_B",
					Config: json.RawMessage("{}"),
				},
				EDSServiceName:             testEDSName,
				LrsLoadReportingServerName: &testLRSName,
			},
			wantErr: false,
		},
		{
			// json with no lrs server name, LrsLoadReportingServerName should
			// be nil (not an empty string).
			name: "no-lrs-server-name",
			js: json.RawMessage(`
{
  "balancerName": "fake.foo.bar",
  "edsServiceName": "eds.service"
}`),
			want: &XDSConfig{
				BalancerName:               "fake.foo.bar",
				EDSServiceName:             testEDSName,
				LrsLoadReportingServerName: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &edsBalancerBuilder{}
			got, err := b.ParseConfig(tt.js)
			if (err != nil) != tt.wantErr {
				t.Errorf("edsBalancerBuilder.ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf(cmp.Diff(got, tt.want))
			}
		})
	}
}
func TestLoadbalancingConfigParsing(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want *XDSConfig
	}{
		{
			name: "empty",
			s:    "{}",
			want: &XDSConfig{},
		},
		{
			name: "success1",
			s:    `{"childPolicy":[{"pick_first":{}}]}`,
			want: &XDSConfig{
				ChildPolicy: &loadBalancingConfig{
					Name:   "pick_first",
					Config: json.RawMessage(`{}`),
				},
			},
		},
		{
			name: "success2",
			s:    `{"childPolicy":[{"round_robin":{}},{"pick_first":{}}]}`,
			want: &XDSConfig{
				ChildPolicy: &loadBalancingConfig{
					Name:   "round_robin",
					Config: json.RawMessage(`{}`),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg XDSConfig
			if err := json.Unmarshal([]byte(tt.s), &cfg); err != nil || !reflect.DeepEqual(&cfg, tt.want) {
				t.Errorf("test name: %s, parseFullServiceConfig() = %+v, err: %v, want %+v, <nil>", tt.name, cfg, err, tt.want)
			}
		})
	}
}

func TestEqualStringPointers(t *testing.T) {
	var (
		ta1 = "test-a"
		ta2 = "test-a"
		tb  = "test-b"
	)
	tests := []struct {
		name string
		a    *string
		b    *string
		want bool
	}{
		{"both-nil", nil, nil, true},
		{"a-non-nil", &ta1, nil, false},
		{"b-non-nil", nil, &tb, false},
		{"equal", &ta1, &ta2, true},
		{"different", &ta1, &tb, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := equalStringPointers(tt.a, tt.b); got != tt.want {
				t.Errorf("equalStringPointers() = %v, want %v", got, tt.want)
			}
		})
	}
}
