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
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

// ValidateAndReturnVisualize returns visualize as porcupine.linearizationInfo used to generate visualization is private.
func ValidateAndReturnVisualize(t *testing.T, lg *zap.Logger, cfg Config, reports []report.ClientReport, persistedRequests []model.EtcdRequest, timeout time.Duration) (visualize func(basepath string) error) {
	err := checkValidationAssumptions(reports)
	if err != nil {
		t.Fatalf("Broken validation assumptions: %s", err)
	}
	patchedOperations := patchedOperationHistory(reports, persistedRequests)
	linearizable, visualize := validateLinearizableOperationsAndVisualize(lg, patchedOperations, timeout)
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

func checkValidationAssumptions(reports []report.ClientReport) error {
	err := validatePutOperationUnique(reports)
	if err != nil {
		return err
	}
	err = validateEmptyDatabaseAtStart(reports)
	if err != nil {
		return err
	}
	err = validateLastOperationAndObservedInWatch(reports)
	if err != nil {
		return err
	}
	err = validateObservedAllRevisionsInWatch(reports)
	if err != nil {
		return err
	}
	err = validateNonConcurrentClientRequests(reports)
	if err != nil {
		return err
	}
	return nil
}

func validatePutOperationUnique(reports []report.ClientReport) error {
	type KV struct {
		Key   string
		Value model.ValueOrHash
	}
	putValue := map[KV]struct{}{}
	for _, r := range reports {
		for _, op := range r.KeyValue {
			request := op.Input.(model.EtcdRequest)
			if request.Type != model.Txn {
				continue
			}
			for _, op := range append(request.Txn.OperationsOnSuccess, request.Txn.OperationsOnFailure...) {
				if op.Type != model.PutOperation {
					continue
				}
				kv := KV{
					Key:   op.Put.Key,
					Value: op.Put.Value,
				}
				if _, ok := putValue[kv]; ok {
					return fmt.Errorf("non unique put %v, required to patch operation history", kv)
				}
				putValue[kv] = struct{}{}
			}
		}
	}
	return nil
}

func validateEmptyDatabaseAtStart(reports []report.ClientReport) error {
	for _, r := range reports {
		for _, op := range r.KeyValue {
			request := op.Input.(model.EtcdRequest)
			response := op.Output.(model.MaybeEtcdResponse)
			if response.Revision == 2 && !request.IsRead() {
				return nil
			}
		}
	}
	return fmt.Errorf("non empty database at start or first write didn't succeed, required by model implementation")
}

func validateLastOperationAndObservedInWatch(reports []report.ClientReport) error {
	var lastOperation porcupine.Operation

	for _, r := range reports {
		for _, op := range r.KeyValue {
			if op.Call > lastOperation.Call {
				lastOperation = op
			}
		}
	}
	lastResponse := lastOperation.Output.(model.MaybeEtcdResponse)
	if lastResponse.PartialResponse || lastResponse.Error != "" {
		return fmt.Errorf("last operation %v failed, its success is required to validate watch", lastOperation)
	}
	for _, r := range reports {
		for _, watch := range r.Watch {
			for _, watchResp := range watch.Responses {
				for _, e := range watchResp.Events {
					if e.Revision == lastResponse.Revision {
						return nil
					}
				}
			}
		}
	}
	return fmt.Errorf("revision from the last operation %d was not observed in watch, required to validate watch", lastResponse.Revision)
}

func validateObservedAllRevisionsInWatch(reports []report.ClientReport) error {
	var maxRevision int64
	for _, r := range reports {
		for _, watch := range r.Watch {
			for _, watchResp := range watch.Responses {
				for _, e := range watchResp.Events {
					if e.Revision > maxRevision {
						maxRevision = e.Revision
					}
				}
			}
		}
	}
	observedRevisions := make([]bool, maxRevision+1)
	for _, r := range reports {
		for _, watch := range r.Watch {
			for _, watchResp := range watch.Responses {
				for _, e := range watchResp.Events {
					observedRevisions[e.Revision] = true
				}
			}
		}
	}
	for i := 2; i < len(observedRevisions); i++ {
		if !observedRevisions[i] {
			return fmt.Errorf("didn't observe revision %d in watch, required to patch operation and validate serializable requests", i)
		}
	}
	return nil
}

func validateNonConcurrentClientRequests(reports []report.ClientReport) error {
	lastClientRequestReturn := map[int]int64{}
	for _, r := range reports {
		for _, op := range r.KeyValue {
			lastRequest := lastClientRequestReturn[op.ClientId]
			if op.Call <= lastRequest {
				return fmt.Errorf("client %d has concurrent request, required for operation linearization", op.ClientId)
			}
			if op.Return <= op.Call {
				return fmt.Errorf("operation %v ends before it starts, required for operation linearization", op)
			}
			lastClientRequestReturn[op.ClientId] = op.Return
		}
	}
	return nil
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
