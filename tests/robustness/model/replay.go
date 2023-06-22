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

package model

import (
	"fmt"
)

func NewReplay(eventHistory []WatchEvent) *EtcdReplay {
	var lastEventRevision int64 = 1
	for _, event := range eventHistory {
		if event.Revision > lastEventRevision && event.Revision != lastEventRevision+1 {
			panic("Replay requires a complete event history")
		}
		lastEventRevision = event.Revision
	}
	return &EtcdReplay{
		eventHistory: eventHistory,
	}
}

type EtcdReplay struct {
	eventHistory []WatchEvent

	// Cached state and event index used for it's calculation
	cachedState       *EtcdState
	eventHistoryIndex int
}

func (r *EtcdReplay) StateForRevision(revision int64) (EtcdState, error) {
	if revision < 1 {
		return EtcdState{}, fmt.Errorf("invalid revision: %d", revision)
	}
	if r.cachedState == nil || r.cachedState.Revision > revision {
		r.reset()
	}

	for r.eventHistoryIndex < len(r.eventHistory) && r.cachedState.Revision < revision {
		nextRequest, nextRevision, nextIndex := r.next()
		newState, _ := r.cachedState.Step(nextRequest)
		if newState.Revision != nextRevision {
			return EtcdState{}, fmt.Errorf("model returned different revision than one present in event history, model: %d, event: %d", newState.Revision, nextRevision)
		}
		r.cachedState = &newState
		r.eventHistoryIndex = nextIndex
	}
	if r.eventHistoryIndex > len(r.eventHistory) && r.cachedState.Revision < revision {
		return EtcdState{}, fmt.Errorf("requested revision higher then available in even history, requested: %d, model: %d", revision, r.cachedState.Revision)
	}
	return *r.cachedState, nil
}

func (r *EtcdReplay) reset() {
	state := freshEtcdState()
	r.cachedState = &state
	r.eventHistoryIndex = 0
}

func (r *EtcdReplay) next() (request EtcdRequest, revision int64, index int) {
	revision = r.eventHistory[r.eventHistoryIndex].Revision
	index = r.eventHistoryIndex
	operations := []EtcdOperation{}
	for r.eventHistory[index].Revision == revision {
		event := r.eventHistory[index]
		operations = append(operations, EtcdOperation{
			Type:   event.Type,
			Put:    event.Put,
			Delete: event.Delete,
		})
		index++
	}
	return EtcdRequest{
		Type: Txn,
		Txn: &TxnRequest{
			OperationsOnSuccess: operations,
		},
	}, revision, index
}

type WatchEvent struct {
	Type     OperationType
	Put      PutOptions
	Delete   DeleteOptions
	Revision int64
}

func (e WatchEvent) Key() string {
	switch e.Type {
	case DeleteOperation:
		return e.Delete.Key
	case PutOperation:
		return e.Put.Key
	default:
		panic(fmt.Sprintf("unknown event type: %s", e.Type))
	}
}
