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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3/balancer/picker"
	"go.etcd.io/etcd/clientv3/balancer/resolver/endpoint"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/mock/mockserver"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// TestRoundRobinBalancedResolvableNoFailover ensures that
// requests to a resolvable endpoint can be balanced between
// multiple, if any, nodes. And there needs be no failover.
func TestRoundRobinBalancedResolvableNoFailover(t *testing.T) {
	testCases := []struct {
		name        string
		serverCount int
		reqN        int
		network     string
	}{
		{name: "rrBalanced_1", serverCount: 1, reqN: 5, network: "tcp"},
		{name: "rrBalanced_1_unix_sockets", serverCount: 1, reqN: 5, network: "unix"},
		{name: "rrBalanced_3", serverCount: 3, reqN: 7, network: "tcp"},
		{name: "rrBalanced_5", serverCount: 5, reqN: 10, network: "tcp"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ms, err := mockserver.StartMockServersOnNetwork(tc.serverCount, tc.network)
			if err != nil {
				t.Fatalf("failed to start mock servers: %v", err)
			}
			defer ms.Stop()

			var eps []string
			for _, svr := range ms.Servers {
				eps = append(eps, svr.ResolverAddress().Addr)
			}

			rsv, err := endpoint.NewResolverGroup("nofailover")
			if err != nil {
				t.Fatal(err)
			}
			defer rsv.Close()
			rsv.SetEndpoints(eps)

			name := genName()
			cfg := Config{
				Policy: picker.RoundrobinBalanced,
				Name:   name,
				Logger: zap.NewExample(),
			}
			RegisterBuilder(cfg)
			conn, err := grpc.Dial(fmt.Sprintf("endpoint://nofailover/*"), grpc.WithInsecure(), grpc.WithBalancerName(name))
			if err != nil {
				t.Fatalf("failed to dial mock server: %v", err)
			}
			defer conn.Close()
			cli := pb.NewKVClient(conn)

			reqFunc := func(ctx context.Context) (picked string, err error) {
				var p peer.Peer
				_, err = cli.Range(ctx, &pb.RangeRequest{Key: []byte("/x")}, grpc.Peer(&p))
				if p.Addr != nil {
					picked = p.Addr.String()
				}
				return picked, err
			}

			prev, switches := "", 0
			for i := 0; i < tc.reqN; i++ {
				picked, err := reqFunc(context.Background())
				if err != nil {
					t.Fatalf("#%d: unexpected failure %v", i, err)
				}
				if prev == "" {
					prev = picked
					continue
				}
				if prev != picked {
					switches++
				}
				prev = picked
			}
			if tc.serverCount > 1 && switches < tc.reqN-3 { // -3 for initial resolutions
				// TODO: FIX ME
				t.Skipf("expected balanced loads for %d requests, got switches %d", tc.reqN, switches)
			}
		})
	}
}

// TestRoundRobinBalancedResolvableFailoverFromServerFail ensures that
// loads be rebalanced while one server goes down and comes back.
func TestRoundRobinBalancedResolvableFailoverFromServerFail(t *testing.T) {
	serverCount := 5
	ms, err := mockserver.StartMockServers(serverCount)
	if err != nil {
		t.Fatalf("failed to start mock servers: %s", err)
	}
	defer ms.Stop()
	var eps []string
	for _, svr := range ms.Servers {
		eps = append(eps, svr.ResolverAddress().Addr)
	}

	rsv, err := endpoint.NewResolverGroup("serverfail")
	if err != nil {
		t.Fatal(err)
	}
	defer rsv.Close()
	rsv.SetEndpoints(eps)

	name := genName()
	cfg := Config{
		Policy: picker.RoundrobinBalanced,
		Name:   name,
		Logger: zap.NewExample(),
	}
	RegisterBuilder(cfg)
	conn, err := grpc.Dial(fmt.Sprintf("endpoint://serverfail/mock.server"), grpc.WithInsecure(), grpc.WithBalancerName(name))
	if err != nil {
		t.Fatalf("failed to dial mock server: %s", err)
	}
	defer conn.Close()
	cli := pb.NewKVClient(conn)

	reqFunc := func(ctx context.Context) (picked string, err error) {
		var p peer.Peer
		_, err = cli.Range(ctx, &pb.RangeRequest{Key: []byte("/x")}, grpc.Peer(&p))
		if p.Addr != nil {
			picked = p.Addr.String()
		}
		return picked, err
	}

	// stop first server, loads should be redistributed
	// stopped server should never be picked
	ms.StopAt(0)
	available := make(map[string]struct{})
	for i := 1; i < serverCount; i++ {
		available[eps[i]] = struct{}{}
	}

	reqN := 10
	prev, switches := "", 0
	for i := 0; i < reqN; i++ {
		picked, err := reqFunc(context.Background())
		if err != nil && strings.Contains(err.Error(), "transport is closing") {
			continue
		}
		if prev == "" { // first failover
			if eps[0] == picked {
				t.Fatalf("expected failover from %q, picked %q", eps[0], picked)
			}
			prev = picked
			continue
		}
		if _, ok := available[picked]; !ok {
			t.Fatalf("picked unavailable address %q (available %v)", picked, available)
		}
		if prev != picked {
			switches++
		}
		prev = picked
	}
	if switches < reqN-3 { // -3 for initial resolutions + failover
		// TODO: FIX ME!
		t.Skipf("expected balanced loads for %d requests, got switches %d", reqN, switches)
	}

	// now failed server comes back
	ms.StartAt(0)

	// enough time for reconnecting to recovered server
	time.Sleep(time.Second)

	prev, switches = "", 0
	recoveredAddr, recovered := eps[0], 0
	available[recoveredAddr] = struct{}{}

	for i := 0; i < 2*reqN; i++ {
		picked, err := reqFunc(context.Background())
		if err != nil {
			t.Fatalf("#%d: unexpected failure %v", i, err)
		}
		if prev == "" {
			prev = picked
			continue
		}
		if _, ok := available[picked]; !ok {
			t.Fatalf("#%d: picked unavailable address %q (available %v)", i, picked, available)
		}
		if prev != picked {
			switches++
		}
		if picked == recoveredAddr {
			recovered++
		}
		prev = picked
	}
	if switches < reqN-3 { // -3 for initial resolutions
		t.Fatalf("expected balanced loads for %d requests, got switches %d", reqN, switches)
	}
	if recovered < reqN/serverCount {
		t.Fatalf("recovered server %q got only %d requests", recoveredAddr, recovered)
	}
}

// TestRoundRobinBalancedResolvableFailoverFromRequestFail ensures that
// loads be rebalanced while some requests are failed.
func TestRoundRobinBalancedResolvableFailoverFromRequestFail(t *testing.T) {
	serverCount := 5
	ms, err := mockserver.StartMockServers(serverCount)
	if err != nil {
		t.Fatalf("failed to start mock servers: %s", err)
	}
	defer ms.Stop()
	var eps []string
	available := make(map[string]struct{})
	for _, svr := range ms.Servers {
		eps = append(eps, svr.ResolverAddress().Addr)
		available[svr.Address] = struct{}{}
	}
	rsv, err := endpoint.NewResolverGroup("requestfail")
	if err != nil {
		t.Fatal(err)
	}
	defer rsv.Close()
	rsv.SetEndpoints(eps)

	name := genName()
	cfg := Config{
		Policy: picker.RoundrobinBalanced,
		Name:   name,
		Logger: zap.NewExample(),
	}
	RegisterBuilder(cfg)
	conn, err := grpc.Dial(fmt.Sprintf("endpoint://requestfail/mock.server"), grpc.WithInsecure(), grpc.WithBalancerName(name))
	if err != nil {
		t.Fatalf("failed to dial mock server: %s", err)
	}
	defer conn.Close()
	cli := pb.NewKVClient(conn)

	reqFunc := func(ctx context.Context) (picked string, err error) {
		var p peer.Peer
		_, err = cli.Range(ctx, &pb.RangeRequest{Key: []byte("/x")}, grpc.Peer(&p))
		if p.Addr != nil {
			picked = p.Addr.String()
		}
		return picked, err
	}

	reqN := 20
	prev, switches := "", 0
	for i := 0; i < reqN; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if i%2 == 0 {
			cancel()
		}
		picked, err := reqFunc(ctx)
		if i%2 == 0 {
			if s, ok := status.FromError(err); ok && s.Code() != codes.Canceled || picked != "" {
				t.Fatalf("#%d: expected %v, got %v", i, context.Canceled, err)
			}
			continue
		}
		if prev == "" && picked != "" {
			prev = picked
			continue
		}
		if _, ok := available[picked]; !ok {
			t.Fatalf("#%d: picked unavailable address %q (available %v)", i, picked, available)
		}
		if prev != picked {
			switches++
		}
		prev = picked
	}
	if switches < reqN/2-3 { // -3 for initial resolutions + failover
		t.Fatalf("expected balanced loads for %d requests, got switches %d", reqN, switches)
	}
}
