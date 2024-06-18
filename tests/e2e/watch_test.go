// Copyright 2023 The etcd Authors
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

// These tests are performance sensitive, addition of cluster proxy makes them unstable.
//go:build !cluster_proxy

package e2e

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	watchResponsePeriod = 100 * time.Millisecond
	watchTestDuration   = 5 * time.Second
	readLoadConcurrency = 10
)

type testCase struct {
	name               string
	client             e2e.ClientConfig
	clientHTTPSeparate bool
	maxWatchDelay      time.Duration
	dbSizeBytes        int
}

const (
	Kilo = 1000
	Mega = 1000 * Kilo
)

// 10 MB is not a bottleneck of grpc server, but filling up etcd with data.
// Keeping it lower so tests don't take too long.
// If we implement reuse of db we could increase the dbSize.
var tcs = []testCase{
	{
		name:          "NoTLS",
		maxWatchDelay: 150 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
	{
		name:          "TLS",
		client:        e2e.ClientConfig{ConnectionType: e2e.ClientTLS},
		maxWatchDelay: 150 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
	{
		name:               "SeparateHTTPNoTLS",
		clientHTTPSeparate: true,
		maxWatchDelay:      150 * time.Millisecond,
		dbSizeBytes:        5 * Mega,
	},
	{
		name:               "SeparateHTTPTLS",
		client:             e2e.ClientConfig{ConnectionType: e2e.ClientTLS},
		clientHTTPSeparate: true,
		maxWatchDelay:      150 * time.Millisecond,
		dbSizeBytes:        5 * Mega,
	},
}

func TestWatchDelayForPeriodicProgressNotification(t *testing.T) {
	e2e.BeforeTest(t)
	for _, tc := range tcs {
		tc := tc
		cfg := e2e.DefaultConfig()
		cfg.ClusterSize = 1
		cfg.ServerConfig.ExperimentalWatchProgressNotifyInterval = watchResponsePeriod
		cfg.Client = tc.client
		cfg.ClientHTTPSeparate = tc.clientHTTPSeparate
		t.Run(tc.name, func(t *testing.T) {
			clus, err := e2e.NewEtcdProcessCluster(context.Background(), t, e2e.WithConfig(cfg))
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsGRPC(), tc.client)
			require.NoError(t, fillEtcdWithData(context.Background(), c, tc.dbSizeBytes))

			ctx, cancel := context.WithTimeout(context.Background(), watchTestDuration)
			defer cancel()
			g := errgroup.Group{}
			continuouslyExecuteGetAll(ctx, t, &g, c)
			validateWatchDelay(t, c.Watch(ctx, "fake-key", clientv3.WithProgressNotify()), tc.maxWatchDelay)
			require.NoError(t, g.Wait())
		})
	}
}

func TestWatchDelayForManualProgressNotification(t *testing.T) {
	e2e.BeforeTest(t)
	for _, tc := range tcs {
		tc := tc
		cfg := e2e.DefaultConfig()
		cfg.ClusterSize = 1
		cfg.Client = tc.client
		cfg.ClientHTTPSeparate = tc.clientHTTPSeparate
		t.Run(tc.name, func(t *testing.T) {
			clus, err := e2e.NewEtcdProcessCluster(context.Background(), t, e2e.WithConfig(cfg))
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsGRPC(), tc.client)
			require.NoError(t, fillEtcdWithData(context.Background(), c, tc.dbSizeBytes))

			ctx, cancel := context.WithTimeout(context.Background(), watchTestDuration)
			defer cancel()
			g := errgroup.Group{}
			continuouslyExecuteGetAll(ctx, t, &g, c)
			g.Go(func() error {
				for {
					err := c.RequestProgress(ctx)
					if err != nil {
						if strings.Contains(err.Error(), "context deadline exceeded") {
							return nil
						}
						return err
					}
					time.Sleep(watchResponsePeriod)
				}
			})
			validateWatchDelay(t, c.Watch(ctx, "fake-key"), tc.maxWatchDelay)
			require.NoError(t, g.Wait())
		})
	}
}

func TestWatchDelayForEvent(t *testing.T) {
	e2e.BeforeTest(t)
	for _, tc := range tcs {
		tc := tc
		cfg := e2e.DefaultConfig()
		cfg.ClusterSize = 1
		cfg.Client = tc.client
		cfg.ClientHTTPSeparate = tc.clientHTTPSeparate
		t.Run(tc.name, func(t *testing.T) {
			clus, err := e2e.NewEtcdProcessCluster(context.Background(), t, e2e.WithConfig(cfg))
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsGRPC(), tc.client)
			require.NoError(t, fillEtcdWithData(context.Background(), c, tc.dbSizeBytes))

			ctx, cancel := context.WithTimeout(context.Background(), watchTestDuration)
			defer cancel()
			g := errgroup.Group{}
			g.Go(func() error {
				i := 0
				for {
					_, err := c.Put(ctx, "key", fmt.Sprintf("%d", i))
					if err != nil {
						if strings.Contains(err.Error(), "context deadline exceeded") {
							return nil
						}
						return err
					}
					time.Sleep(watchResponsePeriod)
				}
			})
			continuouslyExecuteGetAll(ctx, t, &g, c)
			validateWatchDelay(t, c.Watch(ctx, "key"), tc.maxWatchDelay)
			require.NoError(t, g.Wait())
		})
	}
}

func validateWatchDelay(t *testing.T, watch clientv3.WatchChan, maxWatchDelay time.Duration) {
	start := time.Now()
	var maxDelay time.Duration
	for range watch {
		sinceLast := time.Since(start)
		if sinceLast > watchResponsePeriod+maxWatchDelay {
			t.Errorf("Unexpected watch response delayed over allowed threshold %s, delay: %s", maxWatchDelay, sinceLast-watchResponsePeriod)
		} else {
			t.Logf("Got watch response, since last: %s", sinceLast)
		}
		if sinceLast > maxDelay {
			maxDelay = sinceLast
		}
		start = time.Now()
	}
	sinceLast := time.Since(start)
	if sinceLast > maxDelay && sinceLast > watchResponsePeriod+maxWatchDelay {
		t.Errorf("Unexpected watch response delayed over allowed threshold %s, delay: unknown", maxWatchDelay)
		t.Errorf("Test finished while in middle of delayed response, measured delay: %s", sinceLast-watchResponsePeriod)
		t.Logf("Please increase the test duration to measure delay")
	} else {
		t.Logf("Max delay: %s", maxDelay-watchResponsePeriod)
	}
}

func continuouslyExecuteGetAll(ctx context.Context, t *testing.T, g *errgroup.Group, c *clientv3.Client) {
	mux := sync.RWMutex{}
	size := 0
	for i := 0; i < readLoadConcurrency; i++ {
		g.Go(func() error {
			for {
				resp, err := c.Get(ctx, "", clientv3.WithPrefix())
				if err != nil {
					if strings.Contains(err.Error(), "context deadline exceeded") {
						return nil
					}
					return err
				}
				respSize := 0
				for _, kv := range resp.Kvs {
					respSize += kv.Size()
				}
				mux.Lock()
				size += respSize
				mux.Unlock()
			}
		})
	}
	g.Go(func() error {
		lastSize := size
		for range time.Tick(time.Second) {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			mux.RLock()
			t.Logf("Generating read load around %.1f MB/s", float64(size-lastSize)/1000/1000)
			lastSize = size
			mux.RUnlock()
		}
		return nil
	})
}

// TestDeleteEventDrop_Issue18089 is an e2e test to reproduce the issue reported in: https://github.com/etcd-io/etcd/issues/18089
//
// The goal is to reproduce a DELETE event being dropped in a watch after a compaction
// occurs on the revision where the deletion took place. In order to reproduce this, we
// perform the following sequence (steps for reproduction thanks to @ahrtr):
//   - PUT k v2 (assume returned revision = r2)
//   - PUT k v3 (assume returned revision = r3)
//   - PUT k v4 (assume returned revision = r4)
//   - DELETE k (assume returned revision = r5)
//   - PUT k v6 (assume returned revision = r6)
//   - COMPACT r5
//   - WATCH rev=r5
//
// We should get the DELETE event (r5) followed by the PUT event (r6). However, currently we only
// get the PUT event with returned revision of r6 (key=k, val=v6).
func TestDeleteEventDrop_Issue18089(t *testing.T) {
	e2e.BeforeTest(t)
	cfg := e2e.DefaultConfig()
	cfg.ClusterSize = 1
	cfg.Client = e2e.ClientConfig{ConnectionType: e2e.ClientTLS}
	clus, err := e2e.NewEtcdProcessCluster(context.Background(), t, e2e.WithConfig(cfg))
	require.NoError(t, err)
	defer clus.Close()

	c := newClient(t, clus.EndpointsGRPC(), cfg.Client)
	defer c.Close()

	ctx := context.Background()
	const (
		key = "k"
		v2  = "v2"
		v3  = "v3"
		v4  = "v4"
		v6  = "v6"
	)

	t.Logf("PUT key=%s, val=%s", key, v2)
	_, err = c.KV.Put(ctx, key, v2)
	require.NoError(t, err)

	t.Logf("PUT key=%s, val=%s", key, v3)
	_, err = c.KV.Put(ctx, key, v3)
	require.NoError(t, err)

	t.Logf("PUT key=%s, val=%s", key, v4)
	_, err = c.KV.Put(ctx, key, v4)
	require.NoError(t, err)

	t.Logf("DELTE key=%s", key)
	deleteResp, err := c.KV.Delete(ctx, key)
	require.NoError(t, err)

	t.Logf("PUT key=%s, val=%s", key, v6)
	_, err = c.KV.Put(ctx, key, v6)
	require.NoError(t, err)

	t.Logf("COMPACT rev=%d", deleteResp.Header.Revision)
	_, err = c.KV.Compact(ctx, deleteResp.Header.Revision, clientv3.WithCompactPhysical())
	require.NoError(t, err)

	watchChan := c.Watch(ctx, key, clientv3.WithRev(deleteResp.Header.Revision))
	select {
	case watchResp := <-watchChan:
		// TODO(MadhavJivrajani): update conditions once https://github.com/etcd-io/etcd/issues/18089
		// is resolved. The existing conditions do not mimic the desired behaviour and are there to
		// test and reproduce etcd-io/etcd#18089.
		if len(watchResp.Events) != 1 {
			t.Fatalf("expected exactly one event in response, got: %d", len(watchResp.Events))
		}
		if watchResp.Events[0].Type != mvccpb.PUT {
			t.Fatalf("unexpected event type, expected: %s, got: %s", mvccpb.PUT, watchResp.Events[0].Type)
		}
		if string(watchResp.Events[0].Kv.Key) != key {
			t.Fatalf("unexpected key, expected: %s, got: %s", key, string(watchResp.Events[0].Kv.Key))
		}
		if string(watchResp.Events[0].Kv.Value) != v6 {
			t.Fatalf("unexpected valye, expected: %s, got: %s", v6, string(watchResp.Events[0].Kv.Value))
		}
	case <-time.After(100 * time.Millisecond):
		// we care only about the first response, but have an
		// escape hatch in case the watch response is delayed.
		t.Fatal("timed out getting watch response")
	}
}
