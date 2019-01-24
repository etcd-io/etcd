// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package tikv

import (
	"fmt"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/metrics"
	"github.com/pingcap/tidb/sessionctx"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var (
	_ kv.Transaction = (*tikvTxn)(nil)
)

// tikvTxn implements kv.Transaction.
type tikvTxn struct {
	snapshot  *tikvSnapshot
	us        kv.UnionStore
	store     *tikvStore // for connection to region.
	startTS   uint64
	startTime time.Time // Monotonic timestamp for recording txn time consuming.
	commitTS  uint64
	valid     bool
	lockKeys  [][]byte
	mu        sync.Mutex // For thread-safe LockKeys function.
	dirty     bool
	setCnt    int64
	vars      *kv.Variables
}

func newTiKVTxn(store *tikvStore) (*tikvTxn, error) {
	bo := NewBackoffer(context.Background(), tsoMaxBackoff)
	startTS, err := store.getTimestampWithRetry(bo)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return newTikvTxnWithStartTS(store, startTS)
}

// newTikvTxnWithStartTS creates a txn with startTS.
func newTikvTxnWithStartTS(store *tikvStore, startTS uint64) (*tikvTxn, error) {
	ver := kv.NewVersion(startTS)
	snapshot := newTiKVSnapshot(store, ver)
	return &tikvTxn{
		snapshot:  snapshot,
		us:        kv.NewUnionStore(snapshot),
		store:     store,
		startTS:   startTS,
		startTime: time.Now(),
		valid:     true,
		vars:      kv.DefaultVars,
	}, nil
}

func (txn *tikvTxn) SetVars(vars *kv.Variables) {
	txn.vars = vars
	txn.snapshot.vars = vars
}

// SetCap sets the transaction's MemBuffer capability, to reduce memory allocations.
func (txn *tikvTxn) SetCap(cap int) {
	txn.us.SetCap(cap)
}

// Reset reset tikvTxn's membuf.
func (txn *tikvTxn) Reset() {
	txn.us.Reset()
}

// Get implements transaction interface.
func (txn *tikvTxn) Get(k kv.Key) ([]byte, error) {
	metrics.TiKVTxnCmdCounter.WithLabelValues("get").Inc()
	start := time.Now()
	defer func() { metrics.TiKVTxnCmdHistogram.WithLabelValues("get").Observe(time.Since(start).Seconds()) }()

	ret, err := txn.us.Get(k)
	if err != nil {
		return nil, errors.Trace(err)
	}

	err = txn.store.CheckVisibility(txn.startTS)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return ret, nil
}

func (txn *tikvTxn) Set(k kv.Key, v []byte) error {
	txn.setCnt++

	txn.dirty = true
	return txn.us.Set(k, v)
}

func (txn *tikvTxn) String() string {
	return fmt.Sprintf("%d", txn.StartTS())
}

func (txn *tikvTxn) Iter(k kv.Key, upperBound kv.Key) (kv.Iterator, error) {
	metrics.TiKVTxnCmdCounter.WithLabelValues("seek").Inc()
	start := time.Now()
	defer func() { metrics.TiKVTxnCmdHistogram.WithLabelValues("seek").Observe(time.Since(start).Seconds()) }()

	return txn.us.Iter(k, upperBound)
}

// IterReverse creates a reversed Iterator positioned on the first entry which key is less than k.
func (txn *tikvTxn) IterReverse(k kv.Key) (kv.Iterator, error) {
	metrics.TiKVTxnCmdCounter.WithLabelValues("seek_reverse").Inc()
	start := time.Now()
	defer func() {
		metrics.TiKVTxnCmdHistogram.WithLabelValues("seek_reverse").Observe(time.Since(start).Seconds())
	}()

	return txn.us.IterReverse(k)
}

func (txn *tikvTxn) Delete(k kv.Key) error {
	metrics.TiKVTxnCmdCounter.WithLabelValues("delete").Inc()

	txn.dirty = true
	return txn.us.Delete(k)
}

func (txn *tikvTxn) SetOption(opt kv.Option, val interface{}) {
	txn.us.SetOption(opt, val)
	switch opt {
	case kv.Priority:
		txn.snapshot.priority = kvPriorityToCommandPri(val.(int))
	case kv.NotFillCache:
		txn.snapshot.notFillCache = val.(bool)
	case kv.SyncLog:
		txn.snapshot.syncLog = val.(bool)
	case kv.KeyOnly:
		txn.snapshot.keyOnly = val.(bool)
	}
}

func (txn *tikvTxn) DelOption(opt kv.Option) {
	txn.us.DelOption(opt)
}

func (txn *tikvTxn) Commit(ctx context.Context) error {
	if !txn.valid {
		return kv.ErrInvalidTxn
	}
	defer txn.close()

	metrics.TiKVTxnCmdCounter.WithLabelValues("set").Add(float64(txn.setCnt))
	metrics.TiKVTxnCmdCounter.WithLabelValues("commit").Inc()
	start := time.Now()
	defer func() { metrics.TiKVTxnCmdHistogram.WithLabelValues("commit").Observe(time.Since(start).Seconds()) }()

	if err := txn.us.CheckLazyConditionPairs(); err != nil {
		return errors.Trace(err)
	}

	// connID is used for log.
	var connID uint64
	val := ctx.Value(sessionctx.ConnID)
	if val != nil {
		connID = val.(uint64)
	}
	committer, err := newTwoPhaseCommitter(txn, connID)
	if err != nil || committer == nil {
		return errors.Trace(err)
	}
	// latches disabled
	if txn.store.txnLatches == nil {
		err = committer.executeAndWriteFinishBinlog(ctx)
		log.Debug("[kv]", connID, " txnLatches disabled, 2pc directly:", err)
		return errors.Trace(err)
	}

	// latches enabled
	// for transactions which need to acquire latches
	lock := txn.store.txnLatches.Lock(committer.startTS, committer.keys)
	defer txn.store.txnLatches.UnLock(lock)
	if lock.IsStale() {
		err = errors.Errorf("startTS %d is stale", txn.startTS)
		return errors.Annotate(err, txnRetryableMark)
	}
	err = committer.executeAndWriteFinishBinlog(ctx)
	if err == nil {
		lock.SetCommitTS(committer.commitTS)
	}
	log.Debug("[kv]", connID, " txnLatches enabled while txn retryable:", err)
	return errors.Trace(err)
}

func (txn *tikvTxn) close() {
	txn.valid = false
}

func (txn *tikvTxn) Rollback() error {
	if !txn.valid {
		return kv.ErrInvalidTxn
	}
	txn.close()
	log.Debugf("[kv] Rollback txn %d", txn.StartTS())
	metrics.TiKVTxnCmdCounter.WithLabelValues("rollback").Inc()

	return nil
}

func (txn *tikvTxn) LockKeys(keys ...kv.Key) error {
	metrics.TiKVTxnCmdCounter.WithLabelValues("lock_keys").Inc()
	txn.mu.Lock()
	for _, key := range keys {
		txn.lockKeys = append(txn.lockKeys, key)
	}
	txn.dirty = true
	txn.mu.Unlock()
	return nil
}

func (txn *tikvTxn) IsReadOnly() bool {
	return !txn.dirty
}

func (txn *tikvTxn) StartTS() uint64 {
	return txn.startTS
}

func (txn *tikvTxn) Valid() bool {
	return txn.valid
}

func (txn *tikvTxn) Len() int {
	return txn.us.Len()
}

func (txn *tikvTxn) Size() int {
	return txn.us.Size()
}

func (txn *tikvTxn) GetMemBuffer() kv.MemBuffer {
	return txn.us.GetMemBuffer()
}

func (txn *tikvTxn) GetSnapshot() kv.Snapshot {
	return txn.snapshot
}
