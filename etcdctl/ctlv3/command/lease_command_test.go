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

func TestLeaseGrantCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.LeaseGrantResponse{ID: 99, TTL: 60}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		grantFn: func(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
			clientCalled = true
			if ttl != 60 {
				t.Fatalf("ttl = %d, want 60", ttl)
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.LeaseGrantResponse
	withTestDisplay(t, &recordingPrinter{
		grantFn: func(r *clientv3.LeaseGrantResponse) {
			gotResp = r
		},
	})

	leaseGrantCommandFunc(cmd, []string{"60"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestLeaseKeepAliveCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.LeaseKeepAliveResponse{ID: 0x10, TTL: 5}

	leaseKeepAliveOnce = true
	t.Cleanup(func() {
		leaseKeepAliveOnce = false
	})

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		keepAliveOnceFn: func(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error) {
			clientCalled = true
			if id != clientv3.LeaseID(0x10) {
				t.Fatalf("lease id = %x, want 10", int64(id))
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.LeaseKeepAliveResponse
	withTestDisplay(t, &recordingPrinter{
		keepAliveFn: func(r *clientv3.LeaseKeepAliveResponse) {
			gotResp = r
		},
	})

	leaseKeepAliveCommandFunc(cmd, []string{"10"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}
