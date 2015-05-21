package storage

import (
	"encoding/binary"
	"log"
	"sync"
	"time"

	"github.com/coreos/etcd/storage/backend"
	"github.com/coreos/etcd/storage/storagepb"
)

var (
	batchLimit    = 10000
	batchInterval = 100 * time.Millisecond
	keyBucketName = []byte("key")
)

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
	TnxBegin()
	// TnxEnd ends the on-going tnx.
	TnxEnd()
	TnxRange(key, end []byte, limit, rangeIndex int64) (kvs []storagepb.KeyValue, index int64)
	TnxPut(key, value []byte) (index int64)
	TnxDeleteRange(key, end []byte) (n, index int64)
}

type store struct {
	// read operation MUST hold read lock
	// write opeartion MUST hold write lock
	// tnx operation MUST hold write lock
	sync.RWMutex

	b       backend.Backend
	kvindex index

	currentIndex uint64
	marshalBuf   []byte // buffer for marshal protobuf
}

func newStore(path string) *store {
	s := &store{
		b:            backend.New(path, batchInterval, batchLimit),
		kvindex:      newTreeIndex(),
		currentIndex: 0,
		marshalBuf:   make([]byte, 1024*1024),
	}

	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(keyBucketName)
	tx.Unlock()
	s.b.ForceCommit()

	return s
}

func (s *store) Put(key, value []byte) {
	s.Lock()
	defer s.Unlock()

	currentIndex := s.currentIndex + 1

	ibytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ibytes, currentIndex)

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	s.currentIndex = currentIndex

	event := storagepb.Event{
		Type: storagepb.PUT,
		Kv: storagepb.KeyValue{
			Key:   key,
			Value: value,
		},
	}

	var (
		d   []byte
		err error
		n   int
	)

	if event.Size() < len(s.marshalBuf) {
		n, err = event.MarshalTo(s.marshalBuf)
		d = s.marshalBuf[:n]
	} else {
		d, err = event.Marshal()
	}
	if err != nil {
		log.Fatalf("storage: cannot marshal event: %v", err)
	}

	tx.UnsafePut(keyBucketName, ibytes, d)

	s.kvindex.Put(key, currentIndex)
}

func (s *store) Get(key []byte) []byte {
	s.RLock()
	defer s.RUnlock()

	index, err := s.kvindex.Get(key, s.currentIndex)
	if err != nil {
		return nil
	}

	ibytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ibytes, index)
	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	vs := tx.UnsafeRange(keyBucketName, ibytes, nil, 0)
	// TODO: the value will be an event type.
	// TODO: copy out the bytes, decode it, return the value.
	return vs[0]
}

func (s *store) Delete(key []byte) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.kvindex.Get(key, s.currentIndex)
	if err != nil {
		return nil
	}

	currentIndex := s.currentIndex + 1

	ibytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ibytes, currentIndex)
	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	// TODO: the value will be an event type.
	// A tombstone is simple a "Delete" type event.
	tx.UnsafePut(keyBucketName, key, []byte("tombstone"))

	return s.kvindex.Tombstone(key, currentIndex)
}
