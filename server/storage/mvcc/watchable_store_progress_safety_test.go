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
//
// Characterization tests for watchableStore.progressIfSync.
//
// Purpose: lock down the current observable behavior of progressIfSync
// before the PR-D refactor (moving s.rev() outside s.mu.RLock to break the
// three-way deadlock with Compact/Commit and watchableStoreTxnWrite.End).
// Every TestProgressIfSyncSafety_* below MUST pass before AND after the
// refactor.
//
// TestProgressIfSync_NoDeadlockUnderLockChain is the regression test for the
// fix itself — it is skipped pre-refactor (would deadlock the test binary)
// and unskipped during PR-D verification.

package mvcc

import (
	"context"
	"runtime"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

// newProgressTestStore returns a watchableStore without the background
// syncWatchersLoop / syncVictimsLoop so tests have deterministic control over
// the synced/unsynced groups.
func newProgressTestStore(t *testing.T) *watchableStore {
	t.Helper()
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := newWatchableStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	t.Cleanup(func() { cleanup(s, b) })
	return s
}

// newTestWatcher builds a watcher backed by a buffered bidirectional channel
// so tests can both read responses and assign the (send-only) field.
func newTestWatcher(id WatchID, key []byte, startRev int64) (*watcher, chan WatchResponse) {
	ch := make(chan WatchResponse, 32)
	wa := &watcher{
		key:      key,
		startRev: startRev,
		minRev:   startRev,
		id:       id,
		ch:       ch,
	}
	return wa, ch
}

// --- 1. Unsynced watcher → returns false ------------------------------------

func TestProgressIfSyncSafety_ReturnsFalseWhenWatcherUnsynced(t *testing.T) {
	s := newProgressTestStore(t)

	wa, ch := newTestWatcher(1, []byte("k"), 1)
	s.mu.Lock()
	s.unsynced.add(wa)
	s.mu.Unlock()

	got := s.progressIfSync(map[WatchID]*watcher{wa.id: wa}, wa.id)
	if got {
		t.Fatalf("progressIfSync on unsynced watcher = true, want false")
	}
	if len(ch) != 0 {
		t.Fatalf("unsynced watcher must not receive progress; chan len = %d", len(ch))
	}
}

// --- 2. Watcher.startRev ahead of store.Rev → returns false -----------------

func TestProgressIfSyncSafety_ReturnsFalseWhenStartRevAhead(t *testing.T) {
	s := newProgressTestStore(t)

	currentRev := s.store.Rev()
	wa, ch := newTestWatcher(1, []byte("k"), currentRev+100)
	s.mu.Lock()
	s.synced.add(wa)
	s.mu.Unlock()

	got := s.progressIfSync(map[WatchID]*watcher{wa.id: wa}, wa.id)
	if got {
		t.Fatalf("progressIfSync with startRev ahead of store.Rev = true, want false")
	}
	if len(ch) != 0 {
		t.Fatalf("watcher must not receive progress; chan len = %d", len(ch))
	}
}

// --- 3. Synced watcher with startRev<=Rev → returns true + sends progress ---

func TestProgressIfSyncSafety_SyncedWatcherReceivesProgress(t *testing.T) {
	s := newProgressTestStore(t)

	// Put some keys so store.Rev advances past startRev=1.
	s.Put([]byte("k"), []byte("v"), lease.NoLease)
	s.Put([]byte("k"), []byte("v2"), lease.NoLease)

	expectedRev := s.store.Rev()

	wa, ch := newTestWatcher(42, []byte("k"), 1)
	s.mu.Lock()
	s.synced.add(wa)
	s.mu.Unlock()

	got := s.progressIfSync(map[WatchID]*watcher{wa.id: wa}, wa.id)
	if !got {
		t.Fatalf("progressIfSync on synced watcher = false, want true")
	}
	select {
	case resp := <-ch:
		if resp.WatchID != wa.id {
			t.Errorf("WatchID = %d, want %d", resp.WatchID, wa.id)
		}
		if resp.Revision != expectedRev {
			t.Errorf("Revision = %d, want %d (store.Rev)", resp.Revision, expectedRev)
		}
	default:
		t.Fatalf("synced watcher did not receive progress")
	}
}

// --- 4. Empty watchers map → returns true (boundary) ------------------------

func TestProgressIfSyncSafety_EmptyWatchersMapReturnsTrue(t *testing.T) {
	s := newProgressTestStore(t)

	got := s.progressIfSync(map[WatchID]*watcher{}, 0)
	if !got {
		t.Fatalf("progressIfSync with empty watchers map = false, want true (current behavior)")
	}
}

// --- 5. Multi-watcher, any unsynced → returns false -------------------------

func TestProgressIfSyncSafety_MultiWatcherAnyUnsyncedReturnsFalse(t *testing.T) {
	s := newProgressTestStore(t)
	s.Put([]byte("k"), []byte("v"), lease.NoLease)

	syncedW, syncedCh := newTestWatcher(1, []byte("k"), 1)
	unsyncedW, unsyncedCh := newTestWatcher(2, []byte("k"), 1)
	s.mu.Lock()
	s.synced.add(syncedW)
	s.unsynced.add(unsyncedW)
	s.mu.Unlock()

	got := s.progressIfSync(map[WatchID]*watcher{
		syncedW.id:   syncedW,
		unsyncedW.id: unsyncedW,
	}, 0)
	if got {
		t.Fatalf("progressIfSync with one unsynced watcher = true, want false")
	}
	if len(syncedCh) != 0 || len(unsyncedCh) != 0 {
		t.Fatalf("no watcher should receive progress when one is unsynced")
	}
}

// --- 6. Watcher group state unchanged by progressIfSync (read-only) ---------

func TestProgressIfSyncSafety_GroupStateUnchanged(t *testing.T) {
	s := newProgressTestStore(t)
	s.Put([]byte("k"), []byte("v"), lease.NoLease)

	wa, _ := newTestWatcher(7, []byte("k"), 1)
	s.mu.Lock()
	s.synced.add(wa)
	beforeSynced := len(s.synced.watchers)
	beforeUnsynced := s.unsynced.size()
	s.mu.Unlock()

	_ = s.progressIfSync(map[WatchID]*watcher{wa.id: wa}, wa.id)

	s.mu.RLock()
	afterSynced := len(s.synced.watchers)
	afterUnsynced := s.unsynced.size()
	s.mu.RUnlock()
	if beforeSynced != afterSynced || beforeUnsynced != afterUnsynced {
		t.Fatalf("synced/unsynced sizes changed: synced %d→%d, unsynced %d→%d",
			beforeSynced, afterSynced, beforeUnsynced, afterUnsynced)
	}
}

// --- 6b. Concurrent reads must not race (under -race) -----------------------

func TestProgressIfSyncSafety_ConcurrentCallsNoRace(t *testing.T) {
	s := newProgressTestStore(t)
	s.Put([]byte("k"), []byte("v"), lease.NoLease)

	wa, _ := newTestWatcher(9, []byte("k"), 1)
	s.mu.Lock()
	s.synced.add(wa)
	s.mu.Unlock()

	stop := make(chan struct{})
	done := make(chan struct{}, 8)
	for i := 0; i < 8; i++ {
		go func() {
			for {
				select {
				case <-stop:
					done <- struct{}{}
					return
				default:
				}
				_ = s.progressIfSync(map[WatchID]*watcher{wa.id: wa}, wa.id)
			}
		}()
	}
	time.Sleep(150 * time.Millisecond)
	close(stop)
	for i := 0; i < 8; i++ {
		<-done
	}
	// If -race didn't trip, this passes.
}

// --- 7. Regression: no deadlock under the A/B/C lock-order chain ------------
//
// This is the proof for the P0-3 fix. PRE-refactor the test deadlocks the
// goroutines (3 of them) so we keep it skipped until the refactor lands.
//
// Once progressIfSync moves rev := s.rev() before s.mu.RLock, the chain is
// broken and all three goroutines complete within the timeout.

func TestProgressIfSync_NoDeadlockUnderLockChain(t *testing.T) {
	s := newProgressTestStore(t)
	s.Put([]byte("seed"), []byte("v"), lease.NoLease)

	wa, _ := newTestWatcher(1, []byte("seed"), 1)
	s.mu.Lock()
	s.synced.add(wa)
	s.mu.Unlock()

	cWriteAcquired := make(chan struct{})
	cEndDone := make(chan struct{})
	bDone := make(chan struct{})
	aDone := make(chan struct{})

	// C: open Write txn — this grabs store.mu.RLock and holds it until End().
	go func() {
		defer close(cEndDone)
		tw := s.Write(traceutil.TODO())
		tw.Put([]byte("c"), []byte("v"), lease.NoLease)
		close(cWriteAcquired)
		// Give B and A time to position themselves in the lock chain.
		time.Sleep(200 * time.Millisecond)
		tw.End() // PRE-fix: blocks forever (watchableStore.mu.Lock held by A's RLock).
	}()

	<-cWriteAcquired

	// B: request Commit — wants store.mu.Lock (blocked behind C's RLock).
	go func() {
		defer close(bDone)
		s.store.Commit()
	}()

	// Give B time to enter the s.mu.Lock() wait queue.
	time.Sleep(50 * time.Millisecond)

	// A: progressIfSync — pre-fix takes watchableStore.mu.RLock then requests
	// store.mu.RLock (blocked by B's pending writer).
	go func() {
		defer close(aDone)
		_ = s.progressIfSync(map[WatchID]*watcher{wa.id: wa}, wa.id)
	}()

	timeout := time.After(5 * time.Second)
	remaining := map[string]chan struct{}{"A": aDone, "B": bDone, "C": cEndDone}
	for len(remaining) > 0 {
		select {
		case <-aDone:
			delete(remaining, "A")
		case <-bDone:
			delete(remaining, "B")
		case <-cEndDone:
			delete(remaining, "C")
		case <-timeout:
			var keys []string
			for k := range remaining {
				keys = append(keys, k)
			}
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("deadlock: still blocked = %v\n=== goroutine dump ===\n%s",
				keys, buf[:n])
		}
	}
	_ = context.Background()
}
