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
	"hash/fnv"
	"reflect"
	"sort"
	"strings"

	"github.com/anishathalye/porcupine"
)

type OperationType string

const (
	Range  OperationType = "range"
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
	Defragment  RequestType = "defragment"
)

type EtcdRequest struct {
	Type        RequestType
	LeaseGrant  *LeaseGrantRequest
	LeaseRevoke *LeaseRevokeRequest
	Txn         *TxnRequest
	Defragment  *DefragmentRequest
}

type TxnRequest struct {
	Conds []EtcdCondition
	Ops   []EtcdOperation
}

type EtcdCondition struct {
	Key              string
	ExpectedRevision int64
}

type EtcdOperation struct {
	Type       OperationType
	Key        string
	WithPrefix bool
	Value      ValueOrHash
	LeaseID    int64
}

type LeaseGrantRequest struct {
	LeaseID int64
}
type LeaseRevokeRequest struct {
	LeaseID int64
}
type DefragmentRequest struct{}

type EtcdResponse struct {
	Err           error
	Revision      int64
	ResultUnknown bool
	Txn           *TxnResponse
	LeaseGrant    *LeaseGrantReponse
	LeaseRevoke   *LeaseRevokeResponse
	Defragment    *DefragmentResponse
}

type TxnResponse struct {
	TxnResult bool
	OpsResult []EtcdOperationResult
}

type LeaseGrantReponse struct {
	LeaseID int64
}
type LeaseRevokeResponse struct{}
type DefragmentResponse struct{}

func Match(r1, r2 EtcdResponse) bool {
	return ((r1.ResultUnknown || r2.ResultUnknown) && (r1.Revision == r2.Revision)) || reflect.DeepEqual(r1, r2)
}

type EtcdOperationResult struct {
	KVs     []KeyValue
	Deleted int64
}

type KeyValue struct {
	Key string
	ValueRevision
}

var leased = struct{}{}

type EtcdLease struct {
	LeaseID int64
	Keys    map[string]struct{}
}
type PossibleStates []EtcdState

type EtcdState struct {
	Revision  int64
	KeyValues map[string]ValueRevision
	KeyLeases map[string]int64
	Leases    map[int64]EtcdLease
}

type ValueRevision struct {
	Value       ValueOrHash
	ModRevision int64
}

type ValueOrHash struct {
	Value string
	Hash  uint32
}

func ToValueOrHash(value string) ValueOrHash {
	v := ValueOrHash{}
	if len(value) < 20 {
		v.Value = value
	} else {
		h := fnv.New32a()
		h.Write([]byte(value))
		v.Hash = h.Sum32()
	}
	return v
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
	}
	if response.Revision == 0 {
		return "ok"
	}
	return fmt.Sprintf("ok, rev: %d", response.Revision)
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
	case Defragment:
		return fmt.Sprintf("defragment()")
	default:
		return fmt.Sprintf("<! unknown request type: %q !>", request.Type)
	}
}

func describeEtcdConditions(conds []EtcdCondition) string {
	opsDescription := make([]string, len(conds))
	for i := range conds {
		opsDescription[i] = fmt.Sprintf("mod_rev(%s)==%d", conds[i].Key, conds[i].ExpectedRevision)
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
		respDescription[i] = describeEtcdOperationResponse(request.Ops[i], response.OpsResult[i])
	}
	return strings.Join(respDescription, ", ")
}

func describeEtcdOperation(op EtcdOperation) string {
	switch op.Type {
	case Range:
		if op.WithPrefix {
			return fmt.Sprintf("range(%q)", op.Key)
		}
		return fmt.Sprintf("get(%q)", op.Key)
	case Put:
		if op.LeaseID != 0 {
			return fmt.Sprintf("put(%q, %s, %d)", op.Key, describeValueOrHash(op.Value), op.LeaseID)
		}
		return fmt.Sprintf("put(%q, %s)", op.Key, describeValueOrHash(op.Value))
	case Delete:
		return fmt.Sprintf("delete(%q)", op.Key)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", op.Type)
	}
}

func describeEtcdOperationResponse(req EtcdOperation, resp EtcdOperationResult) string {
	switch req.Type {
	case Range:
		if req.WithPrefix {
			kvs := make([]string, len(resp.KVs))
			for i, kv := range resp.KVs {
				kvs[i] = describeValueOrHash(kv.Value)
			}
			return fmt.Sprintf("[%s]", strings.Join(kvs, ","))
		} else {
			if len(resp.KVs) == 0 {
				return "nil"
			} else {
				return describeValueOrHash(resp.KVs[0].Value)
			}
		}
	case Put:
		return fmt.Sprintf("ok")
	case Delete:
		return fmt.Sprintf("deleted: %d", resp.Deleted)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", req.Type)
	}
}

func describeValueOrHash(value ValueOrHash) string {
	if value.Hash != 0 {
		return fmt.Sprintf("hash: %d", value.Hash)
	}
	if value.Value == "" {
		return "nil"
	}
	return fmt.Sprintf("%q", value.Value)
}

func step(states PossibleStates, request EtcdRequest, response EtcdResponse) (bool, PossibleStates) {
	if len(states) == 0 {
		// states were not initialized
		if response.Err != nil || response.ResultUnknown || response.Revision == 0 {
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
		KeyValues: map[string]ValueRevision{},
		KeyLeases: map[string]int64{},
		Leases:    map[int64]EtcdLease{},
	}
	switch request.Type {
	case Txn:
		if response.Txn.TxnResult {
			return state
		}
		if len(request.Txn.Ops) != len(response.Txn.OpsResult) {
			panic(fmt.Sprintf("Incorrect request %s, response %+v", describeEtcdRequest(request), describeEtcdResponse(request, response)))
		}
		for i, op := range request.Txn.Ops {
			opResp := response.Txn.OpsResult[i]
			switch op.Type {
			case Range:
				for _, kv := range opResp.KVs {
					state.KeyValues[kv.Key] = ValueRevision{
						Value:       kv.Value,
						ModRevision: kv.ModRevision,
					}
				}
			case Put:
				state.KeyValues[op.Key] = ValueRevision{
					Value:       op.Value,
					ModRevision: response.Revision,
				}
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
	case Defragment:
	default:
		panic(fmt.Sprintf("Unknown request type: %v", request.Type))
	}
	return state
}

// applyFailedRequest handles a failed requests, one that it's not known if it was persisted or not.
func applyFailedRequest(states PossibleStates, request EtcdRequest) PossibleStates {
	for _, s := range states {
		newState, _ := applyRequestToSingleState(s, request)
		if !reflect.DeepEqual(newState, s) {
			states = append(states, newState)
		}
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
	newKVs := map[string]ValueRevision{}
	for k, v := range s.KeyValues {
		newKVs[k] = v
	}
	s.KeyValues = newKVs
	switch request.Type {
	case Txn:
		success := true
		for _, cond := range request.Txn.Conds {
			if val := s.KeyValues[cond.Key]; val.ModRevision != cond.ExpectedRevision {
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
			case Range:
				opResp[i] = EtcdOperationResult{
					KVs: []KeyValue{},
				}
				if op.WithPrefix {
					for k, v := range s.KeyValues {
						if strings.HasPrefix(k, op.Key) {
							opResp[i].KVs = append(opResp[i].KVs, KeyValue{Key: k, ValueRevision: v})
						}
					}
					sort.Slice(opResp[i].KVs, func(j, k int) bool {
						return opResp[i].KVs[j].Key < opResp[i].KVs[k].Key
					})
				} else {
					value, ok := s.KeyValues[op.Key]
					if ok {
						opResp[i].KVs = append(opResp[i].KVs, KeyValue{
							Key:           op.Key,
							ValueRevision: value,
						})
					}
				}
			case Put:
				_, leaseExists := s.Leases[op.LeaseID]
				if op.LeaseID != 0 && !leaseExists {
					break
				}
				s.KeyValues[op.Key] = ValueRevision{
					Value:       op.Value,
					ModRevision: s.Revision + 1,
				}
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
		for key := range s.Leases[request.LeaseRevoke.LeaseID].Keys {
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
	case Defragment:
		return s, defragmentResponse()
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
