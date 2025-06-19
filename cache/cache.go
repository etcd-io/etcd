// Copyright 2015 The etcd Authors
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

package cache

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync/atomic"

	rpctypes "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrCompacted = rpctypes.ErrCompacted

type Stream struct{ Ch <-chan *clientv3.Event }

// Cache buffers a single etcd Watch for a given key‐prefix and fan‑outs local watchers.
// It is created internally by ShardedCache; callers typically use ShardedCache instead.
type Cache struct {
	prefix             string // keys this shard is responsible for, may be "" (root)
	cfg                Config
	client             *clientv3.Client
	history            History
	demux              *demux
	ctx                context.Context
	cancel             context.CancelFunc
	latestCompactedRev int64         // updated atomically
	ready              chan struct{} // closed when initialLoad finishes
}

// NewWithPrefix builds a cache shard that watches only the requested prefix.
// For the root cache pass "".
func NewWithPrefix(ctx context.Context, client *clientv3.Client, prefix string, opts ...Option) (*Cache, error) {

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.HistoryWindowSize <= 0 {
		return nil, fmt.Errorf("invalid HistoryWindowSize %d (must be > 0)", cfg.HistoryWindowSize)
	}

	ctx, cancel := context.WithCancel(ctx)

	// choose history implementation
	ringBuffer := newRingBuffer(cfg.HistoryWindowSize)

	cache := &Cache{
		prefix:  prefix,
		cfg:     cfg,
		client:  client,
		history: ringBuffer,
		demux:   newDemux(ringBuffer, cfg.ResyncInterval),
		ctx:     ctx,
		cancel:  cancel,
		ready:   make(chan struct{}),
	}

	go func() {
		// initial catch up
		if err := cache.initialLoad(); err != nil {
			log.Printf("cache initial load failed for prefix %q: %v",
				prefix, err)
		}
		close(cache.ready)

		// long-running upstream watch
		serveWatchEvents(
			ctx, client, prefix,
			cache.demux.broadcast,
			cache.demux,
			cache.history,
			func(rev int64) { atomic.StoreInt64(&cache.latestCompactedRev, rev) },
			cfg.InitialBackoff,
			cfg.MaxBackoff,
		)
	}()

	return cache, nil
}

// initialLoad ranges over the prefix and seeds the ring.
func (c *Cache) initialLoad() error {
	opts := []clientv3.OpOption{clientv3.WithPrefix()}
	resp, err := c.client.Get(c.ctx, c.prefix, opts...)
	if err != nil {
		return err
	}
	for _, kv := range resp.Kvs {
		event := &clientv3.Event{
			Type: clientv3.EventTypePut,
			Kv:   kv,
		}
		c.history.Append(event)
	}
	return nil
}

func (c *Cache) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	select {
	case <-c.ready:
	case <-ctx.Done():
		emptyWatchChan := make(chan clientv3.WatchResponse)
		close(emptyWatchChan)
		return emptyWatchChan
	}

	if !strings.HasPrefix(key, c.prefix) {
		// invalid key
		emptyWatchChan := make(chan clientv3.WatchResponse)
		close(emptyWatchChan)
		return emptyWatchChan
	}

	op := clientv3.OpGet(key, opts...)
	requestedRev := op.Rev()

	responseChan := make(chan clientv3.WatchResponse, 1)

	// compaction / gap checks
	if requestedRev > 0 && requestedRev < c.history.OldestRevision() {
		responseChan <- clientv3.WatchResponse{CompactRevision: c.history.OldestRevision()}
		close(responseChan)
		return responseChan
	}

	stream, err := c.newWatchStream(ctx, key, requestedRev)
	if err != nil {
		responseChan <- clientv3.WatchResponse{Canceled: true}
		close(responseChan)
		return responseChan
	}

	go func() {
		defer close(responseChan)
		for {
			select {
			case ev, ok := <-stream.Ch:
				if !ok {
					return
				}
				responseChan <- clientv3.WatchResponse{Events: []*clientv3.Event{ev}}
			case <-ctx.Done():
				return
			}
		}
	}()
	return responseChan
}

func (c *Cache) newWatchStream(ctx context.Context, key string, startRev int64) (*Stream, error) {
	pred := func(k []byte) bool { return strings.HasPrefix(string(k), key) }

	replay, last := c.history.GetSince(startRev, pred)
	w := newWatcher(c.cfg.PerWatcherBufferSize, pred, last)

	for _, ev := range replay {
		select {
		case w.eventChan <- ev:
		case <-ctx.Done():
			w.stop()
			return nil, ctx.Err()
		}
	}

	c.demux.add(w)
	w.autoRemoveOnCancel(ctx, c.demux)

	return &Stream{Ch: w.eventChan}, nil
}

// Close shuts down the shard.
func (c *Cache) Close() { c.cancel() }

// helper exposed for tests
func (c *Cache) OldestRev() int64 { return c.history.OldestRevision() }
