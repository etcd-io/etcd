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
	"maps"
	"reflect"
	"sort"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/server/v3/storage/mvcc"
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
	Init: func() any {
		data, err := json.Marshal(freshEtcdState())
		if err != nil {
			panic(err)
		}
		return string(data)
	},
	Step: func(st any, in any, out any) (bool, any) {
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
	DescribeOperation: func(in, out any) string {
		return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), MaybeEtcdResponse{EtcdResponse: out.(EtcdResponse)}))
	},
}

type EtcdState struct {
	Revision        int64
	CompactRevision int64
	KeyValues       map[string]ValueRevision
	KeyLeases       map[string]int64
	Leases          map[int64]EtcdLease
}

func (s EtcdState) apply(request EtcdRequest, response EtcdResponse) (bool, EtcdState) {
	newState, modelResponse := s.Step(request)
	return Match(MaybeEtcdResponse{EtcdResponse: response}, modelResponse), newState
}

func (s EtcdState) DeepCopy() EtcdState {
	newState := EtcdState{
		Revision:        s.Revision,
		CompactRevision: s.CompactRevision,
	}

	newState.KeyValues = maps.Clone(s.KeyValues)
	newState.KeyLeases = maps.Clone(s.KeyLeases)

	newLeases := map[int64]EtcdLease{}
	for key, val := range s.Leases {
		newLeases[key] = val.DeepCopy()
	}
	newState.Leases = newLeases
	return newState
}

func freshEtcdState() EtcdState {
	return EtcdState{
		Revision: 1,
		// Start from CompactRevision equal -1 as etcd allows client to compact revision 0 for some reason.
		CompactRevision: -1,
		KeyValues:       map[string]ValueRevision{},
		KeyLeases:       map[string]int64{},
		Leases:          map[int64]EtcdLease{},
	}
}

// Step handles a successful request, returning updated state and response it would generate.
func (s EtcdState) Step(request EtcdRequest) (EtcdState, MaybeEtcdResponse) {
	newState := s.DeepCopy()

	switch request.Type {
	case Range:
		if request.Range.Revision == 0 || request.Range.Revision == newState.Revision {
			resp := newState.getRange(request.Range.RangeOptions)
			return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Range: &resp, Revision: newState.Revision}}
		}
		if request.Range.Revision > newState.Revision {
			return newState, MaybeEtcdResponse{Error: ErrEtcdFutureRev.Error()}
		}
		if request.Range.Revision < newState.CompactRevision {
			return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{ClientError: mvcc.ErrCompacted.Error()}}
		}
		return newState, MaybeEtcdResponse{Persisted: true, PersistedRevision: newState.Revision}
	case Txn:
		failure := false
		for _, cond := range request.Txn.Conditions {
			val := newState.KeyValues[cond.Key]
			if cond.ExpectedVersion > 0 {
				if val.Version != cond.ExpectedVersion {
					failure = true
					break
				}
			} else if val.ModRevision != cond.ExpectedRevision {
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
					RangeResponse: newState.getRange(op.Range),
				}
			case PutOperation:
				_, leaseExists := newState.Leases[op.Put.LeaseID]
				if op.Put.LeaseID != 0 && !leaseExists {
					break
				}
				ver := int64(1)
				if val, exists := newState.KeyValues[op.Put.Key]; exists && val.Version > 0 {
					ver = val.Version + 1
				}
				newState.KeyValues[op.Put.Key] = ValueRevision{
					Value:       op.Put.Value,
					ModRevision: newState.Revision + 1,
					Version:     ver,
				}
				increaseRevision = true
				newState = detachFromOldLease(newState, op.Put.Key)
				if leaseExists {
					newState = attachToNewLease(newState, op.Put.LeaseID, op.Put.Key)
				}
			case DeleteOperation:
				if _, ok := newState.KeyValues[op.Delete.Key]; ok {
					delete(newState.KeyValues, op.Delete.Key)
					increaseRevision = true
					newState = detachFromOldLease(newState, op.Delete.Key)
					opResp[i].Deleted = 1
				}
			default:
				panic("unsupported operation")
			}
		}
		if increaseRevision {
			newState.Revision++
		}
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{Failure: failure, Results: opResp}, Revision: newState.Revision}}
	case LeaseGrant:
		lease := EtcdLease{
			LeaseID: request.LeaseGrant.LeaseID,
			Keys:    map[string]struct{}{},
		}
		newState.Leases[request.LeaseGrant.LeaseID] = lease
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Revision: newState.Revision, LeaseGrant: &LeaseGrantReponse{}}}
	case LeaseRevoke:
		// Delete the keys attached to the lease
		keyDeleted := false
		for key := range newState.Leases[request.LeaseRevoke.LeaseID].Keys {
			// same as delete.
			if _, ok := newState.KeyValues[key]; ok {
				if !keyDeleted {
					keyDeleted = true
				}
				delete(newState.KeyValues, key)
				delete(newState.KeyLeases, key)
			}
		}
		// delete the lease
		delete(newState.Leases, request.LeaseRevoke.LeaseID)
		if keyDeleted {
			newState.Revision++
		}
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Revision: newState.Revision, LeaseRevoke: &LeaseRevokeResponse{}}}
	case Defragment:
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Defragment: &DefragmentResponse{}, Revision: newState.Revision}}
	case Compact:
		if request.Compact.Revision <= newState.CompactRevision {
			return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{ClientError: mvcc.ErrCompacted.Error()}}
		}
		newState.CompactRevision = request.Compact.Revision
		// Set fake revision as compaction returns non-linearizable revision.
		// TODO: Model non-linearizable response revision in model.
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Compact: &CompactResponse{}, Revision: -1}}
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
				count++
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
	if oldLeaseID, ok := s.KeyLeases[key]; ok {
		delete(s.Leases[oldLeaseID].Keys, key)
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
	Compact     RequestType = "compact"
)

type EtcdRequest struct {
	Type        RequestType
	LeaseGrant  *LeaseGrantRequest
	LeaseRevoke *LeaseRevokeRequest
	Range       *RangeRequest
	Txn         *TxnRequest
	Defragment  *DefragmentRequest
	Compact     *CompactRequest
}

func (r *EtcdRequest) IsRead() bool {
	if r.Type == Range {
		return true
	}
	if r.Type != Txn {
		return false
	}
	for _, op := range append(r.Txn.OperationsOnSuccess, r.Txn.OperationsOnFailure...) {
		if op.Type != RangeOperation {
			return false
		}
	}
	return true
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
	ExpectedVersion  int64
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

// MaybeEtcdResponse extends EtcdResponse to include partial information about responses to a request.
// Possible response state information:
// * Normal response. Client observed response. Only EtcdResponse is set.
// * Persisted. Client didn't observe response, but we know it was persisted by etcd. Only Persisted is set
// * Persisted with Revision. Client didn't observe response, but we know that it was persisted, and it's revision. Both Persisted and PersistedRevision is set.
// * Error response. Client observed error, but we don't know if it was persisted. Only Error is set.
type MaybeEtcdResponse struct {
	EtcdResponse
	Persisted         bool
	PersistedRevision int64
	Error             string
}

var ErrEtcdFutureRev = errors.New("future rev")

type EtcdResponse struct {
	Txn         *TxnResponse
	Range       *RangeResponse
	LeaseGrant  *LeaseGrantReponse
	LeaseRevoke *LeaseRevokeResponse
	Defragment  *DefragmentResponse
	Compact     *CompactResponse
	ClientError string
	Revision    int64
}

func Match(r1, r2 MaybeEtcdResponse) bool {
	r1Revision := r1.Revision
	if r1.Persisted {
		r1Revision = r1.PersistedRevision
	}
	r2Revision := r2.Revision
	if r2.Persisted {
		r2Revision = r2.PersistedRevision
	}
	return (r1.Persisted && r1.PersistedRevision == 0) || (r2.Persisted && r2.PersistedRevision == 0) || ((r1.Persisted || r2.Persisted) && (r1.Error != "" || r2.Error != "" || r1Revision == r2Revision)) || reflect.DeepEqual(r1, r2)
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
type (
	LeaseRevokeResponse struct{}
	DefragmentResponse  struct{}
)

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

func (el EtcdLease) DeepCopy() EtcdLease {
	return EtcdLease{
		LeaseID: el.LeaseID,
		Keys:    maps.Clone(el.Keys),
	}
}

type ValueRevision struct {
	Value       ValueOrHash
	ModRevision int64
	Version     int64
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

type CompactResponse struct{}

type CompactRequest struct {
	Revision int64
}
