// Copyright 2025 The etcd Authors
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
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	rpctypes "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// TODO: add gap-free replay for arbitrary startRevs and drop this guard.
var ErrUnsupportedWatch = errors.New("cache: unsupported watch parameters")

// Cache buffers a single etcd Watch for a given key‐prefix and fan‑outs local watchers.
type Cache struct {
	prefix      string // prefix is the key-prefix this shard is responsible for ("" = root).
	cfg         Config // immutable runtime configuration
	watcher     clientv3.Watcher
	demux       *demux // demux fans incoming events out to active watchers and manages resync.
	ready       chan struct{}
	stop        context.CancelFunc
	waitGroup   sync.WaitGroup
	internalCtx context.Context
}

// watchCtx collects all the knobs that both serveWatchEvents and watchRetryLoop need.
type watchCtx struct {
	cache           *Cache
	backoffStart    time.Duration
	backoffMax      time.Duration
	onFirstResponse func() // callback to fire once on first upstream response
}

// New builds a cache shard that watches only the requested prefix.
// For the root cache pass "".
func New(watcher clientv3.Watcher, prefix string, opts ...Option) (*Cache, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.HistoryWindowSize <= 0 {
		return nil, fmt.Errorf("invalid HistoryWindowSize %d (must be > 0)", cfg.HistoryWindowSize)
	}

	internalCtx, cancel := context.WithCancel(context.Background())

	cache := &Cache{
		prefix:      prefix,
		cfg:         cfg,
		watcher:     watcher,
		ready:       make(chan struct{}),
		stop:        cancel,
		internalCtx: internalCtx,
	}

	cache.demux = NewDemux(internalCtx, &cache.waitGroup, cfg.HistoryWindowSize, cfg.ResyncInterval)

	cache.waitGroup.Add(1)
	go func() {
		defer cache.waitGroup.Done()
		readyOnce := sync.Once{}

		watchCtx := &watchCtx{
			cache:           cache,
			backoffStart:    cfg.InitialBackoff,
			backoffMax:      cfg.MaxBackoff,
			onFirstResponse: func() { readyOnce.Do(func() { close(cache.ready) }) },
		}
		serveWatchEvents(internalCtx, watchCtx)
	}()

	return cache, nil
}

// Watch registers a cache-backed watcher for a given key or prefix.
// It returns a WatchChan that streams WatchResponses containing events.
func (c *Cache) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	select {
	case <-c.ready:
	case <-ctx.Done():
		emptyWatchChan := make(chan clientv3.WatchResponse)
		close(emptyWatchChan)
		return emptyWatchChan
	}

	op := clientv3.OpWatch(key, opts...)
	startRev := op.Rev()

	if startRev != 0 {
		if oldest := c.demux.PeekOldest(); oldest != 0 && startRev < oldest {
			ch := make(chan clientv3.WatchResponse, 1)
			ch <- clientv3.WatchResponse{
				Canceled:        true,
				CompactRevision: startRev,
			}
			close(ch)
			return ch
		}
	}

	pred, err := c.validateWatch(key, op)
	if err != nil {
		ch := make(chan clientv3.WatchResponse, 1)
		ch <- clientv3.WatchResponse{Canceled: true, CancelReason: err.Error()}
		close(ch)
		return ch
	}

	w := newWatcher(c.cfg.PerWatcherBufferSize, pred)
	c.demux.Register(w, startRev)

	responseChan := make(chan clientv3.WatchResponse)
	c.waitGroup.Add(1)
	go func() {
		defer c.waitGroup.Done()
		defer close(responseChan)
		defer c.demux.Unregister(w)
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.internalCtx.Done():
				return
			case events, ok := <-w.eventQueue:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-c.internalCtx.Done():
					return
				case responseChan <- clientv3.WatchResponse{Events: events}:
				}
			}
		}
	}()
	return responseChan
}

// Ready reports whether the cache has finished its initial load.
func (c *Cache) Ready() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

// WaitReady blocks until the cache is ready or the ctx is cancelled.
func (c *Cache) WaitReady(ctx context.Context) error {
	select {
	case <-c.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close cancels the private context and blocks until all goroutines return.
func (c *Cache) Close() {
	c.stop()
	c.waitGroup.Wait()
}

func serveWatchEvents(ctx context.Context, watchCtx *watchCtx) {
	backoff := watchCtx.backoffStart
	for {
		opts := []clientv3.OpOption{
			clientv3.WithPrefix(),
			clientv3.WithProgressNotify(),
			clientv3.WithCreatedNotify(),
		}
		if oldestRev := watchCtx.cache.demux.PeekOldest(); oldestRev != 0 {
			opts = append(opts,
				clientv3.WithRev(oldestRev+1))
		}
		watchCh := watchCtx.cache.watcher.Watch(ctx, watchCtx.cache.prefix, opts...)

		if err := readWatchChannel(watchCh, watchCtx.cache, watchCtx.cache.demux, watchCtx.onFirstResponse); err == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < watchCtx.backoffMax {
			backoff *= 2
		}
	}
}

// readWatchChannel reads an etcd Watch stream into History and enqueueCh, returning nil on cancel or first error.
func readWatchChannel(
	watchChan clientv3.WatchChan,
	cache *Cache,
	demux *demux,
	onFirstResponse func(),
) error {
	for resp := range watchChan {
		onFirstResponse()

		if err := resp.Err(); err != nil {
			if errors.Is(err, rpctypes.ErrCompacted) {
				select {
				case <-cache.ready:
					// TODO: Reinitialize cache.ready safely; current direct channel assignment can race with concurrent watchers
					cache.ready = make(chan struct{})
				default:
				}
				demux.Purge()
			}
			return err
		}
		demux.Broadcast(resp.Events)
	}
	return nil
}

func (c *Cache) validateWatch(key string, op clientv3.Op) (pred KeyPredicate, err error) {
	if op.IsPrevKV() ||
		op.IsFragment() ||
		op.IsProgressNotify() ||
		op.IsCreatedNotify() ||
		op.IsFilterPut() ||
		op.IsFilterDelete() {
		return nil, ErrUnsupportedWatch
	}

	startKey := []byte(key)
	endKey := op.RangeBytes() // nil = single key, {0}=FromKey, else explicit range

	if err := c.validateRange(startKey, endKey); err != nil {
		return nil, err
	}
	return KeyPredForRange(startKey, endKey), nil
}

func (c *Cache) validateRange(startKey, endKey []byte) error {
	prefixStart := []byte(c.prefix)
	prefixEnd := []byte(clientv3.GetPrefixRangeEnd(c.prefix))

	isSingleKey := len(endKey) == 0
	isFromKey := len(endKey) == 1 && endKey[0] == 0

	switch {
	case isSingleKey:
		if c.prefix == "" {
			return nil
		}
		if bytes.Compare(startKey, prefixStart) < 0 || bytes.Compare(startKey, prefixEnd) >= 0 {
			return ErrUnsupportedWatch
		}
		return nil

	case isFromKey:
		if c.prefix != "" {
			return ErrUnsupportedWatch
		}
		return nil

	default:
		if bytes.Compare(endKey, startKey) <= 0 {
			return ErrUnsupportedWatch
		}
		if c.prefix == "" {
			return nil
		}
		if bytes.Compare(startKey, prefixStart) < 0 || bytes.Compare(endKey, prefixEnd) > 0 {
			return ErrUnsupportedWatch
		}
		return nil
	}
}
