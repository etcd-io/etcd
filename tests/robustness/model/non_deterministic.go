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
	Init: func() interface{} {
		data, err := json.Marshal(nonDeterministicState{freshEtcdState()})
		if err != nil {
			panic(err)
		}
		return string(data)
	},
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
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
	DescribeOperation: func(in, out interface{}) string {
		return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), out.(MaybeEtcdResponse)))
	},
}

type nonDeterministicState []EtcdState

func (states nonDeterministicState) apply(request EtcdRequest, response MaybeEtcdResponse) (bool, nonDeterministicState) {
	var newStates nonDeterministicState
	switch {
	case response.Error != "":
		newStates = states.stepFailedResponse(request)
	case response.PartialResponse:
		newStates = states.applyResponseRevision(request, response.EtcdResponse.Revision)
	default:
		newStates = states.applySuccessfulResponse(request, response.EtcdResponse)
	}
	return len(newStates) > 0, newStates
}

// stepFailedResponse duplicates number of states by considering both cases, request was persisted and request was lost.
func (states nonDeterministicState) stepFailedResponse(request EtcdRequest) nonDeterministicState {
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

// applyResponseRevision filters possible states by leaving ony states that would return proper revision.
func (states nonDeterministicState) applyResponseRevision(request EtcdRequest, responseRevision int64) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, modelResponse := s.Step(request)
		if modelResponse.Revision == responseRevision {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applySuccessfulResponse filters possible states by leaving ony states that would respond correctly.
func (states nonDeterministicState) applySuccessfulResponse(request EtcdRequest, response EtcdResponse) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, modelResponse := s.Step(request)
		if Match(modelResponse, MaybeEtcdResponse{EtcdResponse: response}) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}
