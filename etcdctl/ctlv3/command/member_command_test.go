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

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestMemberAddCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	memberPeerURLs = "http://127.0.0.1:2380"
	isLearner = true
	t.Cleanup(func() {
		memberPeerURLs = ""
		isLearner = false
	})

	resp := &clientv3.MemberAddResponse{
		Member: &pb.Member{ID: 0x42},
	}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		memberAddAsLearnerFn: func(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error) {
			clientCalled = true
			if len(peerAddrs) != 1 || peerAddrs[0] != "http://127.0.0.1:2380" {
				t.Fatalf("peerAddrs = %v, want [http://127.0.0.1:2380]", peerAddrs)
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.MemberAddResponse
	withTestDisplay(t, &recordingPrinter{
		memberAddFn: func(r *clientv3.MemberAddResponse) {
			gotResp = r
		},
	})

	memberAddCommandFunc(cmd, []string{"infra1"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}
