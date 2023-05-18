// Copyright 2023 The etcd Authors
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

// DeterministicModel assumes that all requests succeed and have a correct response.
var DeterministicModel = porcupine.Model{
	Init: func() interface{} {
		var s etcdState
		data, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}
		return string(data)
	},
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
		var s etcdState
		err := json.Unmarshal([]byte(st.(string)), &s)
		if err != nil {
			panic(err)
		}
		ok, s := s.Step(in.(EtcdRequest), out.(EtcdResponse))
		data, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}
		return ok, string(data)
	},
	DescribeOperation: func(in, out interface{}) string {
		return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), out.(EtcdResponse)))
	},
}

type etcdState struct {
	Revision  int64
	KeyValues map[string]ValueRevision
	KeyLeases map[string]int64
	Leases    map[int64]EtcdLease
}

func (s etcdState) Step(request EtcdRequest, response EtcdResponse) (bool, etcdState) {
	if s.Revision == 0 {
		return true, initState(request, response)
	}
	newState, gotResponse := s.step(request)
	return reflect.DeepEqual(response, gotResponse), newState
}

// initState tries to create etcd state based on the first request.
func initState(request EtcdRequest, response EtcdResponse) etcdState {
	state := etcdState{
		Revision:  response.Revision,
		KeyValues: map[string]ValueRevision{},
		KeyLeases: map[string]int64{},
		Leases:    map[int64]EtcdLease{},
	}
	switch request.Type {
	case Txn:
		if response.Txn.Failure {
			return state
		}
		if len(request.Txn.OperationsOnSuccess) != len(response.Txn.Results) {
			panic(fmt.Sprintf("Incorrect request %s, response %+v", describeEtcdRequest(request), describeEtcdResponse(request, response)))
		}
		for i, op := range request.Txn.OperationsOnSuccess {
			opResp := response.Txn.Results[i]
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

// step handles a successful request, returning updated state and response it would generate.
func (s etcdState) step(request EtcdRequest) (etcdState, EtcdResponse) {
	newKVs := map[string]ValueRevision{}
	for k, v := range s.KeyValues {
		newKVs[k] = v
	}
	s.KeyValues = newKVs
	switch request.Type {
	case Txn:
		failure := false
		for _, cond := range request.Txn.Conditions {
			if val := s.KeyValues[cond.Key]; val.ModRevision != cond.ExpectedRevision {
				failure = true
				break
			}
		}
		operations := request.Txn.OperationsOnSuccess
		if failure {
			operations = request.Txn.OperationsOnFailure
		}
		opResp := make([]EtcdOperationResult, len(operations))
		increaseRevision := false
		for i, op := range operations {
			switch op.Type {
			case Range:
				opResp[i] = EtcdOperationResult{
					KVs: []KeyValue{},
				}
				if op.WithPrefix {
					var count int64
					for k, v := range s.KeyValues {
						if strings.HasPrefix(k, op.Key) {
							opResp[i].KVs = append(opResp[i].KVs, KeyValue{Key: k, ValueRevision: v})
							count += 1
						}
					}
					sort.Slice(opResp[i].KVs, func(j, k int) bool {
						return opResp[i].KVs[j].Key < opResp[i].KVs[k].Key
					})
					if op.Limit != 0 && count > op.Limit {
						opResp[i].KVs = opResp[i].KVs[:op.Limit]
					}
					opResp[i].Count = count
				} else {
					value, ok := s.KeyValues[op.Key]
					if ok {
						opResp[i].KVs = append(opResp[i].KVs, KeyValue{
							Key:           op.Key,
							ValueRevision: value,
						})
						opResp[i].Count = 1
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
		return s, EtcdResponse{Txn: &TxnResponse{Failure: failure, Results: opResp}, Revision: s.Revision}
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
		return s, EtcdResponse{Defragment: &DefragmentResponse{}, Revision: s.Revision}
	default:
		panic(fmt.Sprintf("Unknown request type: %v", request.Type))
	}
}

func detachFromOldLease(s etcdState, key string) etcdState {
	if oldLeaseId, ok := s.KeyLeases[key]; ok {
		delete(s.Leases[oldLeaseId].Keys, key)
		delete(s.KeyLeases, key)
	}
	return s
}

func attachToNewLease(s etcdState, leaseID int64, key string) etcdState {
	s.KeyLeases[key] = leaseID
	s.Leases[leaseID].Keys[key] = leased
	return s
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
	Conditions          []EtcdCondition
	OperationsOnSuccess []EtcdOperation
	OperationsOnFailure []EtcdOperation
}

type EtcdCondition struct {
	Key              string
	ExpectedRevision int64
}

type EtcdOperation struct {
	Type       OperationType
	Key        string
	WithPrefix bool
	Limit      int64
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
	Revision    int64
	Txn         *TxnResponse
	LeaseGrant  *LeaseGrantReponse
	LeaseRevoke *LeaseRevokeResponse
	Defragment  *DefragmentResponse
}

type TxnResponse struct {
	Failure bool
	Results []EtcdOperationResult
}

type LeaseGrantReponse struct {
	LeaseID int64
}
type LeaseRevokeResponse struct{}
type DefragmentResponse struct{}

type EtcdOperationResult struct {
	KVs     []KeyValue
	Count   int64
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
