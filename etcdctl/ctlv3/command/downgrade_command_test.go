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

func TestDowngradeEnableCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.DowngradeResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		downgradeFn: func(ctx context.Context, action clientv3.DowngradeAction, version string) (*clientv3.DowngradeResponse, error) {
			clientCalled = true
			if action != clientv3.DowngradeEnable {
				t.Fatalf("action = %v, want %v", action, clientv3.DowngradeEnable)
			}
			if version != "3.5" {
				t.Fatalf("version = %q, want %q", version, "3.5")
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.DowngradeResponse
	withTestDisplay(t, &recordingPrinter{
		downgradeEnableFn: func(r *clientv3.DowngradeResponse) {
			gotResp = r
		},
	})

	downgradeEnableCommandFunc(cmd, []string{"3.5"})

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}

func TestDowngradeCancelCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.DowngradeResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		downgradeFn: func(ctx context.Context, action clientv3.DowngradeAction, version string) (*clientv3.DowngradeResponse, error) {
			clientCalled = true
			if action != clientv3.DowngradeCancel {
				t.Fatalf("action = %v, want %v", action, clientv3.DowngradeCancel)
			}
			if version != "" {
				t.Fatalf("version = %q, want empty string", version)
			}
			return resp, nil
		},
	})

	var gotResp *clientv3.DowngradeResponse
	withTestDisplay(t, &recordingPrinter{
		downgradeCancelFn: func(r *clientv3.DowngradeResponse) {
			gotResp = r
		},
	})

	downgradeCancelCommandFunc(cmd, nil)

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}
