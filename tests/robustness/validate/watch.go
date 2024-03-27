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

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func validateWatch(t *testing.T, lg *zap.Logger, cfg Config, reports []report.ClientReport, eventHistory []model.PersistedEvent) {
	lg.Info("Validating watch")
	// Validate etcd watch properties defined in https://etcd.io/docs/v3.6/learning/api_guarantees/#watch-apis
	for _, r := range reports {
		validateOrdered(t, r)
		validateUnique(t, cfg.ExpectRevisionUnique, r)
		validateAtomic(t, r)
		validateBookmarkable(t, r)
		if eventHistory != nil {
			validateReliable(t, eventHistory, r)
			validateResumable(t, eventHistory, r)
			validatePrevKV(t, r, eventHistory)
			validateCreateEvent(t, r, eventHistory)
		}
	}
}

func validateBookmarkable(t *testing.T, report report.ClientReport) {
	for _, op := range report.Watch {
		var lastProgressNotifyRevision int64
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
	for _, op := range report.Watch {
		var lastEventRevision int64 = 1
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				if event.Revision < lastEventRevision {
					t.Errorf("Broke watch guarantee: Ordered - events are ordered by revision; an event will never appear on a watch if it precedes an event in time that has already been posted, lastRevision: %d, currentRevision: %d, client: %d", lastEventRevision, event.Revision, report.ClientID)
				}
				lastEventRevision = event.Revision
			}
		}
	}
}

func validateUnique(t *testing.T, expectUniqueRevision bool, report report.ClientReport) {
	for _, op := range report.Watch {
		uniqueOperations := map[any]struct{}{}
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				var key any
				if expectUniqueRevision {
					key = event.Revision
				} else {
					key = struct {
						revision int64
						key      string
					}{event.Revision, event.Key}
				}
				if _, found := uniqueOperations[key]; found {
					t.Errorf("Broke watch guarantee: Unique - an event will never appear on a watch twice, key: %q, revision: %d, client: %d", event.Key, event.Revision, report.ClientID)
				}
				uniqueOperations[key] = struct{}{}
			}
		}
	}
}

func validateAtomic(t *testing.T, report report.ClientReport) {
	for _, op := range report.Watch {
		var lastEventRevision int64 = 1
		for _, resp := range op.Responses {
			if len(resp.Events) > 0 {
				if resp.Events[0].Revision == lastEventRevision {
					t.Errorf("Broke watch guarantee: Atomic - a list of events is guaranteed to encompass complete revisions; updates in the same revision over multiple keys will not be split over several lists of events, previousListEventRevision: %d, currentListEventRevision: %d, client: %d", lastEventRevision, resp.Events[0].Revision, report.ClientID)
				}
				lastEventRevision = resp.Events[len(resp.Events)-1].Revision
			}
		}
	}
}

func validateReliable(t *testing.T, events []model.PersistedEvent, report report.ClientReport) {
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
				if events[index].Match(op.Request) && events[index] != event.PersistedEvent {
					t.Errorf("Broke watch guarantee: Reliable - a sequence of events will never drop any subsequence of events; if there are events ordered in time as a < b < c, then if the watch receives events a and c, it is guaranteed to receive b, event missing: %+v, got: %+v", events[index], event)
				}
				index++
			}
		}
	}
}

func validateResumable(t *testing.T, events []model.PersistedEvent, report report.ClientReport) {
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
		if firstEvent != nil && events[index] != firstEvent.PersistedEvent {
			t.Errorf("Resumable - A broken watch can be resumed by establishing a new watch starting after the last revision received in a watch event before the break, so long as the revision is in the history window, watch request: %+v, event missing: %+v, got: %+v", op.Request, events[index], *firstEvent)
		}
	}
}

// validatePrevKV ensures that a watch response (if configured with WithPrevKV()) returns
// the appropriate response.
func validatePrevKV(t *testing.T, report report.ClientReport, history []model.PersistedEvent) {
	replay := model.NewReplay(history)
	for _, op := range report.Watch {
		if !op.Request.WithPrevKV {
			continue
		}
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				// Get state state just before the current event.
				state, err := replay.StateForRevision(event.Revision - 1)
				if err != nil {
					t.Error(err)
				}
				// TODO(MadhavJivrajani): check if compaction has been run as part
				// of failpoint injection. If compaction has run, prevKV can be nil
				// even if it is not a create event.
				//
				// Considering that Kubernetes opens watches to etcd using WithPrevKV()
				// option, ideally we would want to explicitly check the condition that
				// Kubernetes does while parsing events received from etcd:
				// https://github.com/kubernetes/kubernetes/blob/a9e4f5b7862e84c4152eabe2e960f3f6fb9a4867/staging/src/k8s.io/apiserver/pkg/storage/etcd3/event.go#L59
				// i.e. prevKV is nil iff the event is a create event, we cannot reliably
				// check that without knowing if compaction has run.

				// We allow PrevValue to be nil since in the face of compaction, etcd does not
				// guarantee its presence.
				if event.PrevValue != nil && *event.PrevValue != state.KeyValues[event.Key] {
					t.Errorf("PrevKV - PrevValue doesn't match previous value under the key %s, got: %+v, want: %+v", event.Key, *event.PrevValue, state.KeyValues[event.Key])
				}
			}
		}
	}
}

func validateCreateEvent(t *testing.T, report report.ClientReport, history []model.PersistedEvent) {
	replay := model.NewReplay(history)
	for _, op := range report.Watch {
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				// Get state state just before the current event.
				state, err := replay.StateForRevision(event.Revision - 1)
				if err != nil {
					t.Error(err)
				}
				// A create event will not have an entry in our history and a non-create
				// event *should* have an entry in our history.
				if _, prevKeyExists := state.KeyValues[event.Key]; event.IsCreate == prevKeyExists {
					t.Errorf("CreateEvent - unexpected event ecountered, create event should not be in event history and update/delete event should be, event already exists: %t, is create event: %t, event: %+v", prevKeyExists, event.IsCreate, event)
				}
			}
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
