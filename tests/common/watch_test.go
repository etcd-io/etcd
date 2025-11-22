// Copyright 2022 The etcd Authors
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"

	e2e_utils "go.etcd.io/etcd/tests/v3/e2e"
)

func TestWatch(t *testing.T) {
	testRunner.BeforeTest(t)
	watchTimeout := 1 * time.Second
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))

			defer clus.Close()
			cc := testutils.MustClient(clus.Client())
			testutils.ExecuteUntil(ctx, t, func() {
				tests := []struct {
					puts     []testutils.KV
					watchKey string
					opts     config.WatchOptions
					wanted   []testutils.KV
				}{
					{ // watch by revision
						puts:     []testutils.KV{{Key: "bar", Val: "revision_1"}, {Key: "bar", Val: "revision_2"}, {Key: "bar", Val: "revision_3"}},
						watchKey: "bar",
						opts:     config.WatchOptions{Revision: 3},
						wanted:   []testutils.KV{{Key: "bar", Val: "revision_2"}, {Key: "bar", Val: "revision_3"}},
					},
					{ // watch 1 key
						puts:     []testutils.KV{{Key: "sample", Val: "value"}},
						watchKey: "sample",
						opts:     config.WatchOptions{Revision: 1},
						wanted:   []testutils.KV{{Key: "sample", Val: "value"}},
					},
					{ // watch 3 keys by prefix
						puts:     []testutils.KV{{Key: "foo1", Val: "val1"}, {Key: "foo2", Val: "val2"}, {Key: "foo3", Val: "val3"}},
						watchKey: "foo",
						opts:     config.WatchOptions{Revision: 1, Prefix: true},
						wanted:   []testutils.KV{{Key: "foo1", Val: "val1"}, {Key: "foo2", Val: "val2"}, {Key: "foo3", Val: "val3"}},
					},
					{ // watch 3 keys by range
						puts:     []testutils.KV{{Key: "key1", Val: "val1"}, {Key: "key3", Val: "val3"}, {Key: "key2", Val: "val2"}},
						watchKey: "key",
						opts:     config.WatchOptions{Revision: 1, RangeEnd: "key3"},
						wanted:   []testutils.KV{{Key: "key1", Val: "val1"}, {Key: "key2", Val: "val2"}},
					},
				}

				for _, tt := range tests {
					wCtx, wCancel := context.WithCancel(ctx)
					wch := cc.Watch(wCtx, tt.watchKey, tt.opts)
					require.NotNilf(t, wch, "failed to watch %s", tt.watchKey)

					for j := range tt.puts {
						err := cc.Put(ctx, tt.puts[j].Key, tt.puts[j].Val, config.PutOptions{})
						require.NoErrorf(t, err, "can't not put key %q, err: %s", tt.puts[j].Key, err)
					}

					kvs, err := testutils.KeyValuesFromWatchChan(wch, len(tt.wanted), watchTimeout)
					if err != nil {
						wCancel()
						require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
					}
					fmt.Println("kvs: ", kvs)

					wCancel()
					assert.Equal(t, tt.wanted, kvs)
				}
			})
		})
	}
}

func TestWatchDelayForPeriodicProgressNotification(t *testing.T) {
	testRunner.BeforeTest(t)
	watchResponsePeriod := 100 * time.Millisecond
	watchTestDuration := 5 * time.Second
	dbSizeBytes := 5 * 1000 * 1000

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()

			cfg := tc.config
			if cfg.ClusterContext == nil {
				cfg.ClusterContext = &e2e.ClusterContext{}
			}
			cfg.ClusterContext.(*e2e.ClusterContext).ServerWatchProgressNotifyInterval = watchResponsePeriod

			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(cfg))
			defer clus.Close()

			cc := testutils.MustClient(clus.Client())

			wCtx, cancel := context.WithTimeout(t.Context(), watchTestDuration)
			defer cancel()
			require.NoError(t, e2e_utils.FillEtcdWithData(ctx, cc, dbSizeBytes))

			g := errgroup.Group{}

			wch := cc.Watch(wCtx, "fake-key", config.WatchOptions{ProgressNotify: true})
			require.NotNil(t, wch)

			e2e_utils.ContinuouslyExecuteGetAll(wCtx, t, &g, cc)

			e2e_utils.ValidateWatchDelay(t, wch, 150*time.Millisecond)
			require.NoError(t, g.Wait())
		})
	}
}

func TestWatchDelayForEvent(t *testing.T) {
	e2e.BeforeTest(t)
	watchResponsePeriod := 100 * time.Millisecond
	watchTestDuration := 5 * time.Second
	dbSizeBytes := 5 * 1000 * 1000

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()

			cfg := tc.config
			if cfg.ClusterContext == nil {
				cfg.ClusterContext = &e2e.ClusterContext{}
			}
			cfg.ClusterContext.(*e2e.ClusterContext).ServerWatchProgressNotifyInterval = watchResponsePeriod

			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(cfg))
			defer clus.Close()

			cc := testutils.MustClient(clus.Client())

			wCtx, cancel := context.WithTimeout(t.Context(), watchTestDuration)
			defer cancel()
			require.NoError(t, e2e_utils.FillEtcdWithData(ctx, cc, dbSizeBytes))

			g := errgroup.Group{}
			g.Go(func() error {
				i := 0
				for {
					err := cc.Put(ctx, "key", fmt.Sprintf("%d", i), config.PutOptions{})
					if err != nil {
						if strings.Contains(err.Error(), "context deadline exceeded") {
							return nil
						}
						return err
					}
					time.Sleep(watchResponsePeriod)
				}
			})

			wch := cc.Watch(wCtx, "key", config.WatchOptions{})
			require.NotNil(t, wch)

			e2e_utils.ContinuouslyExecuteGetAll(wCtx, t, &g, cc)

			e2e_utils.ValidateWatchDelay(t, wch, 150*time.Millisecond)
			require.NoError(t, g.Wait())
		})
	}
}
