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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

const seedEnvVar = "MVCC_CORRECTNESS_SEED"

// ChoiceWeight pairs a random choice candidate with its selection weight.
type ChoiceWeight[T any] struct {
	Choice T
	Weight int
}

func pickRandom[T any](r *rand.Rand, choices []ChoiceWeight[T]) T {
	sum := 0
	for _, op := range choices {
		sum += op.Weight
	}
	roll := r.Intn(sum)
	for _, op := range choices {
		if roll < op.Weight {
			return op.Choice
		}
		roll -= op.Weight
	}
	panic("unexpected choice roll")
}

// ClientOpType identifies a randomized client workload operation.
type ClientOpType int

const (
	ClientPut ClientOpType = iota
	ClientGet
	ClientRange
	ClientDelete
	ClientDeleteRange
	ClientTxn
	ClientCompact
	ClientReadTxn
	ClientMetaRev
	ClientCommit
	ClientHash
	ClientLeaseRevoke
	ClientBackendPut
	ClientBackendDelete
	ClientBackendRange
	ClientBackendForEach
	ClientBackendHash
	ClientCrossLayerTxn
	ClientDefrag
)

// DefaultLeaseIDs lists pre-granted lease IDs attached during concurrent MVCC fuzzing.
var DefaultLeaseIDs = []int64{101, 102, 103}

// TrafficConfig configures concurrent client, watch, and background maintenance traffic.
type TrafficConfig struct {
	Seed                int64
	NumClients          int
	Duration            time.Duration
	SnapshotInterval    time.Duration
	ForceCommitInterval time.Duration
	Keys                [][]byte
	WatchStartKey       []byte
	WatchEndKey         []byte
	LeaseIDs            []int64
	BackendBuckets      []backend.Bucket
	BackendKeys         [][]byte
	Choices             []ChoiceWeight[ClientOpType]
}

// DefaultTrafficConfig defines the standard concurrent MVCC and backend workload profile.
var DefaultTrafficConfig = TrafficConfig{
	NumClients:          6,
	Duration:            2 * time.Second,
	SnapshotInterval:    200 * time.Millisecond,
	ForceCommitInterval: 100 * time.Millisecond,
	Keys: [][]byte{
		[]byte("key-0"),
		[]byte("key-1"),
		[]byte("key-2"),
		[]byte("key-3"),
		[]byte("key-4"),
	},
	WatchStartKey:  []byte("key-0"),
	WatchEndKey:    []byte("key-5"),
	LeaseIDs:       DefaultLeaseIDs,
	BackendBuckets: []backend.Bucket{schema.AuthUsers, schema.AuthRoles, schema.Alarm},
	BackendKeys: [][]byte{
		[]byte("bkey-0"),
		[]byte("bkey-1"),
		[]byte("bkey-2"),
		[]byte("bkey-3"),
	},
	Choices: []ChoiceWeight[ClientOpType]{
		{Choice: ClientPut, Weight: 15},
		{Choice: ClientGet, Weight: 10},
		{Choice: ClientRange, Weight: 12},
		{Choice: ClientDelete, Weight: 8},
		{Choice: ClientDeleteRange, Weight: 8},
		{Choice: ClientTxn, Weight: 10},
		{Choice: ClientCompact, Weight: 8},
		{Choice: ClientReadTxn, Weight: 8},
		{Choice: ClientMetaRev, Weight: 4},
		{Choice: ClientCommit, Weight: 3},
		{Choice: ClientHash, Weight: 6},
		{Choice: ClientLeaseRevoke, Weight: 4},
		{Choice: ClientBackendPut, Weight: 6},
		{Choice: ClientBackendDelete, Weight: 4},
		{Choice: ClientBackendRange, Weight: 6},
		{Choice: ClientBackendForEach, Weight: 4},
		{Choice: ClientBackendHash, Weight: 3},
		{Choice: ClientCrossLayerTxn, Weight: 8},
		{Choice: ClientDefrag, Weight: 3},
	},
}

// TrafficReport holds the operations, watch streams, snapshot bytes, and telemetry collected during SimulateTraffic.
type TrafficReport struct {
	Seed            int64
	Operations      []porcupine.Operation
	WatchStreams    []RecordedWatchStream
	LatestSnapshot  []byte
	WatchStartKey   []byte
	WatchEndKey     []byte
	SucceededOps    int
	CompactedErrors int
	FutureRevErrors int
	Duration        time.Duration
}

type sharedTrafficState struct {
	cfg             TrafficConfig
	storage         *TestStorage
	valCounter      atomic.Int64
	maxCommittedRev atomic.Int64
	maxCompactedRev atomic.Int64
}

func resolveTrafficSeed(configured int64) int64 {
	if configured != 0 {
		return configured
	}
	if raw := os.Getenv(seedEnvVar); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed != 0 {
			return parsed
		}
	}
	return time.Now().UnixNano()
}

// SimulateTraffic drives concurrent MVCC/backend client workers alongside full-history and
// dynamic fuzzed watch streams and background Snapshot/ForceCommit maintenance loops.
func SimulateTraffic(t *testing.T, s *TestStorage, cfg TrafficConfig) TrafficReport {
	seed := resolveTrafficSeed(cfg.Seed)
	t.Logf("Traffic seed: %d (reproduce with %s=%d)", seed, seedEnvVar, seed)

	shared := &sharedTrafficState{
		cfg:     cfg,
		storage: s,
	}
	shared.maxCommittedRev.Store(1)
	shared.maxCompactedRev.Store(-1)

	stopPrimaryWatch := startPrimaryWatchCollector(t, s.Store, cfg.WatchStartKey, cfg.WatchEndKey)
	stopDynamicWatch := startDynamicWatchFuzzer(seed^0x5a5a5a5a, shared)
	stopMaint := startMaintenanceLoops(t, s.Backend, cfg)

	baseTime := time.Now()
	stopTime := baseTime.Add(cfg.Duration)
	startGate := make(chan struct{})

	clients := make([]*RecordingClient, cfg.NumClients)
	var wg sync.WaitGroup
	wg.Add(cfg.NumClients)
	for cid := range cfg.NumClients {
		client := NewRecordingClient(cid, s, baseTime)
		clients[cid] = client
		workerSeed := seed + int64(cid+1)*9973
		go func(cid int, c *RecordingClient, wSeed int64) {
			defer wg.Done()
			<-startGate
			runClientWorker(t, cid, wSeed, c, shared, stopTime)
		}(cid, client, workerSeed)
	}

	close(startGate)
	wg.Wait()
	dynamicStreams, watchFuzzErr := stopDynamicWatch()
	require.NoError(t, watchFuzzErr)
	latestSnap := stopMaint()
	finalRev := s.Store.Rev()
	primaryStream := stopPrimaryWatch(finalRev)

	duration := time.Since(baseTime)
	var allOps []porcupine.Operation
	for _, c := range clients {
		allOps = append(allOps, c.Operations()...)
	}

	succeeded, compactedErrs, futureErrs := summarizeOperationTelemetry(allOps)
	qps := float64(len(allOps)) / duration.Seconds()
	t.Logf("Total Operations: %d (succeeded=%d, compactedErr=%d, futureRevErr=%d), WatchStreams: %d, Concurrency: %d, Elapsed: %v, Throughput: %.2f ops/sec",
		len(allOps), succeeded, compactedErrs, futureErrs, 1+len(dynamicStreams), cfg.NumClients, duration, qps)

	watchStreams := make([]RecordedWatchStream, 0, 1+len(dynamicStreams))
	watchStreams = append(watchStreams, primaryStream)
	watchStreams = append(watchStreams, dynamicStreams...)

	return TrafficReport{
		Seed:            seed,
		Operations:      allOps,
		WatchStreams:    watchStreams,
		LatestSnapshot:  latestSnap,
		WatchStartKey:   cfg.WatchStartKey,
		WatchEndKey:     cfg.WatchEndKey,
		SucceededOps:    succeeded,
		CompactedErrors: compactedErrs,
		FutureRevErrors: futureErrs,
		Duration:        duration,
	}
}

func summarizeOperationTelemetry(ops []porcupine.Operation) (succeeded, compactedErrs, futureErrs int) {
	for _, op := range ops {
		res := op.Output.(StorageResponse)
		if res.Err == nil {
			succeeded++
		} else if errors.Is(res.Err, mvcc.ErrCompacted) {
			compactedErrs++
		} else if errors.Is(res.Err, mvcc.ErrFutureRev) {
			futureErrs++
		}
	}
	return succeeded, compactedErrs, futureErrs
}

func runClientWorker(t *testing.T, cid int, seed int64, c *RecordingClient, shared *sharedTrafficState, stopTime time.Time) {
	r := rand.New(rand.NewSource(seed))
	for time.Now().Before(stopTime) {
		choice := pickRandom(r, shared.cfg.Choices)
		executeRandomClientOp(t, cid, r, c, shared, choice)
		time.Sleep(time.Duration(1+r.Intn(2)) * time.Millisecond)
	}
}

func executeRandomClientOp(t *testing.T, cid int, r *rand.Rand, c *RecordingClient, shared *sharedTrafficState, choice ClientOpType) {
	keys := shared.cfg.Keys
	k := keys[r.Intn(len(keys))]

	switch choice {
	case ClientPut:
		v := fmt.Appendf(nil, "v%d-c%d", shared.valCounter.Add(1), cid)
		res := c.Execute(StorageRequest{
			Op:    OpPut,
			Key:   k,
			Value: v,
			Lease: shared.pickLeaseID(r),
		})
		shared.advanceMaxCommittedRev(res.Rev)

	case ClientGet:
		targetRev := shared.pickReadTargetRev(r)
		c.Execute(StorageRequest{Op: OpGet, Key: k, Rev: targetRev})

	case ClientRange:
		c.Execute(shared.pickRangeRequest(r))

	case ClientDelete:
		res := c.Execute(StorageRequest{Op: OpDelete, Key: k})
		shared.advanceMaxCommittedRev(res.Rev)

	case ClientDeleteRange:
		startK, endK := shared.pickDeleteRangeBounds(r)
		res := c.Execute(StorageRequest{Op: OpDeleteRange, Key: startK, End: endK})
		shared.advanceMaxCommittedRev(res.Rev)

	case ClientTxn:
		subOps := shared.pickTxnSubOps(r, cid)
		res := c.ExecuteTxnWithReadYourWrites(t, subOps)
		shared.advanceMaxCommittedRev(res.Rev)

	case ClientCompact:
		shared.execRandomCompact(r, c)

	case ClientReadTxn:
		mode := mvcc.ConcurrentReadTxMode
		if r.Intn(2) == 0 {
			mode = mvcc.SharedBufReadTxMode
		}
		wrapReadOnly := r.Intn(4) == 0
		var hold time.Duration
		if mode == mvcc.ConcurrentReadTxMode && r.Intn(10) < 3 {
			hold = time.Millisecond
		}
		rangeReq := shared.pickRangeRequest(r)
		rangeReq.Rev = 0
		c.ExecuteReadTxn(t, mode, wrapReadOnly, hold, rangeReq)

	case ClientMetaRev:
		if r.Intn(2) == 0 {
			c.Execute(StorageRequest{Op: OpFirstRev})
		} else {
			c.Execute(StorageRequest{Op: OpRev})
		}

	case ClientCommit:
		c.Execute(StorageRequest{Op: OpCommit})

	case ClientHash:
		if r.Intn(4) == 0 {
			c.Execute(StorageRequest{Op: OpHash})
		} else {
			c.Execute(StorageRequest{Op: OpHashByRev, Rev: shared.pickHashTargetRev(r)})
		}

	case ClientLeaseRevoke:
		if len(shared.cfg.LeaseIDs) > 0 {
			lid := shared.cfg.LeaseIDs[r.Intn(len(shared.cfg.LeaseIDs))]
			res := c.Execute(StorageRequest{Op: OpLeaseRevoke, Lease: lid})
			shared.advanceMaxCommittedRev(res.Rev)
		}

	case ClientBackendPut:
		bk, bkKey := shared.pickBackendTarget(r)
		v := fmt.Appendf(nil, "bval-%d-c%d", shared.valCounter.Add(1), cid)
		c.Execute(StorageRequest{Op: OpBackendPut, Bucket: string(bk.Name()), Key: bkKey, Value: v})

	case ClientBackendDelete:
		bk, bkKey := shared.pickBackendTarget(r)
		c.Execute(StorageRequest{Op: OpBackendDelete, Bucket: string(bk.Name()), Key: bkKey})

	case ClientBackendRange:
		if r.Intn(4) == 0 {
			c.Execute(StorageRequest{
				Op:     OpBackendRange,
				Bucket: string(schema.Meta.Name()),
				Key:    schema.MetaConsistentIndexKeyName,
				Limit:  1,
			})
		} else {
			bk, bkKey := shared.pickBackendTarget(r)
			c.Execute(StorageRequest{Op: OpBackendRange, Bucket: string(bk.Name()), Key: bkKey, Limit: 1})
		}

	case ClientBackendForEach:
		bk, _ := shared.pickBackendTarget(r)
		c.Execute(StorageRequest{Op: OpBackendForEach, Bucket: string(bk.Name())})

	case ClientBackendHash:
		c.Execute(StorageRequest{Op: OpBackendHash})

	case ClientCrossLayerTxn:
		shared.execRandomCrossLayerTxn(r, cid, k, c)

	case ClientDefrag:
		c.Execute(StorageRequest{Op: OpDefrag})

	default:
		panic(fmt.Sprintf("unknown client op %v", choice))
	}
}

func (s *sharedTrafficState) pickLeaseID(r *rand.Rand) int64 {
	if len(s.cfg.LeaseIDs) == 0 || r.Intn(10) < 6 {
		return int64(lease.NoLease)
	}
	return s.cfg.LeaseIDs[r.Intn(len(s.cfg.LeaseIDs))]
}

func (s *sharedTrafficState) advanceMaxCommittedRev(rev int64) {
	for rev > 0 {
		cur := s.maxCommittedRev.Load()
		if rev <= cur || s.maxCommittedRev.CompareAndSwap(cur, rev) {
			return
		}
	}
}

func (s *sharedTrafficState) pickReadTargetRev(r *rand.Rand) int64 {
	roll := r.Intn(100)
	switch {
	case roll < 35:
		return 0
	case roll < 40:
		// Negative revisions <= 0 resolve to currentRev in mvcc.Range.
		return -1
	default:
		cur := s.maxCommittedRev.Load()
		delta := int64(r.Intn(21) - 10)
		return max(0, cur+delta)
	}
}

func (s *sharedTrafficState) pickHashTargetRev(r *rand.Rand) int64 {
	if r.Intn(100) < 35 {
		return 0
	}
	cur := s.maxCommittedRev.Load()
	delta := int64(r.Intn(21) - 10)
	return max(0, cur+delta)
}

func (s *sharedTrafficState) pickRangeRequest(r *rand.Rand) StorageRequest {
	keys := s.cfg.Keys
	var startK, endK []byte
	switch rangeType := r.Intn(10); {
	case rangeType < 2: // 20% single-key lookup via Range (End == nil)
		startK = keys[r.Intn(len(keys))]
	case rangeType < 4: // 20% unbounded range (>= startK, End == []byte{})
		startK = keys[r.Intn(len(keys))]
		endK = []byte{}
	case rangeType < 9: // 50% bounded non-empty range [startK, endK) including WatchEndKey upper sentinel
		startK, endK = s.pickBoundedKeyRange(r)
	default: // 10% empty range where startK >= endK (len(endK) > 0)
		idx := r.Intn(len(keys))
		startK = keys[idx]
		endK = keys[r.Intn(idx+1)]
	}

	var limit int64
	if r.Intn(10) < 5 {
		limit = int64(1 + r.Intn(3))
	}
	countOnly := r.Intn(10) < 2
	fastKeysOnly := !countOnly && r.Intn(10) < 3
	withTotalCount := !countOnly && r.Intn(2) == 0

	return StorageRequest{
		Op:             OpRange,
		Key:            startK,
		End:            endK,
		Rev:            s.pickReadTargetRev(r),
		Limit:          limit,
		CountOnly:      countOnly,
		FastKeysOnly:   fastKeysOnly,
		WithTotalCount: withTotalCount,
	}
}

func (s *sharedTrafficState) pickDeleteRangeBounds(r *rand.Rand) (startK, endK []byte) {
	keys := s.cfg.Keys
	switch roll := r.Intn(10); {
	case roll < 2:
		return keys[r.Intn(len(keys))], []byte{}
	case roll < 9:
		return s.pickBoundedKeyRange(r)
	default:
		// Empty range startK >= endK (no-op DeleteRange)
		idx := r.Intn(len(keys))
		return keys[idx], keys[r.Intn(idx+1)]
	}
}

// pickBoundedKeyRange selects [startK, endK) with startK < endK, allowing WatchEndKey as the
// upper bound so the highest configured key (e.g. "key-4") is covered by bounded ranges.
func (s *sharedTrafficState) pickBoundedKeyRange(r *rand.Rand) (startK, endK []byte) {
	bounds := make([][]byte, 0, len(s.cfg.Keys)+1)
	bounds = append(bounds, s.cfg.Keys...)
	if len(s.cfg.WatchEndKey) > 0 && bytes.Compare(bounds[len(bounds)-1], s.cfg.WatchEndKey) < 0 {
		bounds = append(bounds, s.cfg.WatchEndKey)
	}
	startIdx := r.Intn(len(bounds) - 1)
	endIdx := startIdx + 1 + r.Intn(len(bounds)-1-startIdx)
	return bounds[startIdx], bounds[endIdx]
}

func (s *sharedTrafficState) pickTxnSubOps(r *rand.Rand, cid int) []TxnSubOp {
	keys := s.cfg.Keys
	numSub := 1 + r.Intn(4)
	subOps := make([]TxnSubOp, numSub)
	for sIdx := range numSub {
		// 40% chance to target the same key as the previous sub-op to exercise intra-txn
		// Put->Put, Put->Delete, Delete->Put, and Delete->Delete generation transitions.
		subK := keys[r.Intn(len(keys))]
		if sIdx > 0 && r.Intn(10) < 4 {
			subK = subOps[sIdx-1].Key
		}

		if r.Intn(2) == 0 {
			subV := fmt.Appendf(nil, "v%d-c%d-s%d", s.valCounter.Add(1), cid, sIdx)
			subOps[sIdx] = TxnSubOp{
				Type:  TxnSubOpPut,
				Key:   subK,
				Value: subV,
				Lease: s.pickLeaseID(r),
			}
			continue
		}
		var endK []byte
		if r.Intn(2) != 0 {
			subK, endK = s.pickBoundedKeyRange(r)
		}
		subOps[sIdx] = TxnSubOp{Type: TxnSubOpDeleteRange, Key: subK, End: endK}
	}
	return subOps
}

func (s *sharedTrafficState) execRandomCompact(r *rand.Rand, c *RecordingClient) {
	cur := s.maxCommittedRev.Load()
	var targetRev int64
	switch roll := r.Intn(10); {
	case roll == 0 && s.maxCompactedRev.Load() < 0:
		// Exercise Compact(0) when initial compactMainRev is -1.
		targetRev = 0
	case roll == 1:
		// Exercise future revision compaction error.
		targetRev = cur + 5
	default:
		if cur > 1 {
			targetRev = int64(r.Intn(int(cur) + 1))
		}
	}
	res := c.Execute(StorageRequest{Op: OpCompact, Rev: targetRev})
	if res.Err == nil {
		for {
			prev := s.maxCompactedRev.Load()
			if targetRev <= prev || s.maxCompactedRev.CompareAndSwap(prev, targetRev) {
				break
			}
		}
	}
}

func (s *sharedTrafficState) pickBackendTarget(r *rand.Rand) (backend.Bucket, []byte) {
	bk := s.cfg.BackendBuckets[r.Intn(len(s.cfg.BackendBuckets))]
	bkKey := s.cfg.BackendKeys[r.Intn(len(s.cfg.BackendKeys))]
	return bk, bkKey
}

func (s *sharedTrafficState) execRandomCrossLayerTxn(r *rand.Rand, cid int, mvccKey []byte, c *RecordingClient) {
	mvccVal := fmt.Appendf(nil, "v%d-c%d-cross", s.valCounter.Add(1), cid)
	leaseID := s.pickLeaseID(r)
	metaKey := schema.MetaConsistentIndexKeyName

	var req StorageRequest
	res := c.Record(StorageRequest{}, func() StorageResponse {
		s.storage.applyMu.Lock()
		defer s.storage.applyMu.Unlock()

		tw := s.storage.Store.Write(traceutil.TODO())
		rev := tw.Put(mvccKey, mvccVal, lease.LeaseID(leaseID))
		metaVal := fmt.Appendf(nil, "%016x", rev)
		s.storage.Backend.BatchTx().UnsafePut(schema.Meta, metaKey, metaVal)
		tw.End()

		req = StorageRequest{
			Op:      OpCrossLayerTxn,
			Bucket:  string(schema.Meta.Name()),
			Key:     mvccKey,
			Value:   mvccVal,
			Lease:   leaseID,
			MetaKey: metaKey,
			MetaVal: metaVal,
		}
		return StorageResponse{Rev: rev}
	})
	ops := c.Operations()
	ops[len(ops)-1].Input = req
	s.advanceMaxCommittedRev(res.Rev)
}

func startPrimaryWatchCollector(t *testing.T, store mvcc.WatchableKV, startKey, endKey []byte) func(finalRev int64) RecordedWatchStream {
	watchStream := store.NewWatchStream()
	watchID, err := watchStream.Watch(context.Background(), clientv3.AutoWatchID, startKey, endKey, 0)
	require.NoError(t, err)

	var responses []mvcc.WatchResponse
	drainTargetCh := make(chan int64, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var maxSeenRev int64
		var compacted bool

		recordResp := func(resp mvcc.WatchResponse) {
			responses = append(responses, cloneWatchResponse(resp))
			if resp.Revision > maxSeenRev {
				maxSeenRev = resp.Revision
			}
			if resp.CompactRevision > 0 {
				compacted = true
			}
		}

		for {
			select {
			case targetRev := <-drainTargetCh:
				// Drain any immediately buffered responses first.
				for {
					select {
					case resp := <-watchStream.Chan():
						recordResp(resp)
					default:
						goto pollQuiescent
					}
				}
			pollQuiescent:
				if maxSeenRev >= targetRev || compacted {
					return
				}
				watchStream.RequestProgress(watchID)
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				deadline := time.After(5 * time.Second)
				for maxSeenRev < targetRev && !compacted {
					select {
					case resp := <-watchStream.Chan():
						recordResp(resp)
					case <-ticker.C:
						watchStream.RequestProgress(watchID)
					case <-deadline:
						t.Errorf("Primary watch stream timed out draining to rev %d (maxSeenRev=%d)", targetRev, maxSeenRev)
						return
					}
				}
				return

			case resp := <-watchStream.Chan():
				recordResp(resp)
			}
		}
	}()

	return func(finalRev int64) RecordedWatchStream {
		drainTargetCh <- finalRev
		wg.Wait()
		watchStream.Close()
		return RecordedWatchStream{
			WatchID:      watchID,
			Key:          bytes.Clone(startKey),
			End:          bytes.Clone(endKey),
			StartRev:     1,
			Filter:       WatchFilterNone,
			RequireExact: true,
			Responses:    responses,
		}
	}
}

// startDynamicWatchFuzzer continuously opens short-lived WatchStreams exercising historical
// unsynced catchup, live synced watches, future startRevs, FilterFuncs, explicit/duplicate WatchIDs,
// ErrEmptyWatcherRange, RequestProgress, RequestProgressAll, Rev, Cancel, and Close.
func startDynamicWatchFuzzer(seed int64, shared *sharedTrafficState) func() ([]RecordedWatchStream, error) {
	stop := make(chan struct{})
	var streams []RecordedWatchStream
	var firstErr error
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		r := rand.New(rand.NewSource(seed))
		iter := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			iter++
			rec, err := fuzzOneWatchStream(r, iter, shared, stop)
			if err != nil && firstErr == nil {
				firstErr = err
				return
			}
			streams = append(streams, rec)
		}
	}()

	return func() ([]RecordedWatchStream, error) {
		close(stop)
		wg.Wait()
		return streams, firstErr
	}
}

func fuzzOneWatchStream(r *rand.Rand, iter int, shared *sharedTrafficState, stop <-chan struct{}) (RecordedWatchStream, error) {
	ws := shared.storage.Store.NewWatchStream()
	if rev := ws.Rev(); rev < 1 {
		ws.Close()
		return RecordedWatchStream{}, fmt.Errorf("WatchStream.Rev returned %d (< 1)", rev)
	}

	if len(shared.cfg.Keys) >= 2 {
		if _, err := ws.Watch(context.Background(), clientv3.AutoWatchID, shared.cfg.Keys[1], shared.cfg.Keys[0], 0); !errors.Is(err, mvcc.ErrEmptyWatcherRange) {
			ws.Close()
			return RecordedWatchStream{}, fmt.Errorf("inverted watch range expected ErrEmptyWatcherRange, got %w", err)
		}
		if _, err := ws.Watch(context.Background(), clientv3.AutoWatchID, shared.cfg.Keys[0], shared.cfg.Keys[0], 0); !errors.Is(err, mvcc.ErrEmptyWatcherRange) {
			ws.Close()
			return RecordedWatchStream{}, fmt.Errorf("equal-bound watch range expected ErrEmptyWatcherRange, got %w", err)
		}
	}

	var startK, endK []byte
	switch r.Intn(3) {
	case 0:
		startK = shared.cfg.Keys[r.Intn(len(shared.cfg.Keys))]
	case 1:
		startK = shared.cfg.Keys[r.Intn(len(shared.cfg.Keys))]
		endK = []byte{}
	default:
		startK, endK = shared.pickBoundedKeyRange(r)
	}

	cur := shared.maxCommittedRev.Load()
	var startRev int64
	switch roll := r.Intn(10); {
	case roll < 3:
		startRev = 0
	case roll < 8:
		startRev = 1 + int64(r.Intn(int(max(1, cur))))
	default:
		startRev = cur + 2
	}

	filters := []WatchFilterType{WatchFilterNone, WatchFilterNoPut, WatchFilterNoDelete}
	filter := filters[r.Intn(len(filters))]

	reqID := mvcc.WatchID(clientv3.AutoWatchID)
	if r.Intn(2) == 0 {
		reqID = mvcc.WatchID(100 + iter)
	}

	wid, err := ws.Watch(context.Background(), reqID, startK, endK, startRev, filter.FilterFuncs()...)
	if err != nil {
		ws.Close()
		return RecordedWatchStream{}, fmt.Errorf("Watch failed: %w", err)
	}
	if reqID != clientv3.AutoWatchID {
		if wid != reqID {
			ws.Close()
			return RecordedWatchStream{}, fmt.Errorf("explicit WatchID mismatch: got %d, want %d", wid, reqID)
		}
		if _, dupErr := ws.Watch(context.Background(), reqID, startK, endK, 0); !errors.Is(dupErr, mvcc.ErrWatcherDuplicateID) {
			ws.Close()
			return RecordedWatchStream{}, fmt.Errorf("duplicate WatchID expected ErrWatcherDuplicateID, got %w", dupErr)
		}
	}

	var responses []mvcc.WatchResponse
	var compacted bool
	window := time.NewTimer(35 * time.Millisecond)
	progressTick := time.NewTicker(12 * time.Millisecond)
collectLoop:
	for {
		select {
		case <-stop:
			break collectLoop
		case <-window.C:
			break collectLoop
		case <-progressTick.C:
			if r.Intn(2) == 0 {
				ws.RequestProgress(wid)
			} else {
				_ = ws.RequestProgressAll()
			}
		case resp := <-ws.Chan():
			responses = append(responses, cloneWatchResponse(resp))
			if resp.CompactRevision > 0 {
				compacted = true
				break collectLoop
			}
		}
	}
	window.Stop()
	progressTick.Stop()

	// Drain any remaining queued frames prior to Cancel/Close.
drainQueued:
	for !compacted {
		select {
		case resp := <-ws.Chan():
			responses = append(responses, cloneWatchResponse(resp))
			if resp.CompactRevision > 0 {
				compacted = true
			}
		default:
			break drainQueued
		}
	}

	if err := ws.Cancel(wid); err != nil {
		ws.Close()
		return RecordedWatchStream{}, fmt.Errorf("first Cancel(%d) failed: %w", wid, err)
	}
	if err := ws.Cancel(wid); !errors.Is(err, mvcc.ErrWatcherNotExist) {
		ws.Close()
		return RecordedWatchStream{}, fmt.Errorf("second Cancel(%d) expected ErrWatcherNotExist, got %w", wid, err)
	}
	ws.Close()
	if _, postCloseWatchErr := ws.Watch(context.Background(), clientv3.AutoWatchID, startK, endK, 0); !errors.Is(postCloseWatchErr, mvcc.ErrEmptyWatcherRange) {
		return RecordedWatchStream{}, fmt.Errorf("post-Close Watch expected ErrEmptyWatcherRange, got %w", postCloseWatchErr)
	}
	if err := ws.Cancel(wid); !errors.Is(err, mvcc.ErrWatcherNotExist) {
		return RecordedWatchStream{}, fmt.Errorf("post-Close Cancel expected ErrWatcherNotExist, got %w", err)
	}

	return RecordedWatchStream{
		WatchID:      wid,
		Key:          bytes.Clone(startK),
		End:          bytes.Clone(endK),
		StartRev:     startRev,
		Filter:       filter,
		RequireExact: false,
		Responses:    responses,
	}, nil
}

func cloneWatchResponse(resp mvcc.WatchResponse) mvcc.WatchResponse {
	cloned := mvcc.WatchResponse{
		WatchID:         resp.WatchID,
		Revision:        resp.Revision,
		CompactRevision: resp.CompactRevision,
	}
	if len(resp.Events) > 0 {
		cloned.Events = make([]*mvccpb.Event, len(resp.Events))
		for i, ev := range resp.Events {
			evCopy := &mvccpb.Event{Type: ev.Type}
			if ev.Kv != nil {
				evCopy.Kv = cloneKeyValue(ev.Kv)
			}
			if ev.PrevKv != nil {
				evCopy.PrevKv = cloneKeyValue(ev.PrevKv)
			}
			cloned.Events[i] = evCopy
		}
	}
	return cloned
}

func startMaintenanceLoops(t *testing.T, be backend.Backend, cfg TrafficConfig) func() []byte {
	stop := make(chan struct{})
	var latestSnap atomic.Value
	var wg sync.WaitGroup

	if cfg.SnapshotInterval > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSnapshotLoop(t, be, cfg.SnapshotInterval, stop, &latestSnap)
		}()
	}
	if cfg.ForceCommitInterval > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runForceCommitLoop(be, cfg.ForceCommitInterval, stop)
		}()
	}

	return func() []byte {
		close(stop)
		wg.Wait()
		if raw, ok := latestSnap.Load().([]byte); ok {
			return raw
		}
		return nil
	}
}

func runSnapshotLoop(t *testing.T, be backend.Backend, interval time.Duration, stop <-chan struct{}, latestSnap *atomic.Value) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	capture := func() {
		snap := be.Snapshot()
		if snap == nil {
			return
		}
		assert.GreaterOrEqual(t, snap.Size(), int64(0))
		var buf bytes.Buffer
		_, err := snap.WriteTo(&buf)
		_ = snap.Close()
		if err != nil {
			t.Errorf("Concurrent snapshot WriteTo failed: %v", err)
			return
		}
		if buf.Len() > 0 {
			latestSnap.Store(bytes.Clone(buf.Bytes()))
		}
	}
	for {
		select {
		case <-stop:
			capture()
			return
		case <-ticker.C:
			capture()
		}
	}
}

func runForceCommitLoop(be backend.Backend, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			be.ForceCommit()
		}
	}
}
