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
	"errors"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/etcd/storage/backend"
	"github.com/coreos/etcd/storage/storagepb"
)

var (
	batchLimit     = 10000
	batchInterval  = 100 * time.Millisecond
	keyBucketName  = []byte("key")
	metaBucketName = []byte("meta")

	scheduledCompactKeyName = []byte("scheduledCompactRev")
	finishedCompactKeyName  = []byte("finishedCompactRev")

	ErrTxnIDMismatch = errors.New("storage: txn id mismatch")
	ErrCompacted     = errors.New("storage: required revision has been compacted")
	ErrFutureRev     = errors.New("storage: required revision is a future revision")
	ErrCanceled      = errors.New("storage: watcher is canceled")
)

type store struct {
	mu sync.RWMutex

	b       backend.Backend
	kvindex index

	currentRev revision
	// the main revision of the last compaction
	compactMainRev int64

	tmu   sync.Mutex // protect the txnID field
	txnID int64      // tracks the current txnID to verify txn operations

	wg    sync.WaitGroup
	stopc chan struct{}
}

func New(path string) KV {
	return newStore(path)
}

func newStore(path string) *store {
	s := &store{
		b:              backend.New(path, batchInterval, batchLimit),
		kvindex:        newTreeIndex(),
		currentRev:     revision{},
		compactMainRev: -1,
		stopc:          make(chan struct{}),
	}

	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(keyBucketName)
	tx.UnsafeCreateBucket(metaBucketName)
	tx.Unlock()
	s.b.ForceCommit()

	return s
}

func (s *store) Rev() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.currentRev.main
}

func (s *store) Put(key, value []byte) int64 {
	id := s.TxnBegin()
	s.put(key, value)
	s.txnEnd(id)

	putCounter.Inc()

	return int64(s.currentRev.main)
}

func (s *store) Range(key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error) {
	id := s.TxnBegin()
	kvs, rev, err = s.rangeKeys(key, end, limit, rangeRev)
	s.txnEnd(id)

	rangeCounter.Inc()

	return kvs, rev, err
}

func (s *store) DeleteRange(key, end []byte) (n, rev int64) {
	id := s.TxnBegin()
	n = s.deleteRange(key, end)
	s.txnEnd(id)

	deleteCounter.Inc()

	return n, int64(s.currentRev.main)
}

func (s *store) TxnBegin() int64 {
	s.mu.Lock()
	s.currentRev.sub = 0

	s.tmu.Lock()
	defer s.tmu.Unlock()
	s.txnID = rand.Int63()
	return s.txnID
}

func (s *store) TxnEnd(txnID int64) error {
	err := s.txnEnd(txnID)
	if err != nil {
		return err
	}

	txnCounter.Inc()
	return nil
}

// txnEnd is used for unlocking an internal txn. It does
// not increase the txnCounter.
func (s *store) txnEnd(txnID int64) error {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if txnID != s.txnID {
		return ErrTxnIDMismatch
	}

	if s.currentRev.sub != 0 {
		s.currentRev.main += 1
	}
	s.currentRev.sub = 0
	s.mu.Unlock()
	return nil
}

func (s *store) TxnRange(txnID int64, key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if txnID != s.txnID {
		return nil, 0, ErrTxnIDMismatch
	}
	return s.rangeKeys(key, end, limit, rangeRev)
}

func (s *store) TxnPut(txnID int64, key, value []byte) (rev int64, err error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if txnID != s.txnID {
		return 0, ErrTxnIDMismatch
	}

	s.put(key, value)
	return int64(s.currentRev.main + 1), nil
}

func (s *store) TxnDeleteRange(txnID int64, key, end []byte) (n, rev int64, err error) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if txnID != s.txnID {
		return 0, 0, ErrTxnIDMismatch
	}

	n = s.deleteRange(key, end)
	if n != 0 || s.currentRev.sub != 0 {
		rev = int64(s.currentRev.main + 1)
	} else {
		rev = int64(s.currentRev.main)
	}
	return n, rev, nil
}

// RangeEvents gets the events from key to end in [startRev, endRev).
// If `end` is nil, the request only observes the events on key.
// If `end` is not nil, it observes the events on key range [key, range_end).
// Limit limits the number of events returned.
// If startRev <=0, rangeEvents returns events from the beginning of uncompacted history.
// If endRev <=0, it indicates there is no end revision.
//
// If the required start rev is compacted, ErrCompacted will be returned.
// If the required start rev has not happened, ErrFutureRev will be returned.
//
// RangeEvents returns events that satisfy the requirement (0 <= n <= limit).
// If events in the revision range have not all happened, it returns immeidately
// what is available.
// It also returns nextRev which indicates the start revision used for the following
// RangeEvents call. The nextRev could be smaller than the given endRev if the store
// has not progressed so far or it hits the event limit.
//
// TODO: return byte slices instead of events to avoid meaningless encode and decode.
func (s *store) RangeEvents(key, end []byte, limit, startRev, endRev int64) (evs []storagepb.Event, nextRev int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if startRev > 0 && startRev <= s.compactMainRev {
		return nil, 0, ErrCompacted
	}
	if startRev > s.currentRev.main {
		return nil, 0, ErrFutureRev
	}

	revs := s.kvindex.RangeEvents(key, end, startRev)
	if len(revs) == 0 {
		return nil, s.currentRev.main + 1, nil
	}

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	// fetch events from the backend using revisions
	for _, rev := range revs {
		if endRev > 0 && rev.main >= endRev {
			return evs, rev.main, nil
		}
		revbytes := newRevBytes()
		revToBytes(rev, revbytes)

		_, vs := tx.UnsafeRange(keyBucketName, revbytes, nil, 0)
		if len(vs) != 1 {
			log.Fatalf("storage: range cannot find rev (%d,%d)", rev.main, rev.sub)
		}

		e := storagepb.Event{}
		if err := e.Unmarshal(vs[0]); err != nil {
			log.Fatalf("storage: cannot unmarshal event: %v", err)
		}
		evs = append(evs, e)
		if limit > 0 && len(evs) >= int(limit) {
			return evs, rev.main + 1, nil
		}
	}
	return evs, s.currentRev.main + 1, nil
}

func (s *store) Compact(rev int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rev <= s.compactMainRev {
		return ErrCompacted
	}
	if rev > s.currentRev.main {
		return ErrFutureRev
	}

	start := time.Now()

	s.compactMainRev = rev

	rbytes := newRevBytes()
	revToBytes(revision{main: rev}, rbytes)

	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafePut(metaBucketName, scheduledCompactKeyName, rbytes)
	tx.Unlock()
	// ensure that desired compaction is persisted
	s.b.ForceCommit()

	keep := s.kvindex.Compact(rev)

	s.wg.Add(1)
	go s.scheduleCompaction(rev, keep)

	indexCompactionPauseDurations.Observe(float64(time.Now().Sub(start) / time.Millisecond))
	return nil
}

func (s *store) Hash() (uint32, error) {
	s.b.ForceCommit()
	return s.b.Hash()
}

func (s *store) Snapshot() Snapshot {
	s.b.ForceCommit()
	return s.b.Snapshot()
}

func (s *store) Restore() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	min, max := newRevBytes(), newRevBytes()
	revToBytes(revision{}, min)
	revToBytes(revision{main: math.MaxInt64, sub: math.MaxInt64}, max)

	// restore index
	tx := s.b.BatchTx()
	tx.Lock()
	_, finishedCompactBytes := tx.UnsafeRange(metaBucketName, finishedCompactKeyName, nil, 0)
	if len(finishedCompactBytes) != 0 {
		s.compactMainRev = bytesToRev(finishedCompactBytes[0]).main
		log.Printf("storage: restore compact to %d", s.compactMainRev)
	}

	// TODO: limit N to reduce max memory usage
	keys, vals := tx.UnsafeRange(keyBucketName, min, max, 0)
	for i, key := range keys {
		e := &storagepb.Event{}
		if err := e.Unmarshal(vals[i]); err != nil {
			log.Fatalf("storage: cannot unmarshal event: %v", err)
		}

		rev := bytesToRev(key)

		// restore index
		switch e.Type {
		case storagepb.PUT:
			s.kvindex.Restore(e.Kv.Key, revision{e.Kv.CreateRevision, 0}, rev, e.Kv.Version)
		case storagepb.DELETE:
			s.kvindex.Tombstone(e.Kv.Key, rev)
		default:
			log.Panicf("storage: unexpected event type %s", e.Type)
		}

		// update revision
		s.currentRev = rev
	}

	_, scheduledCompactBytes := tx.UnsafeRange(metaBucketName, scheduledCompactKeyName, nil, 0)
	if len(scheduledCompactBytes) != 0 {
		scheduledCompact := bytesToRev(scheduledCompactBytes[0]).main
		if scheduledCompact > s.compactMainRev {
			log.Printf("storage: resume scheduled compaction at %d", scheduledCompact)
			go s.Compact(scheduledCompact)
		}
	}

	tx.Unlock()

	return nil
}

func (s *store) Close() error {
	close(s.stopc)
	s.wg.Wait()
	return s.b.Close()
}

func (a *store) Equal(b *store) bool {
	if a.currentRev != b.currentRev {
		return false
	}
	if a.compactMainRev != b.compactMainRev {
		return false
	}
	return a.kvindex.Equal(b.kvindex)
}

// range is a keyword in Go, add Keys suffix.
func (s *store) rangeKeys(key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error) {
	curRev := int64(s.currentRev.main)
	if s.currentRev.sub > 0 {
		curRev += 1
	}

	if rangeRev > curRev {
		return nil, s.currentRev.main, ErrFutureRev
	}
	if rangeRev <= 0 {
		rev = curRev
	} else {
		rev = rangeRev
	}
	if rev <= s.compactMainRev {
		return nil, 0, ErrCompacted
	}

	_, revpairs := s.kvindex.Range(key, end, int64(rev))
	if len(revpairs) == 0 {
		return nil, rev, nil
	}

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	for _, revpair := range revpairs {
		revbytes := newRevBytes()
		revToBytes(revpair, revbytes)

		_, vs := tx.UnsafeRange(keyBucketName, revbytes, nil, 0)
		if len(vs) != 1 {
			log.Fatalf("storage: range cannot find rev (%d,%d)", revpair.main, revpair.sub)
		}

		e := &storagepb.Event{}
		if err := e.Unmarshal(vs[0]); err != nil {
			log.Fatalf("storage: cannot unmarshal event: %v", err)
		}
		kvs = append(kvs, *e.Kv)
		if limit > 0 && len(kvs) >= int(limit) {
			break
		}
	}
	return kvs, rev, nil
}

func (s *store) put(key, value []byte) {
	rev := s.currentRev.main + 1
	c := rev

	// if the key exists before, use its previous created
	_, created, ver, err := s.kvindex.Get(key, rev)
	if err == nil {
		c = created.main
	}

	ibytes := newRevBytes()
	revToBytes(revision{main: rev, sub: s.currentRev.sub}, ibytes)

	ver = ver + 1
	event := storagepb.Event{
		Type: storagepb.PUT,
		Kv: &storagepb.KeyValue{
			Key:            key,
			Value:          value,
			CreateRevision: c,
			ModRevision:    rev,
			Version:        ver,
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
	s.kvindex.Put(key, revision{main: rev, sub: s.currentRev.sub})
	s.currentRev.sub += 1
}

func (s *store) deleteRange(key, end []byte) int64 {
	rrev := s.currentRev.main
	if s.currentRev.sub > 0 {
		rrev += 1
	}
	keys, _ := s.kvindex.Range(key, end, rrev)

	if len(keys) == 0 {
		return 0
	}

	for _, key := range keys {
		s.delete(key)
	}
	return int64(len(keys))
}

func (s *store) delete(key []byte) {
	mainrev := s.currentRev.main + 1

	tx := s.b.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	ibytes := newRevBytes()
	revToBytes(revision{main: mainrev, sub: s.currentRev.sub}, ibytes)

	event := storagepb.Event{
		Type: storagepb.DELETE,
		Kv: &storagepb.KeyValue{
			Key: key,
		},
	}

	d, err := event.Marshal()
	if err != nil {
		log.Fatalf("storage: cannot marshal event: %v", err)
	}

	tx.UnsafePut(keyBucketName, ibytes, d)
	err = s.kvindex.Tombstone(key, revision{main: mainrev, sub: s.currentRev.sub})
	if err != nil {
		log.Fatalf("storage: cannot tombstone an existing key (%s): %v", string(key), err)
	}
	s.currentRev.sub += 1
}
