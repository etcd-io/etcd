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

// NonDeterministicModel extends DeterministicModel to handle requests that have unknown or error response.
// Unknown/error response doesn't inform whether request was persisted or not, so model
// considers both cases. This is represented as multiple equally possible deterministic states.
// Failed requests fork the possible states, while successful requests merge and filter them.
var NonDeterministicModel = porcupine.Model{
	Init: func() interface{} {
		var states nonDeterministicState
		data, err := json.Marshal(states)
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
		ok, states := states.Step(in.(EtcdRequest), out.(EtcdNonDeterministicResponse))
		data, err := json.Marshal(states)
		if err != nil {
			panic(err)
		}
		return ok, string(data)
	},
	DescribeOperation: func(in, out interface{}) string {
		return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdNonDeterministicResponse(in.(EtcdRequest), out.(EtcdNonDeterministicResponse)))
	},
}

type nonDeterministicState []etcdState

type EtcdNonDeterministicResponse struct {
	EtcdResponse
	Err           error
	ResultUnknown bool
}

func (states nonDeterministicState) Step(request EtcdRequest, response EtcdNonDeterministicResponse) (bool, nonDeterministicState) {
	if len(states) == 0 {
		if response.Err == nil && !response.ResultUnknown {
			return true, nonDeterministicState{initState(request, response.EtcdResponse)}
		}
		states = nonDeterministicState{emptyState()}
	}
	var newStates nonDeterministicState
	if response.Err != nil {
		newStates = states.stepFailedRequest(request)
	} else {
		newStates = states.stepSuccessfulRequest(request, response)
	}
	return len(newStates) > 0, newStates
}

// stepFailedRequest duplicates number of states by considering request persisted and lost.
func (states nonDeterministicState) stepFailedRequest(request EtcdRequest) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states)*2)
	for _, s := range states {
		newStates = append(newStates, s)
		newState, _ := s.step(request)
		if !reflect.DeepEqual(newState, s) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// stepSuccessfulRequest filters possible states by leaving ony states that would respond correctly.
func (states nonDeterministicState) stepSuccessfulRequest(request EtcdRequest, response EtcdNonDeterministicResponse) nonDeterministicState {
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		newState, gotResponse := s.step(request)
		if Match(EtcdNonDeterministicResponse{EtcdResponse: gotResponse}, response) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

func Match(r1, r2 EtcdNonDeterministicResponse) bool {
	return ((r1.ResultUnknown || r2.ResultUnknown) && (r1.Revision == r2.Revision)) || reflect.DeepEqual(r1, r2)
}
