// Copyright 2025 The etcd Authors
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

package cache

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestRangeTracking(t *testing.T) {
	tests := []struct {
		name      string
		capacity  int
		initRev   int64
		eventRevs []int64
		wantMin   int64
		wantMax   int64
	}{
		{
			name:      "history not full",
			capacity:  2,
			initRev:   1,
			eventRevs: []int64{1},
			wantMin:   1,
			wantMax:   1,
		},
		{
			name:      "history at exact capacity",
			capacity:  2,
			initRev:   1,
			eventRevs: []int64{1, 2},
			wantMin:   1,
			wantMax:   2,
		},
		{
			name:      "history overflow with eviction",
			capacity:  2,
			initRev:   1,
			eventRevs: []int64{1, 2, 3},
			wantMin:   2,
			wantMax:   3,
		},
		{
			name:      "empty broadcast is no-op",
			capacity:  8,
			initRev:   10,
			eventRevs: []int64{},
			wantMin:   10,
			wantMax:   0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(tt.capacity, 10*time.Millisecond)
			d.Init(tt.initRev)

			if err := d.Broadcast(respWithEventRevs(tt.eventRevs...)); err != nil {
				t.Fatalf("Broadcast(%v) error: %v", tt.eventRevs, err)
			}

			if d.minRev != tt.wantMin || d.maxRev != tt.wantMax {
				t.Fatalf("range (minRev, maxRev) got=(%d,%d) want=(%d,%d)", d.minRev, d.maxRev, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestRegisterWatcher(t *testing.T) {
	d := newDemux(8, 10*time.Millisecond)
	d.Init(10)
	if err := d.Broadcast(respWithEventRevs(10, 11, 12)); err != nil {
		t.Fatalf("seed Broadcast: %v", err)
	}
	if d.minRev != 10 || d.maxRev != 12 {
		t.Fatalf("seed range got=(%d,%d) want=(10,12)", d.minRev, d.maxRev)
	}

	tests := []struct {
		name            string
		startingRev     int64
		expectActive    bool
		expectedNextRev int64
	}{
		{
			name:            "zero revision becomes active at maxRev+1",
			startingRev:     0,
			expectActive:    true,
			expectedNextRev: 13,
		},
		{
			name:            "revision below minRev goes to lagging",
			startingRev:     9,
			expectActive:    false,
			expectedNextRev: 9,
		},
		{
			name:            "revision within range goes to lagging",
			startingRev:     11,
			expectActive:    false,
			expectedNextRev: 11,
		},
		{
			name:            "revision at maxRev range goes to lagging",
			startingRev:     12,
			expectActive:    false,
			expectedNextRev: 12,
		},
		{
			name:            "revision above maxRev goes to active",
			startingRev:     13,
			expectActive:    true,
			expectedNextRev: 13,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			watcher := newWatcher(1, nil)
			d.Register(watcher, test.startingRev)

			activeRev, inActive := d.activeWatchers[watcher]
			laggingRev, inLagging := d.laggingWatchers[watcher]

			if test.expectActive {
				if !inActive {
					t.Errorf("expected watcher in active set, but not found")
				}
				if inLagging {
					t.Errorf("watcher unexpectedly found in lagging set")
				}
				if activeRev != test.expectedNextRev {
					t.Errorf("active nextRev = %d, want %d", activeRev, test.expectedNextRev)
				}
			} else {
				if !inLagging {
					t.Errorf("expected watcher in lagging set, but not found")
				}
				if inActive {
					t.Errorf("watcher unexpectedly found in active set")
				}
				if laggingRev != test.expectedNextRev {
					t.Errorf("lagging nextRev = %d, want %d", laggingRev, test.expectedNextRev)
				}
			}

			delete(d.activeWatchers, watcher)
			delete(d.laggingWatchers, watcher)
		})
	}
}

func TestBroadcastValidatesRevisions(t *testing.T) {
	tests := []struct {
		name         string
		initialRevs  []int64
		followupRevs []int64
		shouldError  bool
	}{
		{
			name:         "revisions below maxRev are rejected",
			initialRevs:  []int64{5, 6},
			followupRevs: []int64{4},
			shouldError:  true,
		},
		{
			name:         "revisions equal to maxRev are rejected",
			initialRevs:  []int64{5, 6},
			followupRevs: []int64{6},
			shouldError:  true,
		},
		{
			name:         "revisions above maxRev are accepted",
			initialRevs:  []int64{5, 6},
			followupRevs: []int64{9, 14},
			shouldError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(8, 10*time.Millisecond)
			d.Init(5)

			if err := d.Broadcast(respWithEventRevs(tt.initialRevs...)); err != nil {
				t.Fatalf("unexpected error seeding with %v: %v", tt.initialRevs, err)
			}

			err := d.Broadcast(respWithEventRevs(tt.followupRevs...))

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error for revisions %v after maxRev %d; got nil",
						tt.followupRevs, tt.initialRevs[len(tt.initialRevs)-1])
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for valid revisions %v: %v", tt.followupRevs, err)
				}
			}
		})
	}
}

func TestBroadcastBatching(t *testing.T) {
	tests := []struct {
		name      string
		input     []int64
		wantRevs  []int64
		wantSizes []int
	}{
		{
			name:      "two groups",
			input:     []int64{14, 14, 15, 15, 15},
			wantRevs:  []int64{14},
			wantSizes: []int{5},
		},
		{
			name:      "single group",
			input:     []int64{7, 7, 7},
			wantRevs:  []int64{7},
			wantSizes: []int{3},
		},
		{
			name:      "all distinct",
			input:     []int64{1, 2, 3},
			wantRevs:  []int64{1},
			wantSizes: []int{3},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(16, 10*time.Millisecond)
			w := newWatcher(len(tt.input)+1, nil)
			d.Register(w, 0)

			d.Broadcast(respWithEventRevs(tt.input...))

			gotRevs, gotSizes := readBatches(t, w, len(tt.wantRevs))

			if diff := cmp.Diff(tt.wantRevs, gotRevs); diff != "" {
				t.Fatalf("revision mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantSizes, gotSizes); diff != "" {
				t.Fatalf("batch size mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDemuxInitPurgeVsPreserve(t *testing.T) {
	type watcherCounts struct {
		active  int
		lagging int
	}
	type revisionRange struct {
		min, max int64
	}
	type historyRange struct {
		oldest, latest int64
	}
	tests := []struct {
		name        string
		setup       func(d *demux) watcherCounts
		initRev     int64
		shouldPurge bool
		wantRange   revisionRange
		wantHistory historyRange
	}{
		{
			name: "first init sets minRev and preserves empty state",
			setup: func(d *demux) watcherCounts {
				return watcherCounts{active: 0, lagging: 0}
			},
			initRev:     5,
			shouldPurge: false,
			wantRange:   revisionRange{min: 5, max: 0},
			wantHistory: historyRange{oldest: 0, latest: 0},
		},
		{
			name: "init with gap from maxRev purges watchers",
			setup: func(d *demux) watcherCounts {
				d.Init(10)
				watcher := newWatcher(1, nil)
				d.Register(watcher, 0)
				return watcherCounts{active: 1, lagging: 0}
			},
			initRev:     20,
			shouldPurge: true,
			wantRange:   revisionRange{min: 20, max: 0},
			wantHistory: historyRange{oldest: 0, latest: 0},
		},
		{
			name: "init continuation at maxRev+1 preserves everything",
			setup: func(d *demux) watcherCounts {
				d.Init(10)
				if err := d.Broadcast(respWithEventRevs(10, 11, 12)); err != nil {
					t.Fatalf("setup broadcast failed: %v", err)
				}

				activeWatcher := newWatcher(1, nil)
				laggingWatcher := newWatcher(1, nil)
				d.Register(activeWatcher, 0)
				d.Register(laggingWatcher, 11)
				return watcherCounts{active: 1, lagging: 1}
			},
			initRev:     13, // Continuation: maxRev(12) + 1
			shouldPurge: false,
			wantRange:   revisionRange{min: 10, max: 12},
			wantHistory: historyRange{oldest: 10, latest: 12},
		},
		{
			name: "init with non-continuation revision purges everything",
			setup: func(d *demux) watcherCounts {
				d.Init(10)
				if err := d.Broadcast(respWithEventRevs(10, 11, 12)); err != nil {
					t.Fatalf("setup broadcast failed: %v", err)
				}

				watcher := newWatcher(1, nil)
				d.Register(watcher, 0)
				return watcherCounts{active: 1, lagging: 0}
			},
			initRev:     20,
			shouldPurge: true,
			wantRange:   revisionRange{min: 20, max: 0},
			wantHistory: historyRange{oldest: 0, latest: 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			demux := newDemux(8, 10*time.Millisecond)
			expectedWatchers := test.setup(demux)

			demux.Init(test.initRev)

			demux.mu.RLock()
			actualRange := revisionRange{min: demux.minRev, max: demux.maxRev}
			actualHistory := historyRange{
				oldest: demux.history.PeekOldest(),
				latest: demux.history.PeekLatest(),
			}
			actualWatchers := watcherCounts{
				active:  len(demux.activeWatchers),
				lagging: len(demux.laggingWatchers),
			}
			demux.mu.RUnlock()

			if actualRange != test.wantRange {
				t.Errorf("revision range: got (min=%d, max=%d), want (min=%d, max=%d)",
					actualRange.min, actualRange.max, test.wantRange.min, test.wantRange.max)
			}

			if actualHistory != test.wantHistory {
				t.Errorf("history range: got (oldest=%d, latest=%d), want (oldest=%d, latest=%d)",
					actualHistory.oldest, actualHistory.latest, test.wantHistory.oldest, test.wantHistory.latest)
			}

			if test.shouldPurge {
				if actualWatchers.active != 0 || actualWatchers.lagging != 0 {
					t.Errorf("expected purge to clear all watchers, but found active=%d, lagging=%d",
						actualWatchers.active, actualWatchers.lagging)
				}
			} else {
				if actualWatchers != expectedWatchers {
					t.Errorf("watchers should be preserved: got (active=%d, lagging=%d), want (active=%d, lagging=%d)",
						actualWatchers.active, actualWatchers.lagging, expectedWatchers.active, expectedWatchers.lagging)
				}
			}
		})
	}
}

func TestRangeAfterPurge(t *testing.T) {
	tests := []struct {
		name      string
		capacity  int
		initRev   int64
		eventRevs []int64
	}{
		{
			name:      "purge clears range with events",
			capacity:  8,
			initRev:   10,
			eventRevs: []int64{10, 11, 12},
		},
		{
			name:      "purge clears range without events",
			capacity:  8,
			initRev:   10,
			eventRevs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(tt.capacity, 10*time.Millisecond)
			d.Init(tt.initRev)

			if len(tt.eventRevs) > 0 {
				if err := d.Broadcast(respWithEventRevs(tt.eventRevs...)); err != nil {
					t.Fatalf("Broadcast(%v) error: %v", tt.eventRevs, err)
				}
			}

			d.Purge()

			if d.minRev != 0 || d.maxRev != 0 {
				t.Errorf("after Purge(): range got=(%d,%d) want=(0,0)", d.minRev, d.maxRev)
			}
		})
	}
}

func TestRangeAfterCompact(t *testing.T) {
	tests := []struct {
		name       string
		capacity   int
		initRev    int64
		eventRevs  []int64
		compactRev int64
		wantMin    int64
		wantMax    int64
	}{
		{
			name:       "compact below minRev is no-op",
			capacity:   8,
			initRev:    5,
			eventRevs:  []int64{6, 7, 8},
			compactRev: 3,
			wantMin:    5,
			wantMax:    8,
		},
		{
			name:       "compact at minRev purges history",
			capacity:   8,
			initRev:    5,
			eventRevs:  []int64{6, 7, 8},
			compactRev: 6,
			wantMin:    0,
			wantMax:    0,
		},
		{
			name:       "compact above minRev purges history",
			capacity:   8,
			initRev:    5,
			eventRevs:  []int64{6, 7, 8},
			compactRev: 7,
			wantMin:    0,
			wantMax:    0,
		},
		{
			name:       "compact when no events received",
			capacity:   8,
			initRev:    10,
			eventRevs:  nil,
			compactRev: 5,
			wantMin:    10,
			wantMax:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(tt.capacity, 10*time.Millisecond)
			d.Init(tt.initRev)

			if len(tt.eventRevs) > 0 {
				if err := d.Broadcast(respWithEventRevs(tt.eventRevs...)); err != nil {
					t.Fatalf("Broadcast(%v) error: %v", tt.eventRevs, err)
				}
			}

			d.Compact(tt.compactRev)

			if d.minRev != tt.wantMin || d.maxRev != tt.wantMax {
				t.Errorf("after Compact(%d): range got=(%d,%d) want=(%d,%d)",
					tt.compactRev, d.minRev, d.maxRev, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSlowWatcherResync(t *testing.T) {
	tests := []struct {
		name             string
		input            []int64
		wantInitialRevs  []int64
		wantInitialSizes []int
		wantResyncRevs   []int64
		wantResyncSizes  []int
	}{
		{
			name:             "single event overflow",
			input:            []int64{1, 2, 3},
			wantInitialRevs:  []int64{1},
			wantInitialSizes: []int{3},
			wantResyncRevs:   []int64{},
			wantResyncSizes:  []int{},
		},
		{
			name:             "multi events batch overflow",
			input:            []int64{10, 10, 11, 12, 12},
			wantInitialRevs:  []int64{10},
			wantInitialSizes: []int{5},
			wantResyncRevs:   []int64{},
			wantResyncSizes:  []int{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(16, 10*time.Millisecond)
			w := newWatcher(1, nil)
			d.Register(w, 0)

			d.Broadcast(respWithEventRevs(tt.input...))

			gotInitRevs, gotInitSizes := readBatches(t, w, len(tt.wantInitialRevs))
			if diff := cmp.Diff(tt.wantInitialRevs, gotInitRevs); diff != "" {
				t.Fatalf("initial revs mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantInitialSizes, gotInitSizes); diff != "" {
				t.Fatalf("initial batch sizes mismatch (-want +got):\n%s", diff)
			}

			gotRevs, gotSizes := make([]int64, 0, len(tt.wantResyncRevs)), make([]int, 0, len(tt.wantResyncRevs))
			for len(gotRevs) < len(tt.wantResyncRevs) {
				d.resyncLaggingWatchers()
				revs, sizes := readBatches(t, w, 1)
				gotRevs = append(gotRevs, revs...)
				gotSizes = append(gotSizes, sizes...)
			}
			if diff := cmp.Diff(tt.wantResyncRevs, gotRevs); diff != "" {
				t.Fatalf("resync revs mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantResyncSizes, gotSizes); diff != "" {
				t.Fatalf("resync batch sizes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResyncLaggingHandlesNextRev(t *testing.T) {
	tests := []struct {
		name          string
		initRev       int64
		seedRevs      []int64
		laggingNext   int64
		wantCanceled  bool
		wantCompactAt int64
		wantActive    bool
		wantNextRev   int64
	}{
		{
			name:          "next < minRev ⇒ compact & drop",
			initRev:       20,
			seedRevs:      []int64{21, 22, 23},
			laggingNext:   10,
			wantCanceled:  true,
			wantCompactAt: 10,
			wantActive:    false,
		},
		{
			name:         "next == minRev ⇒ replay to maxRev and promote",
			initRev:      20,
			seedRevs:     []int64{21, 22, 23},
			laggingNext:  20,
			wantCanceled: false,
			wantActive:   true,
			wantNextRev:  24,
		},
		{
			name:         "minRev < next <= maxRev ⇒ replay tail and promote",
			initRev:      20,
			seedRevs:     []int64{21, 22, 23},
			laggingNext:  22,
			wantCanceled: false,
			wantActive:   true,
			wantNextRev:  24,
		},
		{
			name:         "next == maxRev+1 ⇒ nothing to replay, just promote",
			initRev:      20,
			seedRevs:     []int64{21, 22, 23},
			laggingNext:  24,
			wantCanceled: false,
			wantActive:   true,
			wantNextRev:  24,
		},
		{
			name:         "next > maxRev ⇒ ahead, promote and wait",
			initRev:      20,
			seedRevs:     []int64{21, 22, 23},
			laggingNext:  30,
			wantCanceled: false,
			wantActive:   true,
			wantNextRev:  30,
		},
		{
			name:         "next == 0 ⇒ treated as too old in resync; compact & drop",
			initRev:      20,
			seedRevs:     []int64{21, 22, 23},
			laggingNext:  0,
			wantCanceled: true,
			wantActive:   false,
			wantNextRev:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(32, 5*time.Millisecond)
			d.Init(tt.initRev)
			if len(tt.seedRevs) > 0 {
				if err := d.Broadcast(respWithEventRevs(tt.seedRevs...)); err != nil {
					t.Fatalf("seeding history failed: %v", err)
				}
			}
			minBefore, maxBefore := d.minRev, d.maxRev

			w := newWatcher(16, nil)
			d.mu.Lock()
			d.laggingWatchers[w] = tt.laggingNext
			d.mu.Unlock()

			d.resyncLaggingWatchers()

			_, stillLagging := d.laggingWatchers[w]
			activeNext, nowActive := d.activeWatchers[w]

			if tt.wantCanceled {
				if stillLagging || nowActive {
					t.Fatalf("watcher should be removed on compaction; lagging=%v active=%v", stillLagging, nowActive)
				}
				if w.cancelResp == nil || !w.cancelResp.Canceled || w.cancelResp.CompactRevision != tt.wantCompactAt {
					t.Fatalf("expected compact cancel at %d; got %+v (min,max before resync=(%d,%d))",
						tt.wantCompactAt, w.cancelResp, minBefore, maxBefore)
				}
				return
			}

			if stillLagging {
				t.Fatalf("watcher unexpectedly remains lagging (min,max before resync=(%d,%d))", minBefore, maxBefore)
			}
			if tt.wantActive != nowActive {
				t.Fatalf("active=%v; want %v", nowActive, tt.wantActive)
			}
			if tt.wantActive && activeNext != tt.wantNextRev {
				t.Fatalf("nextRev=%d; want %d (min,max before resync=(%d,%d))",
					activeNext, tt.wantNextRev, minBefore, maxBefore)
			}
			if w.cancelResp != nil {
				t.Fatalf("unexpected cancelResp: %+v", w.cancelResp)
			}
		})
	}
}

func respWithEventRevs(revs ...int64) clientv3.WatchResponse {
	events := make([]*clientv3.Event, 0, len(revs))
	for _, r := range revs {
		kv := &mvccpb.KeyValue{
			Key:         []byte("k"),
			Value:       []byte("v"),
			ModRevision: r,
		}
		events = append(events, &clientv3.Event{
			Type: clientv3.EventTypePut,
			Kv:   kv,
		})
	}
	return clientv3.WatchResponse{Events: events}
}

func readBatches(t *testing.T, w *watcher, n int) (revs []int64, sizes []int) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for len(revs) < n {
		select {
		case resp := <-w.respCh:
			if resp.Canceled {
				t.Fatalf("unexpected canceled response in test: %v", resp.CancelReason)
			}
			if len(resp.Events) == 0 {
				continue
			}
			revs = append(revs, resp.Events[0].Kv.ModRevision)
			sizes = append(sizes, len(resp.Events))
		case <-timeout:
			t.Fatalf("timed out waiting for %d batches; got %d", n, len(revs))
		}
	}
	return revs, sizes
}
