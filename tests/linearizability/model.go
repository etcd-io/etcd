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
	getData string
	err     error
}

type EtcdState struct {
	Key          string
	Value        string
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
		if state.FailedWrites == nil {
			state.FailedWrites = map[string]struct{}{}
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
		var resp string
		switch request.op {
		case Get:
			if response.err != nil {
				resp = response.err.Error()
			} else {
				resp = response.getData
			}
			return fmt.Sprintf("get(%q) -> %q", request.key, resp)
		case Put:
			if response.err != nil {
				resp = response.err.Error()
			} else {
				resp = "ok"
			}
			return fmt.Sprintf("put(%q, %q) -> %s", request.key, request.putData, resp)
		default:
			return "<invalid>"
		}
	},
}

func step(state EtcdState, request etcdRequest, response etcdResponse) (bool, EtcdState) {
	if request.key == "" {
		panic("Invalid request")
	}
	if state.Key == "" {
		state.Key = request.key
	}
	if state.Key != request.key {
		panic("Multiple keys not supported")
	}
	switch request.op {
	case Get:
		if state.Value == response.getData {
			return true, state
		}
		for write := range state.FailedWrites {
			if write == response.getData {
				state.Value = response.getData
				delete(state.FailedWrites, write)
				return true, state
			}
		}
	case Put:
		if response.err == nil {
			state.Value = request.putData
		} else {
			state.FailedWrites[request.putData] = struct{}{}
		}
		return true, state
	}
	return false, state
}
