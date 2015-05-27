package storage

import "github.com/coreos/etcd/storage/storagepb"

type KV interface {
	// Range gets the keys in the range at rangeIndex.
	// If rangeIndex <=0, range gets the keys at currentIndex.
	// If `end` is nil, the request returns the key.
	// If `end` is not nil, it gets the keys in range [key, range_end).
	// Limit limits the number of keys returned.
	Range(key, end []byte, limit, rangeIndex int64) (kvs []storagepb.KeyValue, index int64)

	// Put puts the given key,value into the store.
	// A put also increases the index of the store, and generates one event in the event history.
	Put(key, value []byte) (index int64)

	// DeleteRange deletes the given range from the store.
	// A deleteRange increases the index of the store if any key in the range exists.
	// The number of key deleted will be returned.
	// It also generates one event for each key delete in the event history.
	// if the `end` is nil, deleteRange deletes the key.
	// if the `end` is not nil, deleteRange deletes the keys in range [key, range_end).
	DeleteRange(key, end []byte) (n, index int64)

	// TnxBegin begins a tnx. Only Tnx prefixed operation can be executed, others will be blocked
	// until tnx ends. Only one on-going tnx is allowed.
	TnxBegin() int64
	// TnxEnd ends the on-going tnx.
	// TODO: generate and verify tnx id for safty.
	TnxEnd(tnxID int64) error
	TnxRange(tnxID int64, key, end []byte, limit, rangeIndex int64) (kvs []storagepb.KeyValue, index int64, err error)
	TnxPut(tnxID int64, key, value []byte) (index int64, err error)
	TnxDeleteRange(tnxID int64, key, end []byte) (n, index int64, err error)
}
