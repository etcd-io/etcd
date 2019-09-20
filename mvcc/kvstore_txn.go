// Copyright 2017 The etcd Authors
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

package mvcc

import (
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/mvcc/backend"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type storeTxnRead struct {
	s  *store
	tx backend.ReadTx

	firstRev int64
	rev      int64
}

func (s *store) Read() TxnRead {
	s.mu.RLock()
	tx := s.b.ReadTx()
	s.revMu.RLock()
	tx.Lock()
	firstRev, rev := s.compactMainRev, s.currentRev
	s.revMu.RUnlock()
	return newMetricsTxnRead(&storeTxnRead{s, tx, firstRev, rev})
}

func (tr *storeTxnRead) FirstRev() int64 { return tr.firstRev }
func (tr *storeTxnRead) Rev() int64      { return tr.rev }

func (tr *storeTxnRead) Range(key, end []byte, ro RangeOptions) (r *RangeResult, err error) {
	return tr.rangeKeys(key, end, tr.Rev(), ro)
}

func (tr *storeTxnRead) GetPrototypeInfo(key []byte, atRev int64) PrototypeInfo {
	_, _, _, pi, _ := tr.s.kvindex.Get(key, atRev)
	return pi
}

func (tr *storeTxnRead) RangeEx(key, end []byte, ro RangeOptions) (*RangeExResult, error) {
	return tr.rangeKeysEx(key, end, tr.Rev(), ro)
}

func (tr *storeTxnRead) RangeExReadKV(r []byte, kv *mvccpb.KeyValue) {
	_, vs := tr.tx.UnsafeRange(keyBucketName, r, nil, 0)
	if len(vs) != 1 {
		plog.Fatalf("range cannot find rev (%v)", r)
	}
	if err := kv.Unmarshal(vs[0]); err != nil {
		plog.Fatalf("cannot unmarshal event: %v", err)
	}
}

func (tr *storeTxnRead) End() {
	tr.tx.Unlock()
	tr.s.mu.RUnlock()
}

type storeTxnWrite struct {
	storeTxnRead
	tx backend.BatchTx
	// beginRev is the revision where the txn begins; it will write to the next revision.
	beginRev int64
	changes  []mvccpb.KeyValue
}

func (s *store) Write() TxnWrite {
	s.mu.RLock()
	tx := s.b.BatchTx()
	tx.Lock()
	tw := &storeTxnWrite{
		storeTxnRead: storeTxnRead{s, tx, 0, 0},
		tx:           tx,
		beginRev:     s.currentRev,
		changes:      make([]mvccpb.KeyValue, 0, 4),
	}
	return newMetricsTxnWrite(tw)
}

func (tw *storeTxnWrite) Rev() int64 { return tw.beginRev }

func (tw *storeTxnWrite) Range(key, end []byte, ro RangeOptions) (r *RangeResult, err error) {
	rev := tw.beginRev
	if len(tw.changes) > 0 {
		rev++
	}
	return tw.rangeKeys(key, end, rev, ro)
}

func (tw *storeTxnWrite) RangeEx(key, end []byte, ro RangeOptions) (*RangeExResult, error) {
	rev := tw.beginRev
	if len(tw.changes) > 0 {
		rev++
	}
	return tw.rangeKeysEx(key, end, rev, ro)
}

func (tw *storeTxnWrite) DeleteRangeExPrepare(key, end []byte) ([][]byte, []revision, []PrototypeInfo) {
	rev := tw.beginRev
	if len(tw.changes) > 0 {
		rev++
	}
	return tw.s.kvindex.Range(key, end, rev)
}

func (tw *storeTxnWrite) DeleteRangeExPrevKV(keys [][]byte, revs []revision, canRead []bool) []*mvccpb.KeyValue {
	kvs := make([]*mvccpb.KeyValue, len(revs))
	revBytes := newRevBytes()
	for i, revpair := range revs {
		kvs[i] = &mvccpb.KeyValue{}
		if !canRead[i] {
			// We can't read the key, don't return anything even the key name,
			// that's in order to be consistent with Put's behavior.
			continue
		}
		revToBytes(revpair, revBytes)
		_, vs := tw.tx.UnsafeRange(keyBucketName, revBytes, nil, 0)
		if len(vs) != 1 {
			plog.Fatalf("range cannot find rev (%d,%d)", revpair.main, revpair.sub)
		}
		if err := kvs[i].Unmarshal(vs[0]); err != nil {
			plog.Fatalf("cannot unmarshal event: %v", err)
		}
	}
	return kvs
}

func (tw *storeTxnWrite) DeleteRangeEx(keys [][]byte, revs []revision, pi []PrototypeInfo) int64 {
	for i, key := range keys {
		tw.delete(key, revs[i], pi[i])
	}
	if len(keys) != 0 || len(tw.changes) > 0 {
		return int64(tw.beginRev + 1)
	}
	return int64(tw.beginRev)
}

func (tw *storeTxnWrite) DeleteRange(key, end []byte) (n, rev int64) {
	keys, revs, pi := tw.DeleteRangeExPrepare(key, end)
	return int64(len(keys)), tw.DeleteRangeEx(keys, revs, pi)
}

func (tw *storeTxnWrite) Put(key, value []byte, lease lease.LeaseID, pi PrototypeInfo) int64 {
	tw.put(key, value, lease, pi)
	return int64(tw.beginRev + 1)
}

func (tw *storeTxnWrite) End() {
	// only update index if the txn modifies the mvcc state.
	if len(tw.changes) != 0 {
		tw.s.saveIndex(tw.tx)
		// hold revMu lock to prevent new read txns from opening until writeback.
		tw.s.revMu.Lock()
		tw.s.currentRev++
	}
	tw.tx.Unlock()
	if len(tw.changes) != 0 {
		tw.s.revMu.Unlock()
	}
	tw.s.mu.RUnlock()
}

func (tr *storeTxnRead) rangeKeysEx(key, end []byte, curRev int64, ro RangeOptions) (*RangeExResult, error) {
	rev := ro.Rev
	if rev > curRev {
		return &RangeExResult{Revs: nil, Limit: 0, Count: -1, Rev: curRev}, ErrFutureRev
	}
	if rev <= 0 {
		rev = curRev
	}
	if rev < tr.s.compactMainRev {
		return &RangeExResult{Revs: nil, Limit: 0, Count: -1, Rev: 0}, ErrCompacted
	}

	revpairs, _ := tr.s.kvindex.Revisions(key, end, int64(rev))
	if len(revpairs) == 0 {
		return &RangeExResult{Revs: nil, Limit: 0, Count: 0, Rev: curRev}, nil
	}
	if ro.Count {
		return &RangeExResult{Revs: nil, Limit: 0, Count: len(revpairs), Rev: curRev}, nil
	}

	limit := int(ro.Limit)
	if limit <= 0 || limit > len(revpairs) {
		limit = len(revpairs)
	}

	return &RangeExResult{Revs: revpairs, Limit: limit, Count: len(revpairs), Rev: curRev}, nil
}

func (tr *storeTxnRead) rangeKeys(key, end []byte, curRev int64, ro RangeOptions) (*RangeResult, error) {
	rer, err := tr.rangeKeysEx(key, end, curRev, ro)
	rr := &RangeResult{KVs: nil, Count: rer.Count, Rev: rer.Rev}
	if rer.Limit > 0 {
		rr.KVs = make([]mvccpb.KeyValue, rer.Limit)
		revBytes := newRevBytes()
		for i, rev := range rer.Revs[:len(rr.KVs)] {
			revToBytes(rev, revBytes)
			tr.RangeExReadKV(revBytes, &rr.KVs[i])
		}
	}
	return rr, err
}

func (tw *storeTxnWrite) put(key, value []byte, leaseID lease.LeaseID, pi PrototypeInfo) {
	rev := tw.beginRev + 1
	c := rev
	oldLease := lease.NoLease

	// if the key exists before, use its previous created and
	// get its previous leaseID
	_, created, ver, _, err := tw.s.kvindex.Get(key, rev)
	if err == nil {
		c = created.main
		oldLease = tw.s.le.GetLease(lease.LeaseItem{Key: string(key)})
	}

	ibytes := newRevBytes()
	idxRev := revision{main: rev, sub: int64(len(tw.changes))}
	revToBytes(idxRev, ibytes)

	ver = ver + 1
	kv := mvccpb.KeyValue{
		Key:            key,
		Value:          value,
		CreateRevision: c,
		ModRevision:    rev,
		Version:        ver,
		Lease:          int64(leaseID),
		PrototypeIdx:   pi.PrototypeIdx,
		ForceFindDepth: pi.ForceFindDepth,
	}

	d, err := kv.Marshal()
	if err != nil {
		plog.Fatalf("cannot marshal event: %v", err)
	}

	tw.tx.UnsafeSeqPut(keyBucketName, ibytes, d)
	tw.s.kvindex.Put(key, idxRev, pi)
	tw.changes = append(tw.changes, kv)

	if oldLease != lease.NoLease {
		if tw.s.le == nil {
			panic("no lessor to detach lease")
		}
		err = tw.s.le.Detach(oldLease, []lease.LeaseItem{{Key: string(key)}})
		if err != nil {
			plog.Errorf("unexpected error from lease detach: %v", err)
		}
	}
	if leaseID != lease.NoLease {
		if tw.s.le == nil {
			panic("no lessor to attach lease")
		}
		err = tw.s.le.Attach(leaseID, []lease.LeaseItem{{Key: string(key),
			PrototypeIdx: pi.PrototypeIdx, ForceFindDepth: pi.ForceFindDepth}})
		if err != nil {
			panic("unexpected error from lease Attach")
		}
	}
}

func (tw *storeTxnWrite) delete(key []byte, rev revision, pi PrototypeInfo) {
	ibytes := newRevBytes()
	idxRev := revision{main: tw.beginRev + 1, sub: int64(len(tw.changes))}
	revToBytes(idxRev, ibytes)
	ibytes = appendMarkTombstone(ibytes)

	kv := mvccpb.KeyValue{Key: key, PrototypeIdx: pi.PrototypeIdx, ForceFindDepth: pi.ForceFindDepth}

	d, err := kv.Marshal()
	if err != nil {
		plog.Fatalf("cannot marshal event: %v", err)
	}

	tw.tx.UnsafeSeqPut(keyBucketName, ibytes, d)
	err = tw.s.kvindex.Tombstone(key, idxRev, pi)
	if err != nil {
		plog.Fatalf("cannot tombstone an existing key (%s): %v", string(key), err)
	}
	tw.changes = append(tw.changes, kv)

	item := lease.LeaseItem{Key: string(key)}
	leaseID := tw.s.le.GetLease(item)

	if leaseID != lease.NoLease {
		err = tw.s.le.Detach(leaseID, []lease.LeaseItem{item})
		if err != nil {
			plog.Errorf("cannot detach %v", err)
		}
	}
}

func (tw *storeTxnWrite) Changes() []mvccpb.KeyValue { return tw.changes }
