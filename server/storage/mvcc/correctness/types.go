// Copyright 2026 The etcd Authors
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

package correctness

import (
	"bytes"
	"sort"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

// StorageOpType identifies an MVCC or backend operation recorded for linearizability checking.
type StorageOpType int

const (
	OpPut StorageOpType = iota
	OpGet
	OpRange
	OpDelete
	OpDeleteRange
	OpCompact
	OpTxn
	OpReadTxn
	OpFirstRev
	OpRev
	OpCommit
	OpHash
	OpHashByRev
	OpLeaseRevoke
	OpBackendPut
	OpBackendDelete
	OpBackendRange
	OpBackendForEach
	OpBackendHash
	OpCrossLayerTxn
	OpDefrag
)

// TxnSubOpType identifies a write sub-operation executed inside an MVCC write transaction.
type TxnSubOpType int

const (
	TxnSubOpPut TxnSubOpType = iota
	TxnSubOpDeleteRange
)

// TxnSubOp describes a single sub-operation inside a multi-operation MVCC write transaction.
type TxnSubOp struct {
	Type  TxnSubOpType
	Key   []byte
	End   []byte
	Value []byte
	Lease int64
}

// StorageRequest represents an input operation to the storage linearizability model.
type StorageRequest struct {
	Op             StorageOpType
	Bucket         string
	Key            []byte
	End            []byte
	Value          []byte
	Lease          int64
	Rev            int64
	Limit          int64
	CountOnly      bool
	FastKeysOnly   bool
	WithTotalCount bool
	TxnOps         []TxnSubOp
	MetaKey        []byte
	MetaVal        []byte
}

// StorageResponse represents the observed output of a StorageRequest.
type StorageResponse struct {
	KVs         []*mvccpb.KeyValue
	BackendKeys [][]byte
	BackendVals [][]byte
	Count       int
	Deleted     int64
	Rev         int64
	FirstRev    int64
	Hash        uint32
	HashRev     int64
	CompactRev  int64
	Err         error
}

// WatchFilterType identifies an event filter applied to a fuzzed WatchStream watcher.
type WatchFilterType int

const (
	WatchFilterNone WatchFilterType = iota
	WatchFilterNoPut
	WatchFilterNoDelete
)

// AllowsEvent reports whether f retains an event of type t.
func (f WatchFilterType) AllowsEvent(t mvccpb.Event_EventType) bool {
	switch f {
	case WatchFilterNoPut:
		return t != mvccpb.Event_PUT
	case WatchFilterNoDelete:
		return t != mvccpb.Event_DELETE
	default:
		return true
	}
}

// FilterFuncs returns the mvcc.FilterFunc slice corresponding to f.
func (f WatchFilterType) FilterFuncs() []mvcc.FilterFunc {
	switch f {
	case WatchFilterNoPut:
		return []mvcc.FilterFunc{func(e *mvccpb.Event) bool { return e.Type == mvccpb.Event_PUT }}
	case WatchFilterNoDelete:
		return []mvcc.FilterFunc{func(e *mvccpb.Event) bool { return e.Type == mvccpb.Event_DELETE }}
	default:
		return nil
	}
}

// RecordedWatchStream captures the configuration and observed WatchResponse frames of a watcher.
type RecordedWatchStream struct {
	WatchID      mvcc.WatchID
	Key          []byte
	End          []byte
	StartRev     int64
	Filter       WatchFilterType
	RequireExact bool
	Responses    []mvcc.WatchResponse
}

func matchKeyRange(k string, key, end []byte) bool {
	if end == nil {
		return k == string(key)
	}
	if len(end) == 0 {
		return k >= string(key)
	}
	return k >= string(key) && k < string(end)
}

func cloneKeyValue(v *mvccpb.KeyValue) *mvccpb.KeyValue {
	return &mvccpb.KeyValue{
		Key:            bytes.Clone(v.Key),
		Value:          bytes.Clone(v.Value),
		CreateRevision: v.CreateRevision,
		ModRevision:    v.ModRevision,
		Version:        v.Version,
		Lease:          v.Lease,
	}
}

func sortKeyValues(kvs []*mvccpb.KeyValue) {
	sort.Slice(kvs, func(i, j int) bool {
		return bytes.Compare(kvs[i].Key, kvs[j].Key) < 0
	})
}

type kvPair struct {
	k []byte
	v []byte
}

func sortBackendKVs(keys, vals [][]byte) {
	pairs := make([]kvPair, len(keys))
	for i := range keys {
		pairs[i] = kvPair{k: keys[i], v: vals[i]}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return bytes.Compare(pairs[i].k, pairs[j].k) < 0
	})
	for i := range pairs {
		keys[i] = pairs[i].k
		vals[i] = pairs[i].v
	}
}
