package storage

import (
	"bytes"
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

type store struct {
	// read operation MUST hold read lock
	// write opeartion MUST hold write lock
	// tnx operation MUST hold write lock
	sync.RWMutex

	b       backend.Backend
	kvindex index

	currentIndex uint64
}

func newStore(path string) *store {
	s := &store{
		b:            backend.New(path, batchInterval, batchLimit),
		kvindex:      newTreeIndex(),
		currentIndex: 0,
	}

	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(keyBucketName)
	tx.Unlock()
	s.b.ForceCommit()

	return s
}

func (s *store) Put(key, value []byte) int64 {
	s.Lock()
	defer s.Unlock()

	s.put(key, value, s.currentIndex+1, 0)
	s.currentIndex = s.currentIndex + 1
	return int64(s.currentIndex)
}

func (s *store) Range(key, end []byte, limit, rangeIndex int64) (kvs []storagepb.KeyValue, index int64) {
	s.RLock()
	defer s.RUnlock()

	if rangeIndex <= 0 {
		index = int64(s.currentIndex)
	} else {
		index = rangeIndex
	}

	pairs := s.kvindex.Range(key, end, uint64(index))
	if len(pairs) == 0 {
		return nil, index
	}
	if limit > 0 && len(pairs) > int(limit) {
		pairs = pairs[:limit]
	}

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	for _, pair := range pairs {
		ibytes := make([]byte, 8)
		endbytes := make([]byte, 8)
		binary.BigEndian.PutUint64(ibytes, pair.index)
		binary.BigEndian.PutUint64(endbytes, pair.index+1)

		found := false

		vs := tx.UnsafeRange(keyBucketName, ibytes, endbytes, 0)
		for _, v := range vs {
			var e storagepb.Event
			err := e.Unmarshal(v)
			if err != nil {
				log.Fatalf("storage: range cannot unmarshal event: %v", err)
			}
			if bytes.Equal(e.Kv.Key, pair.key) {
				if e.Type == storagepb.PUT {
					kvs = append(kvs, e.Kv)
				}
				found = true
				break
			}
		}

		if !found {
			log.Fatalf("storage: range cannot find key %s at index %d", string(pair.key), pair.index)
		}
	}
	return kvs, index
}

func (s *store) DeleteRange(key, end []byte) (n, index int64) {
	s.Lock()
	defer s.Unlock()

	index = int64(s.currentIndex) + 1

	pairs := s.kvindex.Range(key, end, s.currentIndex)
	if len(pairs) == 0 {
		return 0, int64(s.currentIndex)
	}

	for i, pair := range pairs {
		ok := s.delete(pair.key, uint64(index), uint32(i))
		if ok {
			n++
		}
	}
	if n != 0 {
		s.currentIndex = s.currentIndex + 1
	}
	return n, int64(s.currentIndex)
}

func (s *store) put(key, value []byte, index uint64, subindex uint32) {
	ibytes := make([]byte, 8+1+4)
	indexToBytes(index, subindex, ibytes)

	event := storagepb.Event{
		Type: storagepb.PUT,
		Kv: storagepb.KeyValue{
			Key:   key,
			Value: value,
		},
	}

	d, err := event.Marshal()
	if err != nil {
		log.Fatalf("storage: cannot marshal event: %v", err)
	}

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafePut(keyBucketName, ibytes, d)
	s.kvindex.Put(key, index)
}

func (s *store) delete(key []byte, index uint64, subindex uint32) bool {
	_, err := s.kvindex.Get(key, index)
	if err != nil {
		// key not exist
		return false
	}

	ibytes := make([]byte, 8+1+4)
	indexToBytes(index, subindex, ibytes)

	event := storagepb.Event{
		Type: storagepb.DELETE,
		Kv: storagepb.KeyValue{
			Key: key,
		},
	}

	d, err := event.Marshal()
	if err != nil {
		log.Fatalf("storage: cannot marshal event: %v", err)
	}

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafePut(keyBucketName, ibytes, d)
	err = s.kvindex.Tombstone(key, index)
	if err != nil {
		log.Fatalf("storage: cannot tombstone an existing key (%s): %v", string(key), err)
	}

	return true
}

func indexToBytes(index uint64, subindex uint32, bytes []byte) {
	binary.BigEndian.PutUint64(bytes, index)
	bytes[8] = '_'
	binary.BigEndian.PutUint32(bytes[9:], subindex)
}
