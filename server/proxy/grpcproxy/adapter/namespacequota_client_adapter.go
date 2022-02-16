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

package adapter

import (
	"context"
	"google.golang.org/grpc"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

type nqs2nqc struct {
	namespaceQuotaServer pb.NamespaceQuotaServer
}

func NamespaceQuotaServerToNamespaceQuotaClient(nqs pb.NamespaceQuotaServer) pb.NamespaceQuotaClient {
	return &nqs2nqc{nqs}
}

func (c *nqs2nqc) NamespaceQuotaSet(ctx context.Context, in *pb.NamespaceQuotaSetRequest, opts ...grpc.CallOption) (*pb.NamespaceQuotaResponse, error) {
	return c.namespaceQuotaServer.NamespaceQuotaSet(ctx, in)
}

func (c *nqs2nqc) NamespaceQuotaGet(ctx context.Context, in *pb.NamespaceQuotaGetRequest, opts ...grpc.CallOption) (*pb.NamespaceQuotaResponse, error) {
	return c.namespaceQuotaServer.NamespaceQuotaGet(ctx, in)
}

func (c *nqs2nqc) NamespaceQuotaDelete(ctx context.Context, in *pb.NamespaceQuotaDeleteRequest, opts ...grpc.CallOption) (*pb.NamespaceQuotaResponse, error) {
	return c.namespaceQuotaServer.NamespaceQuotaDelete(ctx, in)
}

func (c *nqs2nqc) NamespaceQuotaList(ctx context.Context, in *pb.NamespaceQuotaListRequest, opts ...grpc.CallOption) (*pb.NamespaceQuotaListResponse, error) {
	return c.namespaceQuotaServer.NamespaceQuotaList(ctx, in)
}
