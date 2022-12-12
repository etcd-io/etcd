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
	Op  Operation
	Key string

	PutData       string
	TxnExpectData string
	TxnNewData    string
}

type EtcdResponse struct {
	Revision int64
	Err      error

	GetData        string
	GetModRevision int64
	Deleted        int64
	TxnSucceeded   bool
}

type EtcdState struct {
	Revision  int64
	KeyValues map[string]Value
}

type Value struct {
	Data        string
	ModRevision int64
}

var etcdModel = porcupine.Model{
	Init: func() interface{} { return "[]" },
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
		var states []EtcdState
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
				return fmt.Sprintf("get(%q) -> %q, mod-rev: %d, rev: %d", request.Key, response.GetData, response.GetModRevision, response.Revision)
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
				return fmt.Sprintf("delete(%q) -> deleted:%d, rev: %d", request.Key, response.Revision, response.Deleted)
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

func step(states []EtcdState, request EtcdRequest, response EtcdResponse) (bool, []EtcdState) {
	if len(states) == 0 {
		return true, initStates(request, response)
	}
	if response.Err != nil {
		// Add addition states for failed request in case of failed request was persisted.
		states = append(states, applyRequest(states, request)...)
	} else {
		// Remove states that didn't lead to response we got.
		states = filterStateMatchesResponse(states, request, response)
	}
	return len(states) > 0, states
}

func applyRequest(states []EtcdState, request EtcdRequest) []EtcdState {
	newStates := make([]EtcdState, 0, len(states))
	for _, s := range states {
		newState, _ := stepState(s, request)
		newStates = append(newStates, newState)
	}
	return newStates
}

func filterStateMatchesResponse(states []EtcdState, request EtcdRequest, response EtcdResponse) []EtcdState {
	newStates := make([]EtcdState, 0, len(states))
	for _, s := range states {
		newState, expectResponse := stepState(s, request)
		if expectResponse == response {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

func initStates(request EtcdRequest, response EtcdResponse) []EtcdState {
	if response.Err != nil {
		return []EtcdState{}
	}
	state := EtcdState{
		Revision:  response.Revision,
		KeyValues: map[string]Value{},
	}
	switch request.Op {
	case Get:
		if response.GetData != "" {
			state.KeyValues[request.Key] = Value{Data: response.GetData, ModRevision: response.GetModRevision}
		}
	case Put:
		state.KeyValues[request.Key] = Value{Data: request.PutData, ModRevision: response.Revision}
	case Delete:
	case Txn:
		if response.TxnSucceeded {
			state.KeyValues[request.Key] = Value{Data: request.TxnNewData, ModRevision: response.Revision}
		}
		return []EtcdState{}
	default:
		panic("Unknown operation")
	}
	return []EtcdState{state}
}

func stepState(s EtcdState, request EtcdRequest) (EtcdState, EtcdResponse) {
	newKVs := map[string]Value{}
	for k, v := range s.KeyValues {
		newKVs[k] = v
	}
	s.KeyValues = newKVs
	resp := EtcdResponse{}
	switch request.Op {
	case Get:
		resp.GetData = s.KeyValues[request.Key].Data
		resp.GetModRevision = s.KeyValues[request.Key].ModRevision
	case Put:
		s.Revision += 1
		s.KeyValues[request.Key] = Value{Data: request.PutData, ModRevision: s.Revision}
	case Delete:
		if _, ok := s.KeyValues[request.Key]; ok {
			delete(s.KeyValues, request.Key)
			s.Revision += 1
			resp.Deleted = 1
		}
	case Txn:
		if val := s.KeyValues[request.Key]; val.Data == request.TxnExpectData {
			s.Revision += 1
			s.KeyValues[request.Key] = Value{Data: request.TxnNewData, ModRevision: s.Revision}
			resp.TxnSucceeded = true
		}
	default:
		panic("unsupported operation")
	}
	resp.Revision = s.Revision
	return s, resp
}
