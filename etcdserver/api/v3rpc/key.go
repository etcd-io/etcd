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

package v3rpc

import (
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc/codes"
	"github.com/coreos/etcd/etcdserver"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/storage"
)

type handler struct {
	server etcdserver.V3DemoServer
}

func New(s etcdserver.V3DemoServer) pb.EtcdServer {
	return &handler{s}
}

func (h *handler) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	if err := checkRangeRequest(r); err != nil {
		return nil, err
	}

	resp, err := h.server.V3DemoDo(ctx, pb.InternalRaftRequest{Range: r})
	if err != nil {
		return nil, togRPCError(err)
	}

	return resp.(*pb.RangeResponse), err
}

func (h *handler) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	if err := checkPutRequest(r); err != nil {
		return nil, err
	}

	resp, err := h.server.V3DemoDo(ctx, pb.InternalRaftRequest{Put: r})
	if err != nil {
		return nil, togRPCError(err)
	}

	return resp.(*pb.PutResponse), err
}

func (h *handler) DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	if err := checkDeleteRequest(r); err != nil {
		return nil, err
	}

	resp, err := h.server.V3DemoDo(ctx, pb.InternalRaftRequest{DeleteRange: r})
	if err != nil {
		return nil, togRPCError(err)
	}

	return resp.(*pb.DeleteRangeResponse), err
}

func (h *handler) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error) {
	if err := checkTxnRequest(r); err != nil {
		return nil, err
	}

	resp, err := h.server.V3DemoDo(ctx, pb.InternalRaftRequest{Txn: r})
	if err != nil {
		return nil, togRPCError(err)
	}

	return resp.(*pb.TxnResponse), err
}

func (h *handler) Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	resp, err := h.server.V3DemoDo(ctx, pb.InternalRaftRequest{Compaction: r})
	if err != nil {
		return nil, togRPCError(err)
	}

	return resp.(*pb.CompactionResponse), nil
}

func checkRangeRequest(r *pb.RangeRequest) error {
	if len(r.Key) == 0 {
		return ErrEmptyKey
	}
	return nil
}

func checkPutRequest(r *pb.PutRequest) error {
	if len(r.Key) == 0 {
		return ErrEmptyKey
	}
	return nil
}

func checkDeleteRequest(r *pb.DeleteRangeRequest) error {
	if len(r.Key) == 0 {
		return ErrEmptyKey
	}
	return nil
}

func checkTxnRequest(r *pb.TxnRequest) error {
	for _, c := range r.Compare {
		if len(c.Key) == 0 {
			return ErrEmptyKey
		}
	}

	for _, u := range r.Success {
		if err := checkRequestUnion(u); err != nil {
			return err
		}
	}

	for _, u := range r.Failure {
		if err := checkRequestUnion(u); err != nil {
			return err
		}
	}

	return nil
}

func checkRequestUnion(u *pb.RequestUnion) error {
	// TODO: ensure only one of the field is set.
	switch {
	case u.RequestRange != nil:
		return checkRangeRequest(u.RequestRange)
	case u.RequestPut != nil:
		return checkPutRequest(u.RequestPut)
	case u.RequestDeleteRange != nil:
		return checkDeleteRequest(u.RequestDeleteRange)
	default:
		// empty union
		return nil
	}
}

func togRPCError(err error) error {
	switch err {
	case storage.ErrCompacted:
		return ErrCompacted
	case storage.ErrFutureRev:
		return ErrFutureRev
	// TODO: handle error from raft and timeout
	default:
		return grpc.Errorf(codes.Unknown, err.Error())
	}
}
