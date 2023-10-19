// Copyright 2023 The etcd Authors
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

package endpoints

// TODO: The API is not yet implemented.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/naming/endpoints/internal"
)

type endpointManager struct {
	client *clientv3.Client
	target string
}

func NewManager(client *clientv3.Client, target string) (Manager, error) {
	if client == nil {
		return nil, errors.New("invalid etcd client")
	}

	if target == "" {
		return nil, errors.New("invalid target")
	}

	em := &endpointManager{
		client: client,
		target: target,
	}
	return em, nil
}

func (m *endpointManager) Update(ctx context.Context, updates []*UpdateWithOpts) (err error) {
	ops := make([]clientv3.Op, 0, len(updates))
	for _, update := range updates {
		if !strings.HasPrefix(update.Key, m.target+"/") {
			return status.Errorf(codes.InvalidArgument, "endpoints: endpoint key should be prefixed with %s/", m.target)
		}
		switch update.Op {
		case Add:
			internalUpdate := &internal.Update{
				Op:       internal.Add,
				Addr:     update.Endpoint.Addr,
				Metadata: update.Endpoint.Metadata,
			}

			var v []byte
			if v, err = json.Marshal(internalUpdate); err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			ops = append(ops, clientv3.OpPut(update.Key, string(v), update.Opts...))
		case Delete:
			ops = append(ops, clientv3.OpDelete(update.Key, update.Opts...))
		default:
			return status.Error(codes.InvalidArgument, "endpoints: bad update op")
		}
	}
	_, err = m.client.KV.Txn(ctx).Then(ops...).Commit()
	return err
}

func (m *endpointManager) AddEndpoint(ctx context.Context, key string, endpoint Endpoint, opts ...clientv3.OpOption) error {
	return m.Update(ctx, []*UpdateWithOpts{NewAddUpdateOpts(key, endpoint, opts...)})
}

func (m *endpointManager) DeleteEndpoint(ctx context.Context, key string, opts ...clientv3.OpOption) error {
	return m.Update(ctx, []*UpdateWithOpts{NewDeleteUpdateOpts(key, opts...)})
}

func (m *endpointManager) NewWatchChannel(ctx context.Context) (WatchChannel, error) {
	return nil, fmt.Errorf("Not implemented yet")

	// TODO: Implementation to be inspired by:
	// Next gets the next set of updates from the etcd resolver.
	//// Calls to Next should be serialized; concurrent calls are not safe since
	//// there is no way to reconcile the update ordering.
	//func (gw *gRPCWatcher) Next() ([]*naming.Update, error) {
	//	if gw.wch == nil {
	//		// first Next() returns all addresses
	//		return gw.firstNext()
	//	}
	//	if gw.err != nil {
	//		return nil, gw.err
	//	}
	//
	//	// process new events on target/*
	//	wr, ok := <-gw.wch
	//	if !ok {
	//		gw.err = status.Error(codes.Unavailable, ErrWatcherClosed.Error())
	//		return nil, gw.err
	//	}
	//	if gw.err = wr.Err(); gw.err != nil {
	//		return nil, gw.err
	//	}
	//
	//	updates := make([]*naming.Update, 0, len(wr.Events))
	//	for _, e := range wr.Events {
	//		var jupdate naming.Update
	//		var err error
	//		switch e.Type {
	//		case etcd.EventTypePut:
	//			err = json.Unmarshal(e.Kv.Value, &jupdate)
	//			jupdate.Op = naming.Add
	//		case etcd.EventTypeDelete:
	//			err = json.Unmarshal(e.PrevKv.Value, &jupdate)
	//			jupdate.Op = naming.Delete
	//		default:
	//			continue
	//		}
	//		if err == nil {
	//			updates = append(updates, &jupdate)
	//		}
	//	}
	//	return updates, nil
	//}
	//
	//func (gw *gRPCWatcher) firstNext() ([]*naming.Update, error) {
	//	// Use serialized request so resolution still works if the target etcd
	//	// server is partitioned away from the quorum.
	//	resp, err := gw.c.Get(gw.ctx, gw.target, etcd.WithPrefix(), etcd.WithSerializable())
	//	if gw.err = err; err != nil {
	//		return nil, err
	//	}
	//
	//	updates := make([]*naming.Update, 0, len(resp.Kvs))
	//	for _, kv := range resp.Kvs {
	//		var jupdate naming.Update
	//		if err := json.Unmarshal(kv.Value, &jupdate); err != nil {
	//			continue
	//		}
	//		updates = append(updates, &jupdate)
	//	}
	//
	//	opts := []etcd.OpOption{etcd.WithRev(resp.Header.Revision + 1), etcd.WithPrefix(), etcd.WithPrevKV()}
	//	gw.wch = gw.c.Watch(gw.ctx, gw.target, opts...)
	//	return updates, nil
	//}
}

func (m *endpointManager) List(ctx context.Context) (Key2EndpointMap, error) {
	resp, err := m.client.Get(ctx, m.target, clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		return nil, err
	}

	eps := make(Key2EndpointMap)
	for _, kv := range resp.Kvs {
		var iup internal.Update
		if err := json.Unmarshal(kv.Value, &iup); err != nil {
			continue
		}

		eps[string(kv.Key)] = Endpoint{Addr: iup.Addr, Metadata: iup.Metadata}
	}
	return eps, nil
}
