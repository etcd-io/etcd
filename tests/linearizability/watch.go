// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/linearizability/model"
)

func collectClusterWatchEvents(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) [][]watchResponse {
	mux := sync.Mutex{}
	var wg sync.WaitGroup
	memberResponses := make([][]watchResponse, len(clus.Procs))
	for i, member := range clus.Procs {
		c, err := clientv3.New(clientv3.Config{
			Endpoints:            member.EndpointsV3(),
			Logger:               zap.NewNop(),
			DialKeepAliveTime:    1 * time.Millisecond,
			DialKeepAliveTimeout: 5 * time.Millisecond,
		})
		if err != nil {
			t.Fatal(err)
		}

		wg.Add(1)
		go func(i int, c *clientv3.Client) {
			defer wg.Done()
			defer c.Close()
			responses := watchMember(ctx, lg, c)
			mux.Lock()
			memberResponses[i] = responses
			mux.Unlock()
		}(i, c)
	}
	wg.Wait()
	return memberResponses
}

func watchMember(ctx context.Context, lg *zap.Logger, c *clientv3.Client) (resps []watchResponse) {
	var lastRevision int64 = 0
	for {
		select {
		case <-ctx.Done():
			return resps
		default:
		}
		for resp := range c.Watch(ctx, "", clientv3.WithPrefix(), clientv3.WithRev(lastRevision+1), clientv3.WithProgressNotify()) {
			resps = append(resps, watchResponse{resp, time.Now()})
			lastRevision = resp.Header.Revision
			if resp.Err() != nil {
				lg.Info("Watch error", zap.Error(resp.Err()))
			}
		}
	}
}

func validateWatchResponses(t *testing.T, responses [][]watchResponse, expectProgressNotify bool) {
	for _, memberResponses := range responses {
		validateMemberWatchResponses(t, memberResponses, expectProgressNotify)
	}
}

func validateMemberWatchResponses(t *testing.T, responses []watchResponse, expectProgressNotify bool) {
	var gotProgressNotify = false
	var lastRevision int64 = 1
	for _, resp := range responses {
		if resp.Header.Revision < lastRevision {
			t.Errorf("Revision should never decrease, last: %d, latest: %d", lastRevision, resp.Header.Revision)
		}
		if resp.IsProgressNotify() && resp.Header.Revision == lastRevision {
			gotProgressNotify = true
		}
		if resp.Header.Revision == lastRevision && len(resp.Events) != 0 {
			t.Errorf("Got two non-empty responses about same revision: %d", lastRevision)
		}
		for _, event := range resp.Events {
			if event.Kv.ModRevision != lastRevision+1 {
				t.Errorf("Expect revision to grow by 1, last: %d, mod: %d", lastRevision, event.Kv.ModRevision)
			}
			lastRevision = event.Kv.ModRevision
		}
		if resp.Header.Revision != lastRevision {
			t.Errorf("Expect response revision equal last event mod revision, last: %d, latest: %d", lastRevision, resp.Header.Revision)
		}
		lastRevision = resp.Header.Revision
	}
	if gotProgressNotify != expectProgressNotify {
		t.Errorf("Expected progress notify: %v, got: %v", expectProgressNotify, gotProgressNotify)
	}
}

func toWatchEvents(responses []watchResponse) (events []watchEvent) {
	for _, resp := range responses {
		for _, event := range resp.Events {
			var op model.OperationType
			switch event.Type {
			case mvccpb.PUT:
				op = model.Put
			case mvccpb.DELETE:
				op = model.Delete
			}
			events = append(events, watchEvent{
				Time:     resp.time,
				Revision: event.Kv.ModRevision,
				Op: model.EtcdOperation{
					Type:  op,
					Key:   string(event.Kv.Key),
					Value: model.ToValueOrHash(string(event.Kv.Value)),
				},
			})
		}
	}
	return events
}

type watchResponse struct {
	clientv3.WatchResponse
	time time.Time
}

type watchEvent struct {
	Op       model.EtcdOperation
	Revision int64
	Time     time.Time
}
