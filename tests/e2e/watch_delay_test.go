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
	"errors"
	"fmt"
	"math/rand"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	v3rpc "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	watchResponsePeriod = 100 * time.Millisecond
	watchTestDuration   = 5 * time.Second
	readLoadConcurrency = 10
)

type testCase struct {
	name          string
	config        e2e.EtcdProcessClusterConfig
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
		config:        e2e.EtcdProcessClusterConfig{ClusterSize: 1},
		maxWatchDelay: 150 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
	{
		name:          "TLS",
		config:        e2e.EtcdProcessClusterConfig{ClusterSize: 1, IsClientAutoTLS: true, ClientTLS: e2e.ClientTLS},
		maxWatchDelay: 150 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
	{
		name:          "SeparateHttpNoTLS",
		config:        e2e.EtcdProcessClusterConfig{ClusterSize: 1, ClientHttpSeparate: true},
		maxWatchDelay: 150 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
	{
		name:          "SeparateHttpTLS",
		config:        e2e.EtcdProcessClusterConfig{ClusterSize: 1, IsClientAutoTLS: true, ClientTLS: e2e.ClientTLS, ClientHttpSeparate: true},
		maxWatchDelay: 150 * time.Millisecond,
		dbSizeBytes:   5 * Mega,
	},
}

func TestWatchDelayForPeriodicProgressNotification(t *testing.T) {
	e2e.BeforeTest(t)
	for _, tc := range tcs {
		tc := tc
		tc.config.WatchProcessNotifyInterval = watchResponsePeriod
		t.Run(tc.name, func(t *testing.T) {
			clus, err := e2e.NewEtcdProcessCluster(t, &tc.config)
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsV3(), tc.config.ClientTLS, tc.config.IsClientAutoTLS)
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
		t.Run(tc.name, func(t *testing.T) {
			clus, err := e2e.NewEtcdProcessCluster(t, &tc.config)
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsV3(), tc.config.ClientTLS, tc.config.IsClientAutoTLS)
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
		t.Run(tc.name, func(t *testing.T) {
			clus, err := e2e.NewEtcdProcessCluster(t, &tc.config)
			require.NoError(t, err)
			defer clus.Close()
			c := newClient(t, clus.EndpointsV3(), tc.config.ClientTLS, tc.config.IsClientAutoTLS)
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

// TestWatchOnStreamMultiplex ensures slow etcd watchers throws terminal ErrCompacted error if its
// next batch of events to be sent are compacted.
func TestWatchOnStreamMultiplex(t *testing.T) {
	e2e.BeforeTest(t)
	clus, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{ClusterSize: 1, LogLevel: "info", EnablePprof: true})
	require.NoError(t, err)
	defer clus.Close()
	endpoints := clus.EndpointsV3()
	c := newClient(t, endpoints, e2e.ClientNonTLS, false)
	rootCtx, rootCtxCancel := context.WithCancel(context.Background())
	defer rootCtxCancel()

	g := errgroup.Group{}
	watchKeyPrefix := "/registry/pods/"
	commonWatchOpts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		clientv3.WithPrevKV(),
	}

	watchCacheEventsReceived := atomic.Int64{}
	watchCacheWatchInitialized := make(chan struct{})
	watchCacheWatcherExited := make(chan struct{})
	g.Go(func() error {
		// simulate watch cache
		lg := zaptest.NewLogger(t).Named("watch-cache")
		defer func() { lg.Debug("watcher exited") }()
		gresp, err := c.Get(rootCtx, "foo")
		if err != nil {
			panic(err)
		}
		rev := gresp.Header.Revision

		lastEventModifiedRevision := rev
		watchCacheWatchOpts := append([]clientv3.OpOption{clientv3.WithCreatedNotify(), clientv3.WithRev(rev), clientv3.WithProgressNotify()}, commonWatchOpts...)
		for wres := range c.Watch(rootCtx, watchKeyPrefix, watchCacheWatchOpts...) {
			if wres.Err() != nil {
				lg.Warn("got watch response error",
					zap.Int64("last-received-events-kv-mod-revision", lastEventModifiedRevision),
					zap.Int64("compact-revision", wres.CompactRevision),
					zap.String("error", wres.Err().Error()))
				close(watchCacheWatcherExited)
				return nil
			}
			if wres.Created {
				close(watchCacheWatchInitialized)
			}
			watchCacheEventsReceived.Add(int64(len(wres.Events)))
			for _, ev := range wres.Events {
				if ev.Kv.ModRevision != lastEventModifiedRevision+1 {
					close(watchCacheWatcherExited)
					return fmt.Errorf("event loss detected; want rev %d but got rev %d", lastEventModifiedRevision+1, ev.Kv.ModRevision)
				}
				lastEventModifiedRevision = ev.Kv.ModRevision
			}
		}
		return nil
	})
	<-watchCacheWatchInitialized

	var wg sync.WaitGroup
	numOfDirectWatches := 800
	for i := 0; i < numOfDirectWatches; i++ {
		wg.Add(1)
		g.Go(func() error {
			perDirectWatchContext, perDirectWatchCancelFn := context.WithCancel(rootCtx)
			retry := 0
			for {
				watchOpts := append([]clientv3.OpOption{}, commonWatchOpts...)
				if retry == 0 {
					watchOpts = append(watchOpts, clientv3.WithCreatedNotify())
				}
				err := directWatch(perDirectWatchContext, &wg, c, watchKeyPrefix, watchOpts)
				if errors.Is(err, v3rpc.ErrCompacted) {
					retry++
					continue
				}
				// if watch is cancelled by client or closed by server, we should exit
				perDirectWatchCancelFn()
				return nil
			}
		})
	}
	wg.Wait()

	eventsTriggered := atomic.Int64{}
	loadCtx, loadCtxCancel := context.WithTimeout(rootCtx, time.Minute)
	defer loadCtxCancel()
	generateLoad(loadCtx, c, watchKeyPrefix, &g, &eventsTriggered)
	compaction(t, loadCtx, &g, c)

	// validate whether watch cache watcher is compacted or get all the events.
	compareEventsReceivedAndTriggered(t, rootCtxCancel, loadCtx, &g, &watchCacheEventsReceived, &eventsTriggered, watchCacheWatcherExited)
	require.NoError(t, g.Wait())
}

func directWatch(ctx context.Context, wg *sync.WaitGroup, c *clientv3.Client, keyPrefix string, watchOpts []clientv3.OpOption) error {
	wch := c.Watch(ctx, keyPrefix, watchOpts...)
	for wres := range wch {
		if wres.Err() != nil {
			return wres.Err()
		}
		if wres.Created {
			wg.Done()
		}
	}
	return nil
}

func generateLoad(ctx context.Context, c *clientv3.Client, watchKeyPrefix string, group *errgroup.Group, counter *atomic.Int64) {
	numOfUpdater := 200
	keyValueSize := 1000
	keyValueSizeUpperLimit := 1200
	for i := 0; i < numOfUpdater; i++ {
		writeKeyPrefix := path.Join(watchKeyPrefix, fmt.Sprintf("%d", i))
		group.Go(func() error {
			count := 0
			keyValuePayload := randomStringMaker.makeString(keyValueSize, keyValueSizeUpperLimit)
			for {
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				count++
				key := path.Join(writeKeyPrefix, fmt.Sprintf("%d", count))
				if _, err := c.Put(ctx, key, keyValuePayload); err == nil {
					counter.Add(1)
				}
				if _, err := c.Delete(ctx, key); err == nil {
					counter.Add(1)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

type randomStringAlphabet string

func (a randomStringAlphabet) makeString(minLen, maxLen int) string {
	n := minLen
	if minLen < maxLen {
		n += rand.Intn(maxLen - minLen)
	}
	var s string
	for i := 0; i < n; i++ {
		s += string(a[rand.Intn(len(a))])
	}
	return s
}

var randomStringMaker = randomStringAlphabet("abcdefghijklmnopqrstuvwxyz0123456789")

func compaction(t *testing.T, ctx context.Context, group *errgroup.Group, c *clientv3.Client) {
	group.Go(func() error {
		lg := zaptest.NewLogger(t).Named("compaction")
		lastCompactRev := int64(-1)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				lg.Warn("context deadline exceeded, exit compaction routine")
				return nil
			case <-ticker.C:
			}
			if lastCompactRev < 0 {
				gresp, err := c.Get(ctx, "foo")
				if err != nil {
					panic(err)
				}
				lastCompactRev = gresp.Header.Revision
				continue
			}
			cres, err := c.Compact(ctx, lastCompactRev, clientv3.WithCompactPhysical())
			if err != nil {
				lg.Warn("failed to compact", zap.Error(err))
				continue
			}
			lg.Debug("compacted rev", zap.Int64("compact-revision", lastCompactRev))
			lastCompactRev = cres.Header.Revision
		}
	})
}

func compareEventsReceivedAndTriggered(
	t *testing.T,
	rootCtxCancel context.CancelFunc,
	loadCtx context.Context,
	group *errgroup.Group,
	watchCacheEventsReceived *atomic.Int64,
	eventsTriggered *atomic.Int64,
	watchCacheWatcherExited <-chan struct{},
) {
	group.Go(func() error {
		defer rootCtxCancel() // cancel all the watchers and load.

		lg := zaptest.NewLogger(t).Named("compareEvents")
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		timer := time.NewTimer(2 * time.Minute)
		defer timer.Stop()

		var once sync.Once
		for {
			// block until traffic is done.
			select {
			case <-loadCtx.Done():
				once.Do(func() { lg.Info("load generator context is done") })
			case <-watchCacheWatcherExited:
				// watch cache watcher channel is expected to be closed with compacted error
				// then there is no need to wait for load to verify watch cache receives all the events.
				return nil
			}

			select {
			case <-ticker.C:
			case <-timer.C:
				triggered := eventsTriggered.Load()
				received := watchCacheEventsReceived.Load()
				return fmt.Errorf("5 minutes passed since load generation is done, watch cache lost event detected; "+
					"watch evetns received %d, received %d", received, triggered)
			}
			triggered := eventsTriggered.Load()
			received := watchCacheEventsReceived.Load()
			if received >= triggered {
				lg.Info("The number of events watch cache received is high than or equal to events triggered on client side",
					zap.Int64("watch-cache-received", received),
					zap.Int64("traffic-triggered", triggered))
				return nil
			}
			lg.Warn("watch events received is lagging behind",
				zap.Int64("watch-events-received", received),
				zap.Int64("events-triggered", triggered))
		}
	})
}
