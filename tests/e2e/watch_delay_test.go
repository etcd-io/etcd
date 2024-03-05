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

var watchKeyPrefix = "/registry/pods/"

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

func TestWatchDelayOnStreamMultiplex(t *testing.T) {
	e2e.BeforeTest(t)
	clus, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{ClusterSize: 1, LogLevel: "info"})
	require.NoError(t, err)
	defer clus.Close()
	endpoints := clus.EndpointsV3()
	c := newClient(t, endpoints, e2e.ClientNonTLS, false)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g := errgroup.Group{}

	commonWatchOpts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		clientv3.WithPrevKV(),
	}

	watchCacheEventsReceived := atomic.Int64{}
	watchCacheWatchInitialized := make(chan struct{})
	watchCacheWatcherErrCompacted := atomic.Bool{}
	watchCacheWatcherErrCompacted.Store(false)
	g.Go(func() error {
		// simulate watch cache
		lg := zaptest.NewLogger(t).Named("watch-cache")
		defer func() { lg.Debug("watcher exited") }()
		gresp, err := c.Get(rootCtx, "foo")
		if err != nil {
			panic(err)
		}
		rev := gresp.Header.Revision
		watchCacheWatchOpts := append([]clientv3.OpOption{clientv3.WithCreatedNotify(), clientv3.WithRev(rev), clientv3.WithProgressNotify()}, commonWatchOpts...)
		//lastTimeGotResponse := time.Now()
		for wres := range c.Watch(rootCtx, watchKeyPrefix, watchCacheWatchOpts...) {
			if wres.Err() != nil {
				lg.Warn("got watch response error", zap.String("error", wres.Err().Error()), zap.Int64("compact-revision", wres.CompactRevision))
				watchCacheWatcherErrCompacted.Store(true)
				return nil
			}
			if wres.Created {
				close(watchCacheWatchInitialized)
			}
			watchCacheEventsReceived.Add(int64(len(wres.Events)))
			//elapsed := time.Since(lastTimeGotResponse)
			//if elapsed > 10*time.Second {
			//	handleWatchResponse(lg, &wres, false)
			//}
			handleWatchResponse(lg, &wres, false)
			//lastTimeGotResponse = time.Now()
		}
		return nil
	})
	<-watchCacheWatchInitialized

	var wg sync.WaitGroup
	numOfDirectWatches := 800
	for i := 0; i < numOfDirectWatches; i++ {
		wg.Add(1)
		lg := zaptest.NewLogger(t).Named(fmt.Sprintf("direct-watch-%d", i))
		g.Go(func() error {
			perDirectWatchContext, perDirectWatchCancelFn := context.WithCancel(rootCtx)
			retry := 0
			for {
				var watchOpts = append([]clientv3.OpOption{}, commonWatchOpts...)
				if retry == 0 {
					watchOpts = append(watchOpts, clientv3.WithCreatedNotify())
				}
				err := directWatch(perDirectWatchContext, lg, &wg, c, watchKeyPrefix, watchOpts)
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

	var clients []*clientv3.Client
	for ci := 0; ci < 1; ci++ {
		clients = append(clients, newClient(t, endpoints, e2e.ClientNonTLS, false))
	}
	generateLoad(loadCtx, clients, &g, &eventsTriggered)
	compaction(t, loadCtx, &g, c)
	compareEventsReceivedAndTriggered(t, cancel, loadCtx, &g, &watchCacheEventsReceived, &eventsTriggered, &watchCacheWatcherErrCompacted)
	require.NoError(t, g.Wait())
}

func directWatch(ctx context.Context, lg *zap.Logger, wg *sync.WaitGroup, c *clientv3.Client, keyPrefix string, watchOpts []clientv3.OpOption) error {
	wch := c.Watch(ctx, keyPrefix, watchOpts...)
	for wres := range wch {
		if wres.Err() != nil {
			return wres.Err()
		}
		if wres.Created {
			wg.Done()
		}
		handleWatchResponse(lg, &wres, true)
	}
	return nil
}

func handleWatchResponse(lg *zap.Logger, wres *clientv3.WatchResponse, suppressLogging bool) {
	if !suppressLogging {
		switch {
		case wres.Created:
			lg.Info("got watch created notification", zap.Int64("revision", wres.Header.Revision))
		case len(wres.Events) == 0:
			lg.Info("got progress notify watch response", zap.Int64("revision", wres.Header.Revision))
		case wres.Canceled:
			lg.Warn("got watch cancelled")
		default:
			for _, ev := range wres.Events {
				lg.Info("got watch response", zap.String("event-type", ev.Type.String()), zap.ByteString("key", ev.Kv.Key), zap.Int64("rev", ev.Kv.ModRevision))
			}
		}
	}
}

func generateLoad(ctx context.Context, clients []*clientv3.Client, group *errgroup.Group, counter *atomic.Int64) {
	numOfUpdater := 200
	keyValueSize := 1000
	keyValueSizeUpperLimit := 1200
	for _, c := range clients {
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
}

func compaction(t *testing.T, ctx context.Context, group *errgroup.Group, c *clientv3.Client) {
	group.Go(func() error {
		lg := zaptest.NewLogger(t).Named("compaction")
		lastCompactRev := int64(-1)
		ticker := time.NewTicker(20 * time.Second)
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
				panic(err)
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
	watchCacheWatcherErrCompacted *atomic.Bool,
) {
	group.Go(func() error {
		// cancel all the watchers.
		defer rootCtxCancel()
		lg := zaptest.NewLogger(t).Named("compareEvents")
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		var once sync.Once
		timer := time.NewTimer(2 * time.Hour)
		defer timer.Stop()
		for {
			// block until traffic is done.
			select {
			case <-loadCtx.Done():
				once.Do(func() { lg.Info("load generator context is done") })
			}

			// in case watch cache watcher channel is closed, reproduce the lost events with another retry
			if watchCacheWatcherErrCompacted.Load() {
				lg.Debug("watch cache watcher got compacted error, please retry the reproduce")
				return fmt.Errorf("watch cache watcher got compacted error, please retry the reproduce")
			}

			select {
			case <-ticker.C:
			case <-timer.C:
				triggered := eventsTriggered.Load()
				received := watchCacheEventsReceived.Load()
				return fmt.Errorf("2 hours passed since load generation is done, watch cache lost event detected; "+
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
