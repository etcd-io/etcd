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
		cfg.ServerConfig.WatchProgressNotifyInterval = watchResponsePeriod
		cfg.Client = tc.client
		cfg.ClientHTTPSeparate = tc.clientHTTPSeparate
		t.Run(tc.name, func(t *testing.T) {
			clus, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsGRPC(), tc.client)
			require.NoError(t, fillEtcdWithData(t.Context(), c, tc.dbSizeBytes))

			ctx, cancel := context.WithTimeout(t.Context(), watchTestDuration)
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
			clus, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsGRPC(), tc.client)
			require.NoError(t, fillEtcdWithData(t.Context(), c, tc.dbSizeBytes))

			ctx, cancel := context.WithTimeout(t.Context(), watchTestDuration)
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
			clus, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsGRPC(), tc.client)
			require.NoError(t, fillEtcdWithData(t.Context(), c, tc.dbSizeBytes))

			ctx, cancel := context.WithTimeout(t.Context(), watchTestDuration)
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

// TestResumeCompactionOnTombstone verifies whether a deletion event is preserved
// when etcd restarts and resumes compaction on a key that only has a tombstone revision.
func TestResumeCompactionOnTombstone(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := t.Context()
	compactBatchLimit := 5

	cfg := e2e.DefaultConfig()
	clus, err := e2e.NewEtcdProcessCluster(t.Context(),
		t,
		e2e.WithConfig(cfg),
		e2e.WithClusterSize(1),
		e2e.WithCompactionBatchLimit(compactBatchLimit),
		e2e.WithGoFailEnabled(true),
		e2e.WithWatchProcessNotifyInterval(100*time.Millisecond),
	)
	require.NoError(t, err)
	defer clus.Close()

	c1 := newClient(t, clus.EndpointsGRPC(), cfg.Client)
	defer c1.Close()

	keyPrefix := "/key-"
	for i := 0; i < compactBatchLimit; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		value := fmt.Sprintf("%d", i)

		t.Logf("PUT key=%s, val=%s", key, value)
		_, err = c1.KV.Put(ctx, key, value)
		require.NoError(t, err)
	}

	firstKey := keyPrefix + "0"
	t.Logf("DELETE key=%s", firstKey)
	deleteResp, err := c1.KV.Delete(ctx, firstKey)
	require.NoError(t, err)

	var deleteEvent *clientv3.Event
	select {
	case watchResp := <-c1.Watch(ctx, firstKey, clientv3.WithRev(deleteResp.Header.Revision)):
		require.Len(t, watchResp.Events, 1)

		require.Equal(t, mvccpb.DELETE, watchResp.Events[0].Type)
		deletedKey := string(watchResp.Events[0].Kv.Key)
		require.Equal(t, firstKey, deletedKey)

		deleteEvent = watchResp.Events[0]
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out getting watch response")
	}

	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "compactBeforeSetFinishedCompact", `panic`))

	t.Logf("COMPACT rev=%d", deleteResp.Header.Revision)
	_, err = c1.KV.Compact(ctx, deleteResp.Header.Revision, clientv3.WithCompactPhysical())
	require.Error(t, err)

	require.NoError(t, clus.Restart(ctx))

	c2 := newClient(t, clus.EndpointsGRPC(), cfg.Client)
	defer c2.Close()

	watchChan := c2.Watch(ctx, firstKey, clientv3.WithRev(deleteResp.Header.Revision))
	select {
	case watchResp := <-watchChan:
		require.Equal(t, []*clientv3.Event{deleteEvent}, watchResp.Events)
	case <-time.After(100 * time.Millisecond):
		// we care only about the first response, but have an
		// escape hatch in case the watch response is delayed.
		t.Fatal("timed out getting watch response")
	}
}
