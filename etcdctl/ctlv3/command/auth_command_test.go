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

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestAuthEnableCommandFunc_CreatesRootRoleWhenNeeded(t *testing.T) {
	cmd := newTestCommand(t)
	var authEnableCalls, roleAddCalls, grantRoleCalls int

	withTestClient(t, &fakeCommandClient{
		authEnableFn: func(ctx context.Context) (*clientv3.AuthEnableResponse, error) {
			authEnableCalls++
			if authEnableCalls == 1 {
				return nil, rpctypes.ErrRootRoleNotExist
			}
			return &clientv3.AuthEnableResponse{}, nil
		},
		roleAddFn: func(ctx context.Context, name string) (*clientv3.AuthRoleAddResponse, error) {
			roleAddCalls++
			if name != "root" {
				t.Fatalf("role name = %q, want root", name)
			}
			return &clientv3.AuthRoleAddResponse{}, nil
		},
		userGrantRoleFn: func(ctx context.Context, user, role string) (*clientv3.AuthUserGrantRoleResponse, error) {
			grantRoleCalls++
			if user != "root" || role != "root" {
				t.Fatalf("grant role = (%q, %q), want (root, root)", user, role)
			}
			return &clientv3.AuthUserGrantRoleResponse{}, nil
		},
	})

	out := captureStdout(t, func() {
		authEnableCommandFunc(cmd, nil)
	})

	if authEnableCalls != 2 {
		t.Fatalf("AuthEnable calls = %d, want 2", authEnableCalls)
	}
	if roleAddCalls != 1 {
		t.Fatalf("RoleAdd calls = %d, want 1", roleAddCalls)
	}
	if grantRoleCalls != 1 {
		t.Fatalf("UserGrantRole calls = %d, want 1", grantRoleCalls)
	}
	if out != "Authentication Enabled\n" {
		t.Fatalf("stdout = %q, want %q", out, "Authentication Enabled\n")
	}
}

func TestAuthStatusCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.AuthStatusResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		authStatusFn: func(ctx context.Context) (*clientv3.AuthStatusResponse, error) {
			clientCalled = true
			return resp, nil
		},
	})

	var gotResp *clientv3.AuthStatusResponse
	withTestDisplay(t, &recordingPrinter{
		authStatusFn: func(r *clientv3.AuthStatusResponse) {
			gotResp = r
		},
	})

	authStatusCommandFunc(cmd, nil)

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}
