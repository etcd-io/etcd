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
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type V3DemoServer interface {
	V3DemoDo(ctx context.Context, r pb.InternalRaftRequest) proto.Message
}

func (s *EtcdServer) V3DemoDo(ctx context.Context, r pb.InternalRaftRequest) proto.Message {
	switch {
	case r.Range != nil:
		rr := r.Range
		resp := &pb.RangeResponse{}
		resp.Header = &pb.ResponseHeader{}
		kvs, rev, err := s.kv.Range(rr.Key, rr.RangeEnd, rr.Limit, 0)
		if err != nil {
			panic("not handled error")
		}

		resp.Header.Index = rev
		for i := range kvs {
			resp.Kvs = append(resp.Kvs, &kvs[i])
		}
		return resp
	case r.Put != nil:
		rp := r.Put
		resp := &pb.PutResponse{}
		resp.Header = &pb.ResponseHeader{}
		rev := s.kv.Put(rp.Key, rp.Value)
		resp.Header.Index = rev
		return resp
	case r.DeleteRange != nil:
		rd := r.DeleteRange
		resp := &pb.DeleteRangeResponse{}
		resp.Header = &pb.ResponseHeader{}
		_, rev := s.kv.DeleteRange(rd.Key, rd.RangeEnd)
		resp.Header.Index = rev
		return resp
	case r.Txn != nil:
		panic("not implemented")
	default:
		panic("not implemented")
	}
}
