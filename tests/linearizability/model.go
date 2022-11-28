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

type Operation string

const (
	Get    Operation = "get"
	Put    Operation = "put"
	Delete Operation = "delete"
)

type etcdRequest struct {
	op      Operation
	key     string
	putData string
}

type etcdResponse struct {
	getData  string
	revision int64
	deleted  int64
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
		case Delete:
			if response.err != nil {
				return fmt.Sprintf("delete(%q) -> %s", request.key, response.err)
			} else {
				return fmt.Sprintf("delete(%q) -> ok, rev: %d deleted:%d", request.key, response.revision, response.deleted)
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
	case Delete:
		return stepDelete(state, request, response)
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
	case Delete:
		if response.err != nil {
			state.FailedWrites[""] = struct{}{}
		}
		//TODO preserve information about failed deletes
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

func stepDelete(state EtcdState, request etcdRequest, response etcdResponse) (bool, EtcdState) {
	if response.err != nil {
		state.FailedWrites[""] = struct{}{}
		return true, state
	}
	deleteSucceeded := response.deleted != 0
	keySet := state.Value != ""

	//non-existent key cannot be deleted.
	if deleteSucceeded != keySet {
		return false, state
	}
	//if key was deleted, response revision should go up
	if deleteSucceeded && state.LastRevision >= response.revision {
		return false, state
	}
	//if key was not deleted, response revision should not change
	if !deleteSucceeded && state.LastRevision != response.revision {
		return false, state
	}

	state.Value = ""
	state.LastRevision = response.revision
	return true, state
}
