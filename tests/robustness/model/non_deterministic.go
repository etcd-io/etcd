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

package model

import (
	"encoding/json"
	"fmt"
	"html"
	"reflect"

	"github.com/anishathalye/porcupine"
)

func OperationKeys(operations []porcupine.Operation) []string {
	keysMap := map[string]struct{}{}
	for _, op := range operations {
		RequestKeys(keysMap, op.Input.(EtcdRequest))
	}
	keys := []string{}
	for key := range keysMap {
		keys = append(keys, key)
	}
	return keys
}

func RequestKeys(keysMap map[string]struct{}, req EtcdRequest) {
	switch req.Type {
	case Txn:
		for _, op := range req.Txn.OperationsOnSuccess {
			switch op.Type {
			case PutOperation:
				keysMap[op.Put.Key] = struct{}{}
			case DeleteOperation:
				keysMap[op.Delete.Key] = struct{}{}
			case RangeOperation:
				if op.Range.End == "" {
					keysMap[op.Range.Start] = struct{}{}
				}
			default:
				panic(fmt.Sprintf("Unknown operation type: %v", op.Type))
			}
		}
		for _, op := range req.Txn.OperationsOnFailure {
			switch op.Type {
			case PutOperation:
				keysMap[op.Put.Key] = struct{}{}
			case DeleteOperation:
				keysMap[op.Delete.Key] = struct{}{}
			case RangeOperation:
				if op.Range.End == "" {
					keysMap[op.Range.Start] = struct{}{}
				}
			default:
				panic(fmt.Sprintf("Unknown operation type: %v", op.Type))
			}
		}
	case Range:
		if req.Range.End == "" {
			keysMap[req.Range.Start] = struct{}{}
		}
	}
	return
}

func NonDeterministicModelV2(keys []string) porcupine.Model {
	return porcupine.Model{
		Init: func() any {
			return nonDeterministicState{freshEtcdState(len(keys))}
		},
		Step: func(st any, in any, out any) (bool, any) {
			return st.(nonDeterministicState).apply(in.(EtcdRequest), keys, out.(MaybeEtcdResponse))
		},
		Equal: func(st1, st2 any) bool {
			return st1.(nonDeterministicState).Equal(st2.(nonDeterministicState))
		},
		DescribeOperation: func(in, out any) string {
			return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), out.(MaybeEtcdResponse)))
		},
		DescribeState: func(st any) string {
			data, err := json.MarshalIndent(st, "", "  ")
			if err != nil {
				panic(err)
			}
			return "<pre>" + html.EscapeString(string(data)) + "</pre>"
		},
	}
}

type keyValues struct {
	Keys   []string
	Values []ValueRevision
}

func (kv keyValues) Range(start, end string) []KeyValue {
	result := []KeyValue{}
	for i, k := range kv.Keys {
		if k >= start && k < end {
			if kv.Values[i].ModRevision == 0 {
				continue
			}
			result = append(result, KeyValue{Key: k, ValueRevision: kv.Values[i]})
		}
	}
	return result
}

func (kv keyValues) Get(key string) (ValueRevision, bool) {
	for i, k := range kv.Keys {
		if k == key {
			return kv.Values[i], kv.Values[i].ModRevision != 0
		}
	}
	panic(fmt.Sprintf("Key not found: %q, keys: %v", key, kv.Keys))
}

func (kv keyValues) Set(key string, value ValueRevision) {
	for i, k := range kv.Keys {
		if k == key {
			kv.Values[i] = value
			return
		}
	}
	panic(fmt.Sprintf("Key not found: %q, keys: %v", key, kv.Keys))
}

func (kv keyValues) Delete(key string) {
	for i, k := range kv.Keys {
		if k == key {
			kv.Values[i] = ValueRevision{}
			return
		}
	}
	panic(fmt.Sprintf("Key not found: %q, keys: %v", key, kv.Keys))
}

type nonDeterministicState []EtcdState

func (states nonDeterministicState) Equal(other nonDeterministicState) bool {
	if len(states) != len(other) {
		return false
	}

	otherMatched := make([]bool, len(other))
	for _, sItem := range states {
		foundMatchInOther := false
		for j, otherItem := range other {
			if !otherMatched[j] && sItem.Equal(otherItem) {
				otherMatched[j] = true
				foundMatchInOther = true
				break
			}
		}
		if !foundMatchInOther {
			return false
		}
	}
	return true
}

func (states nonDeterministicState) apply(request EtcdRequest, keys []string, response MaybeEtcdResponse) (bool, nonDeterministicState) {
	var newStates nonDeterministicState
	switch {
	case response.Error != "":
		newStates = states.applyFailedRequest(request, keys)
	case response.Persisted && response.PersistedRevision == 0:
		newStates = states.applyPersistedRequest(request, keys)
	case response.Persisted && response.PersistedRevision != 0:
		newStates = states.applyPersistedRequestWithRevision(request, keys, response.PersistedRevision)
	default:
		newStates = states.applyRequestWithResponse(request, keys, response.EtcdResponse)
	}
	return len(newStates) > 0, newStates
}

// applyFailedRequest returns both the original states and states with applied request. It considers both cases, request was persisted and request was lost.
func (states nonDeterministicState) applyFailedRequest(request EtcdRequest, keys []string) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states)*2)
	for _, s := range states {
		newStates = append(newStates, s)
		newState, _ := s.Step(request, keys)
		if !reflect.DeepEqual(newState, s) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyPersistedRequest applies request to all possible states.
func (states nonDeterministicState) applyPersistedRequest(request EtcdRequest, keys []string) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, _ := s.Step(request, keys)
		newStates = append(newStates, newState)
	}
	return newStates
}

// applyPersistedRequestWithRevision applies request to all possible states, but leaves only states that would return proper revision.
func (states nonDeterministicState) applyPersistedRequestWithRevision(request EtcdRequest, keys []string, responseRevision int64) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, modelResponse := s.Step(request, keys)
		if modelResponse.Revision == responseRevision {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyRequestWithResponse applies request to all possible states, but leaves only state that would return proper response.
func (states nonDeterministicState) applyRequestWithResponse(request EtcdRequest, keys []string, response EtcdResponse) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, modelResponse := s.Step(request, keys)
		if Match(modelResponse, MaybeEtcdResponse{EtcdResponse: response}) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}
