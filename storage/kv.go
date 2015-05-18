package storage

import (
	"encoding/binary"
	"log"
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
	b       backend.Backend
	kvindex index

	now        uint64 // current index of the store
	marshalBuf []byte // buffer for marshal protobuf
}

func newStore(path string) *store {
	s := &store{
		b:          backend.New(path, batchInterval, batchLimit),
		kvindex:    newTreeIndex(),
		now:        0,
		marshalBuf: make([]byte, 1024*1024),
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
	// TODO: the value will be an event type.
	// TODO: copy out the bytes, decode it, return the value.
	return vs[0]
}

func (s *store) Delete(key []byte) error {
	now := s.now + 1

	err := s.kvindex.Tombstone(key, now)
	if err != nil {
		return err
	}

	ibytes := make([]byte, 8)
	binary.BigEndian.PutUint64(ibytes, now)
	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	// TODO: the value will be an event type.
	// A tombstone is simple a "Delete" type event.
	tx.UnsafePut(keyBucketName, key, []byte("tombstone"))
	return nil
}
