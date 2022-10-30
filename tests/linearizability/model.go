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

package linearizability

import (
	"encoding/json"
	"fmt"

	"github.com/anishathalye/porcupine"
)

type Operation int8

const Get Operation = 0
const Put Operation = 1

type etcdRequest struct {
	op      Operation
	key     string
	putData string
}

type etcdResponse struct {
	getData  string
	revision int64
	err      error
}

type EtcdState struct {
	Key          string
	Value        string
	LastRevision int64
	FailedWrites map[string]struct{}
}

var etcdModel = porcupine.Model{
	Init: func() interface{} { return "{}" },
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
		var state EtcdState
		err := json.Unmarshal([]byte(st.(string)), &state)
		if err != nil {
			panic(err)
		}
		ok, state := step(state, in.(etcdRequest), out.(etcdResponse))
		data, err := json.Marshal(state)
		if err != nil {
			panic(err)
		}
		return ok, string(data)
	},
	DescribeOperation: func(in, out interface{}) string {
		request := in.(etcdRequest)
		response := out.(etcdResponse)
		switch request.op {
		case Get:
			if response.err != nil {
				return fmt.Sprintf("get(%q) -> %q", request.key, response.err)
			} else {
				return fmt.Sprintf("get(%q) -> %q, rev: %d", request.key, response.getData, response.revision)
			}
		case Put:
			if response.err != nil {
				return fmt.Sprintf("put(%q, %q) -> %s", request.key, request.putData, response.err)
			} else {
				return fmt.Sprintf("put(%q, %q) -> ok, rev: %d", request.key, request.putData, response.revision)
			}
		default:
			return "<invalid>"
		}
	},
}

func step(state EtcdState, request etcdRequest, response etcdResponse) (bool, EtcdState) {
	if request.key == "" {
		panic("invalid request")
	}
	if state.Key == "" {
		return true, initState(request, response)
	}
	if state.Key != request.key {
		panic("Multiple keys not supported")
	}
	switch request.op {
	case Get:
		return stepGet(state, request, response)
	case Put:
		return stepPut(state, request, response)
	default:
		panic("Unknown operation")
	}
}

func initState(request etcdRequest, response etcdResponse) EtcdState {
	state := EtcdState{
		Key:          request.key,
		LastRevision: response.revision,
		FailedWrites: map[string]struct{}{},
	}
	switch request.op {
	case Get:
		state.Value = response.getData
	case Put:
		if response.err == nil {
			state.Value = request.putData
		} else {
			state.FailedWrites[request.putData] = struct{}{}
		}
	default:
		panic("Unknown operation")
	}
	return state
}

func stepGet(state EtcdState, request etcdRequest, response etcdResponse) (bool, EtcdState) {
	if state.Value == response.getData && state.LastRevision <= response.revision {
		return true, state
	}
	_, ok := state.FailedWrites[response.getData]
	if ok && state.LastRevision < response.revision {
		state.Value = response.getData
		state.LastRevision = response.revision
		delete(state.FailedWrites, response.getData)
		return true, state
	}
	return false, state
}

func stepPut(state EtcdState, request etcdRequest, response etcdResponse) (bool, EtcdState) {
	if response.err != nil {
		state.FailedWrites[request.putData] = struct{}{}
		return true, state
	}
	if state.LastRevision >= response.revision {
		return false, state
	}
	state.Value = request.putData
	state.LastRevision = response.revision
	return true, state
}
