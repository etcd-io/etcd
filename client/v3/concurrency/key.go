// Copyright 2016 The etcd Authors
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

package concurrency

import (
	"context"
	"fmt"

	"go.etcd.io/etcd/api/v3/mvccpb"
	v3 "go.etcd.io/etcd/client/v3"
)

func waitDelete(ctx context.Context, client *v3.Client, key string, rev int64) error {
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wr v3.WatchResponse
	wch := client.Watch(cctx, key, v3.WithRev(rev))
	for wr = range wch {
		for _, ev := range wr.Events {
			if ev.Type == mvccpb.DELETE {
				return nil
			}
		}
	}
	if err := wr.Err(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("lost watcher waiting for delete")
}

// waitDeletes efficiently waits until all keys matching the prefix and no greater
// than the create revision.
func waitDeletes(ctx context.Context, client *v3.Client, pfx string, maxCreateRev int64) error {
	getOpts := append(v3.WithLastCreate(), v3.WithMaxCreateRev(maxCreateRev))

	resp, err := client.Get(ctx, pfx, getOpts...)
	if err != nil {
		return err
	}
	if len(resp.Kvs) == 0 {
		return nil
	}
	errc := make(chan error)
	closec := make(chan struct{})
	cctx, cancel := context.WithCancel(ctx)
	for _, kv := range resp.Kvs {
		go func(kvKey string) {
			waitErr := waitDelete(cctx, client, kvKey, resp.Header.Revision)
			select {
			case <-closec:
			case errc <- waitErr:
				cancel()
			}
		}(string(kv.Key))
	}

	for i := 0; i < len(resp.Kvs); i++ {
		err = <-errc
		if err != nil {
			close(closec)
			close(errc)
			break
		}
	}
	cancel()
	return err
}
