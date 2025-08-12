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

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	// TODO: add gap-free replay for arbitrary startRevs and drop this guard.
	// Returned when an option combination isn’t yet handled by the cache (e.g. WithPrevKV, WithProgressNotify for Watch(), WithCountOnly for Get()).
	ErrUnsupportedRequest = errors.New("cache: unsupported request parameters")
	// Returned when the requested key or key‑range is invalid (empty or reversed) or lies outside c.prefix.
	ErrKeyRangeInvalid = errors.New("cache: invalid or out‑of‑range key range")
)

// Cache buffers a single etcd Watch for a given key‐prefix and fan‑outs local watchers.
type Cache struct {
	prefix      string // prefix is the key-prefix this shard is responsible for ("" = root).
	cfg         Config // immutable runtime configuration
	watcher     clientv3.Watcher
	kv          clientv3.KV
	demux       *demux // demux fans incoming events out to active watchers and manages resync.
	store       *store // last‑observed snapshot
	ready       *ready
	stop        context.CancelFunc
	waitGroup   sync.WaitGroup
	internalCtx context.Context
}

// New builds a cache shard that watches only the requested prefix.
// For the root cache pass "".
func New(client *clientv3.Client, prefix string, opts ...Option) (*Cache, error) {
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
		watcher:     client.Watcher,
		kv:          client.KV,
		store:       newStore(),
		ready:       newReady(),
		stop:        cancel,
		internalCtx: internalCtx,
	}

	cache.demux = NewDemux(internalCtx, &cache.waitGroup, cfg.HistoryWindowSize, cfg.ResyncInterval)

	cache.waitGroup.Add(1)
	go func() {
		defer cache.waitGroup.Done()
		cache.getWatchLoop(internalCtx)
	}()

	return cache, nil
}

// Watch registers a cache-backed watcher for a given key or prefix.
// It returns a WatchChan that streams WatchResponses containing events.
func (c *Cache) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	if err := c.WaitReady(ctx); err != nil {
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

func (c *Cache) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if err := c.WaitReady(ctx); err != nil {
		return nil, err
	}
	op := clientv3.OpGet(key, opts...)

	if _, err := c.validateGet(key, op); err != nil {
		return nil, err
	}

	startKey := []byte(key)
	endKey := op.RangeBytes()
	kvs, rev, err := c.store.Get(startKey, endKey)
	if err != nil {
		return nil, err
	}

	return &clientv3.GetResponse{
		Header: &pb.ResponseHeader{Revision: rev},
		Kvs:    kvs,
		Count:  int64(len(kvs)),
	}, nil
}

// Ready returns true if the snapshot has been loaded and the first watch has been confirmed.
func (c *Cache) Ready() bool {
	return c.ready.Ready()
}

// WaitReady blocks until the cache is ready or the ctx is cancelled.
func (c *Cache) WaitReady(ctx context.Context) error {
	return c.ready.WaitReady(ctx)
}

func (c *Cache) WaitForRevision(ctx context.Context, rev int64) error {
	for {
		if c.store.LatestRev() >= rev {
			return nil
		}
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Close cancels the private context and blocks until all goroutines return.
func (c *Cache) Close() {
	c.stop()
	c.waitGroup.Wait()
}

func (c *Cache) getWatchLoop(ctx context.Context) {
	cfg := defaultConfig()
	backoff := cfg.InitialBackoff
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := c.getWatch(ctx); err != nil {
			fmt.Printf("getWatch failed, will retry after %v: %v\n", backoff, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func (c *Cache) getWatch(ctx context.Context) error {
	getResp, err := c.get(ctx)
	if err != nil {
		return err
	}
	return c.watch(ctx, getResp.Header.Revision+1)
}

func (c *Cache) get(ctx context.Context) (*clientv3.GetResponse, error) {
	resp, err := c.kv.Get(ctx, c.prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	c.store.Restore(resp.Kvs, resp.Header.Revision)
	return resp, nil
}

func (c *Cache) watch(ctx context.Context, rev int64) error {
	readyOnce := sync.Once{}
	for {
		watchCh := c.watcher.Watch(
			ctx,
			c.prefix,
			clientv3.WithPrefix(),
			clientv3.WithRev(rev),
			clientv3.WithProgressNotify(),
			clientv3.WithCreatedNotify(),
		)

		for resp := range watchCh {
			readyOnce.Do(func() { c.ready.Set() })
			if err := resp.Err(); err != nil {
				c.ready.Reset()
				c.demux.Purge()
				c.store.Reset()
				return err
			}

			if err := c.store.Apply(resp.Events); err != nil {
				c.ready.Reset()
				c.demux.Purge()
				c.store.Reset()
				return err
			}
			c.demux.Broadcast(resp.Events)
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (c *Cache) validateWatch(key string, op clientv3.Op) (pred KeyPredicate, err error) {
	switch {
	case op.IsPrevKV():
		return nil, fmt.Errorf("%w: PrevKV not supported", ErrUnsupportedRequest)
	case op.IsFragment():
		return nil, fmt.Errorf("%w: Fragment not supported", ErrUnsupportedRequest)
	case op.IsProgressNotify():
		return nil, fmt.Errorf("%w: ProgressNotify not supported", ErrUnsupportedRequest)
	case op.IsCreatedNotify():
		return nil, fmt.Errorf("%w: CreatedNotify not supported", ErrUnsupportedRequest)
	case op.IsFilterPut():
		return nil, fmt.Errorf("%w: FilterPut not supported", ErrUnsupportedRequest)
	case op.IsFilterDelete():
		return nil, fmt.Errorf("%w: FilterDelete not supported", ErrUnsupportedRequest)
	}

	startKey := []byte(key)
	endKey := op.RangeBytes() // nil = single key, {0}=FromKey, else explicit range

	if err := c.validateRange(startKey, endKey); err != nil {
		return nil, err
	}
	return KeyPredForRange(startKey, endKey), nil
}

func (c *Cache) validateGet(key string, op clientv3.Op) (KeyPredicate, error) {
	switch {
	case op.IsCountOnly():
		return nil, fmt.Errorf("%w: CountOnly not supported", ErrUnsupportedRequest)
	case op.IsPrevKV():
		return nil, fmt.Errorf("%w: PrevKV not supported", ErrUnsupportedRequest)
	case op.IsSortSet():
		return nil, fmt.Errorf("%w: SortSet not supported", ErrUnsupportedRequest)
	case op.Limit() != 0:
		return nil, fmt.Errorf("%w: Limit(%d) not supported", ErrUnsupportedRequest, op.Limit())
	case op.MinModRev() != 0:
		return nil, fmt.Errorf("%w: MinModRev(%d) not supported", ErrUnsupportedRequest, op.MinModRev())
	case op.MaxModRev() != 0:
		return nil, fmt.Errorf("%w: MaxModRev(%d) not supported", ErrUnsupportedRequest, op.MaxModRev())
	case op.MinCreateRev() != 0:
		return nil, fmt.Errorf("%w: MinCreateRev(%d) not supported", ErrUnsupportedRequest, op.MinCreateRev())
	case op.MaxCreateRev() != 0:
		return nil, fmt.Errorf("%w: MaxCreateRev(%d) not supported", ErrUnsupportedRequest, op.MaxCreateRev())
	// cache now only serves serializable reads of the latest revision (rev == 0).
	case op.Rev() != 0:
		return nil, fmt.Errorf("%w: Rev(%d) not supported", ErrUnsupportedRequest, op.Rev())
	case !op.IsSerializable():
		return nil, fmt.Errorf("%w: non-serializable request", ErrUnsupportedRequest)
	}

	startKey := []byte(key)
	endKey := op.RangeBytes()

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
			return ErrKeyRangeInvalid
		}
		return nil

	case isFromKey:
		if c.prefix != "" {
			return ErrKeyRangeInvalid
		}
		return nil

	default:
		if bytes.Compare(endKey, startKey) <= 0 {
			return ErrKeyRangeInvalid
		}
		if c.prefix == "" {
			return nil
		}
		if bytes.Compare(startKey, prefixStart) < 0 || bytes.Compare(endKey, prefixEnd) > 0 {
			return ErrKeyRangeInvalid
		}
		return nil
	}
}
