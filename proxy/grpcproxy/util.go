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

package grpcproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func getAuthTokenFromClient(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ts, ok := md[rpctypes.TokenFieldNameGRPC]
		if ok {
			return ts[0]
		}
	}
	return ""
}

func withClientAuthToken(ctx, ctxWithToken context.Context) context.Context {
	token := getAuthTokenFromClient(ctxWithToken)
	if token != "" {
		ctx = context.WithValue(ctx, rpctypes.TokenFieldNameGRPC, token)
	}
	return ctx
}

type proxyTokenCredential struct {
	token string
}

func (cred *proxyTokenCredential) RequireTransportSecurity() bool {
	return false
}

func (cred *proxyTokenCredential) GetRequestMetadata(ctx context.Context, s ...string) (map[string]string, error) {
	return map[string]string{
		rpctypes.TokenFieldNameGRPC: cred.token,
	}, nil
}

func AuthUnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	token := getAuthTokenFromClient(ctx)
	if token != "" {
		tokenCred := &proxyTokenCredential{token}
		opts = append(opts, grpc.PerRPCCredentials(tokenCred))
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func AuthStreamClientInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	tokenif := ctx.Value(rpctypes.TokenFieldNameGRPC)
	if tokenif != nil {
		tokenCred := &proxyTokenCredential{tokenif.(string)}
		opts = append(opts, grpc.PerRPCCredentials(tokenCred))
	}
	return streamer(ctx, desc, cc, method, opts...)
}

func shuffleEndpoints(r *rand.Rand, eps []string) []string {
	// copied from Go 1.9<= rand.Rand.Perm
	n := len(eps)
	p := make([]int, n)
	for i := 0; i < n; i++ {
		j := r.Intn(i + 1)
		p[i] = p[j]
		p[j] = i
	}
	neps := make([]string, n)
	for i, k := range p {
		neps[i] = eps[k]
	}
	return neps
}

func createTarget(eps []string, path string, tls *tls.ConnectionState, random bool) string {
	if random {
		// random shuffle endpoints
		r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
		if len(eps) > 1 {
			eps = shuffleEndpoints(r, eps)
		}
	}
	target := fmt.Sprintf("%s%s", eps[0], path)
	if !strings.HasPrefix(target, "http") {
		scheme := "http"
		if tls != nil {
			scheme = "https"
		}
		target = fmt.Sprintf("%s://%s", scheme, target)
	}
	return target
}
