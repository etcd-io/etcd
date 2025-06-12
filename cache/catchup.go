package cache

import (
	"context"
	"errors"

	rpctypes "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// streams events (fromRev, untilRev] for requested prefix
// if requested rev got compacted -> retries from compactRev+1
func gapReplay(ctx context.Context, cli *clientv3.Client, prefix string,
	fromRev, untilRev int64, pred func([]byte) bool, out chan<- *clientv3.Event) error {

	rev := fromRev + 1
	for {
		rch := cli.Watch(ctx, prefix,
			clientv3.WithPrefix(),
			clientv3.WithRev(rev))
		for wr := range rch {
			if err := wr.Err(); err != nil {
				if errors.Is(err, rpctypes.ErrCompacted) {
					rev = wr.CompactRevision + 1
					break // restart watch
				}
				return err
			}
			for _, ev := range wr.Events {
				if ev.Kv.ModRevision > untilRev {
					return nil
				}
				if pred(ev.Kv.Key) {
					out <- ev
				}
			}
			if wr.Header.Revision >= untilRev {
				return nil
			}
		}
	}
}

// converts rpctypes ErrCompacted -> cache ErrCompacted
func translateCompacted(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, rpctypes.ErrCompacted) {
		return ErrCompacted
	}
	return err
}
