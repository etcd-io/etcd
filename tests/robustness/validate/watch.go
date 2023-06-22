// Copyright 2023 The etcd Authors
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

package validate

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

func validateWatch(t *testing.T, cfg Config, reports []traffic.ClientReport) []model.WatchEvent {
	// Validate etcd watch properties defined in https://etcd.io/docs/v3.6/learning/api_guarantees/#watch-apis
	for _, r := range reports {
		validateOrdered(t, r)
		validateUnique(t, cfg.ExpectRevisionUnique, r)
		validateAtomic(t, r)
		// TODO: Validate Resumable
		validateBookmarkable(t, r)
	}
	validateEventsMatch(t, reports)
	// Expects that longest history encompasses all events.
	// TODO: Use combined events from all histories instead of the longest history.
	eventHistory := longestEventHistory(reports)
	// TODO: Validate that each watch report is reliable, not only the longest one.
	validateReliable(t, eventHistory)
	return watchEvents(eventHistory)
}

func validateBookmarkable(t *testing.T, report traffic.ClientReport) {
	var lastProgressNotifyRevision int64 = 0
	for _, resp := range report.Watch {
		for _, event := range resp.Events {
			if event.Revision <= lastProgressNotifyRevision {
				t.Errorf("Broke watch guarantee: Bookmarkable - Progress notification events guarantee that all events up to a revision have been already delivered, eventRevision: %d, progressNotifyRevision: %d", event.Revision, lastProgressNotifyRevision)
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
				}{event.Revision, event.Key}
			}
			if _, found := uniqueOperations[key]; found {
				t.Errorf("Broke watch guarantee: Unique - an event will never appear on a watch twice, key: %q, revision: %d, client: %d", event.Key, event.Revision, report.ClientId)
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

func validateEventsMatch(t *testing.T, reports []traffic.ClientReport) {
	type revisionKey struct {
		revision int64
		key      string
	}
	type eventClientId struct {
		model.WatchEvent
		ClientId int
	}
	revisionKeyToEvent := map[revisionKey]eventClientId{}
	for _, r := range reports {
		for _, resp := range r.Watch {
			for _, event := range resp.Events {
				rk := revisionKey{key: event.Key, revision: event.Revision}
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

func watchEvents(timed []traffic.TimedWatchEvent) []model.WatchEvent {
	result := make([]model.WatchEvent, 0, len(timed))
	for _, event := range timed {
		result = append(result, event.WatchEvent)
	}
	return result
}
