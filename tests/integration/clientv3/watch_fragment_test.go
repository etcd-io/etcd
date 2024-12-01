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

//go:build !cluster_proxy

package clientv3test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestWatchFragmentDisable(t *testing.T) {
	testWatchFragment(t, false, false)
}

func TestWatchFragmentDisableWithGRPCLimit(t *testing.T) {
	testWatchFragment(t, false, true)
}

func TestWatchFragmentEnable(t *testing.T) {
	testWatchFragment(t, true, false)
}

func TestWatchFragmentEnableWithGRPCLimit(t *testing.T) {
	testWatchFragment(t, true, true)
}

// testWatchFragment triggers watch response that spans over multiple
// revisions exceeding server request limits when combined.
func testWatchFragment(t *testing.T, fragment, exceedRecvLimit bool) {
	integration2.BeforeTest(t)

	cfg := &integration2.ClusterConfig{
		Size:            1,
		MaxRequestBytes: 1.5 * 1024 * 1024,
	}
	if exceedRecvLimit {
		cfg.ClientMaxCallRecvMsgSize = 1.5 * 1024 * 1024
	}
	clus := integration2.NewCluster(t, cfg)
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
		err := <-errc
		require.NoErrorf(t, err, "failed to put")
	}

	opts := []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(1)}
	if fragment {
		opts = append(opts, clientv3.WithFragment())
	}
	wch := cli.Watch(context.TODO(), "foo", opts...)

	// expect 10 MiB watch response
	eventCount := 0
	timeout := time.After(testutil.RequestTimeout)
	for eventCount < 10 {
		select {
		case ws := <-wch:
			// still expect merged watch events
			if ws.Err() != nil {
				t.Fatalf("unexpected error %v", ws.Err())
			}
			eventCount += len(ws.Events)

		case <-timeout:
			t.Fatalf("took too long to receive events")
		}
	}
	if eventCount != 10 {
		t.Fatalf("expected 10 events with watch fragmentation, got %d", eventCount)
	}
}
