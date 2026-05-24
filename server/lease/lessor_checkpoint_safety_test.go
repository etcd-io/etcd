// Copyright 2026 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Characterization tests for lessor.Checkpoint.
//
// Purpose: lock down the current observable behavior of Checkpoint before
// any refactor that moves persistTo out of le.mu. Every assertion here MUST
// keep passing after the refactor.

package lease

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// --- silent no-op when lease missing ----------------------------------------

func TestCheckpointSafety_NonExistentIDReturnsNilSilently(t *testing.T) {
	le := newGrantTestLessor(t)
	defer le.Stop()
	le.Promote(0)

	// Lease 999 does not exist.
	if err := le.Checkpoint(LeaseID(999), 42); err != nil {
		t.Fatalf("Checkpoint(missing) err = %v, want nil", err)
	}
	if le.Lookup(LeaseID(999)) != nil {
		t.Fatalf("Checkpoint(missing) must NOT create a lease")
	}
}

// --- remainingTTL updated ----------------------------------------------------

func TestCheckpointSafety_UpdatesRemainingTTL(t *testing.T) {
	le := newGrantTestLessor(t)
	defer le.Stop()
	le.Promote(0)

	l, err := le.Grant(LeaseID(1), 600)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if l.remainingTTL != 0 {
		t.Fatalf("pre-checkpoint remainingTTL = %d, want 0", l.remainingTTL)
	}
	if err := le.Checkpoint(LeaseID(1), 300); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if l.remainingTTL != 300 {
		t.Fatalf("post-checkpoint remainingTTL = %d, want 300", l.remainingTTL)
	}
}

// --- persistence gating by cluster version ----------------------------------

func TestCheckpointSafety_PersistsOnV37(t *testing.T) {
	le := newGrantTestLessor(t) // clusterLatest() ≥ 3.6
	defer le.Stop()
	le.Promote(0)

	if _, err := le.Grant(LeaseID(1), 600); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if err := le.Checkpoint(LeaseID(1), 300); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if got := readPersistedRemainingTTL(t, le.b, LeaseID(1)); got != 300 {
		t.Fatalf("persisted RemainingTTL = %d, want 300", got)
	}
}

func TestCheckpointSafety_DoesNotPersistOnV35WithoutFlag(t *testing.T) {
	_, be := NewTestBackend(t)
	t.Cleanup(func() { be.Close() })
	le := newLessor(zap.NewNop(), be, clusterV3_5(), LessorConfig{MinLeaseTTL: minLeaseTTL})
	defer le.Stop()
	le.Promote(0)

	if _, err := le.Grant(LeaseID(1), 600); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	// Grant itself persists with RemainingTTL=0.
	if got := readPersistedRemainingTTL(t, be, LeaseID(1)); got != 0 {
		t.Fatalf("post-Grant persisted RemainingTTL = %d, want 0", got)
	}
	if err := le.Checkpoint(LeaseID(1), 300); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	// On v3.5 without CheckpointPersist, Checkpoint must NOT touch bbolt.
	if got := readPersistedRemainingTTL(t, be, LeaseID(1)); got != 0 {
		t.Fatalf("post-Checkpoint persisted RemainingTTL = %d, want 0 (no persist on v3.5)", got)
	}
}

func TestCheckpointSafety_PersistsOnV35IfCheckpointPersistFlag(t *testing.T) {
	_, be := NewTestBackend(t)
	t.Cleanup(func() { be.Close() })
	le := newLessor(zap.NewNop(), be, clusterV3_5(), LessorConfig{
		MinLeaseTTL:       minLeaseTTL,
		CheckpointPersist: true,
	})
	defer le.Stop()
	le.Promote(0)

	if _, err := le.Grant(LeaseID(1), 600); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if err := le.Checkpoint(LeaseID(1), 300); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if got := readPersistedRemainingTTL(t, be, LeaseID(1)); got != 300 {
		t.Fatalf("persisted RemainingTTL = %d, want 300 (CheckpointPersist on)", got)
	}
}

// --- scheduling on primary only ---------------------------------------------

func TestCheckpointSafety_PrimarySchedulesNextCheckpoint(t *testing.T) {
	le := newCheckpointTestLessor(t, true /* primary */)
	defer le.Stop()

	if _, err := le.Grant(LeaseID(1), 3600); err != nil { // long TTL > checkpointInterval
		t.Fatalf("Grant: %v", err)
	}
	before := le.leaseCheckpointHeap.Len()
	if err := le.Checkpoint(LeaseID(1), 1800); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	after := le.leaseCheckpointHeap.Len()
	if after <= before {
		t.Fatalf("primary Checkpoint must enqueue: heap %d → %d", before, after)
	}
}

func TestCheckpointSafety_NonPrimaryDoesNotSchedule(t *testing.T) {
	le := newCheckpointTestLessor(t, false /* non-primary */)
	defer le.Stop()

	// Non-primary still allows Grant; expiry will be forever.
	if _, err := le.Grant(LeaseID(1), 3600); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	before := le.leaseCheckpointHeap.Len()
	if err := le.Checkpoint(LeaseID(1), 1800); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	after := le.leaseCheckpointHeap.Len()
	if after != before {
		t.Fatalf("non-primary Checkpoint must NOT enqueue: heap %d → %d", before, after)
	}
}

// --- Lookup observes the updated remainingTTL --------------------------------

func TestCheckpointSafety_LookupReturnsUpdatedRemainingTTL(t *testing.T) {
	le := newGrantTestLessor(t)
	defer le.Stop()
	le.Promote(0)

	if _, err := le.Grant(LeaseID(1), 600); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if err := le.Checkpoint(LeaseID(1), 123); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	got := le.Lookup(LeaseID(1))
	if got == nil {
		t.Fatalf("Lookup missing after Checkpoint")
	}
	if got.remainingTTL != 123 {
		t.Fatalf("Lookup().remainingTTL = %d, want 123", got.remainingTTL)
	}
}

// --- concurrency: many Checkpoints same lease — converges, no panic ---------
//
// Checkpoint is normally serialized by Raft apply, but the lock contract
// must still survive concurrent invocation (e.g. tests, future apply
// parallelism). The post-condition: no panic, no corruption, the lease
// still exists, and its remainingTTL is one of the proposed values.

func TestCheckpointSafety_ConcurrentSameLeaseConverges(t *testing.T) {
	le := newGrantTestLessor(t)
	defer le.Stop()
	le.Promote(0)

	if _, err := le.Grant(LeaseID(1), 600); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	const N = 50
	var wg sync.WaitGroup
	var errs atomic.Int32
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(v int64) {
			defer wg.Done()
			if err := le.Checkpoint(LeaseID(1), v); err != nil {
				errs.Add(1)
			}
		}(int64(i + 1))
	}
	wg.Wait()

	if errs.Load() != 0 {
		t.Fatalf("Checkpoint returned non-nil error in %d/%d calls", errs.Load(), N)
	}
	l := le.Lookup(LeaseID(1))
	if l == nil {
		t.Fatalf("lease vanished after concurrent Checkpoint")
	}
	if l.remainingTTL < 1 || l.remainingTTL > N {
		t.Fatalf("final remainingTTL = %d, want one of [1..%d]", l.remainingTTL, N)
	}
}

// --- concurrency: Checkpoint + Lookup must not race -------------------------
//
// Run under -race. Locks down the invariant that Checkpoint's writes to
// l.remainingTTL stay synchronized vs Lookup readers.

func TestCheckpointSafety_ConcurrentCheckpointAndLookup(t *testing.T) {
	le := newGrantTestLessor(t)
	defer le.Stop()
	le.Promote(0)

	if _, err := le.Grant(LeaseID(1), 600); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// writer: repeated checkpoints (simulating apply path)
	wg.Add(1)
	go func() {
		defer wg.Done()
		v := int64(1)
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = le.Checkpoint(LeaseID(1), v)
			v++
		}
	}()

	// readers: many concurrent lookups
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = le.Lookup(LeaseID(1))
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()

	// If we got here without -race tripping, we're good.
	if le.Lookup(LeaseID(1)) == nil {
		t.Fatalf("lease vanished after concurrent Checkpoint+Lookup")
	}
}

// --- helpers -----------------------------------------------------------------

// newCheckpointTestLessor builds a lessor with a Checkpointer wired so that
// scheduleCheckpointIfNeeded actually pushes to the heap. checkpointInterval
// is small enough that any Grant ttl ≥ minLeaseTTL passes the gate.
func newCheckpointTestLessor(t *testing.T, primary bool) *lessor {
	t.Helper()
	_, be := NewTestBackend(t)
	t.Cleanup(func() { be.Close() })
	le := newLessor(zap.NewNop(), be, clusterLatest(), LessorConfig{
		MinLeaseTTL:        minLeaseTTL,
		CheckpointInterval: 1 * time.Second,
	})
	le.SetCheckpointer(func(ctx context.Context, lc *pb.LeaseCheckpointRequest) error { return nil })
	if primary {
		le.Promote(0)
	}
	return le
}

func readPersistedRemainingTTL(t *testing.T, be backend.Backend, id LeaseID) int64 {
	t.Helper()
	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	lpb := schema.MustUnsafeGetLease(tx, int64(id))
	if lpb == nil {
		t.Fatalf("lease %d not in backend", id)
	}
	return lpb.RemainingTTL
}
