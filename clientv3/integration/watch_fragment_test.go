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

// +build !cluster_proxy

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/integration"
	"go.etcd.io/etcd/pkg/testutil"
)

// TestWatchFragmentDisable ensures that large watch
// response exceeding server-side request limit can
// arrive even without watch response fragmentation.
func TestWatchFragmentDisable(t *testing.T) {
	testWatchFragment(t, false, false)
}

// TestWatchFragmentDisableWithGRPCLimit verifies
// large watch response exceeding server-side request
// limit and client-side gRPC response receive limit
// cannot arrive without watch events fragmentation,
// because multiple events exceed client-side gRPC
// response receive limit.
func TestWatchFragmentDisableWithGRPCLimit(t *testing.T) {
	testWatchFragment(t, false, true)
}

// TestWatchFragmentEnable ensures that large watch
// response exceeding server-side request limit arrive
// with watch response fragmentation.
func TestWatchFragmentEnable(t *testing.T) {
	testWatchFragment(t, true, false)
}

// TestWatchFragmentEnableWithGRPCLimit verifies
// large watch response exceeding server-side request
// limit and client-side gRPC response receive limit
// can arrive only when watch events are fragmented.
func TestWatchFragmentEnableWithGRPCLimit(t *testing.T) {
	testWatchFragment(t, true, true)
}

// testWatchFragment triggers watch response that spans over multiple
// revisions exceeding server request limits when combined.
func testWatchFragment(t *testing.T, fragment, exceedRecvLimit bool) {
	cfg := &integration.ClusterConfig{
		Size:            1,
		MaxRequestBytes: 1.5 * 1024 * 1024,
	}
	if exceedRecvLimit {
		cfg.ClientMaxCallRecvMsgSize = 1.5 * 1024 * 1024
	}
	clus := integration.NewClusterV3(t, cfg)
	defer clus.Terminate(t)

	cli := clus.Client(0)
	errc := make(chan error)
	for i := 0; i < 10; i++ {
		go func(i int) {
			_, err := cli.Put(context.TODO(),
				fmt.Sprint("foo", i),
				strings.Repeat("a", 1024*1024),
			)
			errc <- err
		}(i)
	}
	for i := 0; i < 10; i++ {
		if err := <-errc; err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(1)}
	if fragment {
		opts = append(opts, clientv3.WithFragment())
	}
	wch := cli.Watch(context.TODO(), "foo", opts...)

	// expect 10 MiB watch response
	select {
	case ws := <-wch:
		// without fragment, should exceed gRPC client receive limit
		if !fragment && exceedRecvLimit {
			if len(ws.Events) != 0 {
				t.Fatalf("expected 0 events with watch fragmentation, got %d", len(ws.Events))
			}
			exp := "code = ResourceExhausted desc = grpc: received message larger than max ("
			if !strings.Contains(ws.Err().Error(), exp) {
				t.Fatalf("expected 'ResourceExhausted' error, got %v", ws.Err())
			}
			return
		}

		// still expect merged watch events
		if len(ws.Events) != 10 {
			t.Fatalf("expected 10 events with watch fragmentation, got %d", len(ws.Events))
		}
		if ws.Err() != nil {
			t.Fatalf("unexpected error %v", ws.Err())
		}

	case <-time.After(testutil.RequestTimeout):
		t.Fatalf("took too long to receive events")
	}
}
