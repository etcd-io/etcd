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

func TestRoleAddCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.AuthRoleAddResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		roleAddFn: func(ctx context.Context, name string) (*clientv3.AuthRoleAddResponse, error) {
			clientCalled = true
			if name != "ops-admin" {
				t.Fatalf("RoleAdd name = %q, want %q", name, "ops-admin")
			}
			return resp, nil
		},
	})

	var gotRole string
	var gotResp *clientv3.AuthRoleAddResponse
	withTestDisplay(t, &recordingPrinter{
		roleAddFn: func(role string, r *clientv3.AuthRoleAddResponse) {
			gotRole = role
			gotResp = r
		},
	})

	roleAddCommandFunc(cmd, []string{"ops-admin"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotRole != "ops-admin" {
		t.Fatalf("display role = %q, want %q", gotRole, "ops-admin")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestRoleGrantPermissionCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.AuthRoleGrantPermissionResponse{}

	rolePermPrefix = true
	t.Cleanup(func() {
		rolePermPrefix = false
		rolePermFromKey = false
	})

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		roleGrantPermissionFn: func(ctx context.Context, role, key, rangeEnd string, permType clientv3.PermissionType) (*clientv3.AuthRoleGrantPermissionResponse, error) {
			clientCalled = true
			if role != "writers" {
				t.Fatalf("role = %q, want %q", role, "writers")
			}
			if key != "app/" {
				t.Fatalf("key = %q, want %q", key, "app/")
			}
			wantEnd := clientv3.GetPrefixRangeEnd("app/")
			if rangeEnd != wantEnd {
				t.Fatalf("rangeEnd = %q, want %q", rangeEnd, wantEnd)
			}
			wantPerm := clientv3.PermissionType(clientv3.PermRead)
			if permType != wantPerm {
				t.Fatalf("permType = %v, want %v", permType, wantPerm)
			}
			return resp, nil
		},
	})

	var gotRole string
	var gotResp *clientv3.AuthRoleGrantPermissionResponse
	withTestDisplay(t, &recordingPrinter{
		roleGrantPermissionFn: func(role string, r *clientv3.AuthRoleGrantPermissionResponse) {
			gotRole = role
			gotResp = r
		},
	})

	roleGrantPermissionCommandFunc(cmd, []string{"writers", "read", "app/"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotRole != "writers" {
		t.Fatalf("display role = %q, want %q", gotRole, "writers")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestRangeEndFromPermFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		prefix      bool
		fromKey     bool
		want        string
		wantErrText string
	}{
		{
			name: "single key",
			args: []string{"key"},
		},
		{
			name:   "prefix range",
			args:   []string{"key"},
			prefix: true,
			want:   clientv3.GetPrefixRangeEnd("key"),
		},
		{
			name:    "from key range",
			args:    []string{"key"},
			fromKey: true,
			want:    "\x00",
		},
		{
			name:        "prefix and from-key conflict",
			args:        []string{"key"},
			prefix:      true,
			fromKey:     true,
			wantErrText: "--from-key and --prefix flags are mutually exclusive",
		},
		{
			name: "explicit end key",
			args: []string{"key", "end"},
			want: "end",
		},
		{
			name:        "prefix rejects explicit end key",
			args:        []string{"key", "end"},
			prefix:      true,
			wantErrText: "unexpected endkey argument with --prefix flag",
		},
		{
			name:        "from-key rejects explicit end key",
			args:        []string{"key", "end"},
			fromKey:     true,
			wantErrText: "unexpected endkey argument with --from-key flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rolePermPrefix = tt.prefix
			rolePermFromKey = tt.fromKey
			t.Cleanup(func() {
				rolePermPrefix = false
				rolePermFromKey = false
			})

			got, err := rangeEndFromPermFlags(tt.args)
			if tt.wantErrText != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErrText)
				}
				if err.Error() != tt.wantErrText {
					t.Fatalf("unexpected error: got %q want %q", err.Error(), tt.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("rangeEndFromPermFlags(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestPermRange(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		prefix  bool
		fromKey bool
		wantKey string
		wantEnd string
	}{
		{
			name:    "empty key without flags",
			args:    []string{""},
			wantKey: "\x00",
		},
		{
			name:    "empty key with prefix",
			args:    []string{""},
			prefix:  true,
			wantKey: "\x00",
			wantEnd: "\x00",
		},
		{
			name:    "empty key with from-key",
			args:    []string{""},
			fromKey: true,
			wantKey: "\x00",
			wantEnd: "\x00",
		},
		{
			name:    "non-empty key with prefix",
			args:    []string{"abc"},
			prefix:  true,
			wantKey: "abc",
			wantEnd: clientv3.GetPrefixRangeEnd("abc"),
		},
		{
			name:    "non-empty key with explicit end key",
			args:    []string{"abc", "abd"},
			wantKey: "abc",
			wantEnd: "abd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rolePermPrefix = tt.prefix
			rolePermFromKey = tt.fromKey
			t.Cleanup(func() {
				rolePermPrefix = false
				rolePermFromKey = false
			})

			gotKey, gotEnd := permRange(tt.args)
			if gotKey != tt.wantKey || gotEnd != tt.wantEnd {
				t.Fatalf("permRange(%v) = (%q, %q), want (%q, %q)", tt.args, gotKey, gotEnd, tt.wantKey, tt.wantEnd)
			}
		})
	}
}
