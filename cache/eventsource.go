package cache

import (
	"context"
	"sync/atomic"
	"time"

	rpctypes "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type eventSource struct {
	cli       *clientv3.Client
	ctx       context.Context
	cancel    context.CancelFunc
	ch        <-chan clientv3.WatchResponse
	currentRV atomic.Int64
}

func newEventSource(cli *clientv3.Client, startRev int64, parent context.Context) *eventSource {
	ctx, cancel := context.WithCancel(parent)
	es := &eventSource{cli: cli, ctx: ctx, cancel: cancel}
	es.start(startRev)
	return es
}

func (es *eventSource) start(rev int64) {
	opts := []clientv3.OpOption{clientv3.WithPrefix()}
	if rev > 0 {
		opts = append(opts, clientv3.WithRev(rev))
	}
	es.ch = es.cli.Watch(es.ctx, "", opts...)
}

func (es *eventSource) run(bcast func(*clientv3.Event)) {
	backoff := time.Second
	for {
		select {
		case <-es.ctx.Done():
			return
		case wr, ok := <-es.ch:
			if !ok {
				es.restart(es.currentRV.Load()+1, &backoff)
				continue
			}
			if err := wr.Err(); err != nil {
				if err == rpctypes.ErrCompacted {
					// fast‑forward to compaction point contained in Error struct
					es.restart(wr.CompactRevision+1, &backoff)
				} else {
					es.restart(es.currentRV.Load()+1, &backoff)
				}
				continue
			}
			backoff = time.Second // reset on success
			if hdr := wr.Header; hdr.Revision > 0 {
				es.currentRV.Store(hdr.Revision)
			}
			for _, ev := range wr.Events {
				es.currentRV.Store(ev.Kv.ModRevision)
				bcast(ev)
			}
		}
	}
}

func (es *eventSource) restart(rev int64, backoff *time.Duration) {
	time.Sleep(*backoff)
	if *backoff < 30*time.Second {
		*backoff *= 2
	}
	es.start(rev)
}

func (es *eventSource) rv() int64 { return es.currentRV.Load() }

func (es *eventSource) stop() { es.cancel() }
