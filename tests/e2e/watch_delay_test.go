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
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sync/errgroup"
)

const (
	watchResponsePeriod = 100 * time.Millisecond
	watchTestDuration   = 5 * time.Second
	readLoadConcurrency = 10
)

type testCase struct {
	name          string
	config        etcdProcessClusterConfig
	maxWatchDelay time.Duration
	dbSizeBytes   int
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
		config:        etcdProcessClusterConfig{clusterSize: 1},
		maxWatchDelay: 100 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
	{
		name:          "TLS",
		config:        etcdProcessClusterConfig{clusterSize: 1, isClientAutoTLS: true, clientTLS: clientTLS},
		maxWatchDelay: 2 * time.Second,
		dbSizeBytes:   500 * Kilo,
	},
	{
		name:          "SeparateHttpNoTLS",
		config:        etcdProcessClusterConfig{clusterSize: 1, clientHttpSeparate: true},
		maxWatchDelay: 100 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
	{
		name:          "SeparateHttpTLS",
		config:        etcdProcessClusterConfig{clusterSize: 1, isClientAutoTLS: true, clientTLS: clientTLS, clientHttpSeparate: true},
		maxWatchDelay: 100 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
}

func TestWatchDelayForPeriodicProgressNotification(t *testing.T) {
	BeforeTest(t)
	for _, tc := range tcs {
		tc := tc
		tc.config.WatchProcessNotifyInterval = watchResponsePeriod
		t.Run(tc.name, func(t *testing.T) {
			clus, err := newEtcdProcessCluster(t, &tc.config)
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsV3(), tc.config.clientTLS, tc.config.isClientAutoTLS)
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
	BeforeTest(t)
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus, err := newEtcdProcessCluster(t, &tc.config)
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsV3(), tc.config.clientTLS, tc.config.isClientAutoTLS)
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
	BeforeTest(t)
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus, err := newEtcdProcessCluster(t, &tc.config)
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsV3(), tc.config.clientTLS, tc.config.isClientAutoTLS)
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
