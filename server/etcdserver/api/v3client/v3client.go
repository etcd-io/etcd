// Copyright 2017 The etcd Authors
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

package v3client

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3rpc"
	"go.etcd.io/etcd/server/v3/proxy/grpcproxy/adapter"
)

// New creates a clientv3 client that wraps an in-process EtcdServer. Instead
// of making gRPC calls through sockets, the client makes direct function calls
// to the etcd server through its api/v3rpc function interfaces.
func New(s *etcdserver.EtcdServer) *clientv3.Client {
	c := clientv3.NewCtxClient(context.Background(), clientv3.WithZapLogger(s.Logger()))

	kvc := adapter.KvServerToKvClient(&kvServerWrapper{
		KVServer: v3rpc.NewQuotaKVServer(s),
		s:        s,
	})
	c.KV = clientv3.NewKVFromKVClient(kvc, c)

	lc := adapter.LeaseServerToLeaseClient(v3rpc.NewQuotaLeaseServer(s))
	c.Lease = clientv3.NewLeaseFromLeaseClient(lc, c, time.Second)

	wc := adapter.WatchServerToWatchClient(v3rpc.NewWatchServer(s))
	c.Watcher = &watchWrapper{
		Watcher: clientv3.NewWatchFromWatchClient(wc, c),
		s:       s,
	}

	mc := adapter.MaintenanceServerToMaintenanceClient(v3rpc.NewMaintenanceServer(s, nil))
	c.Maintenance = clientv3.NewMaintenanceFromMaintenanceClient(mc, c)

	clc := adapter.ClusterServerToClusterClient(v3rpc.NewClusterServer(s))
	c.Cluster = clientv3.NewClusterFromClusterClient(clc, c)

	a := adapter.AuthServerToAuthClient(v3rpc.NewAuthServer(s))
	c.Auth = clientv3.NewAuthFromAuthClient(a, c)

	return c
}

// BlankContext implements Stringer on a context so the ctx string doesn't
// depend on the context's WithValue data, which tends to be unsynchronized
// (e.g., x/net/trace), causing ctx.String() to throw data races.
type blankContext struct{ context.Context }

func (*blankContext) String() string { return "(blankCtx)" }

// watchWrapper wraps clientv3 watch calls to blank out the context
// to avoid races on trace data, and refreshes expired auth token if needed.
type watchWrapper struct {
	clientv3.Watcher
	s *etcdserver.EtcdServer
}

func (ww *watchWrapper) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	if ww.s != nil && ww.s.AuthStore() != nil && ww.s.AuthStore().IsAuthEnabled() {
		if _, err := ww.s.AuthStore().AuthInfoFromCtx(ctx); err != nil && isInvalidAuthToken(err) {
			if newCtx, ok := renewContextToken(ctx, ww.s); ok {
				ctx = newCtx
			}
		}
	}
	return ww.Watcher.Watch(&blankContext{ctx}, key, opts...)
}

type kvServerWrapper struct {
	pb.KVServer
	s *etcdserver.EtcdServer
}

func (kw *kvServerWrapper) Range(ctx context.Context, req *pb.RangeRequest) (*pb.RangeResponse, error) {
	resp, err := kw.KVServer.Range(ctx, req)
	if isInvalidAuthToken(err) {
		if newCtx, ok := renewContextToken(ctx, kw.s); ok {
			return kw.KVServer.Range(newCtx, req)
		}
	}
	return resp, err
}

func (kw *kvServerWrapper) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	resp, err := kw.KVServer.Put(ctx, req)
	if isInvalidAuthToken(err) {
		if newCtx, ok := renewContextToken(ctx, kw.s); ok {
			return kw.KVServer.Put(newCtx, req)
		}
	}
	return resp, err
}

func (kw *kvServerWrapper) DeleteRange(ctx context.Context, req *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	resp, err := kw.KVServer.DeleteRange(ctx, req)
	if isInvalidAuthToken(err) {
		if newCtx, ok := renewContextToken(ctx, kw.s); ok {
			return kw.KVServer.DeleteRange(newCtx, req)
		}
	}
	return resp, err
}

func (kw *kvServerWrapper) Txn(ctx context.Context, req *pb.TxnRequest) (*pb.TxnResponse, error) {
	resp, err := kw.KVServer.Txn(ctx, req)
	if isInvalidAuthToken(err) {
		if newCtx, ok := renewContextToken(ctx, kw.s); ok {
			return kw.KVServer.Txn(newCtx, req)
		}
	}
	return resp, err
}

func (kw *kvServerWrapper) Compact(ctx context.Context, req *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	resp, err := kw.KVServer.Compact(ctx, req)
	if isInvalidAuthToken(err) {
		if newCtx, ok := renewContextToken(ctx, kw.s); ok {
			return kw.KVServer.Compact(newCtx, req)
		}
	}
	return resp, err
}

func isInvalidAuthToken(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, rpctypes.ErrGRPCInvalidAuthToken) || errors.Is(err, rpctypes.ErrInvalidAuthToken) {
		return true
	}
	if errors.Is(err, rpctypes.ErrGRPCAuthOldRevision) || errors.Is(err, rpctypes.ErrAuthOldRevision) {
		return true
	}
	if errors.Is(rpctypes.Error(err), rpctypes.ErrInvalidAuthToken) || errors.Is(rpctypes.Error(err), rpctypes.ErrAuthOldRevision) {
		return true
	}
	if s, ok := status.FromError(err); ok {
		msg := s.Message()
		if msg == "etcdserver: invalid auth token" || msg == "etcdserver: revision of auth is old" {
			return true
		}
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "invalid auth token") || strings.Contains(errMsg, "revision of auth is old")
}

func renewContextToken(ctx context.Context, s *etcdserver.EtcdServer) (context.Context, bool) {
	if s == nil || s.AuthStore() == nil || !s.AuthStore().IsAuthEnabled() {
		return ctx, false
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, false
	}
	ts, ok := md[rpctypes.TokenFieldNameGRPC]
	hasSwagger := false
	if !ok || len(ts) == 0 {
		ts, ok = md[rpctypes.TokenFieldNameSwagger]
		hasSwagger = ok && len(ts) > 0
	}
	if !ok || len(ts) == 0 {
		return ctx, false
	}

	token := ts[0]
	hasBearer := strings.HasPrefix(token, "Bearer ")
	rawToken := strings.TrimPrefix(token, "Bearer ")

	newToken, err := s.AuthStore().RenewToken(ctx, rawToken)
	if err != nil {
		return ctx, false
	}

	if hasBearer {
		newToken = "Bearer " + newToken
	}

	newMD := md.Copy()
	newMD[rpctypes.TokenFieldNameGRPC] = []string{newToken}
	if hasSwagger {
		newMD[rpctypes.TokenFieldNameSwagger] = []string{newToken}
	}
	return metadata.NewIncomingContext(ctx, newMD), true
}
