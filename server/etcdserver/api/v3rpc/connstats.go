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

package v3rpc

import (
	"context"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
)

type connStatsHandler struct{}

func (h *connStatsHandler) HandleConn(ctx context.Context, s stats.ConnStats) {
	if _, ok := s.(*stats.ConnBegin); !ok {
		return
	}
	encryption := "plaintext"
	if p, ok := peer.FromContext(ctx); ok && p.AuthInfo != nil {
		if _, isTLS := p.AuthInfo.(credentials.TLSInfo); isTLS {
			encryption = "tls"
		}
	}
	clientConnectionsByEncryptionTotal.WithLabelValues(encryption).Inc()
}

func (h *connStatsHandler) HandleRPC(context.Context, stats.RPCStats) {}
func (h *connStatsHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}
func (h *connStatsHandler) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	return ctx
}
