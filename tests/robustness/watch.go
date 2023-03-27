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

package robustness

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func collectClusterWatchEvents(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, maxRevisionChan <-chan int64) [][]watchResponse {
	mux := sync.Mutex{}
	var wg sync.WaitGroup
	memberResponses := make([][]watchResponse, len(clus.Procs))
	memberMaxRevisionChans := make([]chan int64, len(clus.Procs))
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
		memberChan := make(chan int64, 1)
		memberMaxRevisionChans[i] = memberChan
		wg.Add(1)
		go func(i int, c *clientv3.Client) {
			defer wg.Done()
			defer c.Close()
			responses := watchMember(ctx, t, c, memberChan)
			mux.Lock()
			memberResponses[i] = responses
			mux.Unlock()
		}(i, c)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		maxRevision := <-maxRevisionChan
		for _, memberChan := range memberMaxRevisionChans {
			memberChan <- maxRevision
		}
	}()
	wg.Wait()
	return memberResponses
}

// watchMember collects all responses until context is cancelled or has observed revision provided via maxRevisionChan.
func watchMember(ctx context.Context, t *testing.T, c *clientv3.Client, maxRevisionChan <-chan int64) (resps []watchResponse) {
	var maxRevision int64 = 0
	var lastRevision int64 = 0
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watch := c.Watch(ctx, "", clientv3.WithPrefix(), clientv3.WithRev(1), clientv3.WithProgressNotify())
	for {
		select {
		case <-ctx.Done():
			revision := watchResponsesMaxRevision(resps)
			if maxRevision == 0 {
				t.Errorf("Client didn't collect all events, max revision not set")
			}
			if revision < maxRevision {
				t.Errorf("Client didn't collect all events, revision got %d, expected: %d", revision, maxRevision)
			}
			return resps
		case maxRevision = <-maxRevisionChan:
			if lastRevision >= maxRevision {
				cancel()
			}
		case resp := <-watch:
			if resp.Err() == nil {
				resps = append(resps, watchResponse{resp, time.Now()})
			} else if !resp.Canceled {
				t.Errorf("Watch stream received error, err %v", resp.Err())
			}
			// Assumes that we track all events as we watch all keys.
			if len(resp.Events) > 0 {
				lastRevision = resp.Events[len(resp.Events)-1].Kv.ModRevision
			}
			if maxRevision != 0 && lastRevision >= maxRevision {
				cancel()
			}
		}
	}
}

func watchResponsesMaxRevision(responses []watchResponse) int64 {
	var maxRevision int64
	for _, response := range responses {
		for _, event := range response.Events {
			if event.Kv.ModRevision > maxRevision {
				maxRevision = event.Kv.ModRevision
			}
		}
	}
	return maxRevision
}

func validateWatchResponses(t *testing.T, responses [][]watchResponse, expectProgressNotify bool) {
	for _, memberResponses := range responses {
		validateMemberWatchResponses(t, memberResponses, expectProgressNotify)
	}
}

func validateMemberWatchResponses(t *testing.T, responses []watchResponse, expectProgressNotify bool) {
	var (
		gotProgressNotify                = false
		lastEventModRevision       int64 = 1 // The event.Kv.ModRevision in the latest event.
		lastHeadRevision           int64 = 1 // The resp.Header.Revision in last watch response.
		lastProgressNotifyRevision int64 = 0 // The resp.Header.Revision in the last progress notify watch response.
	)
	type revisionKey struct {
		revision int64
		key      string
	}
	uniqueOperations := map[revisionKey]struct{}{}

	for _, resp := range responses {
		if resp.Header.Revision < lastHeadRevision {
			t.Errorf("Server revision should never decrease, lastHeadRevision: %d, resp.Header.Revision: %d",
				lastHeadRevision, resp.Header.Revision)
		}

		if resp.IsProgressNotify() && resp.Header.Revision == lastHeadRevision {
			gotProgressNotify = true
		}
		if resp.Header.Revision == lastHeadRevision && len(resp.Events) != 0 {
			t.Errorf("Got two non-empty responses about same revision")
		}
		if len(resp.Events) > 0 && resp.Events[0].Kv.ModRevision <= lastEventModRevision {
			t.Errorf("Events with same revision should not be split between multiple responses, lastResponseModRevision: %d, firstEventModRevision: %d", lastEventModRevision, resp.Events[0].Kv.ModRevision)
		}

		for _, event := range resp.Events {
			rk := revisionKey{key: string(event.Kv.Key), revision: event.Kv.ModRevision}
			if _, found := uniqueOperations[rk]; found {
				t.Errorf("BROKE: Within the:wq same revision key can only be modified once. Suspecting duplicate watch event. key: %q, revision: %d", rk.key, rk.revision)
			}
			uniqueOperations[rk] = struct{}{}
			if event.Kv.ModRevision <= lastProgressNotifyRevision {
				t.Errorf("BROKE: etcd will not send progress notification to a watcher until it has synced all events. So a watcher will never receive an event with a lower `ModRevision` than the last progressNotification's revision, eventRevision: %d, progressNotifyRevision: %d", event.Kv.ModRevision, lastProgressNotifyRevision)
			}
			if event.Kv.ModRevision != lastEventModRevision && event.Kv.ModRevision != lastEventModRevision+1 {
				t.Errorf("Event mod revision should stay the same or increment by one, last: %d, current: %d", lastEventModRevision, event.Kv.ModRevision)
			}
			lastEventModRevision = event.Kv.ModRevision
		}

		if resp.Header.Revision < lastEventModRevision {
			t.Errorf("Event revision should never exceed the server's revision, lastEventRevision: %d, resp.Header.Revision: %d",
				lastEventModRevision, resp.Header.Revision)
		}

		if resp.IsProgressNotify() {
			lastProgressNotifyRevision = resp.Header.Revision
		}
		lastHeadRevision = resp.Header.Revision
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

func patchOperationBasedOnWatchEvents(operations []porcupine.Operation, watchEvents []watchEvent) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))
	persisted := map[model.EtcdOperation]watchEvent{}
	for _, op := range watchEvents {
		persisted[op.Op] = op
	}
	lastObservedOperation := lastOperationObservedInWatch(operations, persisted)

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.EtcdResponse)
		if resp.Err == nil || op.Call > lastObservedOperation.Call || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		event := matchWatchEvent(request.Txn, persisted)
		if event != nil {
			// Set revision and time based on watchEvent.
			op.Return = event.Time.UnixNano()
			op.Output = model.EtcdResponse{
				Revision:      event.Revision,
				ResultUnknown: true,
			}
			newOperations = append(newOperations, op)
			continue
		}
		if hasNonUniqueWriteOperation(request.Txn) && !hasUniqueWriteOperation(request.Txn) {
			// Leave operation as it is as we cannot match non-unique operations to watch events.
			newOperations = append(newOperations, op)
			continue
		}
		// Remove non persisted operations
	}
	return newOperations
}

func lastOperationObservedInWatch(operations []porcupine.Operation, watchEvents map[model.EtcdOperation]watchEvent) porcupine.Operation {
	var maxCallTime int64
	var lastOperation porcupine.Operation
	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		if request.Type != model.Txn {
			continue
		}
		event := matchWatchEvent(request.Txn, watchEvents)
		if event != nil && op.Call > maxCallTime {
			maxCallTime = op.Call
			lastOperation = op
		}
	}
	return lastOperation
}

func matchWatchEvent(request *model.TxnRequest, watchEvents map[model.EtcdOperation]watchEvent) *watchEvent {
	for _, etcdOp := range request.Ops {
		if etcdOp.Type == model.Put {
			// Remove LeaseID which is not exposed in watch.
			event, ok := watchEvents[model.EtcdOperation{
				Type:  etcdOp.Type,
				Key:   etcdOp.Key,
				Value: etcdOp.Value,
			}]
			if ok {
				return &event
			}
		}
	}
	return nil
}

func hasNonUniqueWriteOperation(request *model.TxnRequest) bool {
	for _, etcdOp := range request.Ops {
		if etcdOp.Type == model.Put || etcdOp.Type == model.Delete {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(request *model.TxnRequest) bool {
	for _, etcdOp := range request.Ops {
		if etcdOp.Type == model.Put {
			return true
		}
	}
	return false
}

func watchEvents(responses [][]watchResponse) [][]watchEvent {
	ops := make([][]watchEvent, len(responses))
	for i, resps := range responses {
		ops[i] = toWatchEvents(resps)
	}
	return ops
}

func validateEventsMatch(t *testing.T, histories [][]watchEvent) {
	longestHistory := longestHistory(histories)
	for i := 0; i < len(histories); i++ {
		length := len(histories[i])
		// We compare prefix of watch events, as we are not guaranteed to collect all events from each node.
		if diff := cmp.Diff(longestHistory[:length], histories[i][:length], cmpopts.IgnoreFields(watchEvent{}, "Time")); diff != "" {
			t.Error("Events in watches do not match")
		}
	}
}

func longestHistory(histories [][]watchEvent) []watchEvent {
	longestIndex := 0
	for i, history := range histories {
		if len(history) > len(histories[longestIndex]) {
			longestIndex = i
		}
	}
	return histories[longestIndex]
}
