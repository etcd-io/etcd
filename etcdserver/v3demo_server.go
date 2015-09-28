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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	dstorage "github.com/coreos/etcd/storage"
)

type V3DemoServer interface {
	V3DemoDo(ctx context.Context, r pb.InternalRaftRequest) (proto.Message, error)
}

type applyResult struct {
	resp proto.Message
	err  error
}

func (s *EtcdServer) V3DemoDo(ctx context.Context, r pb.InternalRaftRequest) (proto.Message, error) {
	r.ID = s.reqIDGen.Next()

	data, err := r.Marshal()
	if err != nil {
		return &pb.EmptyResponse{}, err
	}
	ch := s.w.Register(r.ID)

	s.r.Propose(ctx, data)

	select {
	case x := <-ch:
		result := x.(*applyResult)
		return result.resp, result.err
	case <-ctx.Done():
		s.w.Trigger(r.ID, nil) // GC wait
		return &pb.EmptyResponse{}, ctx.Err()
	case <-s.done:
		return &pb.EmptyResponse{}, ErrStopped
	}
}

func (s *EtcdServer) applyV3Request(r *pb.InternalRaftRequest) interface{} {
	ar := &applyResult{}

	switch {
	case r.Range != nil:
		ar.resp, ar.err = applyRange(s.kv, r.Range)
	case r.Put != nil:
		ar.resp, ar.err = applyPut(s.kv, r.Put)
	case r.DeleteRange != nil:
		ar.resp, ar.err = applyDeleteRange(s.kv, r.DeleteRange)
	case r.Txn != nil:
		ar.resp, ar.err = applyTxn(s.kv, r.Txn)
	case r.Compaction != nil:
		ar.resp, ar.err = applyCompaction(s.kv, r.Compaction)
	default:
		panic("not implemented")
	}

	return ar
}

func applyPut(kv dstorage.KV, p *pb.PutRequest) (*pb.PutResponse, error) {
	resp := &pb.PutResponse{}
	resp.Header = &pb.ResponseHeader{}
	rev := kv.Put(p.Key, p.Value)
	resp.Header.Revision = rev
	return resp, nil
}

func applyRange(kv dstorage.KV, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	resp := &pb.RangeResponse{}
	resp.Header = &pb.ResponseHeader{}
	kvs, rev, err := kv.Range(r.Key, r.RangeEnd, r.Limit, 0)
	if err != nil {
		return nil, err
	}

	resp.Header.Revision = rev
	for i := range kvs {
		resp.Kvs = append(resp.Kvs, &kvs[i])
	}
	return resp, nil
}

func applyDeleteRange(kv dstorage.KV, dr *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	resp := &pb.DeleteRangeResponse{}
	resp.Header = &pb.ResponseHeader{}
	_, rev := kv.DeleteRange(dr.Key, dr.RangeEnd)
	resp.Header.Revision = rev
	return resp, nil
}

func applyTxn(kv dstorage.KV, rt *pb.TxnRequest) (*pb.TxnResponse, error) {
	var revision int64

	ok := true
	for _, c := range rt.Compare {
		if revision, ok = applyCompare(kv, c); !ok {
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
		resps[i] = applyUnion(kv, reqs[i])
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

func applyUnion(kv dstorage.KV, union *pb.RequestUnion) *pb.ResponseUnion {
	switch {
	case union.RequestRange != nil:
		resp, err := applyRange(kv, union.RequestRange)
		if err != nil {
			panic("unexpected error during txn")
		}
		return &pb.ResponseUnion{ResponseRange: resp}
	case union.RequestPut != nil:
		resp, err := applyPut(kv, union.RequestPut)
		if err != nil {
			panic("unexpected error during txn")
		}
		return &pb.ResponseUnion{ResponsePut: resp}
	case union.RequestDeleteRange != nil:
		resp, err := applyDeleteRange(kv, union.RequestDeleteRange)
		if err != nil {
			panic("unexpected error during txn")
		}
		return &pb.ResponseUnion{ResponseDeleteRange: resp}
	default:
		// empty union
		return nil
	}
}

func applyCompare(kv dstorage.KV, c *pb.Compare) (int64, bool) {
	ckvs, rev, err := kv.Range(c.Key, nil, 1, 0)
	if err != nil {
		return rev, false
	}

	ckv := ckvs[0]

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
