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

func TestPutCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	leaseStr = "0"
	putPrevKV = false
	putIgnoreVal = false
	putIgnoreLease = false
	t.Cleanup(func() {
		leaseStr = "0"
		putPrevKV = false
		putIgnoreVal = false
		putIgnoreLease = false
	})

	resp := &clientv3.PutResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		putFn: func(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
			clientCalled = true
			if key != "foo" || val != "bar" {
				t.Fatalf("Put(%q, %q), want (foo, bar)", key, val)
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.PutResponse
	withTestDisplay(t, &recordingPrinter{
		putFn: func(r *clientv3.PutResponse) {
			gotResp = r
		},
	})

	putCommandFunc(cmd, []string{"foo", "bar"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestGetCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	getConsistency = "l"
	getLimit = 0
	getSortOrder = ""
	getSortTarget = ""
	getPrefix = false
	getFromKey = false
	getRev = 0
	getKeysOnly = false
	getCountOnly = false
	printValueOnly = false
	getMinCreateRev = 0
	getMaxCreateRev = 0
	getMinModRev = 0
	getMaxModRev = 0
	getStream = false
	t.Cleanup(func() {
		getConsistency = "l"
		getLimit = 0
		getSortOrder = ""
		getSortTarget = ""
		getPrefix = false
		getFromKey = false
		getRev = 0
		getKeysOnly = false
		getCountOnly = false
		printValueOnly = false
		getMinCreateRev = 0
		getMaxCreateRev = 0
		getMinModRev = 0
		getMaxModRev = 0
		getStream = false
	})

	resp := &clientv3.GetResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		getFn: func(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
			clientCalled = true
			if key != "foo" {
				t.Fatalf("Get key = %q, want foo", key)
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.GetResponse
	withTestDisplay(t, &recordingPrinter{
		getFn: func(r *clientv3.GetResponse) {
			gotResp = r
		},
	})

	getCommandFunc(cmd, []string{"foo"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestDelCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	delPrefix = false
	delPrevKV = false
	delFromKey = false
	delRange = true
	t.Cleanup(func() {
		delPrefix = false
		delPrevKV = false
		delFromKey = false
		delRange = false
	})

	resp := &clientv3.DeleteResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		deleteFn: func(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
			clientCalled = true
			if key != "foo" {
				t.Fatalf("Delete key = %q, want foo", key)
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.DeleteResponse
	withTestDisplay(t, &recordingPrinter{
		delFn: func(r *clientv3.DeleteResponse) {
			gotResp = r
		},
	})

	delCommandFunc(cmd, []string{"foo"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestCompactionCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	compactPhysical = false
	t.Cleanup(func() {
		compactPhysical = false
	})

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		compactFn: func(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
			clientCalled = true
			if rev != 7 {
				t.Fatalf("revision = %d, want 7", rev)
			}
			return &clientv3.CompactResponse{}, nil
		},
	})

	out := captureStdout(t, func() {
		compactionCommandFunc(cmd, []string{"7"})
	})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if out != "compacted revision 7\n" {
		t.Fatalf("stdout = %q, want %q", out, "compacted revision 7\n")
	}
}
