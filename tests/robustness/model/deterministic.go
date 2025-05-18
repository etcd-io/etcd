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
	"html"
	"maps"
	"reflect"
	"slices"
	"sort"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func DeterministicModelV2(keys []string) porcupine.Model {
	return porcupine.Model{
		Init: func() any {
			return freshEtcdState(len(keys))
		},
		Step: func(st any, in any, out any) (bool, any) {
			return st.(EtcdState).apply(in.(EtcdRequest), keys, out.(EtcdResponse))
		},
		Equal: func(st1, st2 any) bool {
			return st1.(EtcdState).Equal(st2.(EtcdState))
		},
		DescribeOperation: func(in, out any) string {
			return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), MaybeEtcdResponse{EtcdResponse: out.(EtcdResponse)}))
		},
		DescribeState: func(st any) string {
			data, err := json.MarshalIndent(st, "", "  ")
			if err != nil {
				panic(err)
			}
			return "<pre>" + html.EscapeString(string(data)) + "</pre>"
		},
	}
}

type EtcdState struct {
	Revision        int64           `json:",omitempty"`
	CompactRevision int64           `json:",omitempty"`
	Values          []ValueRevision `json:",omitempty"`
}

func (s EtcdState) Equal(other EtcdState) bool {
	if s.Revision != other.Revision {
		return false
	}
	if s.CompactRevision != other.CompactRevision {
		return false
	}
	if !slices.Equal(s.Values, other.Values) {
		return false
	}
	return true
}

func (s EtcdState) apply(request EtcdRequest, keys []string, response EtcdResponse) (bool, EtcdState) {
	newState, modelResponse := s.Step(request, keys)
	return Match(MaybeEtcdResponse{EtcdResponse: response}, modelResponse), newState
}

func (s EtcdState) DeepCopy() EtcdState {
	newState := EtcdState{
		Revision:        s.Revision,
		CompactRevision: s.CompactRevision,
		Values:          slices.Clone(s.Values),
	}
	return newState
}

func freshEtcdState(size int) EtcdState {
	return EtcdState{
		Revision: 1,
		// Start from CompactRevision equal -1 as etcd allows client to compact revision 0 for some reason.
		CompactRevision: -1,
		Values:          make([]ValueRevision, size),
	}
}

// Step handles a successful request, returning updated state and response it would generate.
func (s EtcdState) Step(request EtcdRequest, keys []string) (EtcdState, MaybeEtcdResponse) {
	// TODO: Avoid copying when TXN only has read operations
	kvs := keyValues{
		Keys:   keys,
		Values: s.Values,
	}
	if request.Type == Range {
		if request.Range.Revision == 0 || request.Range.Revision == s.Revision {
			resp := s.getRange(kvs, request.Range.RangeOptions)
			return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Range: &resp, Revision: s.Revision}}
		}
		if request.Range.Revision > s.Revision {
			return s, MaybeEtcdResponse{Error: ErrEtcdFutureRev.Error()}
		}
		if request.Range.Revision < s.CompactRevision {
			return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{ClientError: mvcc.ErrCompacted.Error()}}
		}
		return s, MaybeEtcdResponse{Persisted: true, PersistedRevision: s.Revision}
	}

	newState := s.DeepCopy()
	kvs.Values = newState.Values
	switch request.Type {
	case Txn:
		failure := false
		for _, cond := range request.Txn.Conditions {
			val, _ := kvs.Get(cond.Key)
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
					RangeResponse: newState.getRange(kvs, op.Range),
				}
			case PutOperation:
				ver := int64(1)
				if val, exists := kvs.Get(op.Put.Key); exists && val.Version > 0 {
					ver = val.Version + 1
				}
				kvs.Set(op.Put.Key, ValueRevision{
					Value:       op.Put.Value,
					ModRevision: newState.Revision + 1,
					Version:     ver,
				})
				increaseRevision = true
			case DeleteOperation:
				if _, ok := kvs.Get(op.Delete.Key); ok {
					kvs.Delete(op.Delete.Key)
					increaseRevision = true
					opResp[i].Deleted = 1
				}
			default:
				panic("unsupported operation")
			}
		}
		if increaseRevision {
			newState.Revision++
		}
		newState.Values = kvs.Values
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Txn: &TxnResponse{Failure: failure, Results: opResp}, Revision: newState.Revision}}
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

func (s EtcdState) getRange(kvs keyValues, options RangeOptions) RangeResponse {
	response := RangeResponse{
		KVs: []KeyValue{},
	}
	if options.End != "" {
		response.KVs = kvs.Range(options.Start, options.End)
		count := int64(len(response.KVs))
		sort.Slice(response.KVs, func(j, k int) bool {
			return response.KVs[j].Key < response.KVs[k].Key
		})
		if options.Limit != 0 && count > options.Limit {
			response.KVs = response.KVs[:options.Limit]
		}
		response.Count = count
	} else {
		value, ok := kvs.Get(options.Start)
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
	Value       ValueOrHash `json:",omitempty"`
	ModRevision int64       `json:",omitempty"`
	Version     int64       `json:",omitempty"`
}

type ValueOrHash struct {
	Value string `json:",omitempty"`
	Hash  uint32 `json:",omitempty"`
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
