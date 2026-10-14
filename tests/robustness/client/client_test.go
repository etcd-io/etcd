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

package client

import (
	"context"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

type mockWatcher struct {
	ch chan clientv3.WatchResponse
}

func (m *mockWatcher) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return m.ch
}

func (m *mockWatcher) RequestProgress(ctx context.Context) error {
	return nil
}

func (m *mockWatcher) Close() error {
	return nil
}

func TestRecordingClientWatchRace(t *testing.T) {
	baseTime := time.Now()
	ids := identity.NewIDProvider()

	watchChan := make(chan clientv3.WatchResponse, 10)
	watcher := &mockWatcher{ch: watchChan}

	cc := clientv3.NewCtxClient(context.Background())
	cc.Watcher = watcher

	c := &RecordingClient{
		ID:           1,
		client:       cc,
		kvOperations: model.NewAppendableHistory(ids),
		baseTime:     baseTime,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := model.WatchRequest{Key: "foo"}
	_ = c.watch(ctx, request)

	go func() {
		time.Sleep(2 * time.Millisecond)
		watchChan <- clientv3.WatchResponse{
			Header: &etcdserverpb.ResponseHeader{Revision: 2},
		}
		time.Sleep(2 * time.Millisecond)
		cancel()
		close(watchChan)
	}()

	c.Close()

	report := c.Report()
	for _, op := range report.Watch {
		_ = op.Responses
	}
}
