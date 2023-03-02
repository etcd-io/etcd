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

package common

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/stringutil"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"go.etcd.io/etcd/tests/v3/framework/interfaces"
	"golang.org/x/sync/errgroup"
)

const (
	watchResponsePeriod = 100 * time.Millisecond
	watchTestDuration   = 5 * time.Second
	maxWatchDelay       = 100 * time.Millisecond
	// Configuration for continuous read load on etcd
	numberOfPreexistingKeys = 100
	sizeOfPreexistingValues = 10000
	readLoadConcurrency     = 10
)

func TestWatchDelayForPeriodicProgressNotification(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		tc := tc
		tc.config.WatchProgressNotifyInterval = watchResponsePeriod
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(context.Background(), t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			c := client(t, clus, tc.config)
			require.NoError(t, fillEtcdWithData(context.Background(), c, numberOfPreexistingKeys, sizeOfPreexistingValues))

			ctx, cancel := context.WithTimeout(context.Background(), watchTestDuration)
			defer cancel()
			g := errgroup.Group{}
			continuouslyExecuteGetAll(ctx, t, &g, c)
			validateWatchDelay(t, c.Watch(ctx, "fake-key", clientv3.WithProgressNotify()))
			require.NoError(t, g.Wait())
		})
	}
}

func TestWatchDelayForManualProgressNotification(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(context.Background(), t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			c := client(t, clus, tc.config)
			require.NoError(t, fillEtcdWithData(context.Background(), c, numberOfPreexistingKeys, sizeOfPreexistingValues))

			ctx, cancel := context.WithTimeout(context.Background(), watchTestDuration)
			defer cancel()
			g := errgroup.Group{}
			continuouslyExecuteGetAll(ctx, t, &g, c)
			g.Go(func() error {
				for {
					err := c.RequestProgress(ctx)
					if err != nil {
						// Cannot use error.Is(err, context.DeadlineExceeded) as GRPC can wrap it like status.New(status.Unknown, context.DeadlineExceeded)
						if strings.Contains(err.Error(), "context deadline exceeded") {
							return nil
						} else {
							return err
						}
					}
					time.Sleep(watchResponsePeriod)
				}
			})
			validateWatchDelay(t, c.Watch(ctx, "fake-key"))
			require.NoError(t, g.Wait())
		})
	}
}

func TestWatchDelayForEvent(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(context.Background(), t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			c := client(t, clus, tc.config)
			require.NoError(t, fillEtcdWithData(context.Background(), c, numberOfPreexistingKeys, sizeOfPreexistingValues))

			ctx, cancel := context.WithTimeout(context.Background(), watchTestDuration)
			defer cancel()
			g := errgroup.Group{}
			g.Go(func() error {
				i := 0
				for {
					_, err := c.Put(ctx, "key", fmt.Sprintf("%d", i))
					if err != nil {
						// Cannot use error.Is(err, context.DeadlineExceeded) as GRPC can wrap it like status.New(status.Unknown, context.DeadlineExceeded)
						if strings.Contains(err.Error(), "context deadline exceeded") {
							return nil
						} else {
							return err
						}
					}
					time.Sleep(watchResponsePeriod)
				}
			})
			continuouslyExecuteGetAll(ctx, t, &g, c)
			validateWatchDelay(t, c.Watch(ctx, "key"))
			require.NoError(t, g.Wait())
		})
	}
}

func validateWatchDelay(t *testing.T, watch clientv3.WatchChan) {
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

func fillEtcdWithData(ctx context.Context, c *clientv3.Client, keyCount int, valueSize uint) error {
	g := errgroup.Group{}
	concurrency := 10
	keysPerRoutine := keyCount / concurrency
	for i := 0; i < concurrency; i++ {
		i := i
		g.Go(func() error {
			for j := 0; j < keysPerRoutine; j++ {
				_, err := c.Put(ctx, fmt.Sprintf("%d", i*keysPerRoutine+j), stringutil.RandString(valueSize))
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	return g.Wait()
}

func continuouslyExecuteGetAll(ctx context.Context, t *testing.T, g *errgroup.Group, c *clientv3.Client) {
	mux := sync.RWMutex{}
	size := 0
	for i := 0; i < readLoadConcurrency; i++ {
		g.Go(func() error {
			for {
				_, err := c.Get(ctx, "", clientv3.WithPrefix())
				if err != nil {
					// Cannot use error.Is(err, context.DeadlineExceeded) as GRPC can wrap it like status.New(status.Unknown, context.DeadlineExceeded)
					if strings.Contains(err.Error(), "context deadline exceeded") {
						return nil
					} else {
						return err
					}
				}
				mux.Lock()
				size += numberOfPreexistingKeys * sizeOfPreexistingValues
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

func client(t *testing.T, clus interfaces.Cluster, cfg config.ClusterConfig) *clientv3.Client {
	tls, err := integration.TlsInfo(t, cfg.ClientTLS)
	if err != nil {
		t.Fatal(err)
	}
	c, err := integration.Client(clus.Endpoints(), tls)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Close()
	})
	return c
}
