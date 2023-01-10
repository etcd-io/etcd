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
	"reflect"
	"strings"

	"github.com/anishathalye/porcupine"
)

type OperationType string

const (
	Get          OperationType = "get"
	Put          OperationType = "put"
	Delete       OperationType = "delete"
	Txn          OperationType = "txn"
	PutWithLease OperationType = "putWithLease"
	LeaseGrant   OperationType = "leaseGrant"
	LeaseRevoke  OperationType = "leaseRevoke"
)

func isWrite(t OperationType) bool {
	return t == Put || t == Delete || t == PutWithLease || t == LeaseRevoke || t == LeaseGrant
}

func inUnique(t OperationType) bool {
	return t == Put || t == PutWithLease
}

type EtcdRequest struct {
	Conds []EtcdCondition
	Ops   []EtcdOperation
}

type EtcdCondition struct {
	Key           string
	ExpectedValue string
}

type EtcdOperation struct {
	Type    OperationType
	Key     string
	Value   string
	LeaseID int64
}

type EtcdResponse struct {
	Err           error
	Revision      int64
	ResultUnknown bool
	TxnResult     bool
	OpsResult     []EtcdOperationResult
}

func Match(r1, r2 EtcdResponse) bool {
	return ((r1.ResultUnknown || r2.ResultUnknown) && (r1.Revision == r2.Revision)) || reflect.DeepEqual(r1, r2)
}

type EtcdOperationResult struct {
	Value   string
	Deleted int64
}

var leased = struct{}{}

type EtcdLease struct {
	LeaseID int64
	Keys    map[string]struct{}
}
type PossibleStates []EtcdState

type EtcdState struct {
	Revision  int64
	KeyValues map[string]string
	KeyLeases map[string]int64
	Leases    map[int64]EtcdLease
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
		return describeEtcdRequestResponse(in.(EtcdRequest), out.(EtcdResponse))
	},
}

func describeEtcdRequestResponse(request EtcdRequest, response EtcdResponse) string {
	prefix := describeEtcdOperations(request.Ops)
	if len(request.Conds) != 0 {
		prefix = fmt.Sprintf("if(%s).then(%s)", describeEtcdConditions(request.Conds), prefix)
	}

	return fmt.Sprintf("%s -> %s", prefix, describeEtcdResponse(request.Ops, response))
}

func describeEtcdConditions(conds []EtcdCondition) string {
	opsDescription := make([]string, len(conds))
	for i := range conds {
		opsDescription[i] = fmt.Sprintf("%s==%q", conds[i].Key, conds[i].ExpectedValue)
	}
	return strings.Join(opsDescription, " && ")
}

func describeEtcdOperations(ops []EtcdOperation) string {
	opsDescription := make([]string, len(ops))
	for i := range ops {
		opsDescription[i] = describeEtcdOperation(ops[i])
	}
	return strings.Join(opsDescription, ", ")
}

func describeEtcdResponse(ops []EtcdOperation, response EtcdResponse) string {
	if response.Err != nil {
		return fmt.Sprintf("err: %q", response.Err)
	}
	if response.ResultUnknown {
		return fmt.Sprintf("unknown, rev: %d", response.Revision)
	}
	if response.TxnResult {
		return fmt.Sprintf("txn failed, rev: %d", response.Revision)
	}
	respDescription := make([]string, len(response.OpsResult))
	for i := range response.OpsResult {
		respDescription[i] = describeEtcdOperationResponse(ops[i].Type, response.OpsResult[i])
	}
	respDescription = append(respDescription, fmt.Sprintf("rev: %d", response.Revision))
	return strings.Join(respDescription, ", ")
}

func describeEtcdOperation(op EtcdOperation) string {
	switch op.Type {
	case Get:
		return fmt.Sprintf("get(%q)", op.Key)
	case Put:
		return fmt.Sprintf("put(%q, %q)", op.Key, op.Value)
	case Delete:
		return fmt.Sprintf("delete(%q)", op.Key)
	case Txn:
		return "<! unsupported: nested transaction !>"
	case LeaseGrant:
		return fmt.Sprintf("leaseGrant(%d)", op.LeaseID)
	case LeaseRevoke:
		return fmt.Sprintf("leaseRevoke(%d)", op.LeaseID)
	case PutWithLease:
		return fmt.Sprintf("putWithLease(%q, %q, %d)", op.Key, op.Value, op.LeaseID)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", op.Type)
	}
}

func describeEtcdOperationResponse(op OperationType, resp EtcdOperationResult) string {
	switch op {
	case Get:
		if resp.Value == "" {
			return "nil"
		}
		return fmt.Sprintf("%q", resp.Value)
	case Put:
		return fmt.Sprintf("ok")
	case Delete:
		return fmt.Sprintf("deleted: %d", resp.Deleted)
	case Txn:
		return "<! unsupported: nested transaction !>"
	case LeaseGrant:
		return fmt.Sprintf("ok")
	case LeaseRevoke:
		return fmt.Sprintf("ok")
	case PutWithLease:
		return fmt.Sprintf("ok")
	default:
		return fmt.Sprintf("<! unknown op: %q !>", op)
	}
}

func step(states PossibleStates, request EtcdRequest, response EtcdResponse) (bool, PossibleStates) {
	if len(states) == 0 {
		// states were not initialized
		if response.Err != nil || response.ResultUnknown {
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
		KeyLeases: map[string]int64{},
		Leases:    map[int64]EtcdLease{},
	}
	if response.TxnResult {
		return state
	}
	for i, op := range request.Ops {
		opResp := response.OpsResult[i]
		switch op.Type {
		case Get:
			if opResp.Value != "" {
				state.KeyValues[op.Key] = opResp.Value
			}
		case Put:
			state.KeyValues[op.Key] = op.Value
		case Delete:
		case PutWithLease:
			if _, ok := state.Leases[op.LeaseID]; ok {
				state.KeyValues[op.Key] = op.Value
				//detach from old lease id but we dont expect that at init
				if _, ok := state.KeyLeases[op.Key]; ok {
					panic("old lease id found at init")
				}
				//attach to new lease id
				state.KeyLeases[op.Key] = op.LeaseID
				state.Leases[op.LeaseID].Keys[op.Key] = leased
			}
		case LeaseGrant:
			lease := EtcdLease{
				LeaseID: op.LeaseID,
				Keys:    map[string]struct{}{},
			}
			state.Leases[op.LeaseID] = lease
		case LeaseRevoke:
		default:
			panic("Unknown operation")
		}
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
		if Match(expectResponse, response) {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyRequestToSingleState handles a successful request, returning updated state and response it would generate.
func applyRequestToSingleState(s EtcdState, request EtcdRequest) (EtcdState, EtcdResponse) {
	success := true
	for _, cond := range request.Conds {
		if val := s.KeyValues[cond.Key]; val != cond.ExpectedValue {
			success = false
			break
		}
	}
	if !success {
		return s, EtcdResponse{Revision: s.Revision, TxnResult: true}
	}
	newKVs := map[string]string{}
	for k, v := range s.KeyValues {
		newKVs[k] = v
	}
	s.KeyValues = newKVs
	opResp := make([]EtcdOperationResult, len(request.Ops))
	increaseRevision := false

	for i, op := range request.Ops {
		switch op.Type {
		case Get:
			opResp[i].Value = s.KeyValues[op.Key]
		case Put:
			s.KeyValues[op.Key] = op.Value
			increaseRevision = true
			s = detachFromOldLease(s, op)
		case Delete:
			if _, ok := s.KeyValues[op.Key]; ok {
				delete(s.KeyValues, op.Key)
				increaseRevision = true
				s = detachFromOldLease(s, op)
				opResp[i].Deleted = 1
			}
		case PutWithLease:
			if _, ok := s.Leases[op.LeaseID]; ok {
				//handle put op.
				s.KeyValues[op.Key] = op.Value
				increaseRevision = true
				s = detachFromOldLease(s, op)
				s = attachToNewLease(s, op)
			}
		case LeaseRevoke:
			//Delete the keys attached to the lease
			keyDeleted := false
			for key, _ := range s.Leases[op.LeaseID].Keys {
				//same as delete.
				if _, ok := s.KeyValues[key]; ok {
					if !keyDeleted {
						keyDeleted = true
					}
					delete(s.KeyValues, key)
					delete(s.KeyLeases, key)
				}
			}
			//delete the lease
			delete(s.Leases, op.LeaseID)
			if keyDeleted {
				increaseRevision = true
			}
		case LeaseGrant:
			lease := EtcdLease{
				LeaseID: op.LeaseID,
				Keys:    map[string]struct{}{},
			}
			s.Leases[op.LeaseID] = lease
		default:
			panic("unsupported operation")
		}
	}

	if increaseRevision {
		s.Revision += 1
	}

	return s, EtcdResponse{OpsResult: opResp, Revision: s.Revision}
}

func detachFromOldLease(s EtcdState, op EtcdOperation) EtcdState {
	if oldLeaseId, ok := s.KeyLeases[op.Key]; ok {
		delete(s.Leases[oldLeaseId].Keys, op.Key)
		delete(s.KeyLeases, op.Key)
	}
	return s
}

func attachToNewLease(s EtcdState, op EtcdOperation) EtcdState {
	s.KeyLeases[op.Key] = op.LeaseID
	s.Leases[op.LeaseID].Keys[op.Key] = leased
	return s
}
