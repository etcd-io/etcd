package storage

import (
	"encoding/binary"
	"time"

	"github.com/coreos/etcd/storage/backend"
)

var (
	batchLimit    = 10000
	batchInterval = 100 * time.Millisecond
	keyBucketName = []byte("key")
)

type store struct {
	b       backend.Backend
	kvindex index

	now uint64 // current index of the store
}

func newStore(path string) *store {
	s := &store{
		b:       backend.New(path, batchInterval, batchLimit),
		kvindex: newTreeIndex(),
		now:     0,
	}

	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(keyBucketName)
	tx.Unlock()
	s.b.ForceCommit()

	return s
}

func (s *store) Put(key, value []byte) {
	now := s.now + 1

	s.kvindex.Put(key, now)
	ibytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ibytes, now)

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	s.now = now
	tx.UnsafePut(keyBucketName, ibytes, value)
}

func (s *store) Get(key []byte) []byte {
	index, err := s.kvindex.Get(key, s.now)
	if err != nil {
		return nil
	}

	ibytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ibytes, index)
	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	vs := tx.UnsafeRange(keyBucketName, ibytes, nil, 0)
	return vs[0]
}
