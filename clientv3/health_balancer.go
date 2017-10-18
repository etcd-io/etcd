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

type errorInfo struct {
	failed time.Time
	err    error
}

// healthBalancer wraps a balancer so that it uses health checking
// to choose its endpoints.
type healthBalancer struct {
	balancer

	// healthCheck checks an endpoint's health.
	healthCheck        healthCheckFunc
	healthCheckTimeout time.Duration

	// mu protects addrs, eps, unhealthy map, and stopc.
	mu sync.RWMutex

	// addrs stores all grpc addresses associated with the balancer.
	addrs []grpc.Address

	// eps stores all client endpoints
	eps []string

	// unhealthyHosts tracks the last unhealthy time of endpoints.
	unhealthyHosts map[string]errorInfo

	stopc    chan struct{}
	stopOnce sync.Once

	host2ep map[string]string

	wg sync.WaitGroup
}

func newHealthBalancer(b balancer, timeout time.Duration, hc healthCheckFunc) *healthBalancer {
	hb := &healthBalancer{
		balancer:       b,
		healthCheck:    hc,
		eps:            b.endpoints(),
		addrs:          eps2addrs(b.endpoints()),
		host2ep:        getHost2ep(b.endpoints()),
		unhealthyHosts: make(map[string]errorInfo),
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
		hb.mu.Lock()
		hb.unhealthyHosts[addr.Addr] = errorInfo{failed: time.Now(), err: err}
		hb.mu.Unlock()
		f(err)
		if logger.V(4) {
			logger.Infof("clientv3/health-balancer: %q becomes unhealthy (%q)", addr.Addr, err.Error())
		}
	}
}

func (hb *healthBalancer) up(addr grpc.Address) (func(error), bool) {
	if !hb.mayPin(addr) {
		return func(err error) {}, false
	}
	return hb.balancer.up(addr)
}

func (hb *healthBalancer) Close() error {
	hb.stopOnce.Do(func() { close(hb.stopc) })
	hb.wg.Wait()
	return hb.balancer.Close()
}

func (hb *healthBalancer) updateAddrs(eps ...string) {
	addrs, host2ep := eps2addrs(eps), getHost2ep(eps)
	hb.mu.Lock()
	hb.addrs, hb.eps, hb.host2ep = addrs, eps, host2ep
	hb.unhealthyHosts = make(map[string]errorInfo)
	hb.mu.Unlock()
	hb.balancer.updateAddrs(eps...)
}

func (hb *healthBalancer) endpoint(host string) string {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	return hb.host2ep[host]
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
			for k, v := range hb.unhealthyHosts {
				if _, ok := hb.host2ep[k]; !ok {
					delete(hb.unhealthyHosts, k)
					if logger.V(4) {
						logger.Infof("clientv3/health-balancer: removes stale endpoint %q from unhealthy", k)
					}
					continue
				}
				if time.Since(v.failed) > timeout {
					delete(hb.unhealthyHosts, k)
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
			hb.balancer.updateAddrs(eps...)
		case <-hb.stopc:
			return
		}
	}
}

func (hb *healthBalancer) liveAddrs() []grpc.Address {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	hbAddrs := hb.addrs
	if len(hb.addrs) == 1 || len(hb.unhealthyHosts) == 0 || len(hb.unhealthyHosts) == len(hb.addrs) {
		return hbAddrs
	}
	addrs := make([]grpc.Address, 0, len(hb.addrs)-len(hb.unhealthyHosts))
	for _, addr := range hb.addrs {
		if _, unhealthy := hb.unhealthyHosts[addr.Addr]; !unhealthy {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func (hb *healthBalancer) endpointError(host string, err error) {
	hb.mu.Lock()
	hb.unhealthyHosts[host] = errorInfo{failed: time.Now(), err: err}
	hb.mu.Unlock()
	if logger.V(4) {
		logger.Infof("clientv3/health-balancer: marking %q as unhealthy (%q)", host, err.Error())
	}
}

func (hb *healthBalancer) isFailed(host string) (ev errorInfo, ok bool) {
	hb.mu.RLock()
	ev, ok = hb.unhealthyHosts[host]
	hb.mu.RUnlock()
	return ev, ok
}

func (hb *healthBalancer) mayPin(addr grpc.Address) bool {
	hb.mu.RLock()
	if _, ok := hb.host2ep[addr.Addr]; !ok {
		hb.mu.RUnlock()
		return false
	}
	skip := len(hb.addrs) == 1 || len(hb.unhealthyHosts) == 0 || len(hb.addrs) == len(hb.unhealthyHosts)
	ef, bad := hb.unhealthyHosts[addr.Addr]
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
	if elapsed := time.Since(ef.failed); elapsed < dur {
		if logger.V(4) {
			logger.Infof("clientv3/health-balancer: %q is up but not pinned (failed %v ago, require minimum %v after failure)", addr.Addr, elapsed, dur)
		}
		return false
	}
	ok, err := hb.healthCheck(addr.Addr)
	if ok {
		hb.mu.Lock()
		delete(hb.unhealthyHosts, addr.Addr)
		hb.mu.Unlock()
		if logger.V(4) {
			logger.Infof("clientv3/health-balancer: %q is healthy (health check success)", addr.Addr)
		}
		return true
	}
	hb.mu.Lock()
	hb.unhealthyHosts[addr.Addr] = errorInfo{failed: time.Now(), err: err}
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
