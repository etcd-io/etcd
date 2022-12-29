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
	Get          Operation = "get"
	Put          Operation = "put"
	Delete       Operation = "delete"
	Txn          Operation = "txn"
	PutWithLease Operation = "putWithLease"
	LeaseGrant   Operation = "leaseGrant"
	LeaseRevoke  Operation = "leaseRevoke"
)

type EtcdRequest struct {
	Op            Operation
	Key           string
	PutData       string
	TxnExpectData string
	TxnNewData    string
	LeaseID       int64
}

type EtcdResponse struct {
	GetData      string
	Revision     int64
	Deleted      int64
	TxnSucceeded bool
	Err          error
}

type EtcdLease struct {
	LeaseID int64
	//TODO state about key attachment?
}

type EtcdState struct {
	Revision int64
	Key      string
	Value    string
	Leases   map[int64]EtcdLease
	LeaseID  int64
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
			if response.Err != nil {
				return fmt.Sprintf("txn(if(value(%q)=%q).then(put(%q, %q)) -> %s", request.Key, request.TxnExpectData, request.Key, request.TxnNewData, response.Err)
			} else {
				return fmt.Sprintf("txn(if(value(%q)=%q).then(put(%q, %q)) -> %v, rev: %d", request.Key, request.TxnExpectData, request.Key, request.TxnNewData, response.TxnSucceeded, response.Revision)
			}
		case LeaseGrant:
			if response.Err != nil {
				return fmt.Sprintf("leaseGrant(%q) -> %q", request.LeaseID, response.Err)
			} else {
				return fmt.Sprintf("leaseGrant(%q) -> ok rev: %q", request.LeaseID, response.Revision)
			}
		case LeaseRevoke:
			if response.Err != nil {
				return fmt.Sprintf("leaseRevoke(%q) -> %q", request.LeaseID, response.Err)
			} else {
				return fmt.Sprintf("leaseRevoke(%q) -> ok rev: %q", request.LeaseID, response.Revision)
			}
		case PutWithLease:
			if response.Err != nil {
				return fmt.Sprintf("putWithLease(%q, %q, %d) -> %s", request.Key, request.PutData, request.LeaseID, response.Err)
			} else {
				return fmt.Sprintf("putWithLease(%q, %q, %d) -> ok, rev: %d", request.Key, request.PutData, request.LeaseID, response.Revision)
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
		Key:      request.Key,
		Revision: response.Revision,
	}
	state.Leases = make(map[int64]EtcdLease)

	switch request.Op {
	case Get:
		if response.GetData != "" {
			state.Value = response.GetData
		}
	case Put:
		state.Value = request.PutData
	case Delete:
	case Txn:
		if response.TxnSucceeded {
			state.Value = request.TxnNewData
		}
		return []EtcdState{}
	case PutWithLease:
		if _, ok := state.Leases[request.LeaseID]; ok {
			state.Value = request.PutData
		}
	case LeaseGrant:
		lease := EtcdLease{LeaseID: request.LeaseID}
		state.Leases[request.LeaseID] = lease
	case LeaseRevoke:
	default:
		panic("Unknown operation")
	}
	return []EtcdState{state}
}

func stepState(s EtcdState, request EtcdRequest) (EtcdState, EtcdResponse) {
	if s.Key != request.Key {
		panic("multiple keys not supported")
	}
	resp := EtcdResponse{}
	switch request.Op {
	case Get:
		resp.GetData = s.Value
	case Put:
		s.Value = request.PutData
		s.Revision += 1
		s.LeaseID = 0
	case Delete:
		if s.Value != "" {
			s.Value = ""
			s.Revision += 1
			resp.Deleted = 1
		}
	case Txn:
		if s.Value == request.TxnExpectData {
			s.Value = request.TxnNewData
			s.Revision += 1
			resp.TxnSucceeded = true
		}
	case PutWithLease:
		if _, ok := s.Leases[request.LeaseID]; ok {
			//same as put but only update if the lease is ok
			s.Value = request.PutData
			s.Revision += 1
			s.LeaseID = request.LeaseID
		}
	case LeaseRevoke:
		delete(s.Leases, request.LeaseID)
		//If the lease is attached, delete the key
		if s.LeaseID == request.LeaseID {
			//same as delete.
			if s.Value != "" {
				s.Value = ""
				s.Revision += 1
			}
		}
	case LeaseGrant:
		lease := EtcdLease{LeaseID: request.LeaseID}
		s.Leases[request.LeaseID] = lease
	default:
		panic("unsupported operation")
	}
	resp.Revision = s.Revision
	return s, resp
}
