package cache

import (
	"context"
	"time"

	rpctypes "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// serveWatchEvents runs one upstream watch, feeds ring & demux, and restarts on lag.
func serveWatchEvents(
	ctx context.Context,
	client *clientv3.Client,
	prefix string,
	eventSink func(*clientv3.Event),
	demux *demux,
	ring History,
	setCompaction func(int64),
	backoffStart time.Duration,
	backoffMax time.Duration,
) {
	go func() {
		backoff := backoffStart
		for {
			// resume from the oldest revision we still keep
			oldest := ring.OldestRevision()
			opts := []clientv3.OpOption{
				clientv3.WithPrefix(),
				clientv3.WithProgressNotify(),
			}
			if oldest > 0 {
				opts = append(opts, clientv3.WithRev(oldest+1))
			}

			watchCh := client.Watch(ctx, prefix, opts...)

			for wr := range watchCh {
				// update compaction info on every frame
				if cr := wr.CompactRevision; cr > 0 {
					setCompaction(cr)
					if cr >= ring.LatestRevision() {
						ring.RebaseHistory(cr)
						demux.purgeAll()
					}
				}

				if err := wr.Err(); err != nil {
					if err == rpctypes.ErrCompacted {
						ring.RebaseHistory(ring.LatestRevision())
						demux.purgeAll()
						setCompaction(wr.CompactRevision)
					}
					break // will fall through to retry logic
				}

				for _, evt := range wr.Events {
					ring.Append(evt)
					eventSink(evt)
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			if backoff < backoffMax {
				backoff *= 2
			}
		}
	}()
}
