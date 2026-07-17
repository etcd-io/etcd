// Copyright 2026 The etcd Authors
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
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/proxy/grpcproxy"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// authInjectingKvServer wraps a real KVServer and stamps a fixed auth token
// into the incoming context metadata, simulating a request that arrived at
// the proxy with the given credential attached.
type authInjectingKvServer struct {
	pb.KVServer
	token string
}

func (s *authInjectingKvServer) withToken(ctx context.Context) context.Context {
	if s.token == "" {
		return ctx
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}
	md.Set(rpctypes.TokenFieldNameGRPC, s.token)
	return metadata.NewIncomingContext(ctx, md)
}

func (s *authInjectingKvServer) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	return s.KVServer.Range(s.withToken(ctx), r)
}

func (s *authInjectingKvServer) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	return s.KVServer.Put(s.withToken(ctx), r)
}

func (s *authInjectingKvServer) DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	return s.KVServer.DeleteRange(s.withToken(ctx), r)
}

func (s *authInjectingKvServer) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error) {
	return s.KVServer.Txn(s.withToken(ctx), r)
}

func (s *authInjectingKvServer) Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	return s.KVServer.Compact(s.withToken(ctx), r)
}

// kvAuthTestHarness spins up a real backend member with auth enabled, then
// exposes the same kvProxy through three facades: one that stamps the root
// token, one that stamps a low-privilege token, and one with no token at
// all. This lets us drive the proxy cache exactly the way real gRPC clients
// with different credentials would.
type kvAuthTestHarness struct {
	t       *testing.T
	clus    *integration.Cluster
	rootKV  pb.KVClient
	lowKV   pb.KVClient
	anonKV  pb.KVClient
	cleanup func()
}

func newKVAuthTestHarness(t *testing.T) *kvAuthTestHarness {
	t.Helper()
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})

	h := &kvAuthTestHarness{t: t, clus: clus}

	// Set up root user + a low-priv user with no permissions, then enable auth.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	api := integration.ToGRPC(clus.Client(0))

	_, err := api.Auth.UserAdd(ctx, &pb.AuthUserAddRequest{Name: "root", Password: "rootpw"})
	require.NoError(t, err)
	_, err = api.Auth.RoleAdd(ctx, &pb.AuthRoleAddRequest{Name: "root"})
	require.NoError(t, err)
	_, err = api.Auth.UserGrantRole(ctx, &pb.AuthUserGrantRoleRequest{User: "root", Role: "root"})
	require.NoError(t, err)

	_, err = api.Auth.RoleAdd(ctx, &pb.AuthRoleAddRequest{Name: "lowrole"})
	require.NoError(t, err)
	_, err = api.Auth.UserAdd(ctx, &pb.AuthUserAddRequest{Name: "low", Password: "lowpw"})
	require.NoError(t, err)
	_, err = api.Auth.UserGrantRole(ctx, &pb.AuthUserGrantRoleRequest{User: "low", Role: "lowrole"})
	require.NoError(t, err)

	// Write the protected key before enabling auth.
	_, err = api.KV.Put(ctx, &pb.PutRequest{Key: []byte("/proxy-cache/secret"), Value: []byte("cached-secret")})
	require.NoError(t, err)

	_, err = api.Auth.AuthEnable(ctx, &pb.AuthEnableRequest{})
	require.NoError(t, err)

	// Mint tokens for both users.
	rootAuth, err := api.Auth.Authenticate(ctx, &pb.AuthenticateRequest{Name: "root", Password: "rootpw"})
	require.NoError(t, err)
	lowAuth, err := api.Auth.Authenticate(ctx, &pb.AuthenticateRequest{Name: "low", Password: "lowpw"})
	require.NoError(t, err)

	// Build a backend client that forwards any incoming token straight through,
	// exactly like the production grpc-proxy does (AuthUnaryClientInterceptor).
	backendCfg := clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCURL},
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(grpcproxy.AuthUnaryClientInterceptor),
			grpc.WithStreamInterceptor(grpcproxy.AuthStreamClientInterceptor),
		},
	}
	backendClient, err := integration.NewClient(t, backendCfg)
	require.NoError(t, err)

	kvp, _ := grpcproxy.NewKvProxy(backendClient)

	mkFacade := func(token string) (pb.KVClient, net.Listener, *grpc.Server, *grpc.ClientConn) {
		srv := grpc.NewServer()
		pb.RegisterKVServer(srv, &authInjectingKvServer{KVServer: kvp, token: token})
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		go srv.Serve(l)
		conn, err := grpc.NewClient(l.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		return pb.NewKVClient(conn), l, srv, conn
	}

	var listeners []net.Listener
	var servers []*grpc.Server
	var conns []*grpc.ClientConn

	rootKV, l, s, cc := mkFacade(rootAuth.Token)
	listeners, servers, conns = append(listeners, l), append(servers, s), append(conns, cc)
	lowKV, l, s, cc := mkFacade(lowAuth.Token)
	listeners, servers, conns = append(listeners, l), append(servers, s), append(conns, cc)
	anonKV, l, s, cc := mkFacade("")
	listeners, servers, conns = append(listeners, l), append(servers, s), append(conns, cc)

	h.rootKV, h.lowKV, h.anonKV = rootKV, lowKV, anonKV
	h.cleanup = func() {
		for _, cc := range conns {
			cc.Close()
		}
		for _, s := range servers {
			s.Stop()
		}
		for _, l := range listeners {
			l.Close()
		}
		backendClient.Close()
		clus.Terminate(t)
	}
	return h
}

// TestKVProxySerializableRangeDoesNotLeakAcrossAuth is a regression test for
// the proxy cache auth bypass: a serializable Range served from the proxy
// cache must never be replayed to a caller the backend would deny.
func TestKVProxySerializableRangeDoesNotLeakAcrossAuth(t *testing.T) {
	h := newKVAuthTestHarness(t)
	defer h.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.RangeRequest{Key: []byte("/proxy-cache/secret"), Serializable: true}

	// Baseline: low-priv and unauthenticated direct serializable reads must be denied.
	_, err := h.lowKV.Range(ctx, req)
	require.Error(t, err, "low-priv serializable read should be denied before cache priming")
	_, err = h.anonKV.Range(ctx, req)
	require.Error(t, err, "anonymous serializable read should be denied before cache priming")

	// Root primes the proxy cache through the proxy.
	rootResp, err := h.rootKV.Range(ctx, req)
	require.NoError(t, err)
	require.Len(t, rootResp.Kvs, 1)
	require.Equal(t, []byte("cached-secret"), rootResp.Kvs[0].Value)

	// The exact same serializable request from a different identity must NOT
	// be served from root's cache entry — it has to hit the backend and be denied.
	_, err = h.lowKV.Range(ctx, req)
	require.Error(t, err, "low-priv serializable read must be denied even after cache priming")
	_, err = h.anonKV.Range(ctx, req)
	require.Error(t, err, "anonymous serializable read must be denied even after cache priming")

	// Root must still get a cache hit (proves the cache still works for the
	// identity that primed it).
	rootResp2, err := h.rootKV.Range(ctx, req)
	require.NoError(t, err)
	require.Len(t, rootResp2.Kvs, 1)
	require.Equal(t, []byte("cached-secret"), rootResp2.Kvs[0].Value)
}

// TestKVProxyLinearizableRangeDoesNotPopulateOtherIdentities verifies that a
// linearizable read (which the proxy also caches "as serializable") is stored
// under the caller's identity and cannot be replayed to anyone else.
func TestKVProxyLinearizableRangeDoesNotPopulateOtherIdentities(t *testing.T) {
	h := newKVAuthTestHarness(t)
	defer h.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	linReq := &pb.RangeRequest{Key: []byte("/proxy-cache/secret")}
	serReq := &pb.RangeRequest{Key: []byte("/proxy-cache/secret"), Serializable: true}

	// Root does a linearizable read; the proxy caches it "as serializable".
	_, err := h.rootKV.Range(ctx, linReq)
	require.NoError(t, err)

	// A subsequent serializable read from a different identity must be denied.
	_, err = h.lowKV.Range(ctx, serReq)
	require.Error(t, err, "serializable read after linearizable prime must be denied for low-priv user")
	_, err = h.anonKV.Range(ctx, serReq)
	require.Error(t, err, "serializable read after linearizable prime must be denied for anonymous user")
}

// TestKVProxyCacheClearedOnAuthMutation verifies that when an auth-mutating
// RPC (RoleRevokePermission) flows through the proxy, the KV cache is
// flushed so a caller whose permissions were just revoked cannot keep
// reading previously cached data. This drives the mutation through the real
// AuthProxy (wired to the same cache) exactly like production grpc-proxy.
func TestKVProxyCacheClearedOnAuthMutation(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	api := integration.ToGRPC(clus.Client(0))

	// Set up root + low-priv user, grant read on the key, enable auth.
	_, err := api.Auth.UserAdd(ctx, &pb.AuthUserAddRequest{Name: "root", Password: "rootpw"})
	require.NoError(t, err)
	_, err = api.Auth.RoleAdd(ctx, &pb.AuthRoleAddRequest{Name: "root"})
	require.NoError(t, err)
	_, err = api.Auth.UserGrantRole(ctx, &pb.AuthUserGrantRoleRequest{User: "root", Role: "root"})
	require.NoError(t, err)
	_, err = api.Auth.RoleAdd(ctx, &pb.AuthRoleAddRequest{Name: "lowrole"})
	require.NoError(t, err)
	_, err = api.Auth.UserAdd(ctx, &pb.AuthUserAddRequest{Name: "low", Password: "lowpw"})
	require.NoError(t, err)
	_, err = api.Auth.UserGrantRole(ctx, &pb.AuthUserGrantRoleRequest{User: "low", Role: "lowrole"})
	require.NoError(t, err)
	_, err = api.KV.Put(ctx, &pb.PutRequest{Key: []byte("/proxy-cache/secret"), Value: []byte("cached-secret")})
	require.NoError(t, err)
	_, err = api.Auth.AuthEnable(ctx, &pb.AuthEnableRequest{})
	require.NoError(t, err)

	rootToken := mustToken(t, api.Auth, "root", "rootpw")
	lowToken := mustToken(t, api.Auth, "low", "lowpw")

	rootCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(rpctypes.TokenFieldNameGRPC, rootToken))
	perm := &authpb.Permission{
		PermType: authpb.Permission_READ,
		Key:      []byte("/proxy-cache/secret"),
	}
	_, err = api.Auth.RoleGrantPermission(rootCtx, &pb.AuthRoleGrantPermissionRequest{Name: "lowrole", Perm: perm})
	require.NoError(t, err)

	// Build backend client that forwards tokens (production wiring).
	backendCfg := clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCURL},
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(grpcproxy.AuthUnaryClientInterceptor),
			grpc.WithStreamInterceptor(grpcproxy.AuthStreamClientInterceptor),
		},
	}
	backendClient, err := integration.NewClient(t, backendCfg)
	require.NoError(t, err)
	defer backendClient.Close()

	kvp, _ := grpcproxy.NewKvProxy(backendClient)
	authp := grpcproxy.NewAuthProxy(backendClient, kvp.(*grpcproxy.KvProxy))

	// Facade server: stamps a fixed token into incoming metadata, like real clients.
	mkFacade := func(token string, kvs pb.KVServer, as pb.AuthServer) (pb.KVClient, pb.AuthClient, func()) {
		srv := grpc.NewServer()
		pb.RegisterKVServer(srv, &authInjectingKvServer{KVServer: kvs, token: token})
		pb.RegisterAuthServer(srv, as)
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		go srv.Serve(l)
		conn, err := grpc.NewClient(l.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		return pb.NewKVClient(conn), pb.NewAuthClient(conn), func() {
			conn.Close()
			srv.Stop()
			l.Close()
		}
	}

	rootKV, _, rootCleanup := mkFacade(rootToken, kvp, authp)
	defer rootCleanup()
	lowKV, _, lowCleanup := mkFacade(lowToken, kvp, authp)
	defer lowCleanup()

	req := &pb.RangeRequest{Key: []byte("/proxy-cache/secret"), Serializable: true}

	// Both root and low can read; both populate their own cache entries.
	_, err = rootKV.Range(ctx, req)
	require.NoError(t, err)
	lowResp, err := lowKV.Range(ctx, req)
	require.NoError(t, err)
	require.Len(t, lowResp.Kvs, 1)

	// Sanity: a second low-priv read hits the cache (still succeeds even if
	// the backend would now deny — this is the stale-permission window).
	_, err = lowKV.Range(ctx, req)
	require.NoError(t, err)

	// Now revoke the permission THROUGH the auth proxy, as root. The proxy
	// must flush the KV cache as part of the mutation.
	_, err = authp.RoleRevokePermission(
		metadata.NewIncomingContext(ctx, metadata.Pairs(rpctypes.TokenFieldNameGRPC, rootToken)),
		&pb.AuthRoleRevokePermissionRequest{Role: "lowrole", Key: []byte("/proxy-cache/secret")},
	)
	require.NoError(t, err)

	// The next low-priv serializable read must NOT come from the stale cache
	// entry — it has to reach the backend and be denied.
	_, err = lowKV.Range(ctx, req)
	require.Error(t, err, "low-priv read must be denied after permission revocation through the proxy")

	// Root is unaffected (different identity, fresh cache entry).
	rootResp, err := rootKV.Range(ctx, req)
	require.NoError(t, err)
	require.Len(t, rootResp.Kvs, 1)
}

// mustToken authenticates against the backend and returns the token string.
func mustToken(t *testing.T, authClient pb.AuthClient, name, password string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := authClient.Authenticate(ctx, &pb.AuthenticateRequest{Name: name, Password: password})
	require.NoError(t, err)
	return resp.Token
}
