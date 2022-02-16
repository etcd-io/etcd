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

package grpcproxy

import (
	"context"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"sync"
)

type namespaceQuotaProxy struct {
	// namespaceQuotaClient handles req for NamespaceQuota
	namespaceQuotaClient pb.NamespaceQuotaClient

	namespaceQuotaManager clientv3.NamespaceQuota

	ctx context.Context

	leader *leader

	// mu protects adding outstanding leaseProxyStream through wg.
	mu sync.RWMutex

	// wg waits until all outstanding leaseProxyStream quit.
	wg sync.WaitGroup
}

func NewNamespaceQuotaProxy(ctx context.Context, c *clientv3.Client) (pb.NamespaceQuotaServer, <-chan struct{}) {
	cctx, cancel := context.WithCancel(ctx)
	nqp := &namespaceQuotaProxy{
		namespaceQuotaClient:  pb.NewNamespaceQuotaClient(c.ActiveConnection()),
		namespaceQuotaManager: c.NamespaceQuota,
		ctx:                   cctx,
		leader:                newLeader(cctx, c.Watcher),
	}
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		<-nqp.leader.stopNotify()
		nqp.mu.Lock()
		select {
		case <-nqp.ctx.Done():
		case <-nqp.leader.disconnectNotify():
			cancel()
		}
		<-nqp.ctx.Done()
		nqp.mu.Unlock()
		nqp.wg.Wait()
	}()
	return nqp, ch
}

func (nqp *namespaceQuotaProxy) NamespaceQuotaSet(ctx context.Context, request *pb.NamespaceQuotaSetRequest) (*pb.NamespaceQuotaResponse, error) {
	rp, err := nqp.namespaceQuotaClient.NamespaceQuotaSet(ctx, request, grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	nqp.leader.gotLeader()
	return rp, nil
}

func (nqp *namespaceQuotaProxy) NamespaceQuotaGet(ctx context.Context, request *pb.NamespaceQuotaGetRequest) (*pb.NamespaceQuotaResponse, error) {
	rp, err := nqp.namespaceQuotaClient.NamespaceQuotaGet(ctx, request, grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	nqp.leader.gotLeader()
	return rp, nil
}

func (nqp *namespaceQuotaProxy) NamespaceQuotaDelete(ctx context.Context, request *pb.NamespaceQuotaDeleteRequest) (*pb.NamespaceQuotaResponse, error) {
	rp, err := nqp.namespaceQuotaClient.NamespaceQuotaDelete(ctx, request, grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	nqp.leader.gotLeader()
	return rp, nil
}

func (nqp *namespaceQuotaProxy) NamespaceQuotaList(ctx context.Context, request *pb.NamespaceQuotaListRequest) (*pb.NamespaceQuotaListResponse, error) {
	rp, err := nqp.namespaceQuotaClient.NamespaceQuotaList(ctx, request, grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	nqp.leader.gotLeader()
	return rp, nil
}
