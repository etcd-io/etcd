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
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/linearizability/model"
)

func collectClusterWatchEvents(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) [][]watchEvent {
	mux := sync.Mutex{}
	var wg sync.WaitGroup
	memberEvents := make([][]watchEvent, len(clus.Procs))
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
			events := collectMemberWatchEvents(ctx, lg, c)
			mux.Lock()
			memberEvents[i] = events
			mux.Unlock()
		}(i, c)
	}
	wg.Wait()
	return memberEvents
}

func collectMemberWatchEvents(ctx context.Context, lg *zap.Logger, c *clientv3.Client) []watchEvent {
	gotProcessNotify := false
	events := []watchEvent{}
	operations := map[model.EtcdOperation]clientv3.WatchResponse{}
	var lastRevision int64 = 0
	for {
		select {
		case <-ctx.Done():
			if !gotProcessNotify {
				panic("Expected at least one notify")
			}
			return events
		default:
		}
		for resp := range c.Watch(ctx, "", clientv3.WithPrefix(), clientv3.WithRev(lastRevision+1), clientv3.WithProgressNotify()) {
			if resp.Header.Revision < lastRevision {
				panic("Revision should never decrease")
			}
			if resp.Header.Revision == lastRevision && len(resp.Events) != 0 {
				//panic("Got two non-empty responses about same revision")
			}
			if resp.IsProgressNotify() {
				gotProcessNotify = true
			}
			/*err := c.RequestProgress(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				panic(err)
			}*/
			lastRevision = resp.Header.Revision
			time := time.Now()
			for _, event := range resp.Events {
				var op model.OperationType
				switch event.Type {
				case mvccpb.PUT:
					op = model.Put
				case mvccpb.DELETE:
					op = model.Delete
				}
				event := watchEvent{
					Time:     time,
					Revision: event.Kv.ModRevision,
					Op: model.EtcdOperation{
						Type:  op,
						Key:   string(event.Kv.Key),
						Value: model.ToValueOrHash(string(event.Kv.Value)),
					},
				}
				if prev, found := operations[event.Op]; found {
					currentResponse, err := json.MarshalIndent(resp, "", "    ")
					if err != nil {
						panic(fmt.Sprintf("Failed to marshal response, err: %v", err))
					}
					previousResponse, err := json.MarshalIndent(prev, "", "    ")
					if err != nil {
						panic(fmt.Sprintf("Failed to marshal response, err: %v", err))
					}

					if resp.Events[0].Type != mvccpb.DELETE && prev.Events[0].Type != mvccpb.DELETE {
						panic(fmt.Sprintf("duplicate operation in two responses on revision %d\n---\nfirst:\n%s\n---\nsecond:\n%s", event.Revision, previousResponse, currentResponse))
					}
				}
				operations[event.Op] = resp
				events = append(events, event)
			}
			if resp.Err() != nil {
				lg.Info("Watch error", zap.Error(resp.Err()))
			}
		}
	}
}

type watchEvent struct {
	Op       model.EtcdOperation
	Revision int64
	Time     time.Time
}
