// Copyright 2015 CoreOS, Inc.
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

package storage

import (
	"github.com/coreos/etcd/storage/backend"
	"github.com/coreos/etcd/storage/storagepb"
)

// CancelFunc tells an operation to abandon its work. A CancelFunc does not
// wait for the work to stop.
type CancelFunc func()

type Snapshot backend.Snapshot

type KV interface {
	// Rev returns the current revision of the KV.
	Rev() int64

	// Range gets the keys in the range at rangeRev.
	// If rangeRev <=0, range gets the keys at currentRev.
	// If `end` is nil, the request returns the key.
	// If `end` is not nil, it gets the keys in range [key, range_end).
	// Limit limits the number of keys returned.
	// If the required rev is compacted, ErrCompacted will be returned.
	Range(key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error)

	// Put puts the given key,value into the store.
	// A put also increases the rev of the store, and generates one event in the event history.
	Put(key, value []byte) (rev int64)

	// DeleteRange deletes the given range from the store.
	// A deleteRange increases the rev of the store if any key in the range exists.
	// The number of key deleted will be returned.
	// It also generates one event for each key delete in the event history.
	// if the `end` is nil, deleteRange deletes the key.
	// if the `end` is not nil, deleteRange deletes the keys in range [key, range_end).
	DeleteRange(key, end []byte) (n, rev int64)

	// TxnBegin begins a txn. Only Txn prefixed operation can be executed, others will be blocked
	// until txn ends. Only one on-going txn is allowed.
	// TxnBegin returns an int64 txn ID.
	// All txn prefixed operations with same txn ID will be done with the same rev.
	TxnBegin() int64
	// TxnEnd ends the on-going txn with txn ID. If the on-going txn ID is not matched, error is returned.
	TxnEnd(txnID int64) error
	TxnRange(txnID int64, key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error)
	TxnPut(txnID int64, key, value []byte) (rev int64, err error)
	TxnDeleteRange(txnID int64, key, end []byte) (n, rev int64, err error)

	Compact(rev int64) error

	// Get the hash of KV state.
	// This method is designed for consistency checking purpose.
	Hash() (uint32, error)

	// Snapshot snapshots the full KV store.
	Snapshot() Snapshot

	// Commit commits txns into the underlying backend.
	Commit()

	Restore() error
	Close() error
}

// WatchableKV is a KV that can be watched.
type WatchableKV interface {
	KV

	// Watcher watches the events happening or happened on the given key
	// or key prefix from the given startRev.
	// The whole event history can be watched unless compacted.
	// If `prefix` is true, watch observes all events whose key prefix could be the given `key`.
	// If `startRev` <=0, watch observes events after currentRev.
	//
	// Canceling the watcher releases resources associated with it, so code
	// should always call cancel as soon as watch is done.
	Watcher(key []byte, prefix bool, startRev int64) (Watcher, CancelFunc)
}

// ConsistentWatchableKV is a WatchableKV that understands the consistency
// algorithm and consistent index.
// If the consistent index of executing entry is not larger than the
// consistent index of ConsistentWatchableKV, all operations in
// this entry are skipped and return empty response.
type ConsistentWatchableKV interface {
	WatchableKV
}
