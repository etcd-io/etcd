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

package clientv3

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const minHealthRetryDuration = 3 * time.Second
const unknownService = "unknown service grpc.health.v1.Health"

type healthCheckFunc func(ep string) (bool, error)

// ErrNoAddrAvilable is returned by Get() when the balancer does not have
// any active connection to endpoints at the time.
// This error is returned only when opts.BlockingWait is true.
var ErrNoAddrAvilable = status.Error(codes.Unavailable, "there is no address available")

type notifyMsg int

const (
	notifyReset notifyMsg = iota
	notifyNext
)

// simpleBalancer does the bare minimum to expose multiple eps
// to the grpc reconnection code path
type simpleBalancer struct {
	// addrs are the client's endpoint addresses for grpc
	addrs []grpc.Address

	// eps holds the raw endpoints from the client
	eps []string

	// notifyCh notifies grpc of the set of addresses for connecting
	notifyCh chan []grpc.Address

	// readyc closes once the first connection is up
	readyc    chan struct{}
	readyOnce sync.Once

	// healthCheck checks an endpoint's health.
	healthCheck        healthCheckFunc
	healthCheckTimeout time.Duration

	// mu protects all fields below.
	mu sync.RWMutex

	// upc closes when pinAddr transitions from empty to non-empty or the balancer closes.
	upc chan struct{}

	// downc closes when grpc calls down() on pinAddr
	downc chan struct{}

	// stopc is closed to signal updateNotifyLoop should stop.
	stopc    chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	// donec closes when all goroutines are exited
	donec chan struct{}

	// updateAddrsC notifies updateNotifyLoop to update addrs.
	updateAddrsC chan notifyMsg

	// grpc issues TLS cert checks using the string passed into dial so
	// that string must be the host. To recover the full scheme://host URL,
	// have a map from hosts to the original endpoint.
	hostPort2ep map[string]string

	// unhealthy tracks the last unhealthy time of endpoints.
	unhealthy map[string]time.Time

	// pinAddr is the currently pinned address; set to the empty string on
	// initialization and shutdown.
	pinAddr string

	closed bool
}

func newSimpleBalancer(eps []string, timeout time.Duration, hc healthCheckFunc) *simpleBalancer {
	notifyCh := make(chan []grpc.Address)
	addrs := eps2addrs(eps)
	sb := &simpleBalancer{
		addrs:        addrs,
		eps:          eps,
		notifyCh:     notifyCh,
		readyc:       make(chan struct{}),
		healthCheck:  hc,
		upc:          make(chan struct{}),
		stopc:        make(chan struct{}),
		downc:        make(chan struct{}),
		donec:        make(chan struct{}),
		updateAddrsC: make(chan notifyMsg),
		hostPort2ep:  getHostPort2ep(eps),
		unhealthy:    make(map[string]time.Time),
	}
	if timeout < minHealthRetryDuration {
		timeout = minHealthRetryDuration
	}
	sb.healthCheckTimeout = timeout

	close(sb.downc)
	go sb.updateNotifyLoop()
	sb.wg.Add(1)
	go func() {
		defer sb.wg.Done()
		sb.updateUnhealthy()
	}()
	return sb
}

func (b *simpleBalancer) Start(target string, config grpc.BalancerConfig) error { return nil }

func (b *simpleBalancer) ConnectNotify() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.upc
}

func (b *simpleBalancer) ready() <-chan struct{} { return b.readyc }

func (b *simpleBalancer) endpoint(hostPort string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.hostPort2ep[hostPort]
}

func (b *simpleBalancer) pinned() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.pinAddr
}

func (b *simpleBalancer) hostPortError(hostPort string, err error) {
	b.mu.Lock()
	if _, ok := b.hostPort2ep[hostPort]; ok {
		b.unhealthy[hostPort] = time.Now()
		if logger.V(4) {
			logger.Infof("clientv3/balancer: %q is marked unhealthy (%q)", hostPort, err.Error())
		}
	} else {
		if logger.V(4) {
			logger.Infof("clientv3/balancer: %q is stale (skip marking as unhealthy on %q)", hostPort, err.Error())
		}
	}
	b.mu.Unlock()
}

func (b *simpleBalancer) removeUnhealthy(hostPort, msg string) {
	b.mu.Lock()
	if _, ok := b.unhealthy[hostPort]; ok {
		delete(b.unhealthy, hostPort)
		if logger.V(4) {
			logger.Infof("clientv3/balancer: %q is removed from unhealthy (%q)", hostPort, msg)
		}
	} else {
		if logger.V(4) {
			logger.Infof("clientv3/balancer: %q was not in unhealthy (%q)", hostPort, msg)
		}
	}
	b.mu.Unlock()
}

func (b *simpleBalancer) updateAddrs(eps ...string) {
	np := getHostPort2ep(eps)

	b.mu.Lock()

	match := len(np) == len(b.hostPort2ep)
	for k, v := range np {
		if b.hostPort2ep[k] != v {
			match = false
			break
		}
	}
	if match {
		// same endpoints, so no need to update address
		b.mu.Unlock()
		return
	}

	b.hostPort2ep = np
	b.addrs, b.eps = eps2addrs(eps), eps
	b.unhealthy = make(map[string]time.Time)

	// updating notifyCh can trigger new connections,
	// only update addrs if all connections are down
	// or addrs does not include pinAddr.
	update := !hasAddr(b.addrs, b.pinAddr)
	b.mu.Unlock()

	if update {
		select {
		case b.updateAddrsC <- notifyNext:
		case <-b.stopc:
		}
	}
}

func (b *simpleBalancer) next() {
	b.mu.RLock()
	downc := b.downc
	b.mu.RUnlock()
	select {
	case b.updateAddrsC <- notifyNext:
	case <-b.stopc:
	}
	// wait until disconnect so new RPCs are not issued on old connection
	select {
	case <-downc:
	case <-b.stopc:
	}
}

func (b *simpleBalancer) updateNotifyLoop() {
	defer close(b.donec)

	for {
		b.mu.RLock()
		upc, downc, addr := b.upc, b.downc, b.pinAddr
		b.mu.RUnlock()
		// downc or upc should be closed
		select {
		case <-downc:
			downc = nil
		default:
		}
		select {
		case <-upc:
			upc = nil
		default:
		}
		switch {
		case downc == nil && upc == nil:
			// stale
			select {
			case <-b.stopc:
				return
			default:
			}
		case downc == nil:
			b.notifyAddrs(notifyReset)
			select {
			case <-upc:
			case msg := <-b.updateAddrsC:
				b.notifyAddrs(msg)
			case <-b.stopc:
				return
			}
		case upc == nil:
			select {
			// close connections that are not the pinned address
			case b.notifyCh <- []grpc.Address{{Addr: addr}}:
			case <-downc:
			case <-b.stopc:
				return
			}
			select {
			case <-downc:
				b.notifyAddrs(notifyReset)
			case msg := <-b.updateAddrsC:
				b.notifyAddrs(msg)
			case <-b.stopc:
				return
			}
		}
	}
}

func (b *simpleBalancer) notifyAddrs(msg notifyMsg) {
	if msg == notifyNext {
		select {
		case b.notifyCh <- []grpc.Address{}:
		case <-b.stopc:
			return
		}
	}
	b.mu.RLock()
	addrs := b.addrs
	pinAddr := b.pinAddr
	downc := b.downc
	b.mu.RUnlock()

	var waitDown bool
	if pinAddr != "" {
		waitDown = true
		for _, a := range addrs {
			if a.Addr == pinAddr {
				waitDown = false
			}
		}
	}

	select {
	case b.notifyCh <- addrs:
		if waitDown {
			select {
			case <-downc:
			case <-b.stopc:
			}
		}
	case <-b.stopc:
	}
}

func (b *simpleBalancer) Up(addr grpc.Address) func(error) {
	if !b.mayPin(addr) {
		return func(err error) {}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// gRPC might call Up after it called Close. We add this check
	// to "fix" it up at application layer. Otherwise, will panic
	// if b.upc is already closed.
	if b.closed {
		return func(err error) {}
	}
	// gRPC might call Up on a stale address.
	// Prevent updating pinAddr with a stale address.
	if !hasAddr(b.addrs, addr.Addr) {
		return func(err error) {}
	}
	if b.pinAddr != "" {
		if logger.V(4) {
			logger.Infof("clientv3/balancer: %q is up but not pinned (already pinned %q)", addr.Addr, b.pinAddr)
		}
		return func(err error) {}
	}
	// notify waiting Get()s and pin first connected address
	close(b.upc)
	b.downc = make(chan struct{})
	b.pinAddr = addr.Addr
	if logger.V(4) {
		logger.Infof("clientv3/balancer: pin %q", addr.Addr)
	}
	// notify client that a connection is up
	b.readyOnce.Do(func() { close(b.readyc) })
	return func(err error) {
		// If connected to a black hole endpoint or a killed server, the gRPC ping
		// timeout will induce a network I/O error, and retrying until success;
		// finding healthy endpoint on retry could take several timeouts and redials.
		// To avoid wasting retries, gray-list unhealthy endpoints.
		b.hostPortError(addr.Addr, err)

		b.mu.Lock()
		b.upc = make(chan struct{})
		close(b.downc)
		b.pinAddr = ""
		b.mu.Unlock()
		if logger.V(4) {
			logger.Infof("clientv3/balancer: unpin %q (%q)", addr.Addr, err.Error())
		}
	}
}

func (b *simpleBalancer) mayPin(addr grpc.Address) bool {
	if b.endpoint(addr.Addr) == "" { // stale host:port
		return false
	}
	b.mu.RLock()
	skip := len(b.addrs) == 1 || len(b.unhealthy) == 0 || len(b.addrs) == len(b.unhealthy)
	failedTime, bad := b.unhealthy[addr.Addr]
	b.mu.RUnlock()
	if skip || !bad {
		return true
	}
	// prevent isolated member's endpoint from being infinitely retried, as follows:
	//   1. keepalive pings detects GoAway with http2.ErrCodeEnhanceYourCalm
	//   2. balancer 'Up' unpins with grpc: failed with network I/O error
	//   3. grpc-healthcheck still SERVING, thus retry to pin
	// instead, return before grpc-healthcheck if failed within healthcheck timeout
	if elapsed := time.Since(failedTime); elapsed < b.healthCheckTimeout {
		if logger.V(4) {
			logger.Infof("clientv3/balancer: %q is up but not pinned (failed %v ago, require minimum %v after failure)", addr.Addr, elapsed, b.healthCheckTimeout)
		}
		return false
	}
	if ok, _ := b.healthCheck(addr.Addr); ok {
		b.removeUnhealthy(addr.Addr, "health check success")
		return true
	}
	b.hostPortError(addr.Addr, errors.New("health check failed"))
	return false
}

func (b *simpleBalancer) updateUnhealthy() {
	for {
		select {
		case <-time.After(b.healthCheckTimeout):
			b.mu.Lock()
			for k, v := range b.unhealthy {
				if time.Since(v) > b.healthCheckTimeout {
					delete(b.unhealthy, k)
					if logger.V(4) {
						logger.Infof("clientv3/balancer: removes %q from unhealthy after %v", k, b.healthCheckTimeout)
					}
				}
			}
			b.mu.Unlock()
			eps := []string{}
			for _, addr := range b.liveAddrs() {
				eps = append(eps, b.endpoint(addr.Addr))
			}
			b.updateAddrs(eps...)
		case <-b.stopc:
			return
		}
	}
}

func (b *simpleBalancer) liveAddrs() []grpc.Address {
	b.mu.RLock()
	defer b.mu.RUnlock()
	hbAddrs := b.addrs
	if len(b.addrs) == 1 || len(b.unhealthy) == 0 || len(b.unhealthy) == len(b.addrs) {
		return hbAddrs
	}
	addrs := make([]grpc.Address, 0, len(b.addrs)-len(b.unhealthy))
	for _, addr := range b.addrs {
		if _, ok := b.unhealthy[addr.Addr]; !ok {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func (b *simpleBalancer) Get(ctx context.Context, opts grpc.BalancerGetOptions) (grpc.Address, func(), error) {
	var (
		addr   string
		closed bool
	)

	// If opts.BlockingWait is false (for fail-fast RPCs), it should return
	// an address it has notified via Notify immediately instead of blocking.
	if !opts.BlockingWait {
		b.mu.RLock()
		closed = b.closed
		addr = b.pinAddr
		b.mu.RUnlock()
		if closed {
			return grpc.Address{Addr: ""}, nil, grpc.ErrClientConnClosing
		}
		if addr == "" {
			return grpc.Address{Addr: ""}, nil, ErrNoAddrAvilable
		}
		return grpc.Address{Addr: addr}, func() {}, nil
	}

	for {
		b.mu.RLock()
		ch := b.upc
		b.mu.RUnlock()
		select {
		case <-ch:
		case <-b.donec:
			return grpc.Address{Addr: ""}, nil, grpc.ErrClientConnClosing
		case <-ctx.Done():
			return grpc.Address{Addr: ""}, nil, ctx.Err()
		}
		b.mu.RLock()
		closed = b.closed
		addr = b.pinAddr
		b.mu.RUnlock()
		// Close() which sets b.closed = true can be called before Get(), Get() must exit if balancer is closed.
		if closed {
			return grpc.Address{Addr: ""}, nil, grpc.ErrClientConnClosing
		}
		if addr != "" {
			break
		}
	}
	return grpc.Address{Addr: addr}, func() {}, nil
}

func (b *simpleBalancer) Notify() <-chan []grpc.Address { return b.notifyCh }

func (b *simpleBalancer) Close() error {
	b.mu.Lock()
	// In case gRPC calls close twice. TODO: remove the checking
	// when we are sure that gRPC wont call close twice.
	if b.closed {
		b.mu.Unlock()
		<-b.donec
		return nil
	}
	b.closed = true
	b.stopOnce.Do(func() { close(b.stopc) })
	b.pinAddr = ""

	// In the case of following scenario:
	//	1. upc is not closed; no pinned address
	// 	2. client issues an RPC, calling invoke(), which calls Get(), enters for loop, blocks
	// 	3. client.conn.Close() calls balancer.Close(); closed = true
	// 	4. for loop in Get() never exits since ctx is the context passed in by the client and may not be canceled
	// we must close upc so Get() exits from blocking on upc
	select {
	case <-b.upc:
	default:
		// terminate all waiting Get()s
		close(b.upc)
	}

	b.mu.Unlock()
	b.wg.Wait()

	// wait for updateNotifyLoop to finish
	<-b.donec
	close(b.notifyCh)

	return nil
}

func getHost(ep string) string {
	url, uerr := url.Parse(ep)
	if uerr != nil || !strings.Contains(ep, "://") {
		return ep
	}
	return url.Host
}

func eps2addrs(eps []string) []grpc.Address {
	addrs := make([]grpc.Address, len(eps))
	for i := range eps {
		addrs[i].Addr = getHost(eps[i])
	}
	return addrs
}

func getHostPort2ep(eps []string) map[string]string {
	hm := make(map[string]string, len(eps))
	for i := range eps {
		_, host, _ := parseEndpoint(eps[i])
		hm[host] = eps[i]
	}
	return hm
}

func hasAddr(addrs []grpc.Address, targetAddr string) bool {
	for _, addr := range addrs {
		if targetAddr == addr.Addr {
			return true
		}
	}
	return false
}

func grpcHealthCheck(client *Client, ep string) (bool, error) {
	conn, err := client.dial(ep)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	cli := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	resp, err := cli.Check(ctx, &healthpb.HealthCheckRequest{})
	cancel()
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
			if s.Message() == unknownService {
				// etcd < v3.3.0
				return true, nil
			}
		}
		return false, err
	}
	return resp.Status == healthpb.HealthCheckResponse_SERVING, nil
}
