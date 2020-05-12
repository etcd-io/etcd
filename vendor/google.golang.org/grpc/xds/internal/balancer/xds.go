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

// Package balancer contains xds balancer implementation.
package balancer

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
	"google.golang.org/grpc/xds/internal/balancer/edsbalancer"
	"google.golang.org/grpc/xds/internal/balancer/lrs"
	xdsclient "google.golang.org/grpc/xds/internal/client"
)

const (
	defaultTimeout = 10 * time.Second
	edsName        = "experimental_eds"
)

var (
	// This field is for testing purpose.
	// TODO: if later we make startupTimeout configurable through BuildOptions(maybe?), then we can remove
	// this field and configure through BuildOptions instead.
	startupTimeout = defaultTimeout
	newEDSBalancer = func(cc balancer.ClientConn, loadStore lrs.Store) edsBalancerInterface {
		return edsbalancer.NewXDSBalancer(cc, loadStore)
	}
)

func init() {
	balancer.Register(&edsBalancerBuilder{})
}

type edsBalancerBuilder struct{}

// Build helps implement the balancer.Builder interface.
func (b *edsBalancerBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	ctx, cancel := context.WithCancel(context.Background())
	x := &edsBalancer{
		ctx:             ctx,
		cancel:          cancel,
		buildOpts:       opts,
		startupTimeout:  startupTimeout,
		connStateMgr:    &connStateMgr{},
		startup:         true,
		grpcUpdate:      make(chan interface{}),
		xdsClientUpdate: make(chan interface{}),
		timer:           createDrainedTimer(), // initialized a timer that won't fire without reset
		loadStore:       lrs.NewStore(),
	}
	x.cc = &xdsClientConn{
		updateState: x.connStateMgr.updateState,
		ClientConn:  cc,
	}
	x.client = newXDSClientWrapper(x.handleEDSUpdate, x.loseContact, x.buildOpts, x.loadStore)
	go x.run()
	return x
}

func (b *edsBalancerBuilder) Name() string {
	return edsName
}

func (b *edsBalancerBuilder) ParseConfig(c json.RawMessage) (serviceconfig.LoadBalancingConfig, error) {
	var cfg XDSConfig
	if err := json.Unmarshal(c, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal balancer config %s into xds config, error: %v", string(c), err)
	}
	return &cfg, nil
}

// edsBalancerInterface defines the interface that edsBalancer must implement to
// communicate with edsBalancer.
//
// It's implemented by the real eds balancer and a fake testing eds balancer.
type edsBalancerInterface interface {
	// HandleEDSResponse passes the received EDS message from traffic director to eds balancer.
	HandleEDSResponse(edsResp *xdsclient.EDSUpdate)
	// HandleChildPolicy updates the eds balancer the intra-cluster load balancing policy to use.
	HandleChildPolicy(name string, config json.RawMessage)
	// HandleSubConnStateChange handles state change for SubConn.
	HandleSubConnStateChange(sc balancer.SubConn, state connectivity.State)
	// Close closes the eds balancer.
	Close()
}

var _ balancer.V2Balancer = (*edsBalancer)(nil) // Assert that we implement V2Balancer

// edsBalancer manages xdsClient and the actual balancer that does load balancing (either edsBalancer,
// or fallback LB).
type edsBalancer struct {
	cc                balancer.ClientConn // *xdsClientConn
	buildOpts         balancer.BuildOptions
	startupTimeout    time.Duration
	xdsStaleTimeout   *time.Duration
	connStateMgr      *connStateMgr
	ctx               context.Context
	cancel            context.CancelFunc
	startup           bool // startup indicates whether this edsBalancer is in startup stage.
	inFallbackMonitor bool

	// edsBalancer continuously monitor the channels below, and will handle events from them in sync.
	grpcUpdate      chan interface{}
	xdsClientUpdate chan interface{}
	timer           *time.Timer
	noSubConnAlert  <-chan struct{}

	client           *xdsclientWrapper // may change when passed a different service config
	config           *XDSConfig        // may change when passed a different service config
	xdsLB            edsBalancerInterface
	fallbackLB       balancer.Balancer
	fallbackInitData *resolver.State // may change when HandleResolved address is called
	loadStore        lrs.Store
}

// run gets executed in a goroutine once edsBalancer is created. It monitors updates from grpc,
// xdsClient and load balancer. It synchronizes the operations that happen inside edsBalancer. It
// exits when edsBalancer is closed.
func (x *edsBalancer) run() {
	for {
		select {
		case update := <-x.grpcUpdate:
			x.handleGRPCUpdate(update)
		case update := <-x.xdsClientUpdate:
			x.handleXDSClientUpdate(update)
		case <-x.timer.C: // x.timer.C will block if we are not in fallback monitoring stage.
			x.switchFallback()
		case <-x.noSubConnAlert: // x.noSubConnAlert will block if we are not in fallback monitoring stage.
			x.switchFallback()
		case <-x.ctx.Done():
			if x.client != nil {
				x.client.close()
			}
			if x.xdsLB != nil {
				x.xdsLB.Close()
			}
			if x.fallbackLB != nil {
				x.fallbackLB.Close()
			}
			return
		}
	}
}

func (x *edsBalancer) handleGRPCUpdate(update interface{}) {
	switch u := update.(type) {
	case *subConnStateUpdate:
		if x.xdsLB != nil {
			x.xdsLB.HandleSubConnStateChange(u.sc, u.state.ConnectivityState)
		}
		if x.fallbackLB != nil {
			if lb, ok := x.fallbackLB.(balancer.V2Balancer); ok {
				lb.UpdateSubConnState(u.sc, u.state)
			} else {
				x.fallbackLB.HandleSubConnStateChange(u.sc, u.state.ConnectivityState)
			}
		}
	case *balancer.ClientConnState:
		cfg, _ := u.BalancerConfig.(*XDSConfig)
		if cfg == nil {
			// service config parsing failed. should never happen.
			return
		}

		x.client.handleUpdate(cfg, u.ResolverState.Attributes)

		if x.config == nil {
			// If this is the first config, the edsBalancer is in startup stage.
			// We need to apply the startup timeout for the first xdsClient, in
			// case it doesn't get a response from the traffic director in time.
			if x.startup {
				x.startFallbackMonitoring()
			}
			x.config = cfg
			x.fallbackInitData = &resolver.State{
				Addresses: u.ResolverState.Addresses,
				// TODO(yuxuanli): get the fallback balancer config once the validation change completes, where
				// we can pass along the config struct.
			}
			return
		}

		// We will update the xdsLB with the new child policy, if we got a
		// different one.
		if x.xdsLB != nil && !reflect.DeepEqual(cfg.ChildPolicy, x.config.ChildPolicy) {
			if cfg.ChildPolicy != nil {
				x.xdsLB.HandleChildPolicy(cfg.ChildPolicy.Name, cfg.ChildPolicy.Config)
			} else {
				x.xdsLB.HandleChildPolicy(roundrobin.Name, nil)
			}
		}

		var fallbackChanged bool
		if x.fallbackLB != nil && !reflect.DeepEqual(cfg.FallBackPolicy, x.config.FallBackPolicy) {
			x.fallbackLB.Close()
			x.buildFallBackBalancer(cfg)
			fallbackChanged = true
		}

		if x.fallbackLB != nil && (!reflect.DeepEqual(x.fallbackInitData.Addresses, u.ResolverState.Addresses) || fallbackChanged) {
			x.updateFallbackWithResolverState(&resolver.State{
				Addresses: u.ResolverState.Addresses,
			})
		}

		x.config = cfg
		x.fallbackInitData = &resolver.State{
			Addresses: u.ResolverState.Addresses,
			// TODO(yuxuanli): get the fallback balancer config once the validation change completes, where
			// we can pass along the config struct.
		}
	default:
		// unreachable path
		panic("wrong update type")
	}
}

func (x *edsBalancer) handleXDSClientUpdate(update interface{}) {
	switch u := update.(type) {
	// TODO: this func should accept (*xdsclient.EDSUpdate, error), and process
	// the error, instead of having a separate loseContact signal.
	case *xdsclient.EDSUpdate:
		x.cancelFallbackAndSwitchEDSBalancerIfNecessary()
		x.xdsLB.HandleEDSResponse(u)
	case *loseContact:
		// if we are already doing fallback monitoring, then we ignore new loseContact signal.
		if x.inFallbackMonitor {
			return
		}
		x.inFallbackMonitor = true
		x.startFallbackMonitoring()
	default:
		panic("unexpected xds client update type")
	}
}

type connStateMgr struct {
	mu       sync.Mutex
	curState connectivity.State
	notify   chan struct{}
}

func (c *connStateMgr) updateState(s connectivity.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.curState = s
	if s != connectivity.Ready && c.notify != nil {
		close(c.notify)
		c.notify = nil
	}
}

func (c *connStateMgr) notifyWhenNotReady() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.curState != connectivity.Ready {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	c.notify = make(chan struct{})
	return c.notify
}

// xdsClientConn wraps around the balancer.ClientConn passed in from grpc. The wrapping is to add
// functionality to get notification when no subconn is in READY state.
// TODO: once we have the change that keeps both edsbalancer and fallback balancer alive at the same
// time, we need to make sure to synchronize updates from both entities on the ClientConn.
type xdsClientConn struct {
	updateState func(s connectivity.State)
	balancer.ClientConn
}

func (w *xdsClientConn) UpdateState(s balancer.State) {
	w.updateState(s.ConnectivityState)
	w.ClientConn.UpdateState(s)
}
func (w *xdsClientConn) UpdateBalancerState(s connectivity.State, p balancer.Picker) {
	grpclog.Fatalln("not implemented")
}

type subConnStateUpdate struct {
	sc    balancer.SubConn
	state balancer.SubConnState
}

func (x *edsBalancer) HandleSubConnStateChange(sc balancer.SubConn, state connectivity.State) {
	grpclog.Error("UpdateSubConnState should be called instead of HandleSubConnStateChange")
}

func (x *edsBalancer) HandleResolvedAddrs(addrs []resolver.Address, err error) {
	grpclog.Error("UpdateResolverState should be called instead of HandleResolvedAddrs")
}

func (x *edsBalancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	update := &subConnStateUpdate{
		sc:    sc,
		state: state,
	}
	select {
	case x.grpcUpdate <- update:
	case <-x.ctx.Done():
	}
}

func (x *edsBalancer) ResolverError(error) {
	// TODO: Need to distinguish between connection errors and resource removed
	// errors. For the former, we will need to handle it later on for fallback.
	// For the latter, handle it by stopping the watch, closing sub-balancers
	// and pickers.
}

func (x *edsBalancer) UpdateClientConnState(s balancer.ClientConnState) error {
	select {
	case x.grpcUpdate <- &s:
	case <-x.ctx.Done():
	}
	return nil
}

func (x *edsBalancer) handleEDSUpdate(resp *xdsclient.EDSUpdate) error {
	// TODO: this function should take (resp, error), and send them together on
	// the channel. There doesn't need to be a separate `loseContact` function.
	select {
	case x.xdsClientUpdate <- resp:
	case <-x.ctx.Done():
	}

	return nil
}

type loseContact struct {
}

// TODO: delete loseContact when handleEDSUpdate takes (resp, error).
func (x *edsBalancer) loseContact() {
	select {
	case x.xdsClientUpdate <- &loseContact{}:
	case <-x.ctx.Done():
	}
}

func (x *edsBalancer) switchFallback() {
	if x.xdsLB != nil {
		x.xdsLB.Close()
		x.xdsLB = nil
	}
	x.buildFallBackBalancer(x.config)
	x.updateFallbackWithResolverState(x.fallbackInitData)
	x.cancelFallbackMonitoring()
}

func (x *edsBalancer) updateFallbackWithResolverState(s *resolver.State) {
	if lb, ok := x.fallbackLB.(balancer.V2Balancer); ok {
		lb.UpdateClientConnState(balancer.ClientConnState{ResolverState: resolver.State{
			Addresses: s.Addresses,
			// TODO(yuxuanli): get the fallback balancer config once the validation change completes, where
			// we can pass along the config struct.
		}})
	} else {
		x.fallbackLB.HandleResolvedAddrs(s.Addresses, nil)
	}
}

// x.cancelFallbackAndSwitchEDSBalancerIfNecessary() will be no-op if we have a working xds client.
// It will cancel fallback monitoring if we are in fallback monitoring stage.
// If there's no running edsBalancer currently, it will create one and initialize it. Also, it will
// shutdown the fallback balancer if there's one running.
func (x *edsBalancer) cancelFallbackAndSwitchEDSBalancerIfNecessary() {
	// xDS update will cancel fallback monitoring if we are in fallback monitoring stage.
	x.cancelFallbackMonitoring()

	// xDS update will switch balancer back to edsBalancer if we are in fallback.
	if x.xdsLB == nil {
		if x.fallbackLB != nil {
			x.fallbackLB.Close()
			x.fallbackLB = nil
		}
		x.xdsLB = newEDSBalancer(x.cc, x.loadStore)
		if x.config.ChildPolicy != nil {
			x.xdsLB.HandleChildPolicy(x.config.ChildPolicy.Name, x.config.ChildPolicy.Config)
		}
	}
}

func (x *edsBalancer) buildFallBackBalancer(c *XDSConfig) {
	if c.FallBackPolicy == nil {
		x.buildFallBackBalancer(&XDSConfig{
			FallBackPolicy: &loadBalancingConfig{
				Name: "round_robin",
			},
		})
		return
	}
	// builder will always be non-nil, since when parse JSON into xdsinternal.XDSConfig, we check whether the specified
	// balancer is registered or not.
	builder := balancer.Get(c.FallBackPolicy.Name)

	x.fallbackLB = builder.Build(x.cc, x.buildOpts)
}

// There are three ways that could lead to fallback:
// 1. During startup (i.e. the first xds client is just created and attempts to contact the traffic
//    director), fallback if it has not received any response from the director within the configured
//    timeout.
// 2. After xds client loses contact with the remote, fallback if all connections to the backends are
//    lost (i.e. not in state READY).
func (x *edsBalancer) startFallbackMonitoring() {
	if x.startup {
		x.startup = false
		x.timer.Reset(x.startupTimeout)
		return
	}

	x.noSubConnAlert = x.connStateMgr.notifyWhenNotReady()
	if x.xdsStaleTimeout != nil {
		if !x.timer.Stop() {
			<-x.timer.C
		}
		x.timer.Reset(*x.xdsStaleTimeout)
	}
}

// There are two cases where fallback monitoring should be canceled:
// 1. xDS client returns a new ADS message.
// 2. fallback has been triggered.
func (x *edsBalancer) cancelFallbackMonitoring() {
	if !x.timer.Stop() {
		select {
		case <-x.timer.C:
			// For cases where some fallback condition happens along with the timeout, but timeout loses
			// the race, so we need to drain the x.timer.C. thus we don't trigger fallback again.
		default:
			// if the timer timeout leads us here, then there's no thing to drain from x.timer.C.
		}
	}
	x.noSubConnAlert = nil
	x.inFallbackMonitor = false
}

func (x *edsBalancer) Close() {
	x.cancel()
}

func createDrainedTimer() *time.Timer {
	timer := time.NewTimer(0 * time.Millisecond)
	// make sure initially the timer channel is blocking until reset.
	if !timer.Stop() {
		<-timer.C
	}
	return timer
}
