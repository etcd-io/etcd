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
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Operation string

const (
	Get    Operation = "get"
	Put    Operation = "put"
	Delete Operation = "delete"
	Txn    Operation = "txn"
)

type EtcdRequest struct {
	Op           Operation
	Key          string
	PutData      string
	TxnCondition []clientv3.Cmp
	TxnOnSuccess []clientv3.Op
}

type EtcdResponse struct {
	GetData      string
	Revision     int64
	Deleted      int64
	TxnSucceeded bool
	Err          error
}

type EtcdState struct {
	Revision  int64
	KeyValues map[string]string
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
			// TODO better print
			var conditions []string
			for _, cond := range request.TxnCondition {
				switch cond.Target {
				case etcdserverpb.Compare_VALUE:
					conditions = append(conditions, fmt.Sprintf("value(%q)=%q", string(cond.Key), string(cond.ValueBytes())))
				case etcdserverpb.Compare_CREATE:
					conditions = append(conditions, fmt.Sprintf("exists(%q)", string(cond.Key)))
				default:
					panic("target not supported")
				}
			}
			var onSuccess []string
			for _, op := range request.TxnOnSuccess {
				switch {
				case op.IsPut():
					onSuccess = append(onSuccess, fmt.Sprintf("put(%q, %q)", string(op.KeyBytes()), string(op.ValueBytes())))
				default:
					panic("operation not supported")
				}
			}

			if response.Err != nil {
				return fmt.Sprintf("txn.if(%s).then(%s) -> %s", strings.Join(conditions, ","), strings.Join(onSuccess, ","), response.Err)
			} else {
				return fmt.Sprintf("txn.if(%s).then(%s) -> %v, rev: %d", strings.Join(conditions, ","), strings.Join(onSuccess, ","), response.TxnSucceeded, response.Revision)
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
			for _, op := range request.TxnOnSuccess {
				switch {
				case op.IsPut():
					state.KeyValues[string(op.KeyBytes())] = string(op.ValueBytes())
				default:
					panic("Unhandled txn operation")
				}
			}
		}
		return []EtcdState{}
	default:
		panic("Unknown operation")
	}
	return []EtcdState{state}
}

func stepState(s EtcdState, request EtcdRequest) (EtcdState, EtcdResponse) {
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
		resp.TxnSucceeded = checkTxn(s, request)
		if resp.TxnSucceeded {
			for _, op := range request.TxnOnSuccess {
				switch {
				case op.IsPut():
					s.KeyValues[string(op.KeyBytes())] = string(op.ValueBytes())
					s.Revision += 1
				default:
					panic("Unhandled txn operation")
				}
			}
		}
	default:
		panic("unsupported operation")
	}
	resp.Revision = s.Revision
	return s, resp
}

func checkTxn(s EtcdState, request EtcdRequest) bool {
	for _, cmp := range request.TxnCondition {
		switch cmp.Target {
		case etcdserverpb.Compare_VALUE:
			if val, ok := s.KeyValues[string(cmp.Key)]; !ok || val != string(cmp.ValueBytes()) {
				return false
			}
		case etcdserverpb.Compare_CREATE:
			if (cmp.TargetUnion.(*etcdserverpb.Compare_CreateRevision)).CreateRevision != 0 {
				panic("Unsupported create revision condition")
			}
			if _, ok := s.KeyValues[string(cmp.Key)]; ok {
				return false
			}
		default:
			panic("Unsupported condition")
		}
	}
	return true
}
