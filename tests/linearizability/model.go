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
	PossibleValues []ValueRevision
}

type ValueRevision struct {
	Value    string
	Revision int64
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
		if ok, v := initValueRevision(request, response); ok {
			state.PossibleValues = append(state.PossibleValues, v)
		}
		return true, state
	}
	if state.Key != request.Key {
		panic("multiple keys not supported")
	}
	if response.Err != nil {
		for _, v := range state.PossibleValues {
			newV, _ := stepValue(v, request)
			state.PossibleValues = append(state.PossibleValues, newV)
		}
	} else {
		var i = 0
		for _, v := range state.PossibleValues {
			newV, expectedResponse := stepValue(v, request)
			if expectedResponse == response {
				state.PossibleValues[i] = newV
				i++
			}
		}
		state.PossibleValues = state.PossibleValues[:i]
	}
	return len(state.PossibleValues) > 0, state
}

func initValueRevision(request EtcdRequest, response EtcdResponse) (ok bool, v ValueRevision) {
	if response.Err != nil {
		return false, ValueRevision{}
	}
	switch request.Op {
	case Get:
		return true, ValueRevision{
			Value:    response.GetData,
			Revision: response.Revision,
		}
	case Put:
		return true, ValueRevision{
			Value:    request.PutData,
			Revision: response.Revision,
		}
	case Delete:
		return true, ValueRevision{
			Value:    "",
			Revision: response.Revision,
		}
	case Txn:
		if response.TxnSucceeded {
			return true, ValueRevision{
				Value:    request.TxnNewData,
				Revision: response.Revision,
			}
		}
		return false, ValueRevision{}
	default:
		panic("Unknown operation")
	}
}

func stepValue(v ValueRevision, request EtcdRequest) (ValueRevision, EtcdResponse) {
	resp := EtcdResponse{}
	switch request.Op {
	case Get:
		resp.GetData = v.Value
	case Put:
		v.Value = request.PutData
		v.Revision += 1
	case Delete:
		if v.Value != "" {
			v.Value = ""
			v.Revision += 1
			resp.Deleted = 1
		}
	case Txn:
		if v.Value == request.TxnExpectData {
			v.Value = request.TxnNewData
			v.Revision += 1
			resp.TxnSucceeded = true
		}
	default:
		panic("unsupported operation")
	}
	resp.Revision = v.Revision
	return v, resp
}
