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
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	rpctypes "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrCompacted = rpctypes.ErrCompacted

// TODO: add gap-free replay for arbitrary startRevs and drop this guard.
var ErrUnsupportedStartRev = errors.New("cache: unsupported non-zero start revision")

// WatchEventStream is the read-only channel returned to a single Watch() caller.
type WatchEventStream <-chan *clientv3.Event

// Cache buffers a single etcd Watch for a given key‐prefix and fan‑outs local watchers.
type Cache struct {
	prefix    string // prefix is the key-prefix this shard is responsible for ("" = root).
	cfg       Config // immutable runtime configuration
	client    *clientv3.Client
	history   *ringBuffer // history stores recent events in a fixed-size ring and supports replay; concrete impl until the API stabilises.
	demux     *demux      // demux fans incoming events out to active watchers and manages resync.
	ready     chan struct{}
	stop      context.CancelFunc
	waitGroup sync.WaitGroup
}

// watchCtx collects all the knobs that both serveWatchEvents and watchRetryLoop need.
type watchCtx struct {
	client          *clientv3.Client
	prefix          string
	demux           *demux
	ring            *ringBuffer
	backoffStart    time.Duration
	backoffMax      time.Duration
	broadcast       func(*clientv3.Event)
	onFirstResponse func() // callback to fire once on first upstream response
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

	ringBuffer := newRingBuffer(cfg.HistoryWindowSize)

	internalCtx, cancel := context.WithCancel(context.Background())

	cache := &Cache{
		prefix:  prefix,
		cfg:     cfg,
		client:  client,
		history: ringBuffer,
		ready:   make(chan struct{}),
		stop:    cancel,
	}

	cache.demux = newDemux(internalCtx, &cache.waitGroup, ringBuffer, cfg.ResyncInterval)

	cache.waitGroup.Add(1)
	go func() {
		defer cache.waitGroup.Done()
		readyOnce := sync.Once{}

		watchCtx := &watchCtx{
			client:          client,
			prefix:          prefix,
			demux:           cache.demux,
			ring:            cache.history,
			backoffStart:    cfg.InitialBackoff,
			backoffMax:      cfg.MaxBackoff,
			broadcast:       cache.demux.broadcast,
			onFirstResponse: func() { readyOnce.Do(func() { close(cache.ready) }) },
		}
		serveWatchEvents(internalCtx, watchCtx, cfg.UpstreamBufferSize)
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

	if !strings.HasPrefix(key, c.prefix) {
		emptyWatchChan := make(chan clientv3.WatchResponse)
		close(emptyWatchChan)
		return emptyWatchChan
	}

	op := clientv3.OpGet(key, opts...)
	requestedRev := op.Rev()

	responseChan := make(chan clientv3.WatchResponse, 1)

	// build the internal Watch event stream (for historic reply + live)
	stream, err := c.newWatchEventStream(ctx, key, requestedRev)
	if err != nil {
		if errors.Is(err, ErrCompacted) {
			var compactRev int64
			if oldestEvent := c.history.PeekOldest(); oldestEvent != nil {
				compactRev = oldestEvent.Kv.ModRevision
			}
			responseChan <- clientv3.WatchResponse{CompactRevision: compactRev}
		} else {
			responseChan <- clientv3.WatchResponse{Canceled: true}
		}
		close(responseChan)
		return responseChan
	}

	go func() {
		defer close(responseChan)
		for {
			select {
			case event, ok := <-stream:
				if !ok {
					return
				}
				responseChan <- clientv3.WatchResponse{Events: []*clientv3.Event{event}}
			case <-ctx.Done():
				return
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

// newWatchEventStream builds a single internal Watch event stream for
// both historical replay (>= startRev) and live updates filtered by key.
func (c *Cache) newWatchEventStream(ctx context.Context, key string, startRev int64) (WatchEventStream, error) {
	var pred KeyPredicate
	if key == "\x00" { // special case for "\x00" which means “entire key-space”
		pred = nil // accept every event
	} else {
		pred = func(k []byte) bool { return strings.HasPrefix(string(k), key) }
	}

	// TODO: re-enable arbitrary startRev support once we guarantee gap-free replay.
	if startRev != 0 {
		return nil, ErrUnsupportedStartRev
	}

	var nextRev int64
	if latestEvent := c.history.PeekLatest(); latestEvent != nil {
		nextRev = latestEvent.Kv.ModRevision + 1
	}

	w := newWatcher(c.cfg.PerWatcherBufferSize, pred)
	c.demux.add(w, nextRev)

	go func() {
		<-ctx.Done()
		c.demux.remove(w)
	}()

	return w.eventQueue, nil
}

func serveWatchEvents(ctx context.Context, watchCtx *watchCtx, bufSize int) {
	enqueueCh := make(chan *clientv3.Event, bufSize)

	go func() {
		for event := range enqueueCh {
			watchCtx.broadcast(event)
		}
	}()

	// upstream watch/retry loop
	go func() {
		defer close(enqueueCh)
		watchRetryLoop(ctx, watchCtx, enqueueCh)
	}()
}

func watchRetryLoop(ctx context.Context, watchCtx *watchCtx, enqueueCh chan<- *clientv3.Event) {
	backoff := watchCtx.backoffStart
	for {
		opts := []clientv3.OpOption{
			clientv3.WithPrefix(),
			clientv3.WithProgressNotify(),
			clientv3.WithCreatedNotify(),
		}
		if oldestEvent := watchCtx.ring.PeekOldest(); oldestEvent != nil {
			opts = append(opts,
				clientv3.WithRev(oldestEvent.Kv.ModRevision+1))
		}
		watchCh := watchCtx.client.Watch(ctx, watchCtx.prefix, opts...)

		if drainWatchChannel(ctx, watchCh, watchCtx.ring, watchCtx.demux, enqueueCh, watchCtx.onFirstResponse) == nil {
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

// drainWatchChannel reads an etcd Watch stream into History and enqueueCh, returning nil on cancel or first error.
func drainWatchChannel(
	ctx context.Context,
	watchChan clientv3.WatchChan,
	ring *ringBuffer,
	demux *demux,
	enqueueCh chan<- *clientv3.Event,
	onFirstResponse func(),
) error {
	for resp := range watchChan {
		onFirstResponse()

		if err := resp.Err(); err != nil {
			// compaction -> drop history & restart watchers next time round
			if errors.Is(err, rpctypes.ErrCompacted) {
				ring.RebaseHistory()
				demux.purgeAll()
			}
			return err
		}
		for _, event := range resp.Events {
			ring.Append(event)
			select {
			case enqueueCh <- event:
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}
