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
	Txn    Operation = "txn"
)

type EtcdRequest struct {
	Op            Operation
	Key           string
	PutData       string
	TxnExpectData string
	TxnNewData    string
}

type EtcdResponse struct {
	GetData      string
	Revision     int64
	Deleted      int64
	TxnSucceeded bool
	Err          error
}

type PossibleStates []EtcdState

type EtcdState struct {
	Revision  int64
	KeyValues map[string]string
}

var etcdModel = porcupine.Model{
	Init: func() interface{} {
		return "[]" // empty PossibleStates
	},
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
		var states PossibleStates
		err := json.Unmarshal([]byte(st.(string)), &states)
		if err != nil {
			panic(err)
		}
		ok, states := step(states, in.(EtcdRequest), out.(EtcdResponse))
		data, err := json.Marshal(states)
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
		case Txn:
			if response.Err != nil {
				return fmt.Sprintf("txn(if(value(%q)=%q).then(put(%q, %q)) -> %s", request.Key, request.TxnExpectData, request.Key, request.TxnNewData, response.Err)
			} else {
				return fmt.Sprintf("txn(if(value(%q)=%q).then(put(%q, %q)) -> %v, rev: %d", request.Key, request.TxnExpectData, request.Key, request.TxnNewData, response.TxnSucceeded, response.Revision)
			}
		default:
			return "<invalid>"
		}
	},
}

func step(states PossibleStates, request EtcdRequest, response EtcdResponse) (bool, PossibleStates) {
	if len(states) == 0 {
		// states were not initialized
		if response.Err != nil {
			return true, nil
		}
		return true, PossibleStates{initState(request, response)}
	}
	if response.Err != nil {
		states = applyFailedRequest(states, request)
	} else {
		states = applyRequest(states, request, response)
	}
	return len(states) > 0, states
}

// initState tries to create etcd state based on the first request.
func initState(request EtcdRequest, response EtcdResponse) EtcdState {
	state := EtcdState{
		Revision:  response.Revision,
		KeyValues: map[string]string{},
	}
	switch request.Op {
	case Get:
		if response.GetData != "" {
			state.KeyValues[request.Key] = response.GetData
		}
	case Put:
		state.KeyValues[request.Key] = request.PutData
	case Delete:
	case Txn:
		if response.TxnSucceeded {
			state.KeyValues[request.Key] = request.TxnNewData
		}
	default:
		panic("Unknown operation")
	}
	return state
}

// applyFailedRequest handles a failed requests, one that it's not known if it was persisted or not.
func applyFailedRequest(states PossibleStates, request EtcdRequest) PossibleStates {
	for _, s := range states {
		newState, _ := applyRequestToSingleState(s, request)
		states = append(states, newState)
	}
	return states
}

// applyRequest handles a successful request by applying it to possible states and checking if they match the response.
func applyRequest(states PossibleStates, request EtcdRequest, response EtcdResponse) PossibleStates {
	newStates := make(PossibleStates, 0, len(states))
	for _, s := range states {
		newState, expectResponse := applyRequestToSingleState(s, request)
		if expectResponse == response {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyRequestToSingleState handles a successful request, returning updated state and response it would generate.
func applyRequestToSingleState(s EtcdState, request EtcdRequest) (EtcdState, EtcdResponse) {
	newKVs := map[string]string{}
	for k, v := range s.KeyValues {
		newKVs[k] = v
	}
	s.KeyValues = newKVs
	resp := EtcdResponse{}
	switch request.Op {
	case Get:
		resp.GetData = s.KeyValues[request.Key]
	case Put:
		s.KeyValues[request.Key] = request.PutData
		s.Revision += 1
	case Delete:
		if _, ok := s.KeyValues[request.Key]; ok {
			delete(s.KeyValues, request.Key)
			s.Revision += 1
			resp.Deleted = 1
		}
	case Txn:
		if val := s.KeyValues[request.Key]; val == request.TxnExpectData {
			s.KeyValues[request.Key] = request.TxnNewData
			s.Revision += 1
			resp.TxnSucceeded = true
		}
	default:
		panic("unsupported operation")
	}
	resp.Revision = s.Revision
	return s, resp
}
