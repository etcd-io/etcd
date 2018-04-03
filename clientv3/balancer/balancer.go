// Copyright 2018 The etcd Authors
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

package balancer

import (
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/etcd/clientv3/balancer/picker"

	"go.uber.org/zap"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
	_ "google.golang.org/grpc/resolver/dns"         // register DNS resolver
	_ "google.golang.org/grpc/resolver/passthrough" // register passthrough resolver
)

// Balancer defines client balancer interface.
type Balancer interface {
	// Builder is called at the beginning to initialize sub-connection states and picker.
	balancer.Builder

	// Balancer is called on specified client connection. Client initiates gRPC
	// connection with "grpc.Dial(addr, grpc.WithBalancerName)", and then those resolved
	// addresses are passed to "grpc/balancer.Balancer.HandleResolvedAddrs".
	// For each resolved address, balancer calls "balancer.ClientConn.NewSubConn".
	// "grpc/balancer.Balancer.HandleSubConnStateChange" is called when connectivity state
	// changes, thus requires failover logic in this method.
	balancer.Balancer

	// Picker calls "Pick" for every client request.
	picker.Picker

	// SetEndpoints updates client's endpoints.
	SetEndpoints(eps ...string)
}

type baseBalancer struct {
	policy picker.Policy
	name   string
	lg     *zap.Logger

	mu sync.RWMutex

	eps []string

	addrToSc map[resolver.Address]balancer.SubConn
	scToAddr map[balancer.SubConn]resolver.Address
	scToSt   map[balancer.SubConn]connectivity.State

	currrentConn balancer.ClientConn
	currentState connectivity.State
	csEvltr      *connectivityStateEvaluator

	picker.Picker
}

// New returns a new balancer from specified picker policy.
func New(cfg Config) (Balancer, error) {
	for _, ep := range cfg.Endpoints {
		if !strings.HasPrefix(ep, "etcd://") {
			return nil, fmt.Errorf("'etcd' target schema required for etcd load balancer endpoints but got '%s'", ep)
		}
	}

	bb := &baseBalancer{
		policy: cfg.Policy,
		name:   cfg.Policy.String(),
		lg:     cfg.Logger,

		eps: cfg.Endpoints,

		addrToSc: make(map[resolver.Address]balancer.SubConn),
		scToAddr: make(map[balancer.SubConn]resolver.Address),
		scToSt:   make(map[balancer.SubConn]connectivity.State),

		currrentConn: nil,
		csEvltr:      &connectivityStateEvaluator{},

		// initialize picker always returns "ErrNoSubConnAvailable"
		Picker: picker.NewErr(balancer.ErrNoSubConnAvailable),
	}
	if cfg.Name != "" {
		bb.name = cfg.Name
	}
	if bb.lg == nil {
		bb.lg = zap.NewNop()
	}

	balancer.Register(bb)
	bb.lg.Info(
		"registered balancer",
		zap.String("policy", bb.policy.String()),
		zap.String("name", bb.name),
	)
	return bb, nil
}

// Name implements "grpc/balancer.Builder" interface.
func (bb *baseBalancer) Name() string { return bb.name }

// Build implements "grpc/balancer.Builder" interface.
// Build is called initially when creating "ccBalancerWrapper".
// "grpc.Dial" is called to this client connection.
// Then, resolved addreses will be handled via "HandleResolvedAddrs".
func (bb *baseBalancer) Build(cc balancer.ClientConn, opt balancer.BuildOptions) balancer.Balancer {
	// TODO: support multiple connections
	bb.mu.Lock()
	bb.currrentConn = cc
	bb.mu.Unlock()

	bb.lg.Info(
		"built balancer",
		zap.String("policy", bb.policy.String()),
		zap.String("resolver-target", cc.Target()),
	)
	return bb
}

// HandleResolvedAddrs implements "grpc/balancer.Balancer" interface.
// gRPC sends initial or updated resolved addresses from "Build".
func (bb *baseBalancer) HandleResolvedAddrs(addrs []resolver.Address, err error) {
	if err != nil {
		bb.lg.Warn("HandleResolvedAddrs called with error", zap.Error(err))
		return
	}
	bb.lg.Info("resolved", zap.Strings("addresses", addrsToStrings(addrs)))

	bb.mu.Lock()
	defer bb.mu.Unlock()

	resolved := make(map[resolver.Address]struct{})
	for _, addr := range addrs {
		resolved[addr] = struct{}{}
		if _, ok := bb.addrToSc[addr]; !ok {
			sc, err := bb.currrentConn.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{})
			if err != nil {
				bb.lg.Warn("NewSubConn failed", zap.Error(err), zap.String("address", addr.Addr))
				continue
			}
			bb.addrToSc[addr] = sc
			bb.scToAddr[sc] = addr
			bb.scToSt[sc] = connectivity.Idle
			sc.Connect()
		}
	}

	for addr, sc := range bb.addrToSc {
		if _, ok := resolved[addr]; !ok {
			// was removed by resolver or failed to create subconn
			bb.currrentConn.RemoveSubConn(sc)
			delete(bb.addrToSc, addr)

			bb.lg.Info(
				"removed subconn",
				zap.String("address", addr.Addr),
				zap.String("subconn", scToString(sc)),
			)

			// Keep the state of this sc in bb.scToSt until sc's state becomes Shutdown.
			// The entry will be deleted in HandleSubConnStateChange.
			// (DO NOT) delete(bb.scToAddr, sc)
			// (DO NOT) delete(bb.scToSt, sc)
		}
	}
}

// HandleSubConnStateChange implements "grpc/balancer.Balancer" interface.
func (bb *baseBalancer) HandleSubConnStateChange(sc balancer.SubConn, s connectivity.State) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	old, ok := bb.scToSt[sc]
	if !ok {
		bb.lg.Warn(
			"state change for an unknown subconn",
			zap.String("subconn", scToString(sc)),
			zap.String("state", s.String()),
		)
		return
	}

	bb.lg.Info(
		"state changed",
		zap.Bool("connected", s == connectivity.Ready),
		zap.String("subconn", scToString(sc)),
		zap.String("address", bb.scToAddr[sc].Addr),
		zap.String("old-state", old.String()),
		zap.String("new-state", s.String()),
	)

	bb.scToSt[sc] = s
	switch s {
	case connectivity.Idle:
		sc.Connect()
	case connectivity.Shutdown:
		// When an address was removed by resolver, b called RemoveSubConn but
		// kept the sc's state in scToSt. Remove state for this sc here.
		delete(bb.scToAddr, sc)
		delete(bb.scToSt, sc)
	}

	oldAggrState := bb.currentState
	bb.currentState = bb.csEvltr.recordTransition(old, s)

	// Regenerate picker when one of the following happens:
	//  - this sc became ready from not-ready
	//  - this sc became not-ready from ready
	//  - the aggregated state of balancer became TransientFailure from non-TransientFailure
	//  - the aggregated state of balancer became non-TransientFailure from TransientFailure
	if (s == connectivity.Ready) != (old == connectivity.Ready) ||
		(bb.currentState == connectivity.TransientFailure) != (oldAggrState == connectivity.TransientFailure) {
		bb.regeneratePicker()
	}

	bb.currrentConn.UpdateBalancerState(bb.currentState, bb.Picker)
	return
}

func (bb *baseBalancer) regeneratePicker() {
	if bb.currentState == connectivity.TransientFailure {
		bb.Picker = picker.NewErr(balancer.ErrTransientFailure)
		return
	}

	// only pass ready subconns to picker
	scs := make([]balancer.SubConn, 0)
	addrToSc := make(map[resolver.Address]balancer.SubConn)
	scToAddr := make(map[balancer.SubConn]resolver.Address)
	for addr, sc := range bb.addrToSc {
		if st, ok := bb.scToSt[sc]; ok && st == connectivity.Ready {
			scs = append(scs, sc)
			addrToSc[addr] = sc
			scToAddr[sc] = addr
		}
	}

	switch bb.policy {
	case picker.RoundrobinBalanced:
		bb.Picker = picker.NewRoundrobinBalanced(bb.lg, scs, addrToSc, scToAddr)

	default:
		panic(fmt.Errorf("invalid balancer picker policy (%d)", bb.policy))
	}

	bb.lg.Info(
		"generated picker",
		zap.String("policy", bb.policy.String()),
		zap.Strings("subconn-ready", scsToStrings(addrToSc)),
		zap.Int("subconn-size", len(addrToSc)),
	)
}

// SetEndpoints updates client's endpoints.
// TODO: implement this
func (bb *baseBalancer) SetEndpoints(eps ...string) {
	addrs := epsToAddrs(eps...)
	bb.mu.Lock()
	bb.Picker.UpdateAddrs(addrs)
	bb.mu.Unlock()
}

// Close implements "grpc/balancer.Balancer" interface.
// Close is a nop because base balancer doesn't have internal state to clean up,
// and it doesn't need to call RemoveSubConn for the SubConns.
func (bb *baseBalancer) Close() {
	// TODO
}
