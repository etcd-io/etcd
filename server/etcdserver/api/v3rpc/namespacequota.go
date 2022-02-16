// Copyright 2021 The etcd Authors
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
	"context"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.uber.org/zap"
)

type NamespaceQuotaServer struct {
	lg  *zap.Logger
	hdr header
	nq  etcdserver.NamespaceQuotaManager
}

func NewNamespaceQuotaServer(s *etcdserver.EtcdServer) pb.NamespaceQuotaServer {
	return &NamespaceQuotaServer{lg: s.Cfg.Logger, nq: s, hdr: newHeader(s)}
}

func (n *NamespaceQuotaServer) NamespaceQuotaSet(ctx context.Context, nr *pb.NamespaceQuotaSetRequest) (*pb.NamespaceQuotaResponse, error) {
	resp, err := n.nq.NamespaceQuotaSet(ctx, nr)

	if err != nil {
		return nil, togRPCError(err)
	}
	n.hdr.fill(resp.Header)
	return resp, nil
}

func (n *NamespaceQuotaServer) NamespaceQuotaGet(ctx context.Context, nr *pb.NamespaceQuotaGetRequest) (*pb.NamespaceQuotaResponse, error) {
	resp, err := n.nq.NamespaceQuotaGet(ctx, nr)

	if err != nil {
		return nil, togRPCError(err)
	}
	n.hdr.fill(resp.Header)
	return resp, nil
}

func (n *NamespaceQuotaServer) NamespaceQuotaDelete(ctx context.Context, nr *pb.NamespaceQuotaDeleteRequest) (*pb.NamespaceQuotaResponse, error) {
	resp, err := n.nq.NamespaceQuotaDelete(ctx, nr)
	if err != nil {
		return nil, togRPCError(err)
	}
	n.hdr.fill(resp.Header)
	return resp, nil
}

func (n *NamespaceQuotaServer) NamespaceQuotaList(ctx context.Context, nr *pb.NamespaceQuotaListRequest) (*pb.NamespaceQuotaListResponse, error) {
	resp, err := n.nq.NamespaceQuotaList(ctx, nr)

	if err != nil {
		return nil, togRPCError(err)
	}
	n.hdr.fill(resp.Header)
	return resp, nil
}
