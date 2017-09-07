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
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// ErrNoAddrAvilable is returned by Get() when the balancer does not have
// any active connection to endpoints at the time.
// This error is returned only when opts.BlockingWait is true.
var ErrNoAddrAvilable = grpc.Errorf(codes.Unavailable, "there is no address available")

// simpleBalancer does the bare minimum to expose multiple eps
// to the grpc reconnection code path
type simpleBalancer struct {
	// addrs are the client's endpoints for grpc
	addrs       []grpc.Address
	healthCheck func(ep string) (bool, error)

	// notifyCh notifies grpc of the set of addresses for connecting
	notifyCh chan []grpc.Address

	// readyc closes once the first connection is up
	readyc    chan struct{}
	readyOnce sync.Once

	// mu protects all fields below.
	mu sync.RWMutex

	// failed records last failed time of endpoints.
	failed map[string]time.Time

	// upc closes when pinAddr transitions from empty to non-empty or the balancer closes.
	upc chan struct{}

	// downc closes when grpc calls down() on pinAddr
	downc chan struct{}

	// stopc is closed to signal updateNotifyLoop should stop.
	stopc chan struct{}

	// donec closes when all goroutines are exited
	donec chan struct{}

	// updateAddrsC notifies updateNotifyLoop to update addrs.
	updateAddrsC chan struct{}

	// grpc issues TLS cert checks using the string passed into dial so
	// that string must be the host. To recover the full scheme://host URL,
	// have a map from hosts to the original endpoint.
	host2ep map[string]string

	// pinAddr is the currently pinned address; set to the empty string on
	// initialization and shutdown.
	pinAddr string

	closed bool
}

func newSimpleBalancer(eps []string, dialTimeout time.Duration, healthCheck func(ep string) (bool, error)) *simpleBalancer {
	notifyCh := make(chan []grpc.Address, 1)
	addrs := make([]grpc.Address, len(eps))
	for i := range eps {
		addrs[i].Addr = getHost(eps[i])
	}
	sb := &simpleBalancer{
		addrs:        addrs,
		healthCheck:  healthCheck,
		notifyCh:     notifyCh,
		failed:       make(map[string]time.Time),
		readyc:       make(chan struct{}),
		upc:          make(chan struct{}),
		stopc:        make(chan struct{}),
		downc:        make(chan struct{}),
		donec:        make(chan struct{}),
		updateAddrsC: make(chan struct{}, 1),
		host2ep:      getHost2ep(eps),
	}
	close(sb.downc)
	go sb.updateNotifyLoop()
	go sb.updateFailed(dialTimeout)
	return sb
}

func (b *simpleBalancer) Start(target string, config grpc.BalancerConfig) error { return nil }

func (b *simpleBalancer) ConnectNotify() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.upc
}

func (b *simpleBalancer) getEndpoint(host string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.host2ep[host]
}

func getHost2ep(eps []string) map[string]string {
	hm := make(map[string]string, len(eps))
	for i := range eps {
		_, host, _ := parseEndpoint(eps[i])
		hm[host] = eps[i]
	}
	return hm
}

func (b *simpleBalancer) updateAddrs(eps []string) {
	np := getHost2ep(eps)

	b.mu.Lock()

	match := len(np) == len(b.host2ep)
	for k, v := range np {
		if b.host2ep[k] != v {
			match = false
			break
		}
	}
	if match {
		// same endpoints, so no need to update address
		b.mu.Unlock()
		return
	}

	b.host2ep = np

	addrs := make([]grpc.Address, 0, len(eps))
	all := make(map[string]struct{}, len(eps))
	for i := range eps {
		addr := grpc.Address{Addr: getHost(eps[i])}
		addrs = append(addrs, addr)
		all[addr.Addr] = struct{}{}
	}
	b.addrs = addrs

	failed := b.failed
	for k := range failed {
		if _, ok := all[k]; !ok {
			delete(failed, k)
		}
	}
	b.failed = failed

	// updating notifyCh can trigger new connections,
	// only update addrs if all connections are down
	// or addrs does not include pinAddr.
	update := !hasAddr(addrs, b.pinAddr)
	b.mu.Unlock()

	if update {
		select {
		case b.updateAddrsC <- struct{}{}:
		case <-b.stopc:
		}
	}
}

func hasAddr(addrs []grpc.Address, targetAddr string) bool {
	for _, addr := range addrs {
		if targetAddr == addr.Addr {
			return true
		}
	}
	return false
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
			b.notifyAddrs()
			select {
			case <-upc:
			case <-b.updateAddrsC:
				b.notifyAddrs()
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
			case <-b.updateAddrsC:
			case <-b.stopc:
				return
			}
			b.notifyAddrs()
		}
	}
}

func (b *simpleBalancer) updateFailed(timeout time.Duration) {
	if timeout < 3*time.Second {
		timeout = 3 * time.Second
	}
	for {
		select {
		case <-time.After(timeout):
			b.mu.Lock()
			for k, v := range b.failed {
				if time.Since(v) > timeout {
					delete(b.failed, k)
				}
			}
			b.mu.Unlock()
		case <-b.stopc:
			return
		}
	}
}

type addrConn struct {
	addr   grpc.Address
	failed time.Time
}

func (b *simpleBalancer) notifyAddrs() {
	b.mu.RLock()
	var addrs []grpc.Address
	if len(b.addrs) == 1 ||
		len(b.failed) == 0 ||
		b.healthCheck == nil { // single-endpoint, or no failure
		addrs = b.addrs
	} else if len(b.failed) > 0 { // last pinned failing temporarily
		all := make(map[string]grpc.Address)
		for _, addr := range b.addrs {
			all[addr.Addr] = addr
			if _, ok := b.failed[addr.Addr]; ok {
				continue
			}
			ok, err := b.healthCheck(addr.Addr)
			if ok && err == nil {
				addrs = append(addrs, addr)
			}
		}
		if len(addrs) == 0 { // no better alternative found
			addrs = b.addrs
		} else { // sort that latest failed be at the end
			addrConns := make([]addrConn, 0, len(b.failed))
			for k, v := range b.failed {
				addrConns = append(addrConns, addrConn{addr: all[k], failed: v})
			}
			sort.Slice(addrConns, func(i, j int) bool {
				return addrConns[i].failed.Before(addrConns[j].failed)
			})
			for _, a := range addrConns {
				addrs = append(addrs, a.addr)
			}
		}
	}
	b.mu.RUnlock()
	select {
	case b.notifyCh <- addrs:
	case <-b.stopc:
	}
}

func (b *simpleBalancer) shouldPin(addr grpc.Address) bool {
	if len(b.addrs) == 1 {
		return true
	}

	if _, ok := b.failed[addr.Addr]; !ok { // did not fail
		if b.healthCheck != nil {
			healthy, err := b.healthCheck(addr.Addr)
			if healthy && err == nil { // confirmed healthy
				return true
			}
			b.failed[addr.Addr] = time.Now()
			return false
		}
		return true
	}
	if b.healthCheck == nil {
		return true
	}

	// failing temporarily
	healthy, err := b.healthCheck(addr.Addr)
	if healthy && err == nil { // recovered
		delete(b.failed, addr.Addr)
		return true
	}

	// failing still
	for _, other := range b.addrs {
		if other.Addr == addr.Addr {
			continue
		}
		healthy, err := b.healthCheck(other.Addr)
		if healthy && err == nil { // found better alternative
			return false
		}
	}

	// no better alternatives found; keep retrying
	return true
}

// Up is part of the balancer interface, simpleBalancer implements the interface.
// Balancer notifies a set of endpoints and pin whichever endpoint gRPC
// Balancer.Up first and closes other endpoints. Client could get stuck
// retrying blackholed endpoints, because gRPC marks network I/O errors
// as transient, retrying until success; it takes several seconds
// to find healthy one in next tries. Same applies when server is stopped.
// To avoid wasting retries, gray-list unhealthy endpoints on notifyAddrs,
// unlisting them after dial timeouts, and double-checks endpoint health
// with gRPC health API.
func (b *simpleBalancer) Up(addr grpc.Address) func(error) {
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
		return func(err error) {}
	}
	if !b.shouldPin(addr) {
		return func(err error) {}
	}
	// notify waiting Get()s and pin first connected address
	close(b.upc)
	b.downc = make(chan struct{})
	b.pinAddr = addr.Addr
	// notify client that a connection is up
	b.readyOnce.Do(func() { close(b.readyc) })
	return func(err error) {
		b.mu.Lock()
		b.failed[addr.Addr] = time.Now()
		b.upc = make(chan struct{})
		close(b.downc)
		b.pinAddr = ""
		b.mu.Unlock()
	}
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
	close(b.stopc)
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
