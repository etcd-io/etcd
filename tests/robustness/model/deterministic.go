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
	"html"
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
		return freshEtcdState()
	},
	Step: func(st any, in any, out any) (bool, any) {
		return st.(EtcdState).apply(in.(EtcdRequest), out.(EtcdResponse))
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

type EtcdState struct {
	Revision        int64                    `json:",omitempty"`
	CompactRevision int64                    `json:",omitempty"`
	KeyValues       map[string]ValueRevision `json:",omitempty"`
	KeyLeases       map[string]int64         `json:",omitempty"`
	Leases          map[int64]EtcdLease      `json:",omitempty"`
}

func (s EtcdState) Equal(other EtcdState) bool {
	if s.Revision != other.Revision {
		return false
	}
	if s.CompactRevision != other.CompactRevision {
		return false
	}
	if !reflect.DeepEqual(s.KeyValues, other.KeyValues) {
		return false
	}
	if !reflect.DeepEqual(s.KeyLeases, other.KeyLeases) {
		return false
	}
	return reflect.DeepEqual(s.Leases, other.Leases)
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
	switch request.Type {
	case Range:
		return s.stepRange(request)
	case Txn:
		return s.stepTxn(request)
	case LeaseGrant:
		return s.stepLeaseGrant(request)
	case LeaseRevoke:
		return s.stepLeaseRevoke(request)
	case Defragment:
		return s.stepDefragment()
	case Compact:
		return s.stepCompact(request)
	default:
		panic(fmt.Sprintf("Unknown request type: %v", request.Type))
	}
}

func (s EtcdState) stepRange(request EtcdRequest) (EtcdState, MaybeEtcdResponse) {
	if request.Range.Revision == 0 || request.Range.Revision == s.Revision {
		resp := s.getRange(request.Range.RangeOptions)
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

func (s EtcdState) stepTxn(request EtcdRequest) (EtcdState, MaybeEtcdResponse) {
	// TODO: Avoid copying when TXN only has read operations
	newState := s.DeepCopy()
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
}

func (s EtcdState) stepLeaseGrant(request EtcdRequest) (EtcdState, MaybeEtcdResponse) {
	newState := s.DeepCopy()
	// Empty LeaseID means the request failed and client didn't get response. Ignore it as client cannot use lease without knowing its id.
	if request.LeaseGrant.LeaseID == 0 {
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Revision: newState.Revision, LeaseGrant: &LeaseGrantReponse{}}}
	}
	lease := EtcdLease{
		LeaseID: request.LeaseGrant.LeaseID,
		Keys:    map[string]struct{}{},
	}
	newState.Leases[request.LeaseGrant.LeaseID] = lease
	return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Revision: newState.Revision, LeaseGrant: &LeaseGrantReponse{}}}
}

func (s EtcdState) stepLeaseRevoke(request EtcdRequest) (EtcdState, MaybeEtcdResponse) {
	newState := s.DeepCopy()
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
}

func (s EtcdState) stepDefragment() (EtcdState, MaybeEtcdResponse) {
	return s, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Defragment: &DefragmentResponse{}, Revision: RevisionForNonLinearizableResponse}}
}

func (s EtcdState) stepCompact(request EtcdRequest) (EtcdState, MaybeEtcdResponse) {
	newState := s.DeepCopy()
	if request.Compact.Revision <= newState.CompactRevision {
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{ClientError: mvcc.ErrCompacted.Error()}}
	}
	if request.Compact.Revision > newState.Revision {
		return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{ClientError: mvcc.ErrFutureRev.Error()}}
	}
	newState.CompactRevision = request.Compact.Revision
	return newState, MaybeEtcdResponse{EtcdResponse: EtcdResponse{Compact: &CompactResponse{}, Revision: RevisionForNonLinearizableResponse}}
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
