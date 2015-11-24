// Copyright 2015 CoreOS, Inc.
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

package etcdserver

import (
	"bytes"
	"fmt"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	dstorage "github.com/coreos/etcd/storage"
	"github.com/coreos/etcd/storage/storagepb"
)

type RaftKV interface {
	Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error)
	Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error)
	DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error)
	Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error)
	Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error)
}

func (s *EtcdServer) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Range: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.RangeResponse), result.err
}

func (s *EtcdServer) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Put: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.PutResponse), result.err
}

func (s *EtcdServer) DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{DeleteRange: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.DeleteRangeResponse), result.err
}

func (s *EtcdServer) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Txn: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.TxnResponse), result.err
}

func (s *EtcdServer) Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Compaction: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.CompactionResponse), result.err
}

type applyResult struct {
	resp proto.Message
	err  error
}

func (s *EtcdServer) processInternalRaftRequest(ctx context.Context, r pb.InternalRaftRequest) (*applyResult, error) {
	r.ID = s.reqIDGen.Next()

	data, err := r.Marshal()
	if err != nil {
		return nil, err
	}
	ch := s.w.Register(r.ID)

	s.r.Propose(ctx, data)

	select {
	case x := <-ch:
		return x.(*applyResult), nil
	case <-ctx.Done():
		s.w.Trigger(r.ID, nil) // GC wait
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrStopped
	}
}

// Watcable returns a watchable interface attached to the etcdserver.
func (s *EtcdServer) Watchable() dstorage.Watchable {
	return s.kv
}

const (
	// noTxn is an invalid txn ID.
	// To apply with independent Range, Put, Delete, you can pass noTxn
	// to apply functions instead of a valid txn ID.
	noTxn = -1
)

func (s *EtcdServer) applyV3Request(r *pb.InternalRaftRequest) interface{} {
	ar := &applyResult{}

	switch {
	case r.Range != nil:
		ar.resp, ar.err = applyRange(noTxn, s.kv, r.Range)
	case r.Put != nil:
		ar.resp, ar.err = applyPut(noTxn, s.kv, r.Put)
	case r.DeleteRange != nil:
		ar.resp, ar.err = applyDeleteRange(noTxn, s.kv, r.DeleteRange)
	case r.Txn != nil:
		ar.resp, ar.err = applyTxn(s.kv, r.Txn)
	case r.Compaction != nil:
		ar.resp, ar.err = applyCompaction(s.kv, r.Compaction)
	default:
		panic("not implemented")
	}

	return ar
}

func applyPut(txnID int64, kv dstorage.KV, p *pb.PutRequest) (*pb.PutResponse, error) {
	resp := &pb.PutResponse{}
	resp.Header = &pb.ResponseHeader{}
	var (
		rev int64
		err error
	)
	if txnID != noTxn {
		rev, err = kv.TxnPut(txnID, p.Key, p.Value)
		if err != nil {
			return nil, err
		}
	} else {
		rev = kv.Put(p.Key, p.Value)
	}
	resp.Header.Revision = rev
	return resp, nil
}

func applyRange(txnID int64, kv dstorage.KV, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	resp := &pb.RangeResponse{}
	resp.Header = &pb.ResponseHeader{}

	var (
		kvs []storagepb.KeyValue
		rev int64
		err error
	)

	if txnID != noTxn {
		kvs, rev, err = kv.TxnRange(txnID, r.Key, r.RangeEnd, r.Limit, 0)
		if err != nil {
			return nil, err
		}
	} else {
		kvs, rev, err = kv.Range(r.Key, r.RangeEnd, r.Limit, 0)
		if err != nil {
			return nil, err
		}
	}

	resp.Header.Revision = rev
	for i := range kvs {
		resp.Kvs = append(resp.Kvs, &kvs[i])
	}
	return resp, nil
}

func applyDeleteRange(txnID int64, kv dstorage.KV, dr *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	resp := &pb.DeleteRangeResponse{}
	resp.Header = &pb.ResponseHeader{}

	var (
		rev int64
		err error
	)

	if txnID != noTxn {
		_, rev, err = kv.TxnDeleteRange(txnID, dr.Key, dr.RangeEnd)
		if err != nil {
			return nil, err
		}
	} else {
		_, rev = kv.DeleteRange(dr.Key, dr.RangeEnd)
	}

	resp.Header.Revision = rev
	return resp, nil
}

func applyTxn(kv dstorage.KV, rt *pb.TxnRequest) (*pb.TxnResponse, error) {
	var revision int64

	txnID := kv.TxnBegin()
	defer func() {
		err := kv.TxnEnd(txnID)
		if err != nil {
			panic(fmt.Sprint("unexpected error when closing txn", txnID))
		}
	}()

	ok := true
	for _, c := range rt.Compare {
		if revision, ok = applyCompare(txnID, kv, c); !ok {
			break
		}
	}

	// TODO: check potential errors before actually applying anything

	var reqs []*pb.RequestUnion
	if ok {
		reqs = rt.Success
	} else {
		reqs = rt.Failure
	}

	resps := make([]*pb.ResponseUnion, len(reqs))
	for i := range reqs {
		resps[i] = applyUnion(txnID, kv, reqs[i])
	}

	if len(resps) != 0 {
		revision += 1
	}

	txnResp := &pb.TxnResponse{}
	txnResp.Header = &pb.ResponseHeader{}
	txnResp.Header.Revision = revision
	txnResp.Responses = resps
	txnResp.Succeeded = ok
	return txnResp, nil
}

func applyCompaction(kv dstorage.KV, compaction *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	resp := &pb.CompactionResponse{}
	resp.Header = &pb.ResponseHeader{}
	err := kv.Compact(compaction.Revision)
	if err != nil {
		return nil, err
	}
	// get the current revision. which key to get is not important.
	_, resp.Header.Revision, _ = kv.Range([]byte("compaction"), nil, 1, 0)
	return resp, err
}

func applyUnion(txnID int64, kv dstorage.KV, union *pb.RequestUnion) *pb.ResponseUnion {
	switch {
	case union.RequestRange != nil:
		resp, err := applyRange(txnID, kv, union.RequestRange)
		if err != nil {
			panic("unexpected error during txn")
		}
		return &pb.ResponseUnion{ResponseRange: resp}
	case union.RequestPut != nil:
		resp, err := applyPut(txnID, kv, union.RequestPut)
		if err != nil {
			panic("unexpected error during txn")
		}
		return &pb.ResponseUnion{ResponsePut: resp}
	case union.RequestDeleteRange != nil:
		resp, err := applyDeleteRange(txnID, kv, union.RequestDeleteRange)
		if err != nil {
			panic("unexpected error during txn")
		}
		return &pb.ResponseUnion{ResponseDeleteRange: resp}
	default:
		// empty union
		return nil
	}
}

// applyCompare applies the compare request.
// applyCompare should only be called within a txn request and an valid txn ID must
// be presented. Or applyCompare panics.
// It returns the revision at which the comparison happens. If the comparison
// succeeds, the it returns true. Otherwise it returns false.
func applyCompare(txnID int64, kv dstorage.KV, c *pb.Compare) (int64, bool) {
	if txnID == noTxn {
		panic("applyCompare called with noTxn")
	}
	ckvs, rev, err := kv.TxnRange(txnID, c.Key, nil, 1, 0)
	if err != nil {
		if err == dstorage.ErrTxnIDMismatch {
			panic("unexpected txn ID mismatch error")
		}
		return rev, false
	}
	var ckv storagepb.KeyValue
	if len(ckvs) != 0 {
		ckv = ckvs[0]
	} else {
		// Use the zero value of ckv normally. However...
		if c.Target == pb.Compare_VALUE {
			// Always fail if we're comparing a value on a key that doesn't exist.
			// We can treat non-existence as the empty set explicitly, such that
			// even a key with a value of length 0 bytes is still a real key
			// that was written that way
			return rev, false
		}
	}

	// -1 is less, 0 is equal, 1 is greater
	var result int
	switch c.Target {
	case pb.Compare_VALUE:
		result = bytes.Compare(ckv.Value, c.Value)
	case pb.Compare_CREATE:
		result = compareInt64(ckv.CreateRevision, c.CreateRevision)
	case pb.Compare_MOD:
		result = compareInt64(ckv.ModRevision, c.ModRevision)
	case pb.Compare_VERSION:
		result = compareInt64(ckv.Version, c.Version)
	}

	switch c.Result {
	case pb.Compare_EQUAL:
		if result != 0 {
			return rev, false
		}
	case pb.Compare_GREATER:
		if result != 1 {
			return rev, false
		}
	case pb.Compare_LESS:
		if result != -1 {
			return rev, false
		}
	}
	return rev, true
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
