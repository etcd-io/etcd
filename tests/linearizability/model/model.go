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

package model

import (
	"encoding/json"
	"fmt"
	"github.com/anishathalye/porcupine"
	"reflect"
	"strings"
)

type OperationType string

const (
	Get    OperationType = "get"
	Put    OperationType = "put"
	Delete OperationType = "delete"
)

var Etcd = porcupine.Model{
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

type RequestType string

const (
	Txn         RequestType = "txn"
	LeaseGrant  RequestType = "leaseGrant"
	LeaseRevoke RequestType = "leaseRevoke"
)

type EtcdRequest struct {
	Type        RequestType
	LeaseGrant  *LeaseGrantRequest
	LeaseRevoke *LeaseRevokeRequest
	Txn         *TxnRequest
}

type TxnRequest struct {
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

type LeaseGrantRequest struct {
	LeaseID int64
}
type LeaseRevokeRequest struct {
	LeaseID int64
}

type EtcdResponse struct {
	Err           error
	Revision      int64
	ResultUnknown bool
	Txn           *TxnResponse
	LeaseGrant    *LeaseGrantReponse
	LeaseRevoke   *LeaseRevokeResponse
}

type TxnResponse struct {
	TxnResult bool
	OpsResult []EtcdOperationResult
}

type LeaseGrantReponse struct {
	LeaseID int64
}
type LeaseRevokeResponse struct{}

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

func describeEtcdRequestResponse(request EtcdRequest, response EtcdResponse) string {
	return fmt.Sprintf("%s -> %s", describeEtcdRequest(request), describeEtcdResponse(request, response))
}

func describeEtcdResponse(request EtcdRequest, response EtcdResponse) string {
	if response.Err != nil {
		return fmt.Sprintf("err: %q", response.Err)
	}
	if response.ResultUnknown {
		return fmt.Sprintf("unknown, rev: %d", response.Revision)
	}
	if request.Type == Txn {
		return fmt.Sprintf("%s, rev: %d", describeTxnResponse(request.Txn, response.Txn), response.Revision)
	} else {
		return fmt.Sprintf("ok, rev: %d", response.Revision)
	}
}

func describeEtcdRequest(request EtcdRequest) string {
	switch request.Type {
	case Txn:
		describeOperations := describeEtcdOperations(request.Txn.Ops)
		if len(request.Txn.Conds) != 0 {
			return fmt.Sprintf("if(%s).then(%s)", describeEtcdConditions(request.Txn.Conds), describeOperations)
		}
		return describeOperations
	case LeaseGrant:
		return fmt.Sprintf("leaseGrant(%d)", request.LeaseGrant.LeaseID)
	case LeaseRevoke:
		return fmt.Sprintf("leaseRevoke(%d)", request.LeaseRevoke.LeaseID)
	default:
		return fmt.Sprintf("<! unknown request type: %q !>", request.Type)
	}
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

func describeTxnResponse(request *TxnRequest, response *TxnResponse) string {
	if response.TxnResult {
		return fmt.Sprintf("txn failed")
	}
	respDescription := make([]string, len(response.OpsResult))
	for i := range response.OpsResult {
		respDescription[i] = describeEtcdOperationResponse(request.Ops[i].Type, response.OpsResult[i])
	}
	return strings.Join(respDescription, ", ")
}

func describeEtcdOperation(op EtcdOperation) string {
	switch op.Type {
	case Get:
		return fmt.Sprintf("get(%q)", op.Key)
	case Put:
		if op.LeaseID != 0 {
			return fmt.Sprintf("put(%q, %q, %d)", op.Key, op.Value, op.LeaseID)
		}
		return fmt.Sprintf("put(%q, %q, nil)", op.Key, op.Value)
	case Delete:
		return fmt.Sprintf("delete(%q)", op.Key)
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
	switch request.Type {
	case Txn:
		if response.Txn.TxnResult {
			return state
		}
		for i, op := range request.Txn.Ops {
			opResp := response.Txn.OpsResult[i]
			switch op.Type {
			case Get:
				if opResp.Value != "" {
					state.KeyValues[op.Key] = opResp.Value
				}
			case Put:
				state.KeyValues[op.Key] = op.Value
			case Delete:
			default:
				panic("Unknown operation")
			}
		}
	case LeaseGrant:
		lease := EtcdLease{
			LeaseID: request.LeaseGrant.LeaseID,
			Keys:    map[string]struct{}{},
		}
		state.Leases[request.LeaseGrant.LeaseID] = lease
	case LeaseRevoke:
	default:
		panic(fmt.Sprintf("Unknown request type: %v", request.Type))
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
	newKVs := map[string]string{}
	for k, v := range s.KeyValues {
		newKVs[k] = v
	}
	s.KeyValues = newKVs
	switch request.Type {
	case Txn:
		success := true
		for _, cond := range request.Txn.Conds {
			if val := s.KeyValues[cond.Key]; val != cond.ExpectedValue {
				success = false
				break
			}
		}
		if !success {
			return s, EtcdResponse{Revision: s.Revision, Txn: &TxnResponse{TxnResult: true}}
		}
		opResp := make([]EtcdOperationResult, len(request.Txn.Ops))
		increaseRevision := false
		for i, op := range request.Txn.Ops {
			switch op.Type {
			case Get:
				opResp[i].Value = s.KeyValues[op.Key]
			case Put:
				_, leaseExists := s.Leases[op.LeaseID]
				if op.LeaseID != 0 && !leaseExists {
					break
				}
				s.KeyValues[op.Key] = op.Value
				increaseRevision = true
				s = detachFromOldLease(s, op.Key)
				if leaseExists {
					s = attachToNewLease(s, op.LeaseID, op.Key)
				}
			case Delete:
				if _, ok := s.KeyValues[op.Key]; ok {
					delete(s.KeyValues, op.Key)
					increaseRevision = true
					s = detachFromOldLease(s, op.Key)
					opResp[i].Deleted = 1
				}
			default:
				panic("unsupported operation")
			}
		}
		if increaseRevision {
			s.Revision += 1
		}
		return s, EtcdResponse{Txn: &TxnResponse{OpsResult: opResp}, Revision: s.Revision}
	case LeaseGrant:
		lease := EtcdLease{
			LeaseID: request.LeaseGrant.LeaseID,
			Keys:    map[string]struct{}{},
		}
		s.Leases[request.LeaseGrant.LeaseID] = lease
		return s, EtcdResponse{Revision: s.Revision, LeaseGrant: &LeaseGrantReponse{}}
	case LeaseRevoke:
		//Delete the keys attached to the lease
		keyDeleted := false
		for key, _ := range s.Leases[request.LeaseRevoke.LeaseID].Keys {
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
		delete(s.Leases, request.LeaseRevoke.LeaseID)
		if keyDeleted {
			s.Revision += 1
		}
		return s, EtcdResponse{Revision: s.Revision, LeaseRevoke: &LeaseRevokeResponse{}}
	default:
		panic(fmt.Sprintf("Unknown request type: %v", request.Type))
	}
}

func detachFromOldLease(s EtcdState, key string) EtcdState {
	if oldLeaseId, ok := s.KeyLeases[key]; ok {
		delete(s.Leases[oldLeaseId].Keys, key)
		delete(s.KeyLeases, key)
	}
	return s
}

func attachToNewLease(s EtcdState, leaseID int64, key string) EtcdState {
	s.KeyLeases[key] = leaseID
	s.Leases[leaseID].Keys[key] = leased
	return s
}
