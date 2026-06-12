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

func TestUserAddCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	passwordInteractive = true
	passwordFromFlag = ""
	noPassword = true
	t.Cleanup(func() {
		passwordInteractive = true
		passwordFromFlag = ""
		noPassword = false
	})

	resp := &clientv3.AuthUserAddResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		userAddWithOptionsFn: func(ctx context.Context, name, password string, opt *clientv3.UserAddOptions) (*clientv3.AuthUserAddResponse, error) {
			clientCalled = true
			if name != "alice" {
				t.Fatalf("user name = %q, want alice", name)
			}
			if password != "" {
				t.Fatalf("password = %q, want empty", password)
			}
			if opt == nil || !opt.NoPassword {
				t.Fatal("expected NoPassword option")
			}
			return resp, nil
		},
	})

	var gotUser string
	var gotResp *clientv3.AuthUserAddResponse
	withTestDisplay(t, &recordingPrinter{
		userAddFn: func(user string, r *clientv3.AuthUserAddResponse) {
			gotUser = user
			gotResp = r
		},
	})

	userAddCommandFunc(cmd, []string{"alice"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotUser != "alice" {
		t.Fatalf("display user = %q, want alice", gotUser)
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestUserListCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.AuthUserListResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		userListFn: func(ctx context.Context) (*clientv3.AuthUserListResponse, error) {
			clientCalled = true
			return resp, nil
		},
	})

	var gotResp *clientv3.AuthUserListResponse
	withTestDisplay(t, &recordingPrinter{
		userListFn: func(r *clientv3.AuthUserListResponse) {
			gotResp = r
		},
	})

	userListCommandFunc(cmd, nil)

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}
