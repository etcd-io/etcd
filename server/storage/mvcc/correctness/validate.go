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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

const defaultLinearizationTimeout = 30 * time.Second

// Validate checks linearizability of all recorded operations, then replays committed writes
// via StepDeterministic to verify historical snapshot reads, HashStorage determinism,
// framed watch streams, and Snapshot/Restore consistency.
func Validate(t *testing.T, s *TestStorage, report TrafficReport) {
	require.Positivef(t, report.SucceededOps, "traffic run must complete at least one succeeded operation")

	validateLinearizableOperations(t, report.Operations, defaultLinearizationTimeout)

	replay, err := NewStorageReplay(report.Operations)
	require.NoErrorf(t, err, "NewStorageReplay failed")

	validateHistoricalOperations(t, replay, report.Operations)
	validateHashStorageQuiescent(t, s)
	validateWatchStreams(t, replay, report.WatchStreams)
	validateSnapshotRestore(t, replay, report)
}

func validateLinearizableOperations(t *testing.T, operations []porcupine.Operation, timeout time.Duration) {
	res, info := porcupine.CheckOperationsVerbose(StoragePorcupineModel, operations, timeout)

	if artifacts := os.Getenv("ARTIFACTS"); artifacts != "" {
		testName := strings.ReplaceAll(t.Name(), "/", "_")
		path := filepath.Join(artifacts, fmt.Sprintf("%s_linearization_visualization.html", testName))
		if err := porcupine.VisualizePath(StoragePorcupineModel, info, path); err != nil {
			t.Logf("Failed to write linearization visualization: %v", err)
		} else {
			t.Logf("Linearization visualization written to: %s", path)
		}
	}

	require.Equalf(t, porcupine.Ok, res, "Linearizability check failed across %d operations", len(operations))
	t.Logf("Linearizability check succeeded across %d operations", len(operations))
}

func validateHistoricalOperations(t *testing.T, replay *StorageReplay, operations []porcupine.Operation) {
	for i, op := range operations {
		req := op.Input.(StorageRequest)
		res := op.Output.(StorageResponse)
		// Rev <= 0 (0 and negative revisions) and error responses are fully checked in Phase 1.
		if req.Rev <= 0 || res.Err != nil || (req.Op != OpGet && req.Op != OpRange) {
			continue
		}

		snapState, err := replay.StateAtRevision(req.Rev)
		require.NoErrorf(t, err, "Op[%d] StateAtRevision(%d) failed", i, req.Rev)

		evalReq := req
		evalReq.Rev = 0
		_, expectedRes, _ := StepDeterministic(snapState, evalReq)

		require.Equalf(t, expectedRes.Count, res.Count,
			"Op[%d] %s Count mismatch at rev %d", i, describeRequest(req), req.Rev)
		require.Lenf(t, res.KVs, len(expectedRes.KVs),
			"Op[%d] %s KVs length mismatch at rev %d", i, describeRequest(req), req.Rev)
		for j := range expectedRes.KVs {
			assertKeyValueMatches(t, fmt.Sprintf("Op[%d] %s KV[%d]", i, describeRequest(req), j), expectedRes.KVs[j], res.KVs[j])
		}
	}
}

func validateHashStorageQuiescent(t *testing.T, s *TestStorage) {
	if s == nil || s.Store == nil {
		return
	}
	hs := s.Store.HashStorage()
	finalRev := s.Store.Rev()
	firstRev := s.Store.FirstRev()

	for _, cached := range hs.Hashes() {
		got, curRev, err := hs.HashByRev(cached.Revision)
		require.NoErrorf(t, err, "HashByRev on cached compaction hash failed")
		require.Equalf(t, cached, got, "cached KeyValueHash mismatch")
		require.Equal(t, finalRev, curRev)
	}

	// Verify quiescent determinism across valid uncompacted revisions and boundary errors.
	minValidRev := max(int64(1), firstRev)
	for rev := max(minValidRev, finalRev-5); rev <= finalRev; rev++ {
		h1, cur1, err1 := hs.HashByRev(rev)
		require.NoError(t, err1)
		h2, cur2, err2 := hs.HashByRev(rev)
		require.NoError(t, err2)
		require.Equalf(t, h1, h2, "quiescent HashByRev(%d) must be deterministic", rev)
		require.Equal(t, finalRev, cur1)
		require.Equal(t, finalRev, cur2)
	}

	_, futCurRev, futErr := hs.HashByRev(finalRev + 1)
	require.ErrorIs(t, futErr, mvcc.ErrFutureRev)
	require.Equal(t, finalRev, futCurRev)

	curHash, curRev, err := hs.HashByRev(finalRev)
	require.NoError(t, err)
	require.Equal(t, finalRev, curHash.Revision)
	require.Equal(t, finalRev, curRev)
	hs.Store(curHash)
	hashes := hs.Hashes()
	require.NotEmptyf(t, hashes, "HashStorage.Hashes must contain stored hash")
}

func validateWatchStreams(t *testing.T, replay *StorageReplay, streams []RecordedWatchStream) {
	require.NotEmptyf(t, streams, "at least the primary watch stream must be recorded")
	for idx, stream := range streams {
		require.NoError(t, checkSingleWatchStream(idx, replay, stream))
	}
}

func checkSingleWatchStream(streamIdx int, replay *StorageReplay, stream RecordedWatchStream) error {
	var actualEvents []*mvccpb.Event
	var prevRespRev int64
	var prevEventModRev int64
	var lastFrameMaxEventRev int64
	wasCompacted := false

	for rIdx, resp := range stream.Responses {
		if wasCompacted {
			return fmt.Errorf("WatchStream[%d] received response #%d after CompactRevision cancellation", streamIdx, rIdx)
		}

		if len(resp.Events) == 0 && resp.CompactRevision == 0 {
			if resp.WatchID != stream.WatchID && resp.WatchID != clientv3.InvalidWatchID {
				return fmt.Errorf("WatchStream[%d] progress response #%d unexpected WatchID %d (want %d or -1)",
					streamIdx, rIdx, resp.WatchID, stream.WatchID)
			}
		} else if resp.WatchID != stream.WatchID {
			return fmt.Errorf("WatchStream[%d] response #%d WatchID mismatch: got %d, want %d",
				streamIdx, rIdx, resp.WatchID, stream.WatchID)
		}

		if resp.CompactRevision > 0 {
			if stream.RequireExact {
				return fmt.Errorf("primary full-history WatchStream[%d] must not be compact-cancelled (CompactRevision=%d)",
					streamIdx, resp.CompactRevision)
			}
			if len(resp.Events) > 0 {
				return fmt.Errorf("WatchStream[%d] compaction response #%d must not contain events", streamIdx, rIdx)
			}
			wasCompacted = true
			continue
		}

		if resp.Revision < prevRespRev {
			return fmt.Errorf("WatchStream[%d] response #%d non-monotonic Revision (%d < %d)",
				streamIdx, rIdx, resp.Revision, prevRespRev)
		}
		prevRespRev = resp.Revision

		if len(resp.Events) == 0 {
			if resp.Revision > replay.MaxRevision() {
				return fmt.Errorf("WatchStream[%d] progress response #%d Revision %d exceeds max committed revision %d",
					streamIdx, rIdx, resp.Revision, replay.MaxRevision())
			}
			continue
		}

		// Verify atomic revision batching: a single ModRevision is never split across two WatchResponses.
		firstFrameModRev := resp.Events[0].Kv.ModRevision
		if firstFrameModRev <= lastFrameMaxEventRev {
			return fmt.Errorf("WatchStream[%d] response #%d split ModRevision %d across multiple WatchResponse frames",
				streamIdx, rIdx, firstFrameModRev)
		}

		for eIdx, ev := range resp.Events {
			if ev.Kv.ModRevision < prevEventModRev {
				return fmt.Errorf("WatchStream[%d] resp[%d].Event[%d] non-monotonic ModRevision (%d < %d)",
					streamIdx, rIdx, eIdx, ev.Kv.ModRevision, prevEventModRev)
			}
			prevEventModRev = ev.Kv.ModRevision

			if stream.StartRev > 0 && ev.Kv.ModRevision < stream.StartRev {
				return fmt.Errorf("WatchStream[%d] delivered event at rev %d < startRev %d",
					streamIdx, ev.Kv.ModRevision, stream.StartRev)
			}
			if !stream.Filter.AllowsEvent(ev.Type) {
				return fmt.Errorf("WatchStream[%d] delivered filtered-out event type %v", streamIdx, ev.Type)
			}
			actualEvents = append(actualEvents, ev)
		}

		lastFrameMaxEventRev = resp.Events[len(resp.Events)-1].Kv.ModRevision
		if resp.Revision < lastFrameMaxEventRev {
			return fmt.Errorf("WatchStream[%d] response #%d Revision %d < last event ModRevision %d",
				streamIdx, rIdx, resp.Revision, lastFrameMaxEventRev)
		}
	}

	if stream.RequireExact {
		expected := replay.EventsMatching(1, stream.Key, stream.End, stream.Filter)
		if len(actualEvents) != len(expected) {
			return fmt.Errorf("primary WatchStream[%d] exact event count mismatch: got %d, want %d",
				streamIdx, len(actualEvents), len(expected))
		}
		for i, ev := range actualEvents {
			if err := checkWatchEventMatches(fmt.Sprintf("WatchStream[%d] Event[%d]", streamIdx, i), expected[i], ev); err != nil {
				return err
			}
		}
		return nil
	}

	if len(actualEvents) == 0 {
		return nil
	}
	anchorRev := stream.StartRev
	if anchorRev <= 0 {
		anchorRev = actualEvents[0].Kv.ModRevision
	}
	expected := replay.EventsMatching(anchorRev, stream.Key, stream.End, stream.Filter)
	if len(actualEvents) > len(expected) {
		return fmt.Errorf("WatchStream[%d] received more events (%d) than expected (%d) from rev %d",
			streamIdx, len(actualEvents), len(expected), anchorRev)
	}
	for i, ev := range actualEvents {
		if err := checkWatchEventMatches(fmt.Sprintf("WatchStream[%d] Event[%d]", streamIdx, i), expected[i], ev); err != nil {
			return err
		}
	}
	return nil
}

func checkWatchEventMatches(prefix string, expected, actual *mvccpb.Event) error {
	if expected.Type != actual.Type {
		return fmt.Errorf("%s Type mismatch: got %v, want %v", prefix, actual.Type, expected.Type)
	}
	if !equalKeyValue(expected.Kv, actual.Kv) {
		if expected.Type == mvccpb.Event_DELETE {
			if string(expected.Kv.Key) != string(actual.Kv.Key) || expected.Kv.ModRevision != actual.Kv.ModRevision {
				return fmt.Errorf("%s DELETE key/rev mismatch: got %+v, want %+v", prefix, actual.Kv, expected.Kv)
			}
			if len(actual.Kv.Value) != 0 || actual.Kv.CreateRevision != 0 || actual.Kv.Version != 0 || actual.Kv.Lease != 0 {
				return fmt.Errorf("%s DELETE event must have zeroed Value/CreateRevision/Version/Lease, got %+v", prefix, actual.Kv)
			}
			return nil
		}
		return fmt.Errorf("%s PUT Kv mismatch: got %+v, want %+v", prefix, actual.Kv, expected.Kv)
	}
	return nil
}

func assertKeyValueMatches(t *testing.T, prefix string, expected, actual *mvccpb.KeyValue) {
	require.Equalf(t, expected.Key, actual.Key, "%s Key mismatch", prefix)
	require.Equalf(t, expected.Value, actual.Value, "%s Value mismatch", prefix)
	require.Equalf(t, expected.CreateRevision, actual.CreateRevision, "%s CreateRevision mismatch", prefix)
	require.Equalf(t, expected.ModRevision, actual.ModRevision, "%s ModRevision mismatch", prefix)
	require.Equalf(t, expected.Version, actual.Version, "%s Version mismatch", prefix)
	require.Equalf(t, expected.Lease, actual.Lease, "%s Lease mismatch", prefix)
}

func validateSnapshotRestore(t *testing.T, replay *StorageReplay, report TrafficReport) {
	if len(report.LatestSnapshot) == 0 {
		return
	}

	tmpDir := t.TempDir()
	initPath := filepath.Join(tmpDir, "init.db")
	snapPath := filepath.Join(tmpDir, "restored.db")
	require.NoError(t, os.WriteFile(snapPath, report.LatestSnapshot, 0o600))

	lg := zap.NewNop()
	initBe := backend.NewDefaultBackend(lg, initPath)
	tx := initBe.BatchTx()
	tx.Lock()
	for _, b := range schema.AllBuckets {
		tx.UnsafeCreateBucket(b)
	}
	tx.Unlock()
	initBe.ForceCommit()

	restoredLessor := lease.NewLessor(lg, initBe, nil, lease.LessorConfig{MinLeaseTTL: 3600})
	restoredStore := mvcc.New(lg, initBe, restoredLessor, mvcc.StoreConfig{})
	defer func() {
		_ = restoredStore.Close()
		restoredLessor.Stop()
		_ = initBe.Close()
	}()

	// Attach a live watcher prior to Restore to exercise watchableStore.Restore watcher resync.
	ws := restoredStore.NewWatchStream()
	defer ws.Close()
	sentinelKey := []byte("__restore_sentinel__")
	_, err := ws.Watch(context.Background(), clientv3.AutoWatchID, sentinelKey, nil, 0)
	require.NoError(t, err)

	snapBe := backend.NewDefaultBackend(lg, snapPath)
	defer func() { _ = snapBe.Close() }()

	restoredLessor.Recover(snapBe, func() lease.TxnDelete { return restoredStore.Write(traceutil.TODO()) })
	require.NoError(t, restoredStore.Restore(snapBe))

	restoredRev := restoredStore.Rev()
	require.GreaterOrEqual(t, restoredRev, int64(1))
	require.LessOrEqual(t, restoredRev, replay.MaxRevision())

	snapState, err := replay.StateAtRevision(restoredRev)
	require.NoError(t, err)

	rangeEnd := report.WatchEndKey
	if rangeEnd == nil {
		rangeEnd = []byte{}
	}
	rr, err := restoredStore.Range(context.Background(), report.WatchStartKey, rangeEnd, mvcc.RangeOptions{})
	require.NoError(t, err)

	_, expectedRes, _ := StepDeterministic(snapState, StorageRequest{
		Op:  OpRange,
		Key: report.WatchStartKey,
		End: rangeEnd,
	})
	require.Equalf(t, expectedRes.Count, rr.Count, "restored store Count mismatch at rev %d", restoredRev)
	require.Lenf(t, rr.KVs, len(expectedRes.KVs), "restored store KVs count mismatch at rev %d", restoredRev)
	for i := range expectedRes.KVs {
		assertKeyValueMatches(t, fmt.Sprintf("RestoredStore KV[%d]@rev=%d", i, restoredRev), expectedRes.KVs[i], rr.KVs[i])
	}

	postRestoreRev := restoredStore.Put(sentinelKey, []byte("ok"), lease.NoLease)
	select {
	case wresp := <-ws.Chan():
		if wresp.CompactRevision > 0 {
			// When the restored snapshot was compacted past the pre-restore watcher's minRev (2),
			// syncWatchers cancels the restored watcher with CompactRevision.
			require.Empty(t, wresp.Events)
			require.Greater(t, wresp.CompactRevision, int64(2))
		} else {
			require.Len(t, wresp.Events, 1)
			require.Equal(t, postRestoreRev, wresp.Events[0].Kv.ModRevision)
			require.Equal(t, []byte("ok"), wresp.Events[0].Kv.Value)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("watcher attached before Restore did not receive post-restore response")
	}
}
