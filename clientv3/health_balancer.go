// Copyright 2017 The etcd Authors
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

// healthBalancer wraps a balancer so that it uses health checking
// to choose its endpoints.
type healthBalancer struct {
	*simpleBalancer

	// healthCheck checks an endpoint's health.
	healthCheck        healthCheckFunc
	healthCheckTimeout time.Duration

	// mu protects addrs, eps, unhealthy map, and stopc.
	mu sync.RWMutex

	// addrs stores all grpc addresses associated with the balancer.
	addrs []grpc.Address

	// eps stores all client endpoints
	eps []string

	// unhealthy tracks the last unhealthy time of endpoints.
	unhealthy map[string]time.Time

	stopc    chan struct{}
	stopOnce sync.Once

	hostPort2ep map[string]string

	wg sync.WaitGroup
}

func newHealthBalancer(b *simpleBalancer, timeout time.Duration, hc healthCheckFunc) *healthBalancer {
	hb := &healthBalancer{
		simpleBalancer: b,
		healthCheck:    hc,
		eps:            b.endpoints(),
		addrs:          eps2addrs(b.endpoints()),
		hostPort2ep:    getHostPort2ep(b.endpoints()),
		unhealthy:      make(map[string]time.Time),
		stopc:          make(chan struct{}),
	}
	if timeout < minHealthRetryDuration {
		timeout = minHealthRetryDuration
	}
	hb.healthCheckTimeout = timeout

	hb.wg.Add(1)
	go func() {
		defer hb.wg.Done()
		hb.updateUnhealthy(timeout)
	}()

	return hb
}

func (hb *healthBalancer) Up(addr grpc.Address) func(error) {
	f, used := hb.up(addr)
	if !used {
		return f
	}
	return func(err error) {
		// If connected to a black hole endpoint or a killed server, the gRPC ping
		// timeout will induce a network I/O error, and retrying until success;
		// finding healthy endpoint on retry could take several timeouts and redials.
		// To avoid wasting retries, gray-list unhealthy endpoints.
		hb.hostPortError(addr.Addr, err)
		f(err)
	}
}

func (hb *healthBalancer) up(addr grpc.Address) (func(error), bool) {
	if !hb.mayPin(addr) {
		return func(err error) {}, false
	}
	return hb.simpleBalancer.up(addr)
}

func (hb *healthBalancer) Close() error {
	hb.stopOnce.Do(func() { close(hb.stopc) })
	hb.wg.Wait()
	return hb.simpleBalancer.Close()
}

func (hb *healthBalancer) updateAddrs(eps ...string) {
	addrs, hostPort2ep := eps2addrs(eps), getHostPort2ep(eps)
	hb.mu.Lock()
	hb.addrs, hb.eps, hb.hostPort2ep = addrs, eps, hostPort2ep
	hb.unhealthy = make(map[string]time.Time)
	hb.mu.Unlock()
	hb.simpleBalancer.updateAddrs(eps...)
}

func (hb *healthBalancer) endpoint(host string) string {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	return hb.hostPort2ep[host]
}

func (hb *healthBalancer) endpoints() []string {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	return hb.eps
}

func (hb *healthBalancer) updateUnhealthy(timeout time.Duration) {
	for {
		select {
		case <-time.After(timeout):
			hb.mu.Lock()
			for k, v := range hb.unhealthy {
				if time.Since(v) > timeout {
					delete(hb.unhealthy, k)
					if logger.V(4) {
						logger.Infof("clientv3/health-balancer: removes %q from unhealthy after %v", k, timeout)
					}
				}
			}
			hb.mu.Unlock()
			eps := []string{}
			for _, addr := range hb.liveAddrs() {
				eps = append(eps, hb.endpoint(addr.Addr))
			}
			hb.simpleBalancer.updateAddrs(eps...)
		case <-hb.stopc:
			return
		}
	}
}

func (hb *healthBalancer) liveAddrs() []grpc.Address {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	hbAddrs := hb.addrs
	if len(hb.addrs) == 1 || len(hb.unhealthy) == 0 || len(hb.unhealthy) == len(hb.addrs) {
		return hbAddrs
	}
	addrs := make([]grpc.Address, 0, len(hb.addrs)-len(hb.unhealthy))
	for _, addr := range hb.addrs {
		if _, unhealthy := hb.unhealthy[addr.Addr]; !unhealthy {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func (hb *healthBalancer) hostPortError(hostPort string, err error) {
	hb.mu.Lock()
	if _, ok := hb.hostPort2ep[hostPort]; ok {
		hb.unhealthy[hostPort] = time.Now()
		if logger.V(4) {
			logger.Infof("clientv3/health-balancer: marking %q as unhealthy (%q)", hostPort, err.Error())
		}
	}
	hb.mu.Unlock()
}

func (hb *healthBalancer) mayPin(addr grpc.Address) bool {
	hb.mu.RLock()
	if _, ok := hb.hostPort2ep[addr.Addr]; !ok { // stale host:port
		hb.mu.RUnlock()
		return false
	}
	skip := len(hb.addrs) == 1 || len(hb.unhealthy) == 0 || len(hb.addrs) == len(hb.unhealthy)
	failedTime, bad := hb.unhealthy[addr.Addr]
	dur := hb.healthCheckTimeout
	hb.mu.RUnlock()
	if skip || !bad {
		return true
	}
	// prevent isolated member's endpoint from being infinitely retried, as follows:
	//   1. keepalive pings detects GoAway with http2.ErrCodeEnhanceYourCalm
	//   2. balancer 'Up' unpins with grpc: failed with network I/O error
	//   3. grpc-healthcheck still SERVING, thus retry to pin
	// instead, return before grpc-healthcheck if failed within healthcheck timeout
	if elapsed := time.Since(failedTime); elapsed < dur {
		if logger.V(4) {
			logger.Infof("clientv3/health-balancer: %q is up but not pinned (failed %v ago, require minimum %v after failure)", addr.Addr, elapsed, dur)
		}
		return false
	}
	if ok, _ := hb.healthCheck(addr.Addr); ok {
		hb.mu.Lock()
		delete(hb.unhealthy, addr.Addr)
		hb.mu.Unlock()
		if logger.V(4) {
			logger.Infof("clientv3/health-balancer: %q is healthy (health check success)", addr.Addr)
		}
		return true
	}
	hb.mu.Lock()
	hb.unhealthy[addr.Addr] = time.Now()
	hb.mu.Unlock()
	if logger.V(4) {
		logger.Infof("clientv3/health-balancer: %q becomes unhealthy (health check failed)", addr.Addr)
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
