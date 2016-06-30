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

package naming

import (
	"encoding/json"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"golang.org/x/net/context"
	"google.golang.org/grpc/naming"
)

const (
	gRPCNamingPrefix = "/github.com/grpc/"
)

// GRPCResolver creates a grpc.Watcher for a target to track its resolution changes.
type GRPCResolver struct {
	// Client is an initialized etcd client
	Client *clientv3.Client
	// Timeout for update/delete request.
	Timeout time.Duration
}

func (gr *GRPCResolver) Add(target string, addr string, metadata interface{}) error {
	update := naming.Update{
		Addr:     addr,
		Metadata: metadata,
	}
	val, err := json.Marshal(update)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if gr.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), gr.Timeout)
		defer cancel()
	}

	_, err = gr.Client.KV.Put(ctx, gRPCNamingPrefix+target, string(val))
	return err
}

func (gr *GRPCResolver) Delete(target string) error {
	ctx := context.Background()
	if gr.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), gr.Timeout)
		defer cancel()
	}

	_, err := gr.Client.Delete(ctx, gRPCNamingPrefix+target)
	return err
}

func (gr *GRPCResolver) Resolve(target string) (naming.Watcher, error) {
	cctx, cancel := context.WithCancel(context.Background())

	wch := gr.Client.Watch(cctx, gRPCNamingPrefix+target)

	w := &gRPCWatcher{
		cancel: cancel,
		wch:    wch,
	}

	return w, nil
}

type gRPCWatcher struct {
	cancel context.CancelFunc
	wch    clientv3.WatchChan
}

func (gw *gRPCWatcher) Next() ([]*naming.Update, error) {
	wr, ok := <-gw.wch
	if !ok {
		return nil, wr.Err()
	}

	updates := make([]*naming.Update, 0, len(wr.Events))

	for _, e := range wr.Events {
		switch e.Type {
		case mvccpb.PUT:
			var jupdate naming.Update
			err := json.Unmarshal(e.Kv.Value, &jupdate)
			if err != nil {
				continue
			}
			updates = append(updates, &jupdate)
		case mvccpb.DELETE:
			updates = append(updates, &naming.Update{Op: naming.Delete})
		}
	}

	return updates, nil
}

func (gw *gRPCWatcher) Close() { gw.cancel() }
