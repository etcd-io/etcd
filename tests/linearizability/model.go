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

type EtcdState struct {
	Key            string
	PossibleValues []valueRevision
}

func (resp EtcdResponse) Equals(other EtcdResponse) bool {
	return resp.GetData == other.GetData && resp.Revision == other.Revision && resp.Deleted == other.Deleted && resp.Err == other.Err && resp.TxnSucceeded == other.TxnSucceeded
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

func step(state EtcdState, request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	if len(state.PossibleValues) == 0 {
		state.Key = request.Key
		ok, v := initValueRevision(request, response)
		if ok {
			state.PossibleValues = append(state.PossibleValues, v)
		}
		return true, state
	}
	if state.Key != request.Key {
		panic("multiple keys not supported")
	}
	if response.Err != nil {
		for _, kv := range state.PossibleValues {
			kv.handle(request)
			state.PossibleValues = append(state.PossibleValues, kv)
		}
	} else {
		var i = 0
		for _, kv := range state.PossibleValues {
			expectedResponse := kv.handle(request)
			ok := expectedResponse.Equals(response)
			if ok {
				state.PossibleValues[i] = kv
				i++
			}
		}
		state.PossibleValues = state.PossibleValues[:i]
	}
	return len(state.PossibleValues) > 0, state
}

type valueRevision struct {
	Value    string
	Revision int64
}

func initValueRevision(request EtcdRequest, response EtcdResponse) (ok bool, v valueRevision) {
	if response.Err != nil {
		return false, valueRevision{}
	}
	switch request.Op {
	case Get:
		return true, valueRevision{
			Value:    response.GetData,
			Revision: response.Revision,
		}
	case Put:
		return true, valueRevision{
			Value:    request.PutData,
			Revision: response.Revision,
		}
	case Delete:
		return true, valueRevision{
			Value:    "",
			Revision: response.Revision,
		}
	case Txn:
		if response.TxnSucceeded {
			return true, valueRevision{
				Value:    request.TxnNewData,
				Revision: response.Revision,
			}
		}
		return false, valueRevision{}
	default:
		panic("Unknown operation")
	}
}

func (v *valueRevision) handle(request EtcdRequest) EtcdResponse {
	response := EtcdResponse{}
	switch request.Op {
	case Get:
		response.GetData = v.Value
	case Put:
		v.Value = request.PutData
		v.Revision += 1
	case Delete:
		if v.Value != "" {
			v.Value = ""
			v.Revision += 1
			response.Deleted = 1
		}
	case Txn:
		if v.Value == request.TxnExpectData {
			v.Value = request.TxnNewData
			v.Revision += 1
			response.TxnSucceeded = true
		}
	default:
		panic("unsupported operation")
	}
	response.Revision = v.Revision
	return response
}
