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
	"errors"

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

var (
	errBrokeBookmarkable = errors.New("broke Bookmarkable - Progress notification events guarantee that all events up to a revision have been already delivered")
	errBrokeOrdered      = errors.New("broke Ordered - events are ordered by revision; an event will never appear on a watch if it precedes an event in time that has already been posted")
	errBrokeUnique       = errors.New("broke Unique - an event will never appear on a watch twice")
	errBrokeAtomic       = errors.New("broke Atomic - a list of events is guaranteed to encompass complete revisions; updates in the same revision over multiple keys will not be split over several lists of events")
	errBrokeReliable     = errors.New("broke Reliable - a sequence of events will never drop any subsequence of events; if there are events ordered in time as a < b < c, then if the watch receives events a and c, it is guaranteed to receive b")
	errBrokeResumable    = errors.New("broke Resumable - A broken watch can be resumed by establishing a new watch starting after the last revision received in a watch event before the break, so long as the revision is in the history window")
	errBrokePrevKV       = errors.New("incorrect event prevValue")
	errBrokeIsCreate     = errors.New("incorrect event IsCreate")
)

func validateWatch(lg *zap.Logger, cfg Config, reports []report.ClientReport, eventHistory []model.PersistedEvent) error {
	lg.Info("Validating watch")
	// Validate etcd watch properties defined in https://etcd.io/docs/v3.6/learning/api_guarantees/#watch-apis
	for _, r := range reports {
		err := validateOrdered(lg, r)
		if err != nil {
			return err
		}
		err = validateUnique(lg, cfg.ExpectRevisionUnique, r)
		if err != nil {
			return err
		}
		err = validateAtomic(lg, r)
		if err != nil {
			return err
		}
		err = validateBookmarkable(lg, r)
		if err != nil {
			return err
		}
		if eventHistory != nil {
			err = validateReliable(lg, eventHistory, r)
			if err != nil {
				return err
			}
			err = validateResumable(lg, eventHistory, r)
			if err != nil {
				return err
			}
			err = validatePrevKV(lg, r, eventHistory)
			if err != nil {
				return err
			}
			err = validateEventIsCreate(lg, r, eventHistory)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBookmarkable(lg *zap.Logger, report report.ClientReport) (err error) {
	for _, op := range report.Watch {
		var lastProgressNotifyRevision int64
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				if event.Revision <= lastProgressNotifyRevision {
					lg.Error("Broke watch guarantee", zap.String("guarantee", "bookmarkable"), zap.Int("client", report.ClientID), zap.Int64("revision", event.Revision))
					err = errBrokeBookmarkable
				}
			}
			if resp.IsProgressNotify {
				lastProgressNotifyRevision = resp.Revision
			}
		}
	}
	return err
}

func validateOrdered(lg *zap.Logger, report report.ClientReport) (err error) {
	for _, op := range report.Watch {
		var lastEventRevision int64 = 1
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				if event.Revision < lastEventRevision {
					lg.Error("Broke watch guarantee", zap.String("guarantee", "ordered"), zap.Int("client", report.ClientID), zap.Int64("revision", event.Revision))
					err = errBrokeOrdered
				}
				lastEventRevision = event.Revision
			}
		}
	}
	return err
}

func validateUnique(lg *zap.Logger, expectUniqueRevision bool, report report.ClientReport) (err error) {
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
					lg.Error("Broke watch guarantee", zap.String("guarantee", "unique"), zap.Int("client", report.ClientID), zap.String("key", event.Key), zap.Int64("revision", event.Revision))
					err = errBrokeUnique
				}
				uniqueOperations[key] = struct{}{}
			}
		}
	}
	return err
}

func validateAtomic(lg *zap.Logger, report report.ClientReport) (err error) {
	for _, op := range report.Watch {
		var lastEventRevision int64 = 1
		for _, resp := range op.Responses {
			if len(resp.Events) > 0 {
				if resp.Events[0].Revision == lastEventRevision {
					lg.Error("Broke watch guarantee", zap.String("guarantee", "atomic"), zap.Int("client", report.ClientID), zap.Int64("revision", resp.Events[0].Revision))
					err = errBrokeAtomic
				}
				lastEventRevision = resp.Events[len(resp.Events)-1].Revision
			}
		}
	}
	return err
}

func validateReliable(lg *zap.Logger, events []model.PersistedEvent, report report.ClientReport) (err error) {
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
					lg.Error("Broke watch guarantee", zap.String("guarantee", "reliable"), zap.Int("client", report.ClientID), zap.Any("missing-event", events[index]))
					err = errBrokeReliable
				}
				index++
			}
		}
	}
	return err
}

func validateResumable(lg *zap.Logger, events []model.PersistedEvent, report report.ClientReport) (err error) {
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
			lg.Error("Broke watch guarantee", zap.String("guarantee", "resumable"), zap.Int("client", report.ClientID), zap.Any("request", op.Request), zap.Any("got-event", *firstEvent), zap.Any("want-event", events[index]))
			err = errBrokeResumable
		}
	}
	return err
}

// validatePrevKV ensures that a watch response (if configured with WithPrevKV()) returns
// the appropriate response.
func validatePrevKV(lg *zap.Logger, report report.ClientReport, history []model.PersistedEvent) (err error) {
	replay := model.NewReplay(history)
	for _, op := range report.Watch {
		if !op.Request.WithPrevKV {
			continue
		}
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				// Get state state just before the current event.
				state, _ := replay.StateForRevision(event.Revision - 1)
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
					lg.Error("Incorrect event prevValue field", zap.Int("client", report.ClientID), zap.Any("event", event), zap.Any("previousValue", state.KeyValues[event.Key]))
					err = errBrokePrevKV
				}
			}
		}
	}
	return err
}

func validateEventIsCreate(lg *zap.Logger, report report.ClientReport, history []model.PersistedEvent) (err error) {
	replay := model.NewReplay(history)
	for _, op := range report.Watch {
		for _, resp := range op.Responses {
			for _, event := range resp.Events {
				// Get state state just before the current event.
				state, _ := replay.StateForRevision(event.Revision - 1)
				// A create event will not have an entry in our history and a non-create
				// event *should* have an entry in our history.
				if _, prevKeyExists := state.KeyValues[event.Key]; event.IsCreate == prevKeyExists {
					lg.Error("Incorrect event IsCreate field", zap.Int("client", report.ClientID), zap.Any("event", event))
					err = errBrokeIsCreate
				}
			}
		}
	}
	return err
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
