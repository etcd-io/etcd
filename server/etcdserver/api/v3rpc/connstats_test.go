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
	"crypto/tls"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
)

// TestConnStatsHandler_TLSConnection checks TLS connection metric with "tls" label
func TestConnStatsHandler_TLSConnection(t *testing.T) {
	h := &connStatsHandler{}
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{}},
	})

	before := testutil.ToFloat64(clientConnectionsByEncryptionTotal.WithLabelValues("tls"))
	h.HandleConn(ctx, &stats.ConnBegin{})

	if got := testutil.ToFloat64(clientConnectionsByEncryptionTotal.WithLabelValues("tls")) - before; got != 1 {
		t.Errorf("expected tls counter to increment by 1, got %v", got)
	}
}

// TestConnStatsHandler_PlaintextConnection checks plaintext connection increments metric with "plaintext" label
func TestConnStatsHandler_PlaintextConnection(t *testing.T) {
	h := &connStatsHandler{}
	ctx := peer.NewContext(context.Background(), &peer.Peer{})

	before := testutil.ToFloat64(clientConnectionsByEncryptionTotal.WithLabelValues("plaintext"))
	h.HandleConn(ctx, &stats.ConnBegin{})

	if got := testutil.ToFloat64(clientConnectionsByEncryptionTotal.WithLabelValues("plaintext")) - before; got != 1 {
		t.Errorf("expected plaintext counter to increment by 1, got %v", got)
	}
}
