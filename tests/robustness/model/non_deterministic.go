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
	"reflect"

	"github.com/anishathalye/porcupine"
)

// NonDeterministicModel extends DeterministicModel to allow for clients with imperfect knowledge of request destiny.
// Unknown/error response doesn't inform whether request was persisted or not, so model
// considers both cases. This is represented as multiple equally possible deterministic states.
// Failed requests fork the possible states, while successful requests merge and filter them.
var NonDeterministicModel = porcupine.Model{
	Init: func() any {
		data, err := json.Marshal(nonDeterministicState{freshEtcdState()})
		if err != nil {
			panic(err)
		}
		return string(data)
	},
	Step: func(st any, in any, out any) (bool, any) {
		var states nonDeterministicState
		err := json.Unmarshal([]byte(st.(string)), &states)
		if err != nil {
			panic(err)
		}
		ok, states := states.apply(in.(EtcdRequest), out.(MaybeEtcdResponse))
		data, err := json.Marshal(states)
		if err != nil {
			panic(err)
		}
		return ok, string(data)
	},
	DescribeOperation: func(in, out any) string {
		return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), out.(MaybeEtcdResponse)))
	},
}

type nonDeterministicState []EtcdState

func (states nonDeterministicState) apply(request EtcdRequest, response MaybeEtcdResponse) (bool, nonDeterministicState) {
	var newStates nonDeterministicState
	switch {
	case response.Error != "":
		newStates = states.applyFailedRequest(request)
	case response.Persisted && response.PersistedRevision == 0:
		newStates = states.applyPersistedRequest(request)
	case response.Persisted && response.PersistedRevision != 0:
		newStates = states.applyPersistedRequestWithRevision(request, response.PersistedRevision)
	default:
		newStates = states.applyRequestWithResponse(request, response.EtcdResponse)
	}
	return len(newStates) > 0, newStates
}

// applyFailedRequest returns both the original states and states with applied request. It considers both cases, request was persisted and request was lost.
func (states nonDeterministicState) applyFailedRequest(request EtcdRequest) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states)*2)
	for _, s := range states {
		newStates = append(newStates, s)
		newState, _ := s.Step(request)
		if !reflect.DeepEqual(newState, s) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyPersistedRequest applies request to all possible states.
func (states nonDeterministicState) applyPersistedRequest(request EtcdRequest) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, _ := s.Step(request)
		newStates = append(newStates, newState)
	}
	return newStates
}

// applyPersistedRequestWithRevision applies request to all possible states, but leaves only states that would return proper revision.
func (states nonDeterministicState) applyPersistedRequestWithRevision(request EtcdRequest, responseRevision int64) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, modelResponse := s.Step(request)
		if modelResponse.Revision == responseRevision {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyRequestWithResponse applies request to all possible states, but leaves only state that would return proper response.
func (states nonDeterministicState) applyRequestWithResponse(request EtcdRequest, response EtcdResponse) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, modelResponse := s.Step(request)
		if Match(modelResponse, MaybeEtcdResponse{EtcdResponse: response}) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}
