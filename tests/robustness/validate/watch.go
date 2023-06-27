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
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/tests/v3/robustness/report"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func validateWatch(t *testing.T, cfg Config, reports []report.ClientReport) []model.WatchEvent {
	// Validate etcd watch properties defined in https://etcd.io/docs/v3.6/learning/api_guarantees/#watch-apis
	for _, r := range reports {
		validateOrdered(t, r)
		validateUnique(t, cfg.ExpectRevisionUnique, r)
		validateAtomic(t, r)
		validateBookmarkable(t, r)
	}
	// TODO: Use linearization result instead of event history to get order of events
	// This is currently impossible as porcupine doesn't expose operation order created during linearization.
	eventHistory := mergeWatchEventHistory(t, reports)
	for _, r := range reports {
		validateReliable(t, eventHistory, r)
		validateResumable(t, eventHistory, r)
	}
	return eventHistory
}

func validateBookmarkable(t *testing.T, report report.ClientReport) {
	var lastProgressNotifyRevision int64 = 0
	for _, op := range report.Watch {
		for _, resp := range op.Responses {
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
}

func validateOrdered(t *testing.T, report report.ClientReport) {
	var lastEventRevision int64 = 1
	for _, op := range report.Watch {
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				if event.Revision < lastEventRevision {
					t.Errorf("Broke watch guarantee: Ordered - events are ordered by revision; an event will never appear on a watch if it precedes an event in time that has already been posted, lastRevision: %d, currentRevision: %d, client: %d", lastEventRevision, event.Revision, report.ClientId)
				}
				lastEventRevision = event.Revision
			}
		}
	}
}

func validateUnique(t *testing.T, expectUniqueRevision bool, report report.ClientReport) {
	uniqueOperations := map[interface{}]struct{}{}

	for _, op := range report.Watch {
		for _, resp := range op.Responses {
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
}

func validateAtomic(t *testing.T, report report.ClientReport) {
	var lastEventRevision int64 = 1
	for _, op := range report.Watch {
		for _, resp := range op.Responses {
			if len(resp.Events) > 0 {
				if resp.Events[0].Revision == lastEventRevision {
					t.Errorf("Broke watch guarantee: Atomic - a list of events is guaranteed to encompass complete revisions; updates in the same revision over multiple keys will not be split over several lists of events, previousListEventRevision: %d, currentListEventRevision: %d, client: %d", lastEventRevision, resp.Events[0].Revision, report.ClientId)
				}
				lastEventRevision = resp.Events[len(resp.Events)-1].Revision
			}
		}
	}
}

func validateReliable(t *testing.T, events []model.WatchEvent, report report.ClientReport) {
	for _, op := range report.Watch {
		index := 0
		revision := firstRevision(op)
		for index < len(events) && events[index].Revision < revision {
			index++
		}
		if index == len(events) {
			continue
		}
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				if events[index].Match(op.Request) && events[index] != event {
					t.Errorf("Broke watch guarantee: Reliable - a sequence of events will never drop any subsequence of events; if there are events ordered in time as a < b < c, then if the watch receives events a and c, it is guaranteed to receive b, event missing: %+v", events[index])
				}
				index++
			}
		}
	}
}

func validateResumable(t *testing.T, events []model.WatchEvent, report report.ClientReport) {
	for _, op := range report.Watch {
		index := 0
		revision := op.Request.Revision
		for index < len(events) && (events[index].Revision < revision || !events[index].Match(op.Request)) {
			index++
		}
		if index == len(events) {
			continue
		}
		firstEvent := firstWatchEvent(op)
		// If watch is resumable, first event it gets should the first event that happened after the requested revision.
		if firstEvent != nil && events[index] != *firstEvent {
			t.Errorf("Resumable - A broken watch can be resumed by establishing a new watch starting after the last revision received in a watch event before the break, so long as the revision is in the history window, watch request: %+v, event missing: %+v", op.Request, events[index])
		}
	}
}

func firstRevision(op model.WatchOperation) int64 {
	for _, resp := range op.Responses {
		for _, event := range resp.Events {
			return event.Revision
		}
	}
	return 0
}

func firstWatchEvent(op model.WatchOperation) *model.WatchEvent {
	for _, resp := range op.Responses {
		for _, event := range resp.Events {
			return &event
		}
	}
	return nil
}

func mergeWatchEventHistory(t *testing.T, reports []report.ClientReport) []model.WatchEvent {
	type revisionEvents struct {
		events   []model.WatchEvent
		revision int64
		clientId int
	}
	revisionToEvents := map[int64]revisionEvents{}
	var lastClientId = 0
	var lastRevision int64 = 0
	events := []model.WatchEvent{}
	for _, r := range reports {
		for _, op := range r.Watch {
			for _, resp := range op.Responses {
				for _, event := range resp.Events {
					if event.Revision == lastRevision && lastClientId == r.ClientId {
						events = append(events, event)
					} else {
						if prev, found := revisionToEvents[lastRevision]; found {
							// This assumes that there are txn that would be observed differently by two watches.
							// TODO: Implement merging events from multiple watches about single revision based on operations.
							if diff := cmp.Diff(prev.events, events); diff != "" {
								t.Errorf("Events between clients %d and %d don't match, revision: %d, diff: %s", prev.clientId, lastClientId, lastRevision, diff)
							}
						} else {
							revisionToEvents[lastRevision] = revisionEvents{clientId: lastClientId, events: events, revision: lastRevision}
						}
						lastClientId = r.ClientId
						lastRevision = event.Revision
						events = []model.WatchEvent{event}
					}
				}
			}
		}
	}
	if prev, found := revisionToEvents[lastRevision]; found {
		if diff := cmp.Diff(prev.events, events); diff != "" {
			t.Errorf("Events between clients %d and %d don't match, revision: %d, diff: %s", prev.clientId, lastClientId, lastRevision, diff)
		}
	} else {
		revisionToEvents[lastRevision] = revisionEvents{clientId: lastClientId, events: events, revision: lastRevision}
	}

	var allRevisionEvents []revisionEvents
	for _, revEvents := range revisionToEvents {
		allRevisionEvents = append(allRevisionEvents, revEvents)
	}
	sort.Slice(allRevisionEvents, func(i, j int) bool {
		return allRevisionEvents[i].revision < allRevisionEvents[j].revision
	})
	var eventHistory []model.WatchEvent
	for _, revEvents := range allRevisionEvents {
		eventHistory = append(eventHistory, revEvents.events...)
	}
	return eventHistory
}
