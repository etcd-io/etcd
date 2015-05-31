package storage

import (
	"encoding/binary"
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/etcd/storage/backend"
	"github.com/coreos/etcd/storage/storagepb"
)

var (
	batchLimit    = 10000
	batchInterval = 100 * time.Millisecond
	keyBucketName = []byte("key")

	ErrTnxIDMismatch = errors.New("storage: tnx id mismatch")
)

type store struct {
	mu sync.RWMutex

	b       backend.Backend
	kvindex index

	currentRev reversion

	tmu   sync.Mutex // protect the tnxID field
	tnxID int64      // tracks the current tnxID to verify tnx operations
}

func newStore(path string) KV {
	s := &store{
		b:          backend.New(path, batchInterval, batchLimit),
		kvindex:    newTreeIndex(),
		currentRev: reversion{},
	}

	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(keyBucketName)
	tx.Unlock()
	s.b.ForceCommit()

	return s
}

func (s *store) Put(key, value []byte) int64 {
	id := s.TnxBegin()
	s.put(key, value, s.currentRev.main+1)
	s.TnxEnd(id)

	return int64(s.currentRev.main)
}

func (s *store) Range(key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64) {
	id := s.TnxBegin()
	kvs, rev = s.rangeKeys(key, end, limit, rangeRev)
	s.TnxEnd(id)

	return kvs, rev
}

func (s *store) DeleteRange(key, end []byte) (n, rev int64) {
	id := s.TnxBegin()
	n = s.deleteRange(key, end, s.currentRev.main+1)
	s.TnxEnd(id)

	return n, int64(s.currentRev.main)
}

func (s *store) TnxBegin() int64 {
	s.mu.Lock()
	s.currentRev.sub = 0

	s.tmu.Lock()
	defer s.tmu.Unlock()
	s.tnxID = rand.Int63()
	return s.tnxID
}

func (s *store) TnxEnd(tnxID int64) error {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if tnxID != s.tnxID {
		return ErrTnxIDMismatch
	}

	if s.currentRev.sub != 0 {
		s.currentRev.main += 1
	}
	s.currentRev.sub = 0
	s.mu.Unlock()
	return nil
}

func (s *store) TnxRange(tnxID int64, key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if tnxID != s.tnxID {
		return nil, 0, ErrTnxIDMismatch
	}
	kvs, rev = s.rangeKeys(key, end, limit, rangeRev)
	return kvs, rev, nil
}

func (s *store) TnxPut(tnxID int64, key, value []byte) (rev int64, err error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if tnxID != s.tnxID {
		return 0, ErrTnxIDMismatch
	}

	s.put(key, value, s.currentRev.main+1)
	return int64(s.currentRev.main + 1), nil
}

func (s *store) TnxDeleteRange(tnxID int64, key, end []byte) (n, rev int64, err error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if tnxID != s.tnxID {
		return 0, 0, ErrTnxIDMismatch
	}

	n = s.deleteRange(key, end, s.currentRev.main+1)
	if n != 0 || s.currentRev.sub != 0 {
		rev = int64(s.currentRev.main + 1)
	}
	return n, rev, nil
}

// range is a keyword in Go, add Keys suffix.
func (s *store) rangeKeys(key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64) {
	if rangeRev <= 0 {
		rev = int64(s.currentRev.main)
		if s.currentRev.sub > 0 {
			rev += 1
		}
	} else {
		rev = rangeRev
	}

	_, revs := s.kvindex.Range(key, end, int64(rev))
	if len(revs) == 0 {
		return nil, rev
	}
	if limit > 0 && len(revs) > int(limit) {
		revs = revs[:limit]
	}

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	for _, rev := range revs {
		revbytes := make([]byte, 8+1+8)
		revToBytes(rev.main, rev.sub, revbytes)

		vs := tx.UnsafeRange(keyBucketName, revbytes, nil, 0)
		if len(vs) != 1 {
			log.Fatalf("storage: range cannot find rev (%d,%d)", rev.main, rev.sub)
		}

		e := &storagepb.Event{}
		if err := e.Unmarshal(vs[0]); err != nil {
			log.Fatalf("storage: cannot unmarshal event: %v", err)
		}
		if e.Type == storagepb.PUT {
			kvs = append(kvs, e.Kv)
		}
	}
	return kvs, rev
}

func (s *store) put(key, value []byte, rev int64) {
	ibytes := make([]byte, 8+1+8)
	revToBytes(rev, s.currentRev.sub, ibytes)

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
	s.kvindex.Put(key, reversion{main: rev, sub: s.currentRev.sub})
	s.currentRev.sub += 1
}

func (s *store) deleteRange(key, end []byte, rev int64) int64 {
	var n int64
	rrev := rev
	if s.currentRev.sub > 0 {
		rrev += 1
	}
	keys, _ := s.kvindex.Range(key, end, rrev)

	if len(keys) == 0 {
		return 0
	}

	for _, key := range keys {
		ok := s.delete(key, rev)
		if ok {
			n++
		}
	}
	return n
}

func (s *store) delete(key []byte, mainrev int64) bool {
	grev := mainrev
	if s.currentRev.sub > 0 {
		grev += 1
	}
	rev, err := s.kvindex.Get(key, grev)
	if err != nil {
		// key not exist
		return false
	}

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	revbytes := make([]byte, 8+1+8)
	revToBytes(rev.main, rev.sub, revbytes)

	vs := tx.UnsafeRange(keyBucketName, revbytes, nil, 0)
	if len(vs) != 1 {
		log.Fatalf("storage: delete cannot find rev (%d,%d)", rev.main, rev.sub)
	}

	e := &storagepb.Event{}
	if err := e.Unmarshal(vs[0]); err != nil {
		log.Fatalf("storage: cannot unmarshal event: %v", err)
	}
	if e.Type == storagepb.DELETE {
		return false
	}

	ibytes := make([]byte, 8+1+8)
	revToBytes(mainrev, s.currentRev.sub, ibytes)

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

	tx.UnsafePut(keyBucketName, ibytes, d)
	err = s.kvindex.Tombstone(key, reversion{main: mainrev, sub: s.currentRev.sub})
	if err != nil {
		log.Fatalf("storage: cannot tombstone an existing key (%s): %v", string(key), err)
	}
	s.currentRev.sub += 1
	return true
}

func revToBytes(main int64, sub int64, bytes []byte) {
	binary.BigEndian.PutUint64(bytes, uint64(main))
	bytes[8] = '_'
	binary.BigEndian.PutUint64(bytes[9:], uint64(sub))
}
