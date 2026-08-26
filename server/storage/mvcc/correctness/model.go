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
	"errors"
	"sort"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// StoragePorcupineModel defines the linearizable state machine specification for MVCC and backend operations.
var StoragePorcupineModel = porcupine.Model{
	Init: func() any {
		return newStorageState()
	},
	Step: func(state, input, output any) (bool, any) {
		return stepState(state.(*StorageState), input.(StorageRequest), output.(StorageResponse))
	},
	Equal: func(s1, s2 any) bool {
		return s1.(*StorageState).Equal(s2.(*StorageState))
	},
	DescribeOperation: func(input, output any) string {
		return describeOperation(input.(StorageRequest), output.(StorageResponse))
	},
	DescribeState: func(st any) string {
		return describeState(st.(*StorageState))
	},
}

// StorageState represents a point-in-time snapshot of both MVCC keys and backend buckets.
type StorageState struct {
	items          map[string]*mvccpb.KeyValue
	backendBuckets map[string]map[string][]byte
	compactRev     int64
	currentRev     int64
}

func newStorageState() *StorageState {
	return &StorageState{
		items:          make(map[string]*mvccpb.KeyValue),
		backendBuckets: make(map[string]map[string][]byte),
		compactRev:     -1,
		currentRev:     1,
	}
}

// Equal reports whether s and other represent identical MVCC and backend state.
func (s *StorageState) Equal(other *StorageState) bool {
	if s.compactRev != other.compactRev || s.currentRev != other.currentRev {
		return false
	}
	return equalMVCCItems(s.items, other.items) && equalBackendBuckets(s.backendBuckets, other.backendBuckets)
}

func equalMVCCItems(a, b map[string]*mvccpb.KeyValue) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v1 := range a {
		v2, ok := b[k]
		if !ok || !equalKeyValue(v1, v2) {
			return false
		}
	}
	return true
}

func equalKeyValue(a, b *mvccpb.KeyValue) bool {
	return bytes.Equal(a.Key, b.Key) &&
		bytes.Equal(a.Value, b.Value) &&
		a.CreateRevision == b.CreateRevision &&
		a.ModRevision == b.ModRevision &&
		a.Version == b.Version &&
		a.Lease == b.Lease
}

func equalBackendBuckets(a, b map[string]map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for bk, bmap1 := range a {
		bmap2, ok := b[bk]
		if !ok || len(bmap1) != len(bmap2) {
			return false
		}
		for k, v1 := range bmap1 {
			v2, exists := bmap2[k]
			if !exists || !bytes.Equal(v1, v2) {
				return false
			}
		}
	}
	return true
}

func (s *StorageState) clone() *StorageState {
	n := &StorageState{
		items:          make(map[string]*mvccpb.KeyValue, len(s.items)),
		backendBuckets: make(map[string]map[string][]byte, len(s.backendBuckets)),
		compactRev:     s.compactRev,
		currentRev:     s.currentRev,
	}
	for k, v := range s.items {
		n.items[k] = cloneKeyValue(v)
	}
	for b, kvs := range s.backendBuckets {
		n.backendBuckets[b] = make(map[string][]byte, len(kvs))
		for k, v := range kvs {
			n.backendBuckets[b][k] = bytes.Clone(v)
		}
	}
	return n
}

// StepDeterministic is the single pure state-transition kernel for MVCC and backend operations.
// It returns the resulting state, the exact expected response when executed at st, and any
// watch events emitted by MVCC mutations.
func StepDeterministic(st *StorageState, req StorageRequest) (next *StorageState, expected StorageResponse, events []*mvccpb.Event) {
	switch req.Op {
	case OpPut:
		next = st.clone()
		next.currentRev++
		kv := putMVCCItem(next.items, req.Key, req.Value, req.Lease, next.currentRev)
		return next, StorageResponse{Rev: next.currentRev}, []*mvccpb.Event{{Type: mvccpb.Event_PUT, Kv: cloneKeyValue(kv)}}

	case OpDelete:
		return stepDeterministicDeleteRange(st, req.Key, nil)

	case OpDeleteRange:
		return stepDeterministicDeleteRange(st, req.Key, req.End)

	case OpTxn:
		return stepDeterministicTxn(st, req.TxnOps)

	case OpLeaseRevoke:
		return stepDeterministicLeaseRevoke(st, req.Lease)

	case OpCompact:
		if req.Rev > st.currentRev {
			return st, StorageResponse{Err: mvcc.ErrFutureRev}, nil
		}
		if req.Rev <= st.compactRev {
			return st, StorageResponse{Err: mvcc.ErrCompacted}, nil
		}
		next = st.clone()
		next.compactRev = req.Rev
		return next, StorageResponse{}, nil

	case OpFirstRev:
		return st, StorageResponse{FirstRev: st.compactRev}, nil

	case OpRev:
		return st, StorageResponse{Rev: st.currentRev}, nil

	case OpGet:
		return st, evalMVCCGet(st, req), nil

	case OpRange, OpReadTxn:
		return st, evalMVCCRange(st, req), nil

	case OpCommit, OpDefrag, OpBackendHash:
		return st, StorageResponse{}, nil

	case OpHash:
		return st, StorageResponse{Rev: st.currentRev}, nil

	case OpHashByRev:
		if req.Rev > st.currentRev {
			return st, StorageResponse{Rev: st.currentRev, Err: mvcc.ErrFutureRev}, nil
		}
		if req.Rev > 0 && req.Rev < st.compactRev {
			return st, StorageResponse{Err: mvcc.ErrCompacted}, nil
		}
		effRev := req.Rev
		if effRev == 0 {
			effRev = st.currentRev
		}
		return st, StorageResponse{Rev: st.currentRev, HashRev: effRev, CompactRev: st.compactRev}, nil

	case OpBackendPut:
		next = st.clone()
		putBackendItem(next.backendBuckets, req.Bucket, req.Key, req.Value)
		return next, StorageResponse{}, nil

	case OpBackendDelete:
		next = st.clone()
		if next.backendBuckets[req.Bucket] != nil {
			delete(next.backendBuckets[req.Bucket], string(req.Key))
		}
		return next, StorageResponse{}, nil

	case OpBackendRange:
		return st, evalBackendRange(st, req), nil

	case OpBackendForEach:
		return st, evalBackendForEach(st, req), nil

	case OpCrossLayerTxn:
		next = st.clone()
		next.currentRev++
		kv := putMVCCItem(next.items, req.Key, req.Value, req.Lease, next.currentRev)
		putBackendItem(next.backendBuckets, req.Bucket, req.MetaKey, req.MetaVal)
		return next, StorageResponse{Rev: next.currentRev}, []*mvccpb.Event{{Type: mvccpb.Event_PUT, Kv: cloneKeyValue(kv)}}

	default:
		panic("unknown storage operation")
	}
}

func stepState(st *StorageState, req StorageRequest, res StorageResponse) (bool, *StorageState) {
	switch req.Op {
	case OpGet, OpRange:
		if req.Rev > 0 {
			return checkHistoricalReadStep(st, req.Rev, res), st
		}
	case OpHash:
		return res.Err == nil && res.Rev == st.currentRev, st
	case OpHashByRev:
		return checkHashByRevStep(st, req.Rev, res), st
	case OpBackendHash, OpDefrag, OpCommit:
		return res.Err == nil, st
	case OpBackendRange:
		if req.Bucket != string(schema.Key.Name()) && len(req.End) > 0 && req.Limit != 1 {
			return false, st
		}
	}

	next, expected, _ := StepDeterministic(st, req)
	if !equalStorageResponse(req.Op, expected, res) {
		return false, st
	}
	return true, next
}

// checkHistoricalReadStep validates revision bounds, returned currentRev, and errors for Get/Range
// when reqRev > 0. Historical payloads for [compactRev, currentRev] are verified against StorageReplay in phase 2.
func checkHistoricalReadStep(st *StorageState, reqRev int64, res StorageResponse) bool {
	if reqRev > st.currentRev {
		return errors.Is(res.Err, mvcc.ErrFutureRev) && res.Rev == st.currentRev
	}
	if reqRev < st.compactRev {
		return errors.Is(res.Err, mvcc.ErrCompacted)
	}
	return res.Err == nil && res.Rev == st.currentRev
}

func checkHashByRevStep(st *StorageState, reqRev int64, res StorageResponse) bool {
	if reqRev > st.currentRev {
		return errors.Is(res.Err, mvcc.ErrFutureRev) && res.Rev == st.currentRev
	}
	if reqRev > 0 && reqRev < st.compactRev {
		if res.Err != nil {
			return errors.Is(res.Err, mvcc.ErrCompacted)
		}
		// HashStorage caches up to 10 compaction hashes (with CompactRevision = prevCompactRev < reqRev).
		return res.Rev == st.currentRev && res.HashRev == reqRev && res.CompactRev < reqRev
	}
	if res.Err != nil {
		return false
	}
	if reqRev == 0 {
		// Current hashStorage.HashByRev(0) checks cached s.hashes before mapping 0 -> currentRev,
		// so if Compact(0) cached {Revision: 0, CompactRevision: -1}, HashByRev(0) returns that entry.
		if res.HashRev == 0 && res.CompactRev == -1 && st.compactRev >= 0 {
			return res.Rev == st.currentRev
		}
		return res.Rev == st.currentRev && res.HashRev == st.currentRev && res.CompactRev <= st.compactRev
	}
	return res.Rev == st.currentRev && res.HashRev == reqRev && res.CompactRev <= st.compactRev
}

func stepDeterministicDeleteRange(st *StorageState, key, end []byte) (*StorageState, StorageResponse, []*mvccpb.Event) {
	next := st.clone()
	targetRev := st.currentRev + 1
	evs := deleteMVCCRangeWithEvents(next.items, key, end, targetRev)
	if len(evs) == 0 {
		return st, StorageResponse{Rev: st.currentRev, Deleted: 0}, nil
	}
	next.currentRev = targetRev
	return next, StorageResponse{Rev: next.currentRev, Deleted: int64(len(evs))}, evs
}

func stepDeterministicTxn(st *StorageState, subOps []TxnSubOp) (*StorageState, StorageResponse, []*mvccpb.Event) {
	next := st.clone()
	targetRev := st.currentRev + 1
	var evs []*mvccpb.Event
	var totalDeleted int64
	hasChanges := false

	for _, sub := range subOps {
		switch sub.Type {
		case TxnSubOpPut:
			hasChanges = true
			kv := putMVCCItem(next.items, sub.Key, sub.Value, sub.Lease, targetRev)
			evs = append(evs, &mvccpb.Event{Type: mvccpb.Event_PUT, Kv: cloneKeyValue(kv)})
		case TxnSubOpDeleteRange:
			delEvs := deleteMVCCRangeWithEvents(next.items, sub.Key, sub.End, targetRev)
			if len(delEvs) > 0 {
				hasChanges = true
				totalDeleted += int64(len(delEvs))
				evs = append(evs, delEvs...)
			}
		}
	}

	if !hasChanges {
		return st, StorageResponse{Rev: st.currentRev, Deleted: 0}, nil
	}
	next.currentRev = targetRev
	return next, StorageResponse{Rev: next.currentRev, Deleted: totalDeleted}, evs
}

func stepDeterministicLeaseRevoke(st *StorageState, leaseID int64) (*StorageState, StorageResponse, []*mvccpb.Event) {
	if leaseID == 0 {
		return st, StorageResponse{Rev: st.currentRev, Deleted: 0}, nil
	}
	var attachedKeys []string
	for k, kv := range st.items {
		if kv.Lease == leaseID {
			attachedKeys = append(attachedKeys, k)
		}
	}
	if len(attachedKeys) == 0 {
		return st, StorageResponse{Rev: st.currentRev, Deleted: 0}, nil
	}
	sort.Strings(attachedKeys)

	next := st.clone()
	next.currentRev++
	evs := make([]*mvccpb.Event, 0, len(attachedKeys))
	for _, k := range attachedKeys {
		delete(next.items, k)
		evs = append(evs, &mvccpb.Event{
			Type: mvccpb.Event_DELETE,
			Kv: &mvccpb.KeyValue{
				Key:         []byte(k),
				ModRevision: next.currentRev,
			},
		})
	}
	return next, StorageResponse{Rev: next.currentRev, Deleted: int64(len(evs))}, evs
}

func evalMVCCGet(st *StorageState, req StorageRequest) StorageResponse {
	if req.Rev > st.currentRev {
		return StorageResponse{Count: -1, Rev: st.currentRev, Err: mvcc.ErrFutureRev}
	}
	if req.Rev > 0 && req.Rev < st.compactRev {
		return StorageResponse{Count: -1, Rev: 0, Err: mvcc.ErrCompacted}
	}
	kv, exists := st.items[string(req.Key)]
	if !exists {
		return StorageResponse{KVs: nil, Count: 0, Rev: st.currentRev}
	}
	return StorageResponse{
		KVs:   []*mvccpb.KeyValue{cloneKeyValue(kv)},
		Count: 1,
		Rev:   st.currentRev,
	}
}

func evalMVCCRange(st *StorageState, req StorageRequest) StorageResponse {
	if req.Op != OpReadTxn {
		if req.Rev > st.currentRev {
			return StorageResponse{Count: -1, Rev: st.currentRev, Err: mvcc.ErrFutureRev}
		}
		if req.Rev > 0 && req.Rev < st.compactRev {
			return StorageResponse{Count: -1, Rev: 0, Err: mvcc.ErrCompacted}
		}
	}

	var matched []*mvccpb.KeyValue
	for k, v := range st.items {
		if matchKeyRange(k, req.Key, req.End) {
			matched = append(matched, v)
		}
	}
	sortKeyValues(matched)

	res := StorageResponse{Rev: st.currentRev}
	if req.Op == OpReadTxn {
		res.FirstRev = st.compactRev
	}

	totalMatching := len(matched)
	if req.CountOnly {
		res.Count = totalMatching
		return res
	}

	expectedCount := totalMatching
	if req.Limit > 0 && int64(len(matched)) > req.Limit {
		matched = matched[:req.Limit]
		if !req.WithTotalCount {
			expectedCount = int(req.Limit)
		}
	}
	res.Count = expectedCount

	if len(matched) == 0 {
		return res
	}
	res.KVs = make([]*mvccpb.KeyValue, len(matched))
	for i, v := range matched {
		if req.FastKeysOnly {
			res.KVs[i] = &mvccpb.KeyValue{
				Key:            bytes.Clone(v.Key),
				CreateRevision: v.CreateRevision,
				ModRevision:    v.ModRevision,
				Version:        v.Version,
			}
		} else {
			res.KVs[i] = cloneKeyValue(v)
		}
	}
	return res
}

func evalBackendRange(st *StorageState, req StorageRequest) StorageResponse {
	var expKeys, expVals [][]byte
	safeRange := req.Bucket == string(schema.Key.Name())
	for k, v := range st.backendBuckets[req.Bucket] {
		var ok bool
		if !safeRange && len(req.End) == 0 {
			ok = k == string(req.Key)
		} else {
			ok = matchKeyRange(k, req.Key, req.End)
		}
		if ok {
			expKeys = append(expKeys, []byte(k))
			expVals = append(expVals, bytes.Clone(v))
		}
	}
	sortBackendKVs(expKeys, expVals)
	if req.Limit > 0 && int64(len(expKeys)) > req.Limit {
		expKeys = expKeys[:req.Limit]
		expVals = expVals[:req.Limit]
	}
	return StorageResponse{BackendKeys: expKeys, BackendVals: expVals}
}

func evalBackendForEach(st *StorageState, req StorageRequest) StorageResponse {
	var expKeys, expVals [][]byte
	for k, v := range st.backendBuckets[req.Bucket] {
		expKeys = append(expKeys, []byte(k))
		expVals = append(expVals, bytes.Clone(v))
	}
	sortBackendKVs(expKeys, expVals)
	return StorageResponse{BackendKeys: expKeys, BackendVals: expVals}
}

func equalStorageResponse(op StorageOpType, exp, act StorageResponse) bool {
	if (exp.Err == nil) != (act.Err == nil) {
		return false
	}
	if exp.Err != nil {
		return errors.Is(act.Err, exp.Err)
	}
	if exp.Rev != act.Rev || exp.FirstRev != act.FirstRev || exp.Count != act.Count || exp.Deleted != act.Deleted {
		return false
	}
	if len(exp.KVs) != len(act.KVs) {
		return false
	}
	for i := range exp.KVs {
		if !equalKeyValue(exp.KVs[i], act.KVs[i]) {
			return false
		}
	}
	if op == OpBackendForEach {
		actKeys := append([][]byte(nil), act.BackendKeys...)
		actVals := append([][]byte(nil), act.BackendVals...)
		sortBackendKVs(actKeys, actVals)
		return equalBackendKVSlices(exp.BackendKeys, exp.BackendVals, actKeys, actVals)
	}
	return equalBackendKVSlices(exp.BackendKeys, exp.BackendVals, act.BackendKeys, act.BackendVals)
}

func putMVCCItem(items map[string]*mvccpb.KeyValue, key, val []byte, lease int64, modRev int64) *mvccpb.KeyValue {
	createRev := modRev
	var ver int64 = 1
	if existing, exists := items[string(key)]; exists {
		createRev = existing.CreateRevision
		ver = existing.Version + 1
	}
	kv := &mvccpb.KeyValue{
		Key:            bytes.Clone(key),
		Value:          bytes.Clone(val),
		CreateRevision: createRev,
		ModRevision:    modRev,
		Version:        ver,
		Lease:          lease,
	}
	items[string(key)] = kv
	return kv
}

func deleteMVCCRangeWithEvents(items map[string]*mvccpb.KeyValue, key, end []byte, rev int64) []*mvccpb.Event {
	var toDelete []string
	for k := range items {
		if matchKeyRange(k, key, end) {
			toDelete = append(toDelete, k)
		}
	}
	if len(toDelete) == 0 {
		return nil
	}
	sort.Strings(toDelete)

	evs := make([]*mvccpb.Event, 0, len(toDelete))
	for _, k := range toDelete {
		delete(items, k)
		evs = append(evs, &mvccpb.Event{
			Type: mvccpb.Event_DELETE,
			Kv: &mvccpb.KeyValue{
				Key:         []byte(k),
				ModRevision: rev,
			},
		})
	}
	return evs
}

func putBackendItem(buckets map[string]map[string][]byte, bucket string, key, val []byte) {
	if buckets[bucket] == nil {
		buckets[bucket] = make(map[string][]byte)
	}
	buckets[bucket][string(key)] = bytes.Clone(val)
}

func equalBackendKVSlices(expKeys, expVals, actKeys, actVals [][]byte) bool {
	if len(expKeys) != len(actKeys) || len(expVals) != len(actVals) {
		return false
	}
	for i := range expKeys {
		if !bytes.Equal(expKeys[i], actKeys[i]) || !bytes.Equal(expVals[i], actVals[i]) {
			return false
		}
	}
	return true
}
