// Copyright 2015 The etcd Authors
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
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/pkg/v3/schedule"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"

	"go.uber.org/zap"
)

var (
	scheduledCompactKeyName = []byte("scheduledCompactRev")
	finishedCompactKeyName  = []byte("finishedCompactRev")

	ErrCompacted = errors.New("mvcc: required revision has been compacted")
	ErrFutureRev = errors.New("mvcc: required revision is a future revision")
)

const (
	// markedRevBytesLen is the byte length of marked revision.
	// The first `revBytesLen` bytes represents a normal revision. The last
	// one byte is the mark.
	markedRevBytesLen      = revBytesLen + 1
	markBytePosition       = markedRevBytesLen - 1
	markTombstone     byte = 't'
)

var restoreChunkKeys = 10000 // non-const for testing
var defaultCompactBatchLimit = 1000

type StoreConfig struct {
	CompactionBatchLimit int
}

// 实现了 KV 接口
type store struct {
	ReadView
	WriteView

	cfg StoreConfig

	// mu read locks for txns and write locks for non-txn store changes.
	// 在开启 只读/读写事务时，需要获取该读锁进行 同步， 即在 Read()方法和 Write()方法中获取该读锁，在 End()方法中释放。
	// 在进行压缩等非事务性的操作时，需要加写锁进行同步。
	mu sync.RWMutex

	// 当前 store实例关联的后端存储
	b backend.Backend

	// 当前 store 实例关联 的内存索引，index 接口 及其具体实现 (treeIndex)
	kvindex index

	// 租约相关的内容
	le lease.Lessor

	// revMuLock protects currentRev and compactMainRev.
	// Locked at end of write txn and released after write txn unlock lock.
	// Locked before locking read txn and released after locking.
	// 在修改 currentRev 字段和 compactMainRev 字段时，需要获取该锁进行同步。
	revMu sync.RWMutex
	// currentRev is the revision of the last completed transaction.
	// 记录当前的 revision信息(main revision部分的值)。
	currentRev int64

	// compactMainRev is the main revision of the last compaction.
	// 该字段记录最近一次压缩后最小的 revision信息(main revision 部分的值〉。
	compactMainRev int64

	// FIFO 调度器，
	fifoSched schedule.Scheduler

	stopc chan struct{}

	lg *zap.Logger
}

// NewStore returns a new store. It is useful to create a store inside
// mvcc pkg. It should only be used for testing externally.
func NewStore(lg *zap.Logger, b backend.Backend, le lease.Lessor, cfg StoreConfig) *store {
	if lg == nil {
		lg = zap.NewNop()
	}
	if cfg.CompactionBatchLimit == 0 {
		cfg.CompactionBatchLimit = defaultCompactBatchLimit
	}
	s := &store{
		cfg:     cfg,
		b:       b,                // 创建后端存储实例
		kvindex: newTreeIndex(lg), // 创建内存索引

		le: le, // 赋值租约

		currentRev:     1,
		compactMainRev: -1,

		fifoSched: schedule.NewFIFOScheduler(),

		stopc: make(chan struct{}),

		lg: lg,
	}
	// 创建 readView 实例 和 writeView 实例
	s.ReadView = &readView{s}
	s.WriteView = &writeView{s}
	if s.le != nil {
		s.le.SetRangeDeleter(func() lease.TxnDelete { return s.Write(traceutil.TODO()) })
	}

	// 获取 backend 的读写事务，创建名为 "key" 和 "meta" 的两个 Bucket，然后提交事务
	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(buckets.Key)
	tx.UnsafeCreateBucket(buckets.Meta)
	tx.Unlock()
	s.b.ForceCommit()

	s.mu.Lock()
	defer s.mu.Unlock()
	// 从 backend 中恢复当前 store 的所有状态，其中包括内存中的 BTree 索引等
	if err := s.restore(); err != nil {
		// TODO: return the error instead of panic here?
		panic("failed to recover store from backend")
	}

	return s
}

func (s *store) compactBarrier(ctx context.Context, ch chan struct{}) {
	if ctx == nil || ctx.Err() != nil {
		select {
		case <-s.stopc:
		default:
			// fix deadlock in mvcc,for more information, please refer to pr 11817.
			// s.stopc is only updated in restore operation, which is called by apply
			// snapshot call, compaction and apply snapshot requests are serialized by
			// raft, and do not happen at the same time.
			s.mu.Lock()
			f := func(ctx context.Context) { s.compactBarrier(ctx, ch) }
			s.fifoSched.Schedule(f)
			s.mu.Unlock()
		}
		return
	}
	close(ch)
}

func (s *store) Hash() (hash uint32, revision int64, err error) {
	// TODO: hash and revision could be inconsistent, one possible fix is to add s.revMu.RLock() at the beginning of function, which is costly
	start := time.Now()

	s.b.ForceCommit()
	h, err := s.b.Hash(buckets.DefaultIgnores)

	hashSec.Observe(time.Since(start).Seconds())
	return h, s.currentRev, err
}

func (s *store) HashByRev(rev int64) (hash uint32, currentRev int64, compactRev int64, err error) {
	start := time.Now()

	s.mu.RLock()
	s.revMu.RLock()
	compactRev, currentRev = s.compactMainRev, s.currentRev
	s.revMu.RUnlock()

	if rev > 0 && rev <= compactRev {
		s.mu.RUnlock()
		return 0, 0, compactRev, ErrCompacted
	} else if rev > 0 && rev > currentRev {
		s.mu.RUnlock()
		return 0, currentRev, 0, ErrFutureRev
	}

	if rev == 0 {
		rev = currentRev
	}
	keep := s.kvindex.Keep(rev)

	tx := s.b.ReadTx()
	tx.RLock()
	defer tx.RUnlock()
	s.mu.RUnlock()

	upper := revision{main: rev + 1}
	lower := revision{main: compactRev + 1}
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))

	h.Write(buckets.Key.Name())
	err = tx.UnsafeForEach(buckets.Key, func(k, v []byte) error {
		kr := bytesToRev(k)
		if !upper.GreaterThan(kr) {
			return nil
		}
		// skip revisions that are scheduled for deletion
		// due to compacting; don't skip if there isn't one.
		if lower.GreaterThan(kr) && len(keep) > 0 {
			if _, ok := keep[kr]; !ok {
				return nil
			}
		}
		h.Write(k)
		h.Write(v)
		return nil
	})
	hash = h.Sum32()

	hashRevSec.Observe(time.Since(start).Seconds())
	return hash, currentRev, compactRev, err
}

func (s *store) updateCompactRev(rev int64) (<-chan struct{}, error) {
	s.revMu.Lock()
	if rev <= s.compactMainRev {
		ch := make(chan struct{})
		f := func(ctx context.Context) { s.compactBarrier(ctx, ch) }
		s.fifoSched.Schedule(f)
		s.revMu.Unlock()
		return ch, ErrCompacted
	}
	if rev > s.currentRev {
		s.revMu.Unlock()
		return nil, ErrFutureRev
	}

	s.compactMainRev = rev

	rbytes := newRevBytes()
	revToBytes(revision{main: rev}, rbytes)

	tx := s.b.BatchTx()
	tx.Lock()
	tx.UnsafePut(buckets.Meta, scheduledCompactKeyName, rbytes)
	tx.Unlock()
	// ensure that desired compaction is persisted
	s.b.ForceCommit()

	s.revMu.Unlock()

	return nil, nil
}

func (s *store) compact(trace *traceutil.Trace, rev int64) (<-chan struct{}, error) {
	ch := make(chan struct{})
	var j = func(ctx context.Context) {
		if ctx.Err() != nil {
			s.compactBarrier(ctx, ch)
			return
		}
		start := time.Now()
		keep := s.kvindex.Compact(rev)
		indexCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))
		if !s.scheduleCompaction(rev, keep) {
			s.compactBarrier(context.TODO(), ch)
			return
		}
		close(ch)
	}

	s.fifoSched.Schedule(j)
	trace.Step("schedule compaction")
	return ch, nil
}

func (s *store) compactLockfree(rev int64) (<-chan struct{}, error) {
	ch, err := s.updateCompactRev(rev)
	if err != nil {
		return ch, err
	}

	return s.compact(traceutil.TODO(), rev)
}

func (s *store) Compact(trace *traceutil.Trace, rev int64) (<-chan struct{}, error) {
	s.mu.Lock()

	ch, err := s.updateCompactRev(rev)
	trace.Step("check and update compact revision")
	if err != nil {
		s.mu.Unlock()
		return ch, err
	}
	s.mu.Unlock()

	return s.compact(trace, rev)
}

func (s *store) Commit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b.ForceCommit()
}

// Restore 首先批量读取 BoltDB 中所有的键值对数据， 然后将每个键值对封装成 revKeyValue 实例，并写入一个通道中， 最后由另 一个单独的 goroutine 读取该通道，并完成内存索引的恢复。
func (s *store) Restore(b backend.Backend) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	close(s.stopc)
	s.fifoSched.Stop()

	s.b = b
	s.kvindex = newTreeIndex(s.lg)

	{
		// During restore the metrics might report 'special' values
		s.revMu.Lock()
		s.currentRev = 1
		s.compactMainRev = -1
		s.revMu.Unlock()
	}

	s.fifoSched = schedule.NewFIFOScheduler()
	s.stopc = make(chan struct{})

	return s.restore()
}

func (s *store) restore() error {
	s.setupMetricsReporter()

	// 创建 min 和 max，后 续在 BoltDB 中进行范围查询时的起始 key 和 结束 key 就是 min 和 max
	min, max := newRevBytes(), newRevBytes()
	revToBytes(revision{main: 1}, min) // min 为 1_0
	// max 为 MaxInt64_MaxInt64
	revToBytes(revision{main: math.MaxInt64, sub: math.MaxInt64}, max)

	keyToLease := make(map[string]lease.LeaseID)

	// restore index
	tx := s.b.BatchTx() // 获取读写事务， 并加锁
	tx.Lock()

	// 调用 UnsafeRange()方法，在 meta Bucket 中查询上次的压缩完成时的相关记录 (Key 为 finishedCompactRev}
	_, finishedCompactBytes := tx.UnsafeRange(buckets.Meta, finishedCompactKeyName, nil, 0)
	if len(finishedCompactBytes) != 0 { // 根据查询结果 恢复store.compactMainRev字段
		s.revMu.Lock()
		s.compactMainRev = bytesToRev(finishedCompactBytes[0]).main

		s.lg.Info(
			"restored last compact revision",
			zap.Stringer("meta-bucket-name", buckets.Meta),
			zap.String("meta-bucket-name-key", string(finishedCompactKeyName)),
			zap.Int64("restored-compact-revision", s.compactMainRev),
		)
		s.revMu.Unlock()
	}
	// 调用UnsafeRange()方法，在meta Bucket中查询上次的压缩启动时的相关记录(Key 为 scheduledCompactRev)
	_, scheduledCompactBytes := tx.UnsafeRange(buckets.Meta, scheduledCompactKeyName, nil, 0)
	scheduledCompact := int64(0)
	if len(scheduledCompactBytes) != 0 {
		// 根据查询结果更新 scheduledCompact 交量
		scheduledCompact = bytesToRev(scheduledCompactBytes[0]).main
	}

	// index keys concurrently as they're loaded in from tx
	keysGauge.Set(0)

	// 在 restoreIntoIndex() 方法中会启动一个单独的 goroutine，用于接收从 backend 中读取的键值对数据，并恢复到新建的内存索引中{ store.kvIndex)，
	// 这里需要了解该 方法的返回位 rkvc 是一个通道，也是当前 goroutine 与上述 goroutine 通信的桥梁
	rkvc, revc := restoreIntoIndex(s.lg, s.kvindex)
	for {
		// 调用 UnsafeRange()方法查询 BoltDB 中的 key Bucket，返回键值对数量的上限是 restoreChunkKeys, 默认是 10000
		keys, vals := tx.UnsafeRange(buckets.Key, min, max, int64(restoreChunkKeys))
		if len(keys) == 0 { //查询结果为空，则直接结束当前for循环
			break
		}
		// rkvc blocks if the total pending keys exceeds the restore
		// chunk size to keep keys from consuming too much memory.
		// 将查询到的键值对数据写入 rkvc 这个通道 中，并由 restoreIntoIndex()方法中创建的goroutine进行处理
		restoreChunk(s.lg, rkvc, keys, vals, keyToLease)
		if len(keys) < restoreChunkKeys {
			// partial set implies final set
			// 范围查询得到的结果数小于 restoreChunkKeys，即表示最后一次查询
			break
		}
		// 更新 min 作 为下一次范围查询的起始 key
		// next set begins after where this one ended
		newMin := bytesToRev(keys[len(keys)-1][:revBytesLen])
		newMin.sub++
		revToBytes(newMin, min)
	}
	close(rkvc)

	{
		s.revMu.Lock()
		s.currentRev = <-revc // 从revc通道中获取恢复之后的currentRev

		// keys in the range [compacted revision -N, compaction] might all be deleted due to compaction.
		// the correct revision should be set to compaction revision in the case, not the largest revision
		// we have seen.
		if s.currentRev < s.compactMainRev {
			s.currentRev = s.compactMainRev
		}
		s.revMu.Unlock()
	}

	if scheduledCompact <= s.compactMainRev {
		scheduledCompact = 0
	}

	for key, lid := range keyToLease {
		if s.le == nil {
			tx.Unlock()
			panic("no lessor to attach lease")
		}
		err := s.le.Attach(lid, []lease.LeaseItem{{Key: key}})
		if err != nil {
			s.lg.Error(
				"failed to attach a lease",
				zap.String("lease-id", fmt.Sprintf("%016x", lid)),
				zap.Error(err),
			)
		}
	}

	tx.Unlock()

	s.lg.Info("kvstore restored", zap.Int64("current-rev", s.currentRev))

	if scheduledCompact != 0 {
		if _, err := s.compactLockfree(scheduledCompact); err != nil {
			s.lg.Warn("compaction encountered error", zap.Error(err))
		}

		s.lg.Info(
			"resume scheduled compaction",
			zap.Stringer("meta-bucket-name", buckets.Meta),
			zap.String("meta-bucket-name-key", string(scheduledCompactKeyName)),
			zap.Int64("scheduled-compact-revision", scheduledCompact),
		)
	}

	return nil
}

type revKeyValue struct {
	key  []byte          // BoltDB中的Key值，可以转换得到的revision实例。
	kv   mvccpb.KeyValue // BoltDB 中保存的 Value 值。
	kstr string          // 原始的Key值。
}

func restoreIntoIndex(lg *zap.Logger, idx index) (chan<- revKeyValue, <-chan int64) {
	rkvc, revc := make(chan revKeyValue, restoreChunkKeys), make(chan int64, 1)
	go func() {
		currentRev := int64(1)
		defer func() { revc <- currentRev }()
		// restore the tree index from streaming the unordered index.
		kiCache := make(map[string]*keyIndex, restoreChunkKeys)
		for rkv := range rkvc {
			ki, ok := kiCache[rkv.kstr]
			// purge kiCache if many keys but still missing in the cache
			if !ok && len(kiCache) >= restoreChunkKeys {
				i := 10
				for k := range kiCache {
					delete(kiCache, k)
					if i--; i == 0 {
						break
					}
				}
			}
			// cache miss, fetch from tree index if there
			if !ok {
				ki = &keyIndex{key: rkv.kv.Key}
				if idxKey := idx.KeyIndex(ki); idxKey != nil {
					kiCache[rkv.kstr], ki = idxKey, idxKey
					ok = true
				}
			}
			rev := bytesToRev(rkv.key)
			currentRev = rev.main
			if ok {
				if isTombstone(rkv.key) {
					if err := ki.tombstone(lg, rev.main, rev.sub); err != nil {
						lg.Warn("tombstone encountered error", zap.Error(err))
					}
					continue
				}
				ki.put(lg, rev.main, rev.sub)
			} else if !isTombstone(rkv.key) {
				ki.restore(lg, revision{rkv.kv.CreateRevision, 0}, rev, rkv.kv.Version)
				idx.Insert(ki)
				kiCache[rkv.kstr] = ki
			}
		}
	}()
	return rkvc, revc
}

func restoreChunk(lg *zap.Logger, kvc chan<- revKeyValue, keys, vals [][]byte, keyToLease map[string]lease.LeaseID) {
	for i, key := range keys {
		rkv := revKeyValue{key: key}
		if err := rkv.kv.Unmarshal(vals[i]); err != nil {
			lg.Fatal("failed to unmarshal mvccpb.KeyValue", zap.Error(err))
		}
		rkv.kstr = string(rkv.kv.Key)
		if isTombstone(key) {
			delete(keyToLease, rkv.kstr)
		} else if lid := lease.LeaseID(rkv.kv.Lease); lid != lease.NoLease {
			keyToLease[rkv.kstr] = lid
		} else {
			delete(keyToLease, rkv.kstr)
		}
		kvc <- rkv
	}
}

func (s *store) Close() error {
	close(s.stopc)
	s.fifoSched.Stop()
	return nil
}

func (s *store) setupMetricsReporter() {
	b := s.b
	reportDbTotalSizeInBytesMu.Lock()
	reportDbTotalSizeInBytes = func() float64 { return float64(b.Size()) }
	reportDbTotalSizeInBytesMu.Unlock()
	reportDbTotalSizeInBytesDebugMu.Lock()
	reportDbTotalSizeInBytesDebug = func() float64 { return float64(b.Size()) }
	reportDbTotalSizeInBytesDebugMu.Unlock()
	reportDbTotalSizeInUseInBytesMu.Lock()
	reportDbTotalSizeInUseInBytes = func() float64 { return float64(b.SizeInUse()) }
	reportDbTotalSizeInUseInBytesMu.Unlock()
	reportDbOpenReadTxNMu.Lock()
	reportDbOpenReadTxN = func() float64 { return float64(b.OpenReadTxN()) }
	reportDbOpenReadTxNMu.Unlock()
	reportCurrentRevMu.Lock()
	reportCurrentRev = func() float64 {
		s.revMu.RLock()
		defer s.revMu.RUnlock()
		return float64(s.currentRev)
	}
	reportCurrentRevMu.Unlock()
	reportCompactRevMu.Lock()
	reportCompactRev = func() float64 {
		s.revMu.RLock()
		defer s.revMu.RUnlock()
		return float64(s.compactMainRev)
	}
	reportCompactRevMu.Unlock()
}

// appendMarkTombstone appends tombstone mark to normal revision bytes.
func appendMarkTombstone(lg *zap.Logger, b []byte) []byte {
	if len(b) != revBytesLen {
		lg.Panic(
			"cannot append tombstone mark to non-normal revision bytes",
			zap.Int("expected-revision-bytes-size", revBytesLen),
			zap.Int("given-revision-bytes-size", len(b)),
		)
	}
	return append(b, markTombstone)
}

// isTombstone checks whether the revision bytes is a tombstone.
func isTombstone(b []byte) bool {
	return len(b) == markedRevBytesLen && b[markBytePosition] == markTombstone
}
