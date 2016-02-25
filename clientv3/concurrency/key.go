// Copyright 2016 CoreOS, Inc.
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
	"fmt"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
)

// NewUniqueKey creates a new key from a given prefix.
func NewUniqueKey(ctx context.Context, kv v3.KV, pfx string, opts ...v3.OpOption) (string, int64, error) {
	for {
		newKey := fmt.Sprintf("%s/%v", pfx, time.Now().UnixNano())
		put := v3.OpPut(newKey, "", opts...)
		cmp := v3.Compare(v3.ModifiedRevision(newKey), "=", 0)
		resp, err := kv.Txn(ctx).If(cmp).Then(put).Commit()
		if err != nil {
			return "", 0, err
		}
		if !resp.Succeeded {
			continue
		}
		return newKey, resp.Header.Revision, nil
	}
}

func waitUpdate(ctx context.Context, client *v3.Client, key string, opts ...v3.OpOption) error {
	w := v3.NewWatcher(client)
	defer w.Close()
	wc := w.Watch(ctx, key, opts...)
	if wc == nil {
		return ctx.Err()
	}
	wresp, ok := <-wc
	if !ok {
		return ctx.Err()
	}
	if len(wresp.Events) == 0 {
		return v3rpc.ErrCompacted
	}
	return nil
}
