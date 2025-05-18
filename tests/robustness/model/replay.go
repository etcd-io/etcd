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
	"strings"
)

func NewReplay(persistedRequests []EtcdRequest) *EtcdReplay {
	keysMap := map[string]struct{}{}
	for _, req := range persistedRequests {
		RequestKeys(keysMap, req)
	}
	keys := []string{}
	for key := range keysMap {
		keys = append(keys, key)
	}
	state := freshEtcdState(len(keys))
	// Padding for index 0 and 1, so index matches revision..
	revisionToEtcdState := []EtcdState{state, state}
	var events []PersistedEvent
	for _, request := range persistedRequests {
		newState, response := state.Step(request, keys)
		if state.Revision != newState.Revision {
			revisionToEtcdState = append(revisionToEtcdState, newState)
		}
		events = append(events, toWatchEvents(&state, keys, request, response)...)
		state = newState
	}
	return &EtcdReplay{
		revisionToEtcdState: revisionToEtcdState,
		Events:              events,
		Keys:                keys,
	}
}

type EtcdReplay struct {
	Keys                []string
	revisionToEtcdState []EtcdState
	Events              []PersistedEvent
}

func (r *EtcdReplay) StateForRevision(revision int64) (EtcdState, error) {
	if int(revision) >= len(r.revisionToEtcdState) {
		return EtcdState{}, fmt.Errorf("requested revision %d, higher than observed in replay %d", revision, len(r.revisionToEtcdState)-1)
	}
	return r.revisionToEtcdState[revision], nil
}

func (r *EtcdReplay) EventsForWatch(watch WatchRequest) (events []PersistedEvent) {
	for _, e := range r.Events {
		if e.Revision < watch.Revision || !e.Match(watch) {
			continue
		}
		events = append(events, e)
	}
	return events
}

func toWatchEvents(prevState *EtcdState, keys []string, request EtcdRequest, response MaybeEtcdResponse) (events []PersistedEvent) {
	if response.Error != "" {
		return events
	}
	kvs := keyValues{
		Keys:   keys,
		Values: prevState.Values,
	}

	switch request.Type {
	case Txn:
		var ops []EtcdOperation
		if response.Txn.Failure {
			ops = request.Txn.OperationsOnFailure
		} else {
			ops = request.Txn.OperationsOnSuccess
		}
		for _, op := range ops {
			switch op.Type {
			case RangeOperation:
			case DeleteOperation:
				e := PersistedEvent{
					Event: Event{
						Type: op.Type,
						Key:  op.Delete.Key,
					},
					Revision: response.Revision,
				}
				if _, ok := kvs.Get(op.Delete.Key); ok {
					events = append(events, e)
				}
			case PutOperation:
				e := PersistedEvent{
					Event: Event{
						Type:  op.Type,
						Key:   op.Put.Key,
						Value: op.Put.Value,
					},
					Revision: response.Revision,
				}
				if _, ok := kvs.Get(op.Put.Key); !ok {
					e.IsCreate = true
				}
				events = append(events, e)
			default:
				panic(fmt.Sprintf("unsupported operation type: %v", op))
			}
		}
	}
	return events
}

type WatchEvent struct {
	PersistedEvent `json:",omitempty"`
	PrevValue      *ValueRevision `json:",omitempty"`
}

type PersistedEvent struct {
	Event    `json:",omitempty"`
	Revision int64 `json:",omitempty"`
	IsCreate bool  `json:",omitempty"`
}

type Event struct {
	Type  OperationType `json:",omitempty"`
	Key   string        `json:",omitempty"`
	Value ValueOrHash   `json:",omitempty"`
}

func (e Event) Match(request WatchRequest) bool {
	if request.WithPrefix {
		return strings.HasPrefix(e.Key, request.Key)
	}
	return e.Key == request.Key
}

type WatchRequest struct {
	Key                string
	Revision           int64
	WithPrefix         bool
	WithProgressNotify bool
	WithPrevKV         bool
}
