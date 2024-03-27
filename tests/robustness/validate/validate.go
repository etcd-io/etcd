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
	"fmt"
	"sort"
	"testing"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

// ValidateAndReturnVisualize returns visualize as porcupine.linearizationInfo used to generate visualization is private.
func ValidateAndReturnVisualize(t *testing.T, lg *zap.Logger, cfg Config, reports []report.ClientReport) (visualize func(basepath string) error) {
	patchedOperations := patchedOperationHistory(reports)
	linearizable, visualize := validateLinearizableOperationsAndVisualize(lg, patchedOperations)
	if linearizable != porcupine.Ok {
		t.Error("Failed linearization, skipping further validation")
		return visualize
	}
	// TODO: Don't use watch events to get event history.
	eventHistory, err := mergeWatchEventHistory(reports)
	if err != nil {
		t.Errorf("Failed merging watch history to create event history, err: %s", err)
		validateWatch(t, lg, cfg, reports, nil)
		return visualize
	}
	validateWatch(t, lg, cfg, reports, eventHistory)
	validateSerializableOperations(t, lg, patchedOperations, eventHistory)
	return visualize
}

type Config struct {
	ExpectRevisionUnique bool
}

func mergeWatchEventHistory(reports []report.ClientReport) ([]model.PersistedEvent, error) {
	type revisionEvents struct {
		events   []model.PersistedEvent
		revision int64
		clientID int
	}
	revisionToEvents := map[int64]revisionEvents{}
	var lastClientID = 0
	var lastRevision int64
	events := []model.PersistedEvent{}
	for _, r := range reports {
		for _, op := range r.Watch {
			for _, resp := range op.Responses {
				for _, event := range resp.Events {
					if event.Revision == lastRevision && lastClientID == r.ClientID {
						events = append(events, event.PersistedEvent)
					} else {
						if prev, found := revisionToEvents[lastRevision]; found {
							// This assumes that there are txn that would be observed differently by two watches.
							// TODO: Implement merging events from multiple watches about single revision based on operations.
							if diff := cmp.Diff(prev.events, events); diff != "" {
								return nil, fmt.Errorf("events between clients %d and %d don't match, revision: %d, diff: %s", prev.clientID, lastClientID, lastRevision, diff)
							}
						} else {
							revisionToEvents[lastRevision] = revisionEvents{clientID: lastClientID, events: events, revision: lastRevision}
						}
						lastClientID = r.ClientID
						lastRevision = event.Revision
						events = []model.PersistedEvent{event.PersistedEvent}
					}
				}
			}
		}
	}
	if prev, found := revisionToEvents[lastRevision]; found {
		if diff := cmp.Diff(prev.events, events); diff != "" {
			return nil, fmt.Errorf("events between clients %d and %d don't match, revision: %d, diff: %s", prev.clientID, lastClientID, lastRevision, diff)
		}
	} else {
		revisionToEvents[lastRevision] = revisionEvents{clientID: lastClientID, events: events, revision: lastRevision}
	}

	var allRevisionEvents []revisionEvents
	for _, revEvents := range revisionToEvents {
		allRevisionEvents = append(allRevisionEvents, revEvents)
	}
	sort.Slice(allRevisionEvents, func(i, j int) bool {
		return allRevisionEvents[i].revision < allRevisionEvents[j].revision
	})
	var eventHistory []model.PersistedEvent
	for _, revEvents := range allRevisionEvents {
		eventHistory = append(eventHistory, revEvents.events...)
	}
	return eventHistory, nil
}
