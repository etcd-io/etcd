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
	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

func collectClusterWatchEvents(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, maxRevisionChan <-chan int64, cfg watchConfig, baseTime time.Time, ids identity.Provider) []traffic.ClientReport {
	mux := sync.Mutex{}
	var wg sync.WaitGroup
	reports := make([]traffic.ClientReport, len(clus.Procs))
	memberMaxRevisionChans := make([]chan int64, len(clus.Procs))
	for i, member := range clus.Procs {
		c, err := clientv3.New(clientv3.Config{
			Endpoints:            member.EndpointsGRPC(),
			Logger:               zap.NewNop(),
			DialKeepAliveTime:    10 * time.Second,
			DialKeepAliveTimeout: 100 * time.Millisecond,
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
			responses := watchMember(ctx, t, c, memberChan, cfg, baseTime)
			mux.Lock()
			reports[i] = traffic.ClientReport{
				ClientId: ids.NewClientId(),
				Watch:    responses,
			}
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
	return reports
}

type watchConfig struct {
	requestProgress      bool
	expectUniqueRevision bool
}

// watchMember collects all responses until context is cancelled, it has observed revision provided via maxRevisionChan or maxRevisionChan was closed.
// TODO: Use traffic.RecordingClient instead of clientv3.Client
func watchMember(ctx context.Context, t *testing.T, c *clientv3.Client, maxRevisionChan <-chan int64, cfg watchConfig, baseTime time.Time) (resps []traffic.WatchResponse) {
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
		case revision, ok := <-maxRevisionChan:
			if ok {
				maxRevision = revision
				if lastRevision >= maxRevision {
					cancel()
				}
			} else {
				// Only cancel if maxRevision was never set.
				if maxRevision == 0 {
					cancel()
				}
			}
		case resp := <-watch:
			if cfg.requestProgress {
				c.RequestProgress(ctx)
			}
			if resp.Err() == nil {
				resps = append(resps, traffic.ToWatchResponse(resp, baseTime))
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

func watchResponsesMaxRevision(responses []traffic.WatchResponse) int64 {
	var maxRevision int64
	for _, response := range responses {
		for _, event := range response.Events {
			if event.Revision > maxRevision {
				maxRevision = event.Revision
			}
		}
	}
	return maxRevision
}

func validateWatchCorrectness(t *testing.T, cfg watchConfig, reports []traffic.ClientReport) {
	// Validate etcd watch properties defined in https://etcd.io/docs/v3.6/learning/api_guarantees/#watch-apis
	for _, r := range reports {
		validateOrdered(t, r)
		validateUnique(t, cfg.expectUniqueRevision, r)
		validateAtomic(t, r)
		// TODO: Validate Resumable
		validateBookmarkable(t, r)
	}
	validateEventsMatch(t, reports)
	// Expects that longest history encompasses all events.
	// TODO: Use combined events from all histories instead of the longest history.
	// TODO: Validate that each watch report is reliable, not only the longest one.
	validateReliable(t, longestEventHistory(reports))
}

func validateGotAtLeastOneProgressNotify(t *testing.T, reports []traffic.ClientReport, expectProgressNotify bool) {
	var gotProgressNotify = false
	for _, r := range reports {
		var lastHeadRevision int64 = 1
		for _, resp := range r.Watch {
			if resp.IsProgressNotify && resp.Revision == lastHeadRevision {
				gotProgressNotify = true
				break
			}
			lastHeadRevision = resp.Revision
		}
		if gotProgressNotify {
			break
		}
	}
	if gotProgressNotify != expectProgressNotify {
		t.Errorf("Progress notify does not match, expect: %v, got: %v", expectProgressNotify, gotProgressNotify)
	}
}

func validateBookmarkable(t *testing.T, report traffic.ClientReport) {
	var lastProgressNotifyRevision int64 = 0
	for _, resp := range report.Watch {
		for _, event := range resp.Events {
			if event.Revision <= lastProgressNotifyRevision {
				t.Errorf("Broke watch guarantee: Renewable - watch can renewed using revision in last progress notification; Progress notification guarantees that previous events have been already delivered, eventRevision: %d, progressNotifyRevision: %d", event.Revision, lastProgressNotifyRevision)
			}
		}
		if resp.IsProgressNotify {
			lastProgressNotifyRevision = resp.Revision
		}
	}
}

func validateOrdered(t *testing.T, report traffic.ClientReport) {
	var lastEventRevision int64 = 1
	for _, resp := range report.Watch {
		for _, event := range resp.Events {
			if event.Revision < lastEventRevision {
				t.Errorf("Broke watch guarantee: Ordered - events are ordered by revision; an event will never appear on a watch if it precedes an event in time that has already been posted, lastRevision: %d, currentRevision: %d, client: %d", lastEventRevision, event.Revision, report.ClientId)
			}
			lastEventRevision = event.Revision
		}
	}
}

func validateUnique(t *testing.T, expectUniqueRevision bool, report traffic.ClientReport) {
	uniqueOperations := map[interface{}]struct{}{}

	for _, resp := range report.Watch {
		for _, event := range resp.Events {
			var key interface{}
			if expectUniqueRevision {
				key = event.Revision
			} else {
				key = struct {
					revision int64
					key      string
				}{event.Revision, event.Op.Key}
			}

			if _, found := uniqueOperations[key]; found {
				t.Errorf("Broke watch guarantee: Unique - an event will never appear on a watch twice, key: %q, revision: %d, client: %d", event.Op.Key, event.Revision, report.ClientId)
			}
			uniqueOperations[key] = struct{}{}
		}
	}
}

func validateAtomic(t *testing.T, report traffic.ClientReport) {
	var lastEventRevision int64 = 1
	for _, resp := range report.Watch {
		if len(resp.Events) > 0 {
			if resp.Events[0].Revision == lastEventRevision {
				t.Errorf("Broke watch guarantee: Atomic - a list of events is guaranteed to encompass complete revisions; updates in the same revision over multiple keys will not be split over several lists of events, previousListEventRevision: %d, currentListEventRevision: %d, client: %d", lastEventRevision, resp.Events[0].Revision, report.ClientId)
			}
			lastEventRevision = resp.Events[len(resp.Events)-1].Revision
		}
	}
}

func validateReliable(t *testing.T, events []traffic.TimedWatchEvent) {
	var lastEventRevision int64 = 1
	for _, event := range events {
		if event.Revision > lastEventRevision && event.Revision != lastEventRevision+1 {
			t.Errorf("Broke watch guarantee: Reliable - a sequence of events will never drop any subsequence of events; if there are events ordered in time as a < b < c, then if the watch receives events a and c, it is guaranteed to receive b, missing revisions from range: %d-%d", lastEventRevision, event.Revision)
		}
		lastEventRevision = event.Revision
	}
}

func toWatchEvents(responses []traffic.WatchResponse) (events []traffic.TimedWatchEvent) {
	for _, resp := range responses {
		for _, event := range resp.Events {
			events = append(events, traffic.TimedWatchEvent{
				Time:       resp.Time,
				WatchEvent: event,
			})
		}
	}
	return events
}

func operationsFromClientReports(reports []traffic.ClientReport) []porcupine.Operation {
	operations := []porcupine.Operation{}
	persisted := map[model.EtcdOperation]traffic.TimedWatchEvent{}
	for _, r := range reports {
		operations = append(operations, r.OperationHistory.Operations()...)
		for _, resp := range r.Watch {
			for _, event := range resp.Events {
				persisted[event.Op] = traffic.TimedWatchEvent{Time: resp.Time, WatchEvent: event}
			}
		}
	}
	newOperations := make([]porcupine.Operation, 0, len(operations))
	lastObservedOperation := lastOperationObservedInWatch(operations, persisted)

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.EtcdNonDeterministicResponse)
		if resp.Err == nil || op.Call > lastObservedOperation.Call || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		event := matchWatchEvent(request.Txn, persisted)
		if event != nil {
			// Set revision and time based on watchEvent.
			op.Return = event.Time.Nanoseconds()
			op.Output = model.EtcdNonDeterministicResponse{
				EtcdResponse:  model.EtcdResponse{Revision: event.Revision},
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

func lastOperationObservedInWatch(operations []porcupine.Operation, watchEvents map[model.EtcdOperation]traffic.TimedWatchEvent) porcupine.Operation {
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

func matchWatchEvent(request *model.TxnRequest, watchEvents map[model.EtcdOperation]traffic.TimedWatchEvent) *traffic.TimedWatchEvent {
	for _, etcdOp := range append(request.OperationsOnSuccess, request.OperationsOnFailure...) {
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
	for _, etcdOp := range request.OperationsOnSuccess {
		if etcdOp.Type == model.Put || etcdOp.Type == model.Delete {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(request *model.TxnRequest) bool {
	for _, etcdOp := range request.OperationsOnSuccess {
		if etcdOp.Type == model.Put {
			return true
		}
	}
	return false
}

func validateEventsMatch(t *testing.T, reports []traffic.ClientReport) {
	type revisionKey struct {
		revision int64
		key      string
	}
	type eventClientId struct {
		traffic.WatchEvent
		ClientId int
	}
	revisionKeyToEvent := map[revisionKey]eventClientId{}
	for _, r := range reports {
		for _, resp := range r.Watch {
			for _, event := range resp.Events {
				rk := revisionKey{key: event.Op.Key, revision: event.Revision}
				if prev, found := revisionKeyToEvent[rk]; found {
					if prev.WatchEvent != event {
						t.Errorf("Events between clients %d and %d don't match, key: %q, revision: %d, diff: %s", prev.ClientId, r.ClientId, rk.key, rk.revision, cmp.Diff(prev, event))
					}
				}
				revisionKeyToEvent[rk] = eventClientId{ClientId: r.ClientId, WatchEvent: event}
			}
		}
	}
}

func longestEventHistory(report []traffic.ClientReport) []traffic.TimedWatchEvent {
	longestIndex := 0
	longestEventCount := 0
	for i, r := range report {
		rEventCount := r.WatchEventCount()
		if rEventCount > longestEventCount {
			longestIndex = i
			longestEventCount = rEventCount
		}
	}
	return toWatchEvents(report[longestIndex].Watch)
}
