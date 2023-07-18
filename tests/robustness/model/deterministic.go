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
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"

	"github.com/anishathalye/porcupine"
)

// DeterministicModel assumes a deterministic execution of etcd requests. All
// requests that client called were executed and persisted by etcd. This
// assumption is good for simulating etcd behavior (aka writing a fake), but not
// for validating correctness as requests might be lost or interrupted. It
// requires perfect knowledge of what happened to request which is not possible
// in real systems.
//
// Model can still respond with error or partial response.
//   - Error for etcd known errors, like future revision or compacted revision.
//   - Incomplete response when requests is correct, but model doesn't have all
//     to provide a full response. For example stale reads as model doesn't store
//     whole change history as real etcd does.
var DeterministicModel = porcupine.Model{
	Init: func() interface{} {
		data, err := json.Marshal(freshEtcdState())
		if err != nil {
			panic(err)
		}
		return string(data)
	},
	Step: func(st interface{}, in interface{}, out interface{}) (bool, interface{}) {
		var s EtcdState
		err := json.Unmarshal([]byte(st.(string)), &s)
		if err != nil {
			panic(err)
		}
		ok, s := s.apply(in.(EtcdRequest), out.(EtcdResponse))
		data, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}
		return ok, string(data)
	},
	DescribeOperation: func(in, out interface{}) string {
		return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), MaybeEtcdResponse{EtcdResponse: out.(EtcdResponse)}))
	},
}

type EtcdState struct {
	Revision  int64
	KeyValues map[string]ValueRevision
	KeyLeases map[string]int64
	Leases    map[int64]EtcdLease
}

func (s EtcdState) apply(request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	newState, modelResponse := s.Step(request)
	return Match(MaybeEtcdResponse{EtcdResponse: response}, modelResponse), newState
}

func freshEtcdState() EtcdState {
	return EtcdState{
		Revision:  1,
		KeyValues: map[string]ValueRevision{},
		KeyLeases: map[string]int64{},
		Leases:    map[int64]EtcdLease{},
	}
}

// Step handles a successful request, returning updated state and response it would generate.
func (s EtcdState) Step(request EtcdRequest) (EtcdState, MaybeEtcdResponse) {
	newKVs := map[string]ValueRevision{}
	for k, v := range s.KeyValues {
		newKVs[k] = v
	}
	s.KeyValues = newKVs
	switch request.Type {
	case Range:
		if request.Range.Revision == 0 || request.Range.Revision == s.Revision {
			resp := s.getRange(request.Range.RangeOptions)
			return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Range: &resp, Revision: s.Revision}}
		} else {
			if request.Range.Revision > s.Revision {
				return s, MaybeEtcdResponse{Error: EtcdFutureRevErr.Error()}
			}
			return s, MaybeEtcdResponse{PartialResponse: true, EtcdResponse: EtcdResponse{Revision: s.Revision}}
		}
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
			case RangeOperation:
				opResp[i] = EtcdOperationResult{
					RangeResponse: s.getRange(op.Range),
				}
			case PutOperation:
				_, leaseExists := s.Leases[op.Put.LeaseID]
				if op.Put.LeaseID != 0 && !leaseExists {
					break
				}
				s.KeyValues[op.Put.Key] = ValueRevision{
					Value:       op.Put.Value,
					ModRevision: s.Revision + 1,
				}
				increaseRevision = true
				s = detachFromOldLease(s, op.Put.Key)
				if leaseExists {
					s = attachToNewLease(s, op.Put.LeaseID, op.Put.Key)
				}
			case DeleteOperation:
				if _, ok := s.KeyValues[op.Delete.Key]; ok {
					delete(s.KeyValues, op.Delete.Key)
					increaseRevision = true
					s = detachFromOldLease(s, op.Delete.Key)
					opResp[i].Deleted = 1
				}
			default:
				panic("unsupported operation")
			}
		}
		if increaseRevision {
			s.Revision += 1
		}
		return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{Failure: failure, Results: opResp}, Revision: s.Revision}}
	case LeaseGrant:
		lease := EtcdLease{
			LeaseID: request.LeaseGrant.LeaseID,
			Keys:    map[string]struct{}{},
		}
		s.Leases[request.LeaseGrant.LeaseID] = lease
		return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Revision: s.Revision, LeaseGrant: &LeaseGrantReponse{}}}
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
		return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Revision: s.Revision, LeaseRevoke: &LeaseRevokeResponse{}}}
	case Defragment:
		return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Defragment: &DefragmentResponse{}, Revision: s.Revision}}
	default:
		panic(fmt.Sprintf("Unknown request type: %v", request.Type))
	}
}

func (s EtcdState) getRange(options RangeOptions) RangeResponse {
	response := RangeResponse{
		KVs: []KeyValue{},
	}
	if options.End != "" {
		var count int64
		for k, v := range s.KeyValues {
			if k >= options.Start && k < options.End {
				response.KVs = append(response.KVs, KeyValue{Key: k, ValueRevision: v})
				count += 1
			}
		}
		sort.Slice(response.KVs, func(j, k int) bool {
			return response.KVs[j].Key < response.KVs[k].Key
		})
		if options.Limit != 0 && count > options.Limit {
			response.KVs = response.KVs[:options.Limit]
		}
		response.Count = count
	} else {
		value, ok := s.KeyValues[options.Start]
		if ok {
			response.KVs = append(response.KVs, KeyValue{
				Key:           options.Start,
				ValueRevision: value,
			})
			response.Count = 1
		}
	}
	return response
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

type RequestType string

const (
	Range       RequestType = "range"
	Txn         RequestType = "txn"
	LeaseGrant  RequestType = "leaseGrant"
	LeaseRevoke RequestType = "leaseRevoke"
	Defragment  RequestType = "defragment"
)

type EtcdRequest struct {
	Type        RequestType
	LeaseGrant  *LeaseGrantRequest
	LeaseRevoke *LeaseRevokeRequest
	Range       *RangeRequest
	Txn         *TxnRequest
	Defragment  *DefragmentRequest
}

type RangeRequest struct {
	RangeOptions
	Revision int64
}

type RangeOptions struct {
	Start string
	End   string
	Limit int64
}

type PutOptions struct {
	Key     string
	Value   ValueOrHash
	LeaseID int64
}

type DeleteOptions struct {
	Key string
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
	Type   OperationType
	Range  RangeOptions
	Put    PutOptions
	Delete DeleteOptions
}

type OperationType string

const (
	RangeOperation  OperationType = "range-operation"
	PutOperation    OperationType = "put-operation"
	DeleteOperation OperationType = "delete-operation"
)

type LeaseGrantRequest struct {
	LeaseID int64
}
type LeaseRevokeRequest struct {
	LeaseID int64
}
type DefragmentRequest struct{}

// MaybeEtcdResponse extends EtcdResponse to represent partial or failed responses.
// Possible states:
// * Normal response. Only EtcdResponse is set.
// * Partial response. The EtcdResponse.Revision and PartialResponse are set.
// * Failed response. Only Err is set.
type MaybeEtcdResponse struct {
	EtcdResponse
	PartialResponse bool
	Error           string
}

var EtcdFutureRevErr = errors.New("future rev")

type EtcdResponse struct {
	Txn         *TxnResponse
	Range       *RangeResponse
	LeaseGrant  *LeaseGrantReponse
	LeaseRevoke *LeaseRevokeResponse
	Defragment  *DefragmentResponse
	Revision    int64
}

func Match(r1, r2 MaybeEtcdResponse) bool {
	return ((r1.PartialResponse || r2.PartialResponse) && (r1.Revision == r2.Revision)) || reflect.DeepEqual(r1, r2)
}

type TxnResponse struct {
	Failure bool
	Results []EtcdOperationResult
}

type RangeResponse struct {
	KVs   []KeyValue
	Count int64
}

type LeaseGrantReponse struct {
	LeaseID int64
}
type LeaseRevokeResponse struct{}
type DefragmentResponse struct{}

type EtcdOperationResult struct {
	RangeResponse
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
