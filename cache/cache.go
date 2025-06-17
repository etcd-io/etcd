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
	"sort"
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
	etcd               *clientv3.Client
	hist               History
	mux                *demux
	ctx                context.Context
	cancel             context.CancelFunc
	latestCompactedRev int64         // updated atomically
	ready              chan struct{} // closed when initialLoad finishes
}

// NewWithPrefix builds a cache shard that watches only the requested prefix.
// For the root cache pass "".
func NewWithPrefix(cli *clientv3.Client, prefix string, opts ...Option) (*Cache, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// choose history implementation
	ringBuffer := newRingBuffer(cfg.BufferEntries)

	cache := &Cache{
		prefix: prefix,
		cfg:    cfg,
		etcd:   cli,
		hist:   ringBuffer,
		mux:    newDemux(),
		ctx:    ctx,
		cancel: cancel,
		ready:  make(chan struct{}),
	}

	// initial chatch up
	if err := cache.initialLoad(); err != nil {
		cancel()
		return nil, err
	}
	close(cache.ready)

	// upstream watch goroutine
	serveWatchEvents(
		ctx,
		cli,
		prefix,
		cache.mux.broadcast,
		cache.mux,
		cache.hist,
		func(rev int64) { atomic.StoreInt64(&cache.latestCompactedRev, rev) },
		cfg.InitialBackoff,
		cfg.MaxBackoff,
	)

	return cache, nil
}

// initialLoad ranges over the prefix and seeds the ring.
func (c *Cache) initialLoad() error {
	opts := []clientv3.OpOption{clientv3.WithPrefix()}
	resp, err := c.etcd.Get(c.ctx, c.prefix, opts...)
	if err != nil {
		return err
	}
	for _, kv := range resp.Kvs {
		event := &clientv3.Event{
			Type: clientv3.EventTypePut,
			Kv:   kv,
		}
		c.hist.Append(event)
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

	op := clientv3.OpGet(key, opts...)
	requestedRev := op.Rev()

	responseChan := make(chan clientv3.WatchResponse, 1)

	// compaction / gap checks
	if compactedRev := atomic.LoadInt64(&c.latestCompactedRev); requestedRev > 0 && requestedRev <= compactedRev {
		responseChan <- clientv3.WatchResponse{CompactRevision: compactedRev}
		close(responseChan)
		return responseChan
	}
	if requestedRev > 0 && requestedRev < c.hist.OldestRevision() {
		responseChan <- clientv3.WatchResponse{CompactRevision: c.hist.OldestRevision()}
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

	replay := c.hist.GetSince(startRev, pred)
	ch := make(chan *clientv3.Event, c.cfg.ChannelSize)
	for _, ev := range replay {
		select {
		case ch <- ev:
		case <-ctx.Done():
			close(ch)
			return nil, ctx.Err()
		}
	}

	w := &watcher{
		eventChan:     ch,
		predicate:     pred,
		lastRev:       c.hist.LatestRevision(),
		dropThreshold: c.cfg.DropThreshold,
	}
	c.mux.add(w)

	go func() {
		<-ctx.Done()
		c.mux.remove(w)
	}()
	return &Stream{Ch: ch}, nil
}

// Close shuts down the shard.
func (c *Cache) Close() { c.cancel() }

// ----------------------- SHARDED CACHE --------------------------------------

// ShardedCache isolates per-prefix caches, preventing one prefix from evicting others.
type ShardedCache struct {
	shards   map[string]*Cache // by prefix
	prefixes []string          // deterministic order, longest first
}

// NewShardedCache makes a shard per prefix; "" is the catch-all root shard.
func NewShardedCache(cli *clientv3.Client, prefixes []string, opts ...Option) (*ShardedCache, error) {
	if len(prefixes) == 0 {
		prefixes = []string{""}
	}
	// sort by descending length so "/foo/bar" wins over "/foo"
	sort.Slice(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })

	shards := make(map[string]*Cache, len(prefixes))
	for _, p := range prefixes {
		shard, err := NewWithPrefix(cli, p, opts...)
		if err != nil {
			// close any earlier shards
			for _, s := range shards {
				s.Close()
			}
			return nil, err
		}
		shards[p] = shard
	}
	return &ShardedCache{shards: shards, prefixes: prefixes}, nil
}

// pickShard returns the cache whose prefix is the longest match for key.
func (s *ShardedCache) pickShard(key string) *Cache {
	for _, p := range s.prefixes {
		if strings.HasPrefix(key, p) {
			return s.shards[p]
		}
	}
	return s.shards[""]
}

// Watch selects a shard based on key and delegates.
func (sc *ShardedCache) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	shard := sc.pickShard(key)
	return shard.Watch(ctx, key, opts...)
}

// Close stops all shards.
func (sc *ShardedCache) Close() {
	for _, shard := range sc.shards {
		shard.Close()
	}
}

// --- helpers exposed for tests ------------------------------------------------

func (c *Cache) OldestRev() int64 { return c.hist.OldestRevision() }
