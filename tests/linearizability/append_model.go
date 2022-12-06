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
	"strings"

	"github.com/anishathalye/porcupine"
)

const (
	Append Operation = "append"
)

type AppendRequest struct {
	Op         Operation
	Key        string
	AppendData string
}

type AppendResponse struct {
	GetData string
}

type AppendState struct {
	Key      string
	Elements []string
}

var appendModel = porcupine.Model{
	Init: func() interface{} { return "{}" },
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
		var state AppendState
		err := json.Unmarshal([]byte(st.(string)), &state)
		if err != nil {
			panic(err)
		}
		ok, state := appendModelStep(state, in.(AppendRequest), out.(AppendResponse))
		data, err := json.Marshal(state)
		if err != nil {
			panic(err)
		}
		return ok, string(data)
	},
	DescribeOperation: func(in, out interface{}) string {
		request := in.(AppendRequest)
		response := out.(AppendResponse)
		switch request.Op {
		case Get:
			elements := strings.Split(response.GetData, ",")
			return fmt.Sprintf("get(%q) -> %q", request.Key, elements[len(elements)-1])
		case Append:
			return fmt.Sprintf("append(%q, %q)", request.Key, request.AppendData)
		default:
			return "<invalid>"
		}
	},
}

func appendModelStep(state AppendState, request AppendRequest, response AppendResponse) (bool, AppendState) {
	if request.Key == "" {
		panic("invalid request")
	}
	if state.Key == "" {
		return true, initAppendState(request, response)
	}
	if state.Key != request.Key {
		panic("Multiple keys not supported")
	}
	switch request.Op {
	case Get:
		return stepAppendGet(state, request, response)
	case Append:
		return stepAppend(state, request, response)
	default:
		panic("Unknown operation")
	}
}

func initAppendState(request AppendRequest, response AppendResponse) AppendState {
	state := AppendState{
		Key: request.Key,
	}
	switch request.Op {
	case Get:
		state.Elements = elements(response)
	case Append:
		state.Elements = []string{request.AppendData}
	default:
		panic("Unknown operation")
	}
	return state
}

func stepAppendGet(state AppendState, request AppendRequest, response AppendResponse) (bool, AppendState) {
	newElements := elements(response)
	if len(newElements) < len(state.Elements) {
		return false, state
	}

	for i := 0; i < len(state.Elements); i++ {
		if state.Elements[i] != newElements[i] {
			return false, state
		}
	}
	state.Elements = newElements
	return true, state
}

func stepAppend(state AppendState, request AppendRequest, response AppendResponse) (bool, AppendState) {
	if request.AppendData == "" {
		panic("unsupported empty appendData")
	}
	state.Elements = append(state.Elements, request.AppendData)
	return true, state
}

func elements(response AppendResponse) []string {
	elements := strings.Split(response.GetData, ",")
	if len(elements) == 1 && elements[0] == "" {
		elements = []string{}
	}
	return elements
}
