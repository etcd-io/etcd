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
	"errors"
	"hash/fnv"
	"maps"
	"reflect"
)

// RevisionForNonLinearizableResponse is a fake revision value used to
// separate responses for requests that are not linearizable,
// thus we need to ignore it in model when checking linearization.
const RevisionForNonLinearizableResponse = -1

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
