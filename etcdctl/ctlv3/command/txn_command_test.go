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

package command

import (
	"context"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestTxnCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	txnInteractive = false
	t.Cleanup(func() {
		txnInteractive = false
	})

	fake := &fakeTxn{
		commitFn: func() (*clientv3.TxnResponse, error) {
			return &clientv3.TxnResponse{}, nil
		},
	}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		txnFn: func(ctx context.Context) clientv3.Txn {
			clientCalled = true
			return fake
		},
	})

	var gotResp *clientv3.TxnResponse
	withTestDisplay(t, &recordingPrinter{
		txnFn: func(r *clientv3.TxnResponse) {
			gotResp = r
		},
	})

	withTestStdin(t, "ver(\"foo\") = \"1\"\n\nget foo\n\nput foo bar\n\n")

	txnCommandFunc(cmd, nil)

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if len(fake.ifCmps) != 1 {
		t.Fatalf("If comparisons = %d, want 1", len(fake.ifCmps))
	}
	if len(fake.thenOps) != 1 {
		t.Fatalf("Then ops = %d, want 1", len(fake.thenOps))
	}
	if len(fake.elseOps) != 1 {
		t.Fatalf("Else ops = %d, want 1", len(fake.elseOps))
	}
	if gotResp == nil {
		t.Fatal("expected transaction response to be displayed")
	}
}
