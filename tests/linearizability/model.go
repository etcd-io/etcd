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

type EtcdRequest struct {
	Op      Operation
	Key     string
	PutData string
}

type EtcdResponse struct {
	GetData  string
	Revision int64
	Deleted  int64
	Err      error
}

type EtcdState struct {
	Key          string
	Value        string
	LastRevision int64
	FailedWrite  *EtcdRequest
}

var etcdModel = porcupine.Model{
	Init: func() interface{} { return "{}" },
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
		var state EtcdState
		err := json.Unmarshal([]byte(st.(string)), &state)
		if err != nil {
			panic(err)
		}
		ok, state := step(state, in.(EtcdRequest), out.(EtcdResponse))
		data, err := json.Marshal(state)
		if err != nil {
			panic(err)
		}
		return ok, string(data)
	},
	DescribeOperation: func(in, out interface{}) string {
		request := in.(EtcdRequest)
		response := out.(EtcdResponse)
		switch request.Op {
		case Get:
			if response.Err != nil {
				return fmt.Sprintf("get(%q) -> %q", request.Key, response.Err)
			} else {
				return fmt.Sprintf("get(%q) -> %q, rev: %d", request.Key, response.GetData, response.Revision)
			}
		case Put:
			if response.Err != nil {
				return fmt.Sprintf("put(%q, %q) -> %s", request.Key, request.PutData, response.Err)
			} else {
				return fmt.Sprintf("put(%q, %q) -> ok, rev: %d", request.Key, request.PutData, response.Revision)
			}
		case Delete:
			if response.Err != nil {
				return fmt.Sprintf("delete(%q) -> %s", request.Key, response.Err)
			} else {
				return fmt.Sprintf("delete(%q) -> ok, rev: %d deleted:%d", request.Key, response.Revision, response.Deleted)
			}
		default:
			return "<invalid>"
		}
	},
}

func step(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	if request.Key == "" {
		panic("invalid request")
	}
	if state.Key == "" {
		return true, initState(request, response)
	}
	if state.Key != request.Key {
		panic("Multiple keys not supported")
	}
	switch request.Op {
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

func initState(request EtcdRequest, response EtcdResponse) EtcdState {
	state := EtcdState{
		Key:          request.Key,
		LastRevision: response.Revision,
	}
	switch request.Op {
	case Get:
		state.Value = response.GetData
	case Put:
		if response.Err == nil {
			state.Value = request.PutData
		} else {
			state.FailedWrite = &request
		}
	case Delete:
		if response.Err != nil {
			state.FailedWrite = &request
		}
	default:
		panic("Unknown operation")
	}
	return state
}

func stepGet(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	if state.Value == response.GetData && state.LastRevision == response.Revision {
		state.FailedWrite = nil
		return true, state
	}
	if state.FailedWrite != nil && state.LastRevision < response.Revision {
		var ok bool
		switch state.FailedWrite.Op {
		case Get:
			panic("Expected write")
		case Put:
			ok = response.GetData == state.FailedWrite.PutData
		case Delete:
			ok = response.GetData == ""
		default:
			panic("Unknown operation")
		}
		if ok {
			state.Value = response.GetData
			state.LastRevision = response.Revision
			state.FailedWrite = nil
			return true, state
		}
	}
	return false, state
}

func stepPut(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	if response.Err != nil {
		state.FailedWrite = &request
		return true, state
	}
	if response.Revision <= state.LastRevision {
		return false, state
	}
	if response.Revision != state.LastRevision+1 && state.FailedWrite == nil {
		return false, state
	}
	state.Value = request.PutData
	state.LastRevision = response.Revision
	state.FailedWrite = nil
	return true, state
}

func stepDelete(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	if response.Err != nil {
		state.FailedWrite = &request
		return true, state
	}
	// revision should never decrease
	if response.Revision < state.LastRevision {
		return false, state
	}
	deleteSucceeded := response.Deleted != 0
	keySet := state.Value != ""

	// non-existent key cannot be deleted.
	if deleteSucceeded != keySet && state.FailedWrite == nil {
		return false, state
	}
	//if key was deleted, response revision should increase
	if deleteSucceeded && (response.Revision != state.LastRevision+1 || !keySet) && (state.FailedWrite == nil || response.Revision < state.LastRevision+2) {
		return false, state
	}
	//if key was not deleted, response revision should not change
	if !deleteSucceeded && state.LastRevision != response.Revision && state.FailedWrite == nil {
		return false, state
	}

	state.Value = ""
	state.LastRevision = response.Revision
	return true, state
}
