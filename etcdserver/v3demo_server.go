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
		resp := x.(proto.Message)
		return resp, nil
	case <-ctx.Done():
		s.w.Trigger(r.ID, nil) // GC wait
		return &pb.EmptyResponse{}, ctx.Err()
	case <-s.done:
		return &pb.EmptyResponse{}, ErrStopped
	}
}

func (s *EtcdServer) applyV3Request(r *pb.InternalRaftRequest) interface{} {
	switch {
	case r.Range != nil:
		return applyRange(s.kv, r.Range)
	case r.Put != nil:
		return applyPut(s.kv, r.Put)
	case r.DeleteRange != nil:
		return applyDeleteRange(s.kv, r.DeleteRange)
	case r.Txn != nil:
		return applyTxn(s.kv, r.Txn)
	default:
		panic("not implemented")
	}
}

func applyPut(kv dstorage.KV, p *pb.PutRequest) *pb.PutResponse {
	resp := &pb.PutResponse{}
	resp.Header = &pb.ResponseHeader{}
	rev := kv.Put(p.Key, p.Value)
	resp.Header.Revision = rev
	return resp
}

func applyRange(kv dstorage.KV, r *pb.RangeRequest) *pb.RangeResponse {
	resp := &pb.RangeResponse{}
	resp.Header = &pb.ResponseHeader{}
	kvs, rev, err := kv.Range(r.Key, r.RangeEnd, r.Limit, 0)
	if err != nil {
		panic("not handled error")
	}

	resp.Header.Revision = rev
	for i := range kvs {
		resp.Kvs = append(resp.Kvs, &kvs[i])
	}
	return resp
}

func applyDeleteRange(kv dstorage.KV, dr *pb.DeleteRangeRequest) *pb.DeleteRangeResponse {
	resp := &pb.DeleteRangeResponse{}
	resp.Header = &pb.ResponseHeader{}
	_, rev := kv.DeleteRange(dr.Key, dr.RangeEnd)
	resp.Header.Revision = rev
	return resp
}

func applyTxn(kv dstorage.KV, rt *pb.TxnRequest) *pb.TxnResponse {
	var revision int64

	ok := true
	for _, c := range rt.Compare {
		if revision, ok = applyCompare(kv, c); !ok {
			break
		}
	}

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
	return txnResp
}

func applyUnion(kv dstorage.KV, union *pb.RequestUnion) *pb.ResponseUnion {
	switch {
	case union.RequestRange != nil:
		return &pb.ResponseUnion{ResponseRange: applyRange(kv, union.RequestRange)}
	case union.RequestPut != nil:
		return &pb.ResponseUnion{ResponsePut: applyPut(kv, union.RequestPut)}
	case union.RequestDeleteRange != nil:
		return &pb.ResponseUnion{ResponseDeleteRange: applyDeleteRange(kv, union.RequestDeleteRange)}
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
