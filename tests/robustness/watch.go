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
	// Validate watch is correctly configured to ensure proper testing
	validateGotAtLeastOneProgressNotify(t, responses, expectProgressNotify)

	// Validate etcd watch properties defined in https://etcd.io/docs/v3.6/learning/api/#watch-streams
	validateEventsAreReliable(t, responses)
	validateEventsAreUnique(t, responses)
	validateWatchIsAtomic(t, responses)
	validateWatchIsResumable(t, responses)
}

func validateGotAtLeastOneProgressNotify(t *testing.T, responses []watchResponse, expectProgressNotify bool) {
	var gotProgressNotify = false
	var lastHeadRevision int64 = 1
	for _, resp := range responses {
		if resp.IsProgressNotify() && resp.Header.Revision == lastHeadRevision {
			gotProgressNotify = true
			break
		}
		lastHeadRevision = resp.Header.Revision
	}
	if gotProgressNotify != expectProgressNotify {
		t.Errorf("Expected at least one progress notify: %v, got: %v", expectProgressNotify, gotProgressNotify)
	}
}

func validateWatchIsResumable(t *testing.T, responses []watchResponse) {
	var lastProgressNotifyRevision int64 = 0
	for _, resp := range responses {
		for _, event := range resp.Events {
			if event.Kv.ModRevision <= lastProgressNotifyRevision {
				t.Errorf("BROKE: Watch is resumable - watch can resumed using revision in last progress notification; Progress notification guarantees that previous events have been already delivered, eventRevision: %d, progressNotifyRevision: %d", event.Kv.ModRevision, lastProgressNotifyRevision)
			}
		}
		if resp.IsProgressNotify() {
			lastProgressNotifyRevision = resp.Header.Revision
		}
	}
}

func validateEventsAreReliable(t *testing.T, responses []watchResponse) {
	var lastEventRevision int64 = 1
	for _, resp := range responses {
		for _, event := range resp.Events {
			if event.Kv.ModRevision != lastEventRevision && event.Kv.ModRevision != lastEventRevision+1 {
				t.Errorf("BROKE: Events are reliable - a sequence of events will never drop any subsequence of events; if there are events ordered in time as a < b < c, then if the watch receives events a and c, it is guaranteed to receive b, lastRevision: %d, currentRevision: %d", lastEventRevision, event.Kv.ModRevision)
			}
			lastEventRevision = event.Kv.ModRevision
		}
	}
}

func validateEventsAreUnique(t *testing.T, responses []watchResponse) {
	type revisionKey struct {
		revision int64
		key      string
	}
	uniqueOperations := map[revisionKey]struct{}{}
	for _, resp := range responses {
		for _, event := range resp.Events {
			rk := revisionKey{key: string(event.Kv.Key), revision: event.Kv.ModRevision}
			if _, found := uniqueOperations[rk]; found {
				t.Errorf("BROKE: Events are unique - an event will never appear on a watch twice, key: %q, revision: %d", rk.key, rk.revision)
			}
			uniqueOperations[rk] = struct{}{}
		}
	}
}

func validateWatchIsAtomic(t *testing.T, responses []watchResponse) {
	var lastEventRevision int64 = 1
	for _, resp := range responses {
		if len(resp.Events) > 0 {
			if resp.Events[0].Kv.ModRevision == lastEventRevision {
				t.Errorf("BROKE: Watch is atomic - a list of events is guaranteed to encompass complete revisions; updates in the same revision over multiple keys will not be split over several lists of events, previousListEventRevision: %d, currentListEventRevision: %d", lastEventRevision, resp.Events[0].Kv.ModRevision)
			}
			lastEventRevision = resp.Events[len(resp.Events)-1].Kv.ModRevision
		}
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
