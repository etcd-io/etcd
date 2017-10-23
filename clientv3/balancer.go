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
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// ErrNoAddrAvilable is returned by Get() when the balancer does not have
// any active connection to endpoints at the time.
// This error is returned only when opts.BlockingWait is true.
var ErrNoAddrAvilable = grpc.Errorf(codes.Unavailable, "there is no address available")

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

	// mu protects all fields below.
	mu sync.RWMutex

	// upc closes when pinAddr transitions from empty to non-empty or the balancer closes.
	upc chan struct{}

	// downc closes when grpc calls down() on pinAddr
	downc chan struct{}

	// stopc is closed to signal updateNotifyLoop should stop.
	stopc chan struct{}

	// donec closes when all goroutines are exited
	donec chan struct{}

	// updateAddrsC notifies updateNotifyLoop to update addrs.
	updateAddrsC chan notifyMsg

	// grpc issues TLS cert checks using the string passed into dial so
	// that string must be the host. To recover the full scheme://host URL,
	// have a map from hosts to the original endpoint.
	hostPort2ep map[string]string

	// pinAddr is the currently pinned address; set to the empty string on
	// initialization and shutdown.
	pinAddr string

	closed bool
}

func newSimpleBalancer(eps []string) *simpleBalancer {
	notifyCh := make(chan []grpc.Address)
	addrs := eps2addrs(eps)
	sb := &simpleBalancer{
		addrs:        addrs,
		eps:          eps,
		notifyCh:     notifyCh,
		readyc:       make(chan struct{}),
		upc:          make(chan struct{}),
		stopc:        make(chan struct{}),
		downc:        make(chan struct{}),
		donec:        make(chan struct{}),
		updateAddrsC: make(chan notifyMsg),
		hostPort2ep:  getHostPort2ep(eps),
	}
	close(sb.downc)
	go sb.updateNotifyLoop()
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
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.hostPort2ep[hostPort]
}

func (b *simpleBalancer) endpoints() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.eps
}

func (b *simpleBalancer) pinned() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.pinAddr
}

func getHostPort2ep(eps []string) map[string]string {
	hm := make(map[string]string, len(eps))
	for i := range eps {
		_, host, _ := parseEndpoint(eps[i])
		hm[host] = eps[i]
	}
	return hm
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
	f, _ := b.up(addr)
	return f
}

func (b *simpleBalancer) up(addr grpc.Address) (func(error), bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// gRPC might call Up after it called Close. We add this check
	// to "fix" it up at application layer. Otherwise, will panic
	// if b.upc is already closed.
	if b.closed {
		return func(err error) {}, false
	}
	// gRPC might call Up on a stale address.
	// Prevent updating pinAddr with a stale address.
	if !hasAddr(b.addrs, addr.Addr) {
		return func(err error) {}, false
	}
	if b.pinAddr != "" {
		if logger.V(4) {
			logger.Infof("clientv3/balancer: %q is up but not pinned (already pinned %q)", addr.Addr, b.pinAddr)
		}
		return func(err error) {}, false
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
		b.mu.Lock()
		b.upc = make(chan struct{})
		close(b.downc)
		b.pinAddr = ""
		b.mu.Unlock()
		if logger.V(4) {
			logger.Infof("clientv3/balancer: unpin %q (%q)", addr.Addr, err.Error())
		}
	}, true
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

func eps2addrs(eps []string) []grpc.Address {
	addrs := make([]grpc.Address, len(eps))
	for i := range eps {
		addrs[i].Addr = getHost(eps[i])
	}
	return addrs
}
