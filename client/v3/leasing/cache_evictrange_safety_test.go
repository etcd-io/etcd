// Copyright 2026 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Safety tests for leaseCache.EvictRange (RS-B03-1).
//
// EvictRange must invalidate EVERY cached entry whose key falls in [key, end),
// because it is called when a range-delete RPC fails and the client can no
// longer trust any cached value in that range.
//
// This is a BUG FIX, not a behavior-preserving refactor: the current code uses
// the range-start argument `key` instead of the loop variable `k`, so it only
// ever evicts the single entry equal to the range start. Therefore the
// "EvictsAll" / "MarksAll" / "ToInfinity" tests below are EXPECTED TO FAIL at
// baseline (that failure IS the empirical proof of the bug, §阶段 6.5) and to
// PASS after the two-line fix. The "OutOfRange" / "NoMatch" tests lock
// invariants that must hold both before and after.

package leasing

import (
	"testing"
	"time"
)

func newEvictTestCache(keys ...string) *leaseCache {
	lc := &leaseCache{
		entries: make(map[string]*leaseKey),
		revokes: make(map[string]time.Time),
	}
	for _, k := range keys {
		lc.entries[k] = &leaseKey{}
	}
	return lc
}

func cachedKeys(lc *leaseCache) map[string]bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	out := make(map[string]bool, len(lc.entries))
	for k := range lc.entries {
		out[k] = true
	}
	return out
}

// --- BUG-EXPOSING (FAIL pre-fix, PASS post-fix) ------------------------------

// EvictRange("a","d") covers a,b,c (not d). All three must be removed.
func TestEvictRangeSafety_EvictsAllInRange(t *testing.T) {
	lc := newEvictTestCache("a", "b", "c", "d")
	lc.EvictRange("a", "d")

	got := cachedKeys(lc)
	for _, k := range []string{"a", "b", "c"} {
		if got[k] {
			t.Errorf("key %q still cached after EvictRange[a,d); want evicted", k)
		}
	}
	if !got["d"] {
		t.Errorf("key %q wrongly evicted; it is outside [a,d)", "d")
	}
}

// Every evicted in-range key must be marked in revokes so MayAcquire backs off.
func TestEvictRangeSafety_MarksAllInRangeRevoked(t *testing.T) {
	lc := newEvictTestCache("a", "b", "c")
	lc.EvictRange("a", "d")

	for _, k := range []string{"a", "b", "c"} {
		if lc.MayAcquire(k) {
			t.Errorf("MayAcquire(%q) = true after eviction; want false (revoke backoff)", k)
		}
	}
}

// end == "\x00" means "to end of keyspace": every cached key >= start evicts.
func TestEvictRangeSafety_PrefixToInfinityEvictsAll(t *testing.T) {
	lc := newEvictTestCache("a", "b", "m", "z")
	lc.EvictRange("a", "\x00")

	if n := len(cachedKeys(lc)); n != 0 {
		t.Errorf("EvictRange[a,∞) left %d entries; want 0", n)
	}
}

// --- INVARIANTS (PASS pre-fix AND post-fix) ----------------------------------

// Keys outside [key,end) must never be touched.
func TestEvictRangeSafety_LeavesOutOfRangeUntouched(t *testing.T) {
	lc := newEvictTestCache("a", "z")
	lc.EvictRange("a", "d") // only "a" is in range

	got := cachedKeys(lc)
	if !got["z"] {
		t.Errorf("out-of-range key %q wrongly evicted", "z")
	}
	if lc.MayAcquire("z") == false {
		t.Errorf("out-of-range key %q wrongly marked revoked", "z")
	}
}

// A range matching no cached key must be a no-op.
func TestEvictRangeSafety_NoMatchIsNoop(t *testing.T) {
	lc := newEvictTestCache("x", "y")
	lc.EvictRange("a", "d") // [a,d) matches neither x nor y

	got := cachedKeys(lc)
	if !got["x"] || !got["y"] {
		t.Errorf("no-match EvictRange evicted something: %v", got)
	}
}
