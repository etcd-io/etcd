// Copyright 2026 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Safety tests for lessor.closeRequireLeader (RS-B03-2).
//
// closeRequireLeader removes the require-leader keepalive channels from a
// multiplexed keepAlive and compacts the parallel chs/ctxs slices. The
// invariant: after compaction every surviving chs[i] must still be paired
// with ITS OWN ctxs[i].
//
// This is a BUG FIX, not a behavior-preserving refactor: the compaction reads
// the source ctx as ka.ctxs[newIdx] instead of ka.ctxs[i], so once a nil
// (closed require-leader) slot has been skipped, newIdx lags i and surviving
// channels get cross-wired to an earlier (often a closed require-leader) ctx.
// The "SurvivorKeepsOwnCtx" / "MultiSurvivorAligned" tests are EXPECTED TO
// FAIL at baseline (that failure IS the empirical proof of the bug) and to
// PASS after the one-line fix. The "RequireLeaderAtTail" / "NoRequireLeader" /
// "AllRequireLeader" tests lock invariants that hold both before and after.

package clientv3

import (
	"context"
	"testing"
)

type kaMarkerKey struct{}

func reqLeaderCtx(mark string) context.Context {
	return WithRequireLeader(context.WithValue(context.Background(), kaMarkerKey{}, mark))
}

func plainCtx(mark string) context.Context {
	return context.WithValue(context.Background(), kaMarkerKey{}, mark)
}

// buildKA constructs a multiplexed keepAlive from parallel (ctx, mark) specs,
// returning the lessor, the keepAlive, and the original channels by mark so
// tests can assert channel identity after compaction.
func buildKA(specs []context.Context) (*lessor, *keepAlive, []chan *LeaseKeepAliveResponse) {
	chs := make([]chan<- *LeaseKeepAliveResponse, len(specs))
	orig := make([]chan *LeaseKeepAliveResponse, len(specs))
	for i := range specs {
		c := make(chan *LeaseKeepAliveResponse, 1)
		orig[i] = c
		chs[i] = c
	}
	ka := &keepAlive{
		chs:   chs,
		ctxs:  specs,
		donec: make(chan struct{}),
	}
	l := &lessor{keepAlives: map[LeaseID]*keepAlive{1: ka}}
	return l, ka, orig
}

func marks(ctxs []context.Context) []string {
	out := make([]string, len(ctxs))
	for i, c := range ctxs {
		out[i], _ = c.Value(kaMarkerKey{}).(string)
	}
	return out
}

func sameChan(a chan<- *LeaseKeepAliveResponse, b chan *LeaseKeepAliveResponse) bool {
	return a == (chan<- *LeaseKeepAliveResponse)(b)
}

// --- BUG-EXPOSING (FAIL pre-fix, PASS post-fix) ------------------------------

// [req, plain, req]: only the plain entry survives and must keep ITS ctx.
func TestCloseRequireLeaderSafety_SurvivorKeepsOwnCtx(t *testing.T) {
	l, ka, orig := buildKA([]context.Context{
		reqLeaderCtx("A"), plainCtx("B"), reqLeaderCtx("C"),
	})
	l.closeRequireLeader()

	if len(ka.chs) != 1 || len(ka.ctxs) != 1 {
		t.Fatalf("after close: len(chs)=%d len(ctxs)=%d; want 1,1", len(ka.chs), len(ka.ctxs))
	}
	if got := marks(ka.ctxs)[0]; got != "B" {
		t.Errorf("surviving ctx mark = %q; want \"B\" (chs[0] cross-wired to wrong ctx)", got)
	}
	if !sameChan(ka.chs[0], orig[1]) {
		t.Errorf("surviving channel is not the plain entry's channel")
	}
}

// [req, plain, plain, req]: two survivors, both must keep their own ctx.
func TestCloseRequireLeaderSafety_MultiSurvivorAligned(t *testing.T) {
	l, ka, orig := buildKA([]context.Context{
		reqLeaderCtx("A"), plainCtx("B"), plainCtx("C"), reqLeaderCtx("D"),
	})
	l.closeRequireLeader()

	if got := marks(ka.ctxs); len(got) != 2 || got[0] != "B" || got[1] != "C" {
		t.Errorf("surviving ctx marks = %v; want [B C]", got)
	}
	if len(ka.chs) != 2 || !sameChan(ka.chs[0], orig[1]) || !sameChan(ka.chs[1], orig[2]) {
		t.Errorf("surviving channels not aligned to their ctxs")
	}
}

// --- INVARIANTS (PASS pre-fix AND post-fix) ----------------------------------

// require-leader entries at the tail: no nil precedes a survivor, no misalign.
func TestCloseRequireLeaderSafety_RequireLeaderAtTail(t *testing.T) {
	l, ka, orig := buildKA([]context.Context{
		plainCtx("B"), plainCtx("C"), reqLeaderCtx("A"),
	})
	l.closeRequireLeader()

	if got := marks(ka.ctxs); len(got) != 2 || got[0] != "B" || got[1] != "C" {
		t.Errorf("surviving ctx marks = %v; want [B C]", got)
	}
	if len(ka.chs) != 2 || !sameChan(ka.chs[0], orig[0]) || !sameChan(ka.chs[1], orig[1]) {
		t.Errorf("surviving channels not aligned")
	}
}

// no require-leader entry: closeRequireLeader is a no-op (reqIdxs==0).
func TestCloseRequireLeaderSafety_NoRequireLeaderIsNoop(t *testing.T) {
	l, ka, orig := buildKA([]context.Context{plainCtx("B"), plainCtx("C")})
	l.closeRequireLeader()

	if got := marks(ka.ctxs); len(got) != 2 || got[0] != "B" || got[1] != "C" {
		t.Errorf("no-op expected, got ctx marks %v", got)
	}
	if len(ka.chs) != 2 || !sameChan(ka.chs[0], orig[0]) || !sameChan(ka.chs[1], orig[1]) {
		t.Errorf("no-op expected, channels changed")
	}
}

// all require-leader: every entry removed, slices emptied.
func TestCloseRequireLeaderSafety_AllRequireLeaderClearsAll(t *testing.T) {
	l, ka, _ := buildKA([]context.Context{reqLeaderCtx("A"), reqLeaderCtx("B")})
	l.closeRequireLeader()

	if len(ka.chs) != 0 || len(ka.ctxs) != 0 {
		t.Errorf("all require-leader: want empty, got len(chs)=%d len(ctxs)=%d", len(ka.chs), len(ka.ctxs))
	}
}
