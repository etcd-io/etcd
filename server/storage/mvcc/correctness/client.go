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
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// TestStorage bundles an MVCC WatchableKV store, lessor, and underlying backend for correctness testing.
// applyMu models etcdserver's single-threaded Raft applier across Write, synchronous Compact header,
// Lease Grant/Revoke, and apply-path Backend mutations, while reads, watches, hashes, defrag,
// snapshots, and background compaction batches execute concurrently.
type TestStorage struct {
	Store   mvcc.WatchableKV
	Backend backend.Backend
	Lessor  lease.Lessor
	applyMu sync.Mutex
}

// RecordingClient executes operations against TestStorage while recording Porcupine call/return timestamps.
type RecordingClient struct {
	id         int
	storage    *TestStorage
	baseTime   time.Time
	operations []porcupine.Operation
}

// NewRecordingClient creates a RecordingClient for a single sequential worker goroutine.
func NewRecordingClient(id int, storage *TestStorage, baseTime time.Time) *RecordingClient {
	return &RecordingClient{
		id:       id,
		storage:  storage,
		baseTime: baseTime,
	}
}

// Operations returns all operations recorded by this client.
func (c *RecordingClient) Operations() []porcupine.Operation {
	return c.operations
}

// Execute runs req against the underlying storage and records the operation for linearizability checking.
func (c *RecordingClient) Execute(req StorageRequest) StorageResponse {
	return c.Record(req, func() StorageResponse {
		return ExecuteOp(c.storage, req)
	})
}

// Record captures Call and Return timestamps around fn and appends the resulting Porcupine operation.
func (c *RecordingClient) Record(req StorageRequest, fn func() StorageResponse) StorageResponse {
	callTime := time.Since(c.baseTime).Nanoseconds()
	res := fn()
	returnTime := max(callTime+1, time.Since(c.baseTime).Nanoseconds())
	c.operations = append(c.operations, porcupine.Operation{
		ClientId: c.id,
		Input:    req,
		Call:     callTime,
		Output:   res,
		Return:   returnTime,
	})
	return res
}

// ExecuteTxnWithReadYourWrites executes a multi-op write transaction, verifying in-transaction
// Rev, FirstRev, Changes, and read-your-own-writes/deletes before committing.
func (c *RecordingClient) ExecuteTxnWithReadYourWrites(t *testing.T, subOps []TxnSubOp) StorageResponse {
	req := StorageRequest{Op: OpTxn, TxnOps: subOps}
	return c.Record(req, func() StorageResponse {
		c.storage.applyMu.Lock()
		defer c.storage.applyMu.Unlock()

		tw := c.storage.Store.Write(traceutil.TODO())
		beginRev := tw.Rev()
		assert.Equalf(t, int64(0), tw.FirstRev(), "TxnWrite.FirstRev should be 0")

		finalRev, totalDeleted, expectedChanges := applyTxnSubOpsWithAssertions(t, tw, beginRev, subOps)
		assert.Equalf(t, beginRev, tw.Rev(), "TxnWrite.Rev must remain beginRev prior to End")
		assert.Lenf(t, tw.Changes(), expectedChanges, "TxnWrite.Changes count mismatch")

		if len(subOps) > 0 {
			lastSub := subOps[len(subOps)-1]
			switch lastSub.Type {
			case TxnSubOpPut:
				rr, err := tw.Range(context.Background(), lastSub.Key, nil, mvcc.RangeOptions{})
				if assert.NoErrorf(t, err, "in-txn Range after Put failed") &&
					assert.Lenf(t, rr.KVs, 1, "in-txn Range after Put must return 1 KV") {
					assert.Equalf(t, lastSub.Value, rr.KVs[0].Value, "in-txn read must see latest put value")
					assert.Equalf(t, lastSub.Lease, rr.KVs[0].Lease, "in-txn read must see latest put lease")
					assert.Equalf(t, beginRev+1, rr.KVs[0].ModRevision, "in-txn read must see uncommitted ModRevision")
					assert.Equalf(t, beginRev+1, rr.Rev, "in-txn RangeResult.Rev must advance when changes exist")
				}
			case TxnSubOpDeleteRange:
				rr, err := tw.Range(context.Background(), lastSub.Key, lastSub.End, mvcc.RangeOptions{})
				if assert.NoErrorf(t, err, "in-txn Range after DeleteRange failed") {
					assert.Emptyf(t, rr.KVs, "in-txn read must not observe keys deleted in the same txn")
					assert.Zerof(t, rr.Count, "in-txn read after DeleteRange must report Count 0")
				}
			}
		}

		tw.End()
		return StorageResponse{Rev: finalRev, Deleted: totalDeleted}
	})
}

// ExecuteReadTxn opens an explicit read transaction in the given mode (optionally wrapped via
// NewReadOnlyTxnWrite), verifies FirstRev/Rev and repeatable-read snapshot isolation across
// holdDuration, and records OpReadTxn for linearizability checking.
func (c *RecordingClient) ExecuteReadTxn(
	t *testing.T,
	mode mvcc.ReadTxMode,
	wrapReadOnly bool,
	holdDuration time.Duration,
	req StorageRequest,
) StorageResponse {
	req.Op = OpReadTxn
	req.Rev = 0

	return c.Record(req, func() StorageResponse {
		readTx := c.storage.Store.Read(mode, traceutil.TODO())
		var view mvcc.ReadView = readTx
		if wrapReadOnly {
			roTw := mvcc.NewReadOnlyTxnWrite(readTx)
			assert.Nilf(t, roTw.Changes(), "NewReadOnlyTxnWrite.Changes must be nil")
			view = roTw
		}

		firstRev := view.FirstRev()
		snapRev := view.Rev()
		ro := mvcc.RangeOptions{
			Limit:          req.Limit,
			CountOnly:      req.CountOnly,
			FastKeysOnly:   req.FastKeysOnly,
			WithTotalCount: req.WithTotalCount,
		}

		rr1, err1 := view.Range(context.Background(), req.Key, req.End, ro)
		if holdDuration > 0 {
			time.Sleep(holdDuration)
			rr2, err2 := view.Range(context.Background(), req.Key, req.End, ro)
			assert.Equalf(t, snapRev, view.Rev(), "TxnRead.Rev must remain constant while held open")
			assert.Equalf(t, firstRev, view.FirstRev(), "TxnRead.FirstRev must remain constant while held open")
			assert.Equalf(t, err1, err2, "repeatable TxnRead error mismatch")
			if err1 == nil && rr1 != nil && assert.NotNil(t, rr2) {
				assert.Equalf(t, rr1.Rev, rr2.Rev, "repeatable TxnRead RangeResult.Rev mismatch")
				assert.Equalf(t, rr1.Count, rr2.Count, "repeatable TxnRead Count mismatch")
				if assert.Lenf(t, rr2.KVs, len(rr1.KVs), "repeatable TxnRead KVs length mismatch") {
					for i := range rr1.KVs {
						assert.Truef(t, equalKeyValue(rr1.KVs[i], rr2.KVs[i]),
							"repeatable TxnRead KV[%d] mutated across concurrent writes", i)
					}
				}
			}
		}
		readTx.End()

		res := StorageResponse{
			Rev:      snapRev,
			FirstRev: firstRev,
			Err:      err1,
		}
		if err1 == nil && rr1 != nil {
			assert.Equalf(t, snapRev, rr1.Rev, "TxnRead RangeResult.Rev must equal snapshot revision")
			res.KVs = cloneKeyValues(rr1.KVs)
			res.Count = rr1.Count
			for _, kv := range rr1.KVs {
				assert.LessOrEqualf(t, kv.ModRevision, snapRev,
					"read transaction must not observe mutations newer than its snapshot revision")
			}
		}
		return res
	})
}

// ExecuteOp executes a single StorageRequest against TestStorage and returns the observed StorageResponse.
func ExecuteOp(s *TestStorage, req StorageRequest) StorageResponse {
	switch req.Op {
	case OpPut:
		return execPut(s, req)
	case OpGet:
		return execGet(s.Store, req)
	case OpRange:
		return execRange(s.Store, req)
	case OpReadTxn:
		return execReadTxn(s.Store, req)
	case OpFirstRev:
		return StorageResponse{FirstRev: s.Store.FirstRev()}
	case OpRev:
		return StorageResponse{Rev: s.Store.Rev()}
	case OpDelete:
		return execDeleteRange(s, req.Key, nil)
	case OpDeleteRange:
		return execDeleteRange(s, req.Key, req.End)
	case OpTxn:
		return execTxn(s, req.TxnOps)
	case OpLeaseRevoke:
		return execLeaseRevoke(s, req.Lease)
	case OpCompact:
		return execCompact(s, req.Rev)
	case OpCommit:
		s.applyMu.Lock()
		s.Store.Commit()
		s.applyMu.Unlock()
		return StorageResponse{}
	case OpHash:
		h, rev, err := s.Store.HashStorage().Hash()
		return StorageResponse{Hash: h, Rev: rev, Err: err}
	case OpHashByRev:
		kvh, curRev, err := s.Store.HashStorage().HashByRev(req.Rev)
		return StorageResponse{
			Hash:       kvh.Hash,
			HashRev:    kvh.Revision,
			CompactRev: kvh.CompactRevision,
			Rev:        curRev,
			Err:        err,
		}
	case OpBackendPut:
		return execBackendPut(s, req)
	case OpBackendDelete:
		return execBackendDelete(s, req)
	case OpBackendRange:
		return execBackendRange(s.Backend, req)
	case OpBackendForEach:
		return execBackendForEach(s.Backend, req)
	case OpBackendHash:
		return execBackendHash(s.Backend)
	case OpCrossLayerTxn:
		return execCrossLayerTxn(s, req)
	case OpDefrag:
		return StorageResponse{Err: s.Backend.Defrag()}
	default:
		panic(fmt.Sprintf("unknown storage op %v", req.Op))
	}
}

func execPut(s *TestStorage, req StorageRequest) StorageResponse {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	rev := s.Store.Put(req.Key, req.Value, lease.LeaseID(req.Lease))
	return StorageResponse{Rev: rev}
}

func execGet(store mvcc.WatchableKV, req StorageRequest) StorageResponse {
	rr, err := store.Range(context.Background(), req.Key, nil, mvcc.RangeOptions{Rev: req.Rev})
	if err != nil {
		res := StorageResponse{Err: err, Count: -1}
		if rr != nil {
			res.Rev = rr.Rev
			res.Count = rr.Count
		}
		return res
	}
	var kvs []*mvccpb.KeyValue
	if len(rr.KVs) > 0 {
		kvs = []*mvccpb.KeyValue{cloneKeyValue(rr.KVs[0])}
	}
	return StorageResponse{KVs: kvs, Count: rr.Count, Rev: rr.Rev}
}

func execRange(store mvcc.WatchableKV, req StorageRequest) StorageResponse {
	rr, err := store.Range(context.Background(), req.Key, req.End, mvcc.RangeOptions{
		Rev:            req.Rev,
		Limit:          req.Limit,
		CountOnly:      req.CountOnly,
		FastKeysOnly:   req.FastKeysOnly,
		WithTotalCount: req.WithTotalCount,
	})
	if err != nil {
		res := StorageResponse{Err: err, Count: -1}
		if rr != nil {
			res.Rev = rr.Rev
			res.Count = rr.Count
		}
		return res
	}
	return StorageResponse{KVs: cloneKeyValues(rr.KVs), Count: rr.Count, Rev: rr.Rev}
}

func execReadTxn(store mvcc.WatchableKV, req StorageRequest) StorageResponse {
	readTx := store.Read(mvcc.ConcurrentReadTxMode, traceutil.TODO())
	defer readTx.End()

	firstRev := readTx.FirstRev()
	snapRev := readTx.Rev()
	rr, err := readTx.Range(context.Background(), req.Key, req.End, mvcc.RangeOptions{
		Limit:          req.Limit,
		CountOnly:      req.CountOnly,
		FastKeysOnly:   req.FastKeysOnly,
		WithTotalCount: req.WithTotalCount,
	})
	if err != nil {
		return StorageResponse{Rev: snapRev, FirstRev: firstRev, Count: -1, Err: err}
	}
	return StorageResponse{
		KVs:      cloneKeyValues(rr.KVs),
		Count:    rr.Count,
		Rev:      snapRev,
		FirstRev: firstRev,
	}
}

func execDeleteRange(s *TestStorage, key, end []byte) StorageResponse {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	n, rev := s.Store.DeleteRange(key, end)
	return StorageResponse{Rev: rev, Deleted: n}
}

func execTxn(s *TestStorage, subOps []TxnSubOp) StorageResponse {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	tw := s.Store.Write(traceutil.TODO())
	beginRev := tw.Rev()
	finalRev, totalDeleted, _ := applyTxnSubOpsWithAssertions(nil, tw, beginRev, subOps)
	tw.End()
	return StorageResponse{Rev: finalRev, Deleted: totalDeleted}
}

func applyTxnSubOpsWithAssertions(
	t *testing.T,
	tw mvcc.TxnWrite,
	beginRev int64,
	subOps []TxnSubOp,
) (finalRev, totalDeleted int64, expectedChanges int) {
	finalRev = beginRev
	anyMod := false
	for _, sub := range subOps {
		switch sub.Type {
		case TxnSubOpPut:
			rev := tw.Put(sub.Key, sub.Value, lease.LeaseID(sub.Lease))
			if t != nil {
				assert.Equalf(t, beginRev+1, rev, "TxnWrite.Put must return beginRev+1")
			}
			finalRev = rev
			expectedChanges++
			anyMod = true
		case TxnSubOpDeleteRange:
			n, rev := tw.DeleteRange(sub.Key, sub.End)
			if n > 0 || anyMod {
				if t != nil {
					assert.Equalf(t, beginRev+1, rev, "TxnWrite.DeleteRange with changes must return beginRev+1")
				}
				finalRev = rev
			} else if t != nil {
				assert.Equalf(t, beginRev, rev, "no-op TxnWrite.DeleteRange must return beginRev")
			}
			if n > 0 {
				totalDeleted += n
				expectedChanges += int(n)
				anyMod = true
			}
		}
	}
	return finalRev, totalDeleted, expectedChanges
}

func execLeaseRevoke(s *TestStorage, leaseID int64) StorageResponse {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	if s.Lessor == nil {
		return StorageResponse{Rev: s.Store.Rev()}
	}
	var deleted int64
	if l := s.Lessor.Lookup(lease.LeaseID(leaseID)); l != nil {
		deleted = int64(len(l.Keys()))
	}
	err := s.Lessor.Revoke(lease.LeaseID(leaseID))
	if err == nil {
		_, err = s.Lessor.Grant(lease.LeaseID(leaseID), 3600)
	}
	return StorageResponse{Rev: s.Store.Rev(), Deleted: deleted, Err: err}
}

func execCompact(s *TestStorage, rev int64) StorageResponse {
	s.applyMu.Lock()
	donec, err := s.Store.Compact(traceutil.TODO(), rev)
	s.applyMu.Unlock()
	if err == nil && donec != nil {
		<-donec
	}
	return StorageResponse{Err: err}
}

func execBackendPut(s *TestStorage, req StorageRequest) StorageResponse {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	bk := findBucketByName(req.Bucket)
	tx := s.Backend.BatchTx()
	tx.LockInsideApply()
	tx.UnsafePut(bk, req.Key, req.Value)
	tx.Unlock()
	return StorageResponse{}
}

func execBackendDelete(s *TestStorage, req StorageRequest) StorageResponse {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	bk := findBucketByName(req.Bucket)
	tx := s.Backend.BatchTx()
	tx.LockInsideApply()
	tx.UnsafeDelete(bk, req.Key)
	tx.Unlock()
	return StorageResponse{}
}

func execBackendRange(be backend.Backend, req StorageRequest) StorageResponse {
	bk := findBucketByName(req.Bucket)
	rtx := be.ConcurrentReadTx()
	rtx.RLock()
	resKeys, resVals := rtx.UnsafeRange(bk, req.Key, req.End, req.Limit)
	rtx.RUnlock()

	if len(resKeys) == 0 {
		return StorageResponse{}
	}
	keysCopy := make([][]byte, len(resKeys))
	valsCopy := make([][]byte, len(resVals))
	for i := range resKeys {
		keysCopy[i] = bytes.Clone(resKeys[i])
		valsCopy[i] = bytes.Clone(resVals[i])
	}
	return StorageResponse{BackendKeys: keysCopy, BackendVals: valsCopy}
}

func execBackendForEach(be backend.Backend, req StorageRequest) StorageResponse {
	bk := findBucketByName(req.Bucket)
	rtx := be.ConcurrentReadTx()
	rtx.RLock()
	resMap := make(map[string][]byte)
	err := rtx.UnsafeForEach(bk, func(k, v []byte) error {
		resMap[string(k)] = bytes.Clone(v)
		return nil
	})
	rtx.RUnlock()

	var res StorageResponse
	res.Err = err
	for k, v := range resMap {
		res.BackendKeys = append(res.BackendKeys, []byte(k))
		res.BackendVals = append(res.BackendVals, v)
	}
	sortBackendKVs(res.BackendKeys, res.BackendVals)
	return res
}

func execBackendHash(be backend.Backend) StorageResponse {
	h, err := be.Hash(nil)
	return StorageResponse{Hash: h, Err: err}
}

func execCrossLayerTxn(s *TestStorage, req StorageRequest) StorageResponse {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	bk := findBucketByName(req.Bucket)
	tw := s.Store.Write(traceutil.TODO())
	rev := tw.Put(req.Key, req.Value, lease.LeaseID(req.Lease))
	s.Backend.BatchTx().UnsafePut(bk, req.MetaKey, req.MetaVal)
	tw.End()
	return StorageResponse{Rev: rev}
}

func cloneKeyValues(kvs []*mvccpb.KeyValue) []*mvccpb.KeyValue {
	if len(kvs) == 0 {
		return nil
	}
	out := make([]*mvccpb.KeyValue, len(kvs))
	for i, kv := range kvs {
		out[i] = cloneKeyValue(kv)
	}
	return out
}

func findBucketByName(name string) backend.Bucket {
	for _, b := range schema.AllBuckets {
		if string(b.Name()) == name {
			return b
		}
	}
	panic(fmt.Sprintf("unknown bucket %q", name))
}
