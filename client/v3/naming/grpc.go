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

// DEPRECATION NOTICE
// This package is deprecated and must be implemented as part of grpcproxy package

package naming

import (
	"context"
	"encoding/json"
	"fmt"
	etcd "go.etcd.io/etcd/client/v3"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Code at 31-75 based on deprecated google.golang.org/grpc/naming

// Operation defines the corresponding operations for a name resolution change.
//
// Deprecated: please use package resolver.
type Operation uint8

const (
	// Add indicates a new address is added.
	Add Operation = iota
	// Delete indicates an existing address is deleted.
	Delete
)

// Update defines a name resolution update. Notice that it is not valid having both
// empty string Addr and nil Metadata in an Update.
//
// Deprecated: please use package resolver.
type Update struct {
	// Op indicates the operation of the update.
	Op Operation
	// Addr is the updated address. It is empty string if there is no address update.
	Addr string
	// Metadata is the updated metadata. It is nil if there is no metadata update.
	// Metadata is not required for a custom naming implementation.
	Metadata interface{}
}

// Resolver creates a Watcher for a target to track its resolution changes.
//
// Deprecated: please use package resolver.
type Resolver interface {
	// Resolve creates a Watcher for target.
	Resolve(target string) (Watcher, error)
}

// Watcher watches for the updates on the specified target.
//
// Deprecated: please use package resolver.
type Watcher interface {
	// Next blocks until an update or error happens. It may return one or more
	// updates. The first call should get the full set of the results. It should
	// return an error if and only if Watcher cannot recover.
	Next() ([]*Update, error)
	// Close closes the Watcher.
	Close()
}

var ErrWatcherClosed = fmt.Errorf("naming: watch closed")

// GRPCResolver creates a grpc.Watcher for a target to track its resolution changes.
type GRPCResolver struct {
	// Client is an initialized etcd client.
	Client *etcd.Client
}

func (gr *GRPCResolver) Update(ctx context.Context, target string, nm Update, opts ...etcd.OpOption) (err error) {
	switch nm.Op {
	case Add:
		var v []byte
		if v, err = json.Marshal(nm); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		_, err = gr.Client.KV.Put(ctx, target+"/"+nm.Addr, string(v), opts...)
	case Delete:
		_, err = gr.Client.Delete(ctx, target+"/"+nm.Addr, opts...)
	default:
		return status.Error(codes.InvalidArgument, "naming: bad naming op")
	}
	return err
}

func (gr *GRPCResolver) Resolve(target string) (Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &gRPCWatcher{c: gr.Client, target: target + "/", ctx: ctx, cancel: cancel}
	return w, nil
}

type gRPCWatcher struct {
	c      *etcd.Client
	target string
	ctx    context.Context
	cancel context.CancelFunc
	wch    etcd.WatchChan
	err    error
}

// Next gets the next set of updates from the etcd resolver.
// Calls to Next should be serialized; concurrent calls are not safe since
// there is no way to reconcile the update ordering.
func (gw *gRPCWatcher) Next() ([]*Update, error) {
	if gw.wch == nil {
		// first Next() returns all addresses
		return gw.firstNext()
	}
	if gw.err != nil {
		return nil, gw.err
	}

	// process new events on target/*
	wr, ok := <-gw.wch
	if !ok {
		gw.err = status.Error(codes.Unavailable, ErrWatcherClosed.Error())
		return nil, gw.err
	}
	if gw.err = wr.Err(); gw.err != nil {
		return nil, gw.err
	}

	updates := make([]*Update, 0, len(wr.Events))
	for _, e := range wr.Events {
		var jupdate Update
		var err error
		switch e.Type {
		case etcd.EventTypePut:
			err = json.Unmarshal(e.Kv.Value, &jupdate)
			jupdate.Op = Add
		case etcd.EventTypeDelete:
			err = json.Unmarshal(e.PrevKv.Value, &jupdate)
			jupdate.Op = Delete
		default:
			continue
		}
		if err == nil {
			updates = append(updates, &jupdate)
		}
	}
	return updates, nil
}

func (gw *gRPCWatcher) firstNext() ([]*Update, error) {
	// Use serialized request so resolution still works if the target etcd
	// server is partitioned away from the quorum.
	resp, err := gw.c.Get(gw.ctx, gw.target, etcd.WithPrefix(), etcd.WithSerializable())
	if gw.err = err; err != nil {
		return nil, err
	}

	updates := make([]*Update, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var jupdate Update
		if err := json.Unmarshal(kv.Value, &jupdate); err != nil {
			continue
		}
		updates = append(updates, &jupdate)
	}

	opts := []etcd.OpOption{etcd.WithRev(resp.Header.Revision + 1), etcd.WithPrefix(), etcd.WithPrevKV()}
	gw.wch = gw.c.Watch(gw.ctx, gw.target, opts...)
	return updates, nil
}

func (gw *gRPCWatcher) Close() { gw.cancel() }
