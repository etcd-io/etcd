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

package v3rpc

import (
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
)

type quotaKVServer struct {
	pb.KVServer
	q etcdserver.Quota
}

func NewQuotaKVServer(s *etcdserver.EtcdServer) pb.KVServer {
	return &quotaKVServer{NewKVServer(s), etcdserver.NewBackendQuota(s)}
}

func (s *quotaKVServer) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	if !s.q.Available(r) {
		return nil, rpctypes.ErrNoSpace
	}
	return s.KVServer.Put(ctx, r)
}

func (s *quotaKVServer) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error) {
	if !s.q.Available(r) {
		return nil, rpctypes.ErrNoSpace
	}
	return s.KVServer.Txn(ctx, r)
}

type quotaLeaseServer struct {
	pb.LeaseServer
	q etcdserver.Quota
}

func (s *quotaLeaseServer) LeaseCreate(ctx context.Context, cr *pb.LeaseCreateRequest) (*pb.LeaseCreateResponse, error) {
	if !s.q.Available(cr) {
		return nil, rpctypes.ErrNoSpace
	}
	return s.LeaseServer.LeaseCreate(ctx, cr)
}

func NewQuotaLeaseServer(s *etcdserver.EtcdServer) pb.LeaseServer {
	return &quotaLeaseServer{NewLeaseServer(s), etcdserver.NewBackendQuota(s)}
}
