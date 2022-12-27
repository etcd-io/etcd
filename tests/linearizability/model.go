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
	"time"
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
	leaseID       int64
}

type EtcdResponse struct {
	GetData      string
	Revision     int64
	Deleted      int64
	TxnSucceeded bool
	leaseID      int64
	leaseExpiry  time.Time
	Err          error
}

type EtcdLease struct {
	LeaseID     int64
	LeaseExpiry time.Time
}

type EtcdState struct {
	Key            string
	PossibleValues []ValueRevision
	Leases         map[int64]EtcdLease
}

type ValueRevision struct {
	Value        string
	Revision     int64
	LeaseID      int64
	LeaseExpired bool
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
				return fmt.Sprintf("put(%q, %q, %d) -> %s", request.Key, request.PutData, request.leaseID, response.Err)
			} else {
				return fmt.Sprintf("put(%q, %q, %d) -> ok, rev: %d", request.Key, request.PutData, request.leaseID, response.Revision)
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
				//TODO leaseID may not be populated if the lease req is failing
				return fmt.Sprintf("leaseGrant(%q) -> %q", response.leaseID, response.Err)
			} else {
				return fmt.Sprintf("leaseGrant(%q) -> %q", response.leaseID, response.leaseExpiry)
			}
		case LeaseRevoke:
			if response.Err != nil {
				return fmt.Sprintf("leaseRevoke(%q) -> %q", request.leaseID, response.Err)
			} else {
				return fmt.Sprintf("leaseRevoke(%q) -> %q", request.leaseID, response.Revision)
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

	if !stepLease(request, response, state) {
		return len(state.PossibleValues) > 0, state
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
			v.LeaseExpired = false
			if _, ok := state.Leases[v.LeaseID]; !ok {
				if v.LeaseID != 0 {
					v.LeaseExpired = true
				}
			}
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
		//TODO anything here for lease operations?
	default:
		panic("Unknown operation")
	}
}

func stepValue(v ValueRevision, request EtcdRequest) (ValueRevision, EtcdResponse) {
	resp := EtcdResponse{}

	switch request.Op {
	case Get:
		//if the key is attached to a lease and the lease has expired, the key should not be there
		//when the lease expires, the revision increments.
		resp.GetData = v.Value
	case Put:
		v.Value = request.PutData
		v.Revision += 1
		//a Put with no lease on the same key will detach the key from the lease
		v.LeaseID = 0
		v.LeaseExpired = false
	case PutWithLease:
		if !v.LeaseExpired {
			//only update if the lease is ok
			v.Value = request.PutData
			v.Revision += 1
			v.LeaseID = request.leaseID
		}
	case Delete:
		if v.Value != "" {
			v.Value = ""
			v.Revision += 1
			v.LeaseID = 0
			//reset to default value
			v.LeaseExpired = false
			resp.Deleted = 1
		}
	case Txn:
		if v.Value == request.TxnExpectData {
			v.Value = request.TxnNewData
			v.Revision += 1
			resp.TxnSucceeded = true
		}
	case LeaseRevoke:
		//If the lease is attached, delete the key
		if v.LeaseID == request.leaseID {
			//key gets deleted.
			if v.Value != "" {
				v.Value = ""
				v.Revision += 1
				v.LeaseID = 0
				//reset to default value
				v.LeaseExpired = false
			}
		}
	default:
		panic("unsupported operation")
	}

	resp.Revision = v.Revision
	return v, resp
}

// return true with a lease object if a new lease was acquired
func stepLease(request EtcdRequest, resp EtcdResponse, state EtcdState) bool {
	proceedToNextStep := false
	switch request.Op {
	case LeaseGrant:
		if resp.Err != nil {
			proceedToNextStep = false
			break
		}
		lease := EtcdLease{LeaseID: resp.leaseID, LeaseExpiry: resp.leaseExpiry}
		state.Leases[resp.leaseID] = lease
		proceedToNextStep = true
	case LeaseRevoke:
		if resp.Err != nil {
			proceedToNextStep = false
			break
		}
		delete(state.Leases, resp.leaseID)
		proceedToNextStep = true
	default:
		proceedToNextStep = true
	}
	return proceedToNextStep
}
