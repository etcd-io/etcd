// Copyright 2026 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Safety tests for kvProxy cache invalidation ordering (RS-B13-1).
//
// kvProxy.Put / DeleteRange invalidate the cache and then write to the
// backend. A concurrent serializable Range can slip into the window between
// the invalidate and the backend write committing, read the pre-write value,
// and re-populate the cache with it — leaving an unbounded stale entry until
// the next write through this proxy.
//
// This is a BUG FIX, not a behavior-preserving refactor. The "StaleReAdd"
// tests deterministically model the concurrent window by having the fake KV,
// during the backend Do, perform the cache.Add a racing Range would do (gRPC
// runs each RPC on its own goroutine, so this interleaving is production-real;
// the hook just makes it reliable instead of flaky). They are EXPECTED TO FAIL
// at baseline (proof of the bug) and to PASS once Put/DeleteRange invalidate
// again after the backend write. The other tests lock invariants that hold
// both before and after.

package grpcproxy

import (
	"context"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/proxy/grpcproxy/cache"
)

// hookKV stands in for the backend client. onDo runs inside Do — used to
// simulate a concurrent serializable Range that re-cached the stale value
// during the write window.
type hookKV struct {
	clientv3.KV
	onDo func(op clientv3.Op)
}

func (k *hookKV) Do(_ context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	if k.onDo != nil {
		k.onDo(op)
	}
	return clientv3.OpResponse{}, nil
}

// serializableRange returns a (req, resp) pair for a serializable Range of key
// with Revision==0 so cache.Add both stores it and registers it for
// key-range invalidation.
func serializableRange(key string) (*pb.RangeRequest, *pb.RangeResponse) {
	return &pb.RangeRequest{Key: []byte(key), Serializable: true},
		&pb.RangeResponse{Header: &pb.ResponseHeader{Revision: 1}}
}

// --- BUG-EXPOSING (FAIL pre-fix, PASS post-fix) ------------------------------

func TestKVProxyInvalidateOrderSafety_PutStaleReAddCleared(t *testing.T) {
	c := cache.NewCache(cache.DefaultMaxEntries)
	staleReq, staleResp := serializableRange("k")

	p := &kvProxy{cache: c}
	p.kv = &hookKV{onDo: func(op clientv3.Op) {
		if op.IsPut() { // concurrent serializable Range re-adds the pre-write value
			c.Add(staleReq, staleResp)
		}
	}}

	if _, err := p.Put(context.Background(), &pb.PutRequest{Key: []byte("k"), Value: []byte("v1")}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := c.Get(staleReq); err == nil {
		t.Errorf("serializable Range(k) hit a stale cached value after Put returned; cache must be invalidated after the backend write")
	}
}

func TestKVProxyInvalidateOrderSafety_DeleteRangeStaleReAddCleared(t *testing.T) {
	c := cache.NewCache(cache.DefaultMaxEntries)
	staleReq, staleResp := serializableRange("k")

	p := &kvProxy{cache: c}
	p.kv = &hookKV{onDo: func(op clientv3.Op) {
		if op.IsDelete() {
			c.Add(staleReq, staleResp)
		}
	}}

	if _, err := p.DeleteRange(context.Background(), &pb.DeleteRangeRequest{Key: []byte("k")}); err != nil {
		t.Fatalf("DeleteRange: %v", err)
	}
	if _, err := c.Get(staleReq); err == nil {
		t.Errorf("serializable Range(k) hit a stale cached value after DeleteRange returned; cache must be invalidated after the backend write")
	}
}

// --- INVARIANTS (PASS pre-fix AND post-fix) ----------------------------------

// A Put still invalidates a value cached before the write (the no-race path).
func TestKVProxyInvalidateOrderSafety_PutClearsPreCached(t *testing.T) {
	c := cache.NewCache(cache.DefaultMaxEntries)
	req, resp := serializableRange("k")
	c.Add(req, resp) // pre-cached

	p := &kvProxy{cache: c, kv: &hookKV{}}
	if _, err := p.Put(context.Background(), &pb.PutRequest{Key: []byte("k"), Value: []byte("v1")}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := c.Get(req); err == nil {
		t.Errorf("Put did not invalidate the pre-cached value for its own key")
	}
}

// A Put must not evict an unrelated key's cache entry.
func TestKVProxyInvalidateOrderSafety_PutLeavesUnrelatedKey(t *testing.T) {
	c := cache.NewCache(cache.DefaultMaxEntries)
	reqA, respA := serializableRange("a")
	c.Add(reqA, respA)

	p := &kvProxy{cache: c, kv: &hookKV{}}
	if _, err := p.Put(context.Background(), &pb.PutRequest{Key: []byte("b"), Value: []byte("v1")}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := c.Get(reqA); err != nil {
		t.Errorf("Put(b) wrongly evicted unrelated cached key a: %v", err)
	}
}
