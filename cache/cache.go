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
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	ErrDisabled  = errors.New("cache disabled")
	ErrCompacted = errors.New("revision compacted")
)

type Cache struct {
	cfg Config

	cli *clientv3.Client
	es  *eventSource
	dx  *demux

	stop context.CancelFunc
	wg   sync.WaitGroup

	gapSem chan struct{}
	closed atomic.Bool
}

func New(cli *clientv3.Client, opts ...Option) (*Cache, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.Disable {
		return &Cache{cfg: cfg}, nil
	}

	// discover current revision
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	resp, err := cli.Get(ctx, "\x00", clientv3.WithLimit(1))
	cancel()
	if err != nil {
		return nil, err
	}

	parentCtx, stop := context.WithCancel(context.Background())

	es := newEventSource(cli, resp.Header.Revision, parentCtx)

	c := &Cache{
		cfg:    cfg,
		cli:    cli,
		es:     es,
		dx:     newDemux(),
		stop:   stop,
		gapSem: make(chan struct{}, cfg.MaxGapWatch),
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		es.run(func(ev *clientv3.Event) { c.dx.broadcast(ev, cfg.DropThreshold) })
	}()
	return c, nil
}

func (c *Cache) Watch(ctx context.Context, prefix string, rev int64) (*Stream, error) {
	if c.cfg.Disable {
		return nil, ErrDisabled
	}
	pred := newPrefixPred(prefix)
	attach := c.es.rv()
	out := make(chan *clientv3.Event, c.cfg.ChannelSize)

	// repeat gap replay until attach point stabilizes
	for {
		if rev > 0 && rev < attach {
			if err := c.replayGap(ctx, prefix, rev, attach, pred, out); err != nil {
				return nil, err
			}
		}
		newRv := c.es.rv()
		if newRv == attach {
			break
		}
		rev = attach
		attach = newRv
	}

	w := &watcher{ch: out, pred: pred, lastSent: attach}
	c.dx.add(w)

	go func() {
		<-ctx.Done()
		c.dx.remove(w)
	}()
	return &Stream{C: out}, nil
}

func (c *Cache) replayGap(ctx context.Context, prefix string, from, to int64,
	pred func([]byte) bool, out chan<- *clientv3.Event) error {

	select {
	case c.gapSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	gapCtx, cancel := context.WithTimeout(ctx, c.cfg.GapWatchTimeout)
	err := gapReplay(gapCtx, c.cli, prefix, from, to, pred, out)
	cancel()
	<-c.gapSem
	return translateCompacted(err)
}

func (c *Cache) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	c.stop()
	c.es.stop()
	c.cli.Close()
	c.wg.Wait()
	return nil
}

func (c *Cache) CurrentRevision() int64 { return c.es.rv() }

type Stream struct {
	C <-chan *clientv3.Event
}
