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
	Key            string
	PossibleValues map[string]int64 // Value -> Revision mapping
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
	if len(state.PossibleValues) == 0 {
		state.Key = request.Key
		if ok, val, rev := initValueRevision(request, response); ok {
			state.PossibleValues = map[string]int64{
				val: rev,
			}
		}
		return true, state
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

func initValueRevision(request EtcdRequest, response EtcdResponse) (bool, string, int64) {
	if response.Err != nil {
		return false, "", -1
	}
	switch request.Op {
	case Get:
		return true, response.GetData, response.Revision
	case Put:
		return true, request.PutData, response.Revision
	case Delete:
		return true, "", response.Revision
	default:
		panic("Unknown operation")
	}
}

func stepGet(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	// The returned value (and revision) must be one of the historical possible values
	matched := false
	for val, rev := range state.PossibleValues {
		if val == response.GetData {
			// The revision should never decrease.
			if response.Revision < rev {
				matched = false
				break
			}
			// Normal case: the read revision matches previous revision
			if response.Revision == rev {
				matched = true
				continue
			}
			// Abnormal case: previous write operation failed.
			if rev == -1 {
				matched = true
				continue
			}
		}
	}
	// Always cleanup all historical data, and only keep the latest value & revision.
	return matched, EtcdState{
		Key: request.Key,
		PossibleValues: map[string]int64{
			response.GetData: response.Revision,
		},
	}
}

func stepPut(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	if response.Err != nil {
		state.PossibleValues[request.PutData] = -1
		return true, state
	}
	matched := true
	for _, rev := range state.PossibleValues {
		// response.Revision must be at least `kv.Revision + 1`
		if response.Revision < (rev + 1) {
			matched = false
			break
		}
	}

	// Always cleanup all historical data, and only keep the latest value & revision.
	return matched, EtcdState{
		Key: request.Key,
		PossibleValues: map[string]int64{
			request.PutData: response.Revision,
		},
	}
}

func stepDelete(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	if response.Err != nil {
		// -1 means the request fails
		// TODO(ahrtr): The downside is the history entry with empty value is overwritten.
		state.PossibleValues[""] = -1
		return true, state
	}

	matched := true
	var revAdded int64 = 0
	if response.Deleted != 0 {
		revAdded = 1
	}

	for _, rev := range state.PossibleValues {
		if response.Revision < (rev + revAdded) {
			matched = false
			break
		}
	}

	// Always cleanup all historical data, and only keep the latest value & revision.
	return matched, EtcdState{
		Key: request.Key,
		PossibleValues: map[string]int64{
			"": response.Revision,
		},
	}
}
