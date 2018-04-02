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
	"testing"

	"github.com/coreos/etcd/clientv3/balancer/picker"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/mock/mockserver"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

// TestRoundRobinBalancedResolvableNoFailover ensures that
// requests to a resolvable endpoint can be balanced between
// multiple, if any, nodes. And there needs be no failover.
func TestRoundRobinBalancedResolvableNoFailover(t *testing.T) {
	testCases := []struct {
		name        string
		serverCount int
		reqN        int
	}{
		{name: "rrBalanced_1", serverCount: 1, reqN: 5},
		{name: "rrBalanced_3", serverCount: 3, reqN: 7},
		{name: "rrBalanced_5", serverCount: 5, reqN: 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ms, err := mockserver.StartMockServers(tc.serverCount)
			if err != nil {
				t.Fatalf("failed to start mock servers: %v", err)
			}
			defer ms.Stop()
			var resolvedAddrs []resolver.Address
			for _, svr := range ms.Servers {
				resolvedAddrs = append(resolvedAddrs, resolver.Address{Addr: svr.Address})
			}

			rsv, closeResolver := manual.GenerateAndRegisterManualResolver()
			defer closeResolver()
			cfg := Config{
				Policy:    picker.RoundrobinBalanced,
				Name:      genName(),
				Logger:    zap.NewExample(),
				Endpoints: []string{fmt.Sprintf("%s:///mock.server", rsv.Scheme())},
			}
			rrb := New(cfg)
			conn, err := grpc.Dial(cfg.Endpoints[0], grpc.WithInsecure(), grpc.WithBalancerName(rrb.Name()))
			if err != nil {
				t.Fatalf("failed to dial mock server: %s", err)
			}
			defer conn.Close()
			rsv.NewAddress(resolvedAddrs)
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
				t.Fatalf("expected balanced loads for %d requests, got switches %d", tc.reqN, switches)
			}
		})
	}
}
