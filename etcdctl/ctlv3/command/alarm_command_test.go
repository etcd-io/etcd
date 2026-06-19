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

func TestAlarmListCommandFunc_UsesInjectedClient(t *testing.T) {
	cmd := newTestCommand(t)
	resp := &clientv3.AlarmResponse{}

	var clientCalled bool
	withTestClient(t, &fakeCommandClient{
		alarmListFn: func(ctx context.Context) (*clientv3.AlarmResponse, error) {
			clientCalled = true
			return resp, nil
		},
	})

	var gotResp *clientv3.AlarmResponse
	withTestDisplay(t, &recordingPrinter{
		alarmFn: func(r *clientv3.AlarmResponse) {
			gotResp = r
		},
	})

	alarmListCommandFunc(cmd, nil)

	if !clientCalled {
		t.Fatal("expected fake client to be called")
	}
	if gotResp != resp {
		t.Fatalf("display response = %p, want %p", gotResp, resp)
	}
}
