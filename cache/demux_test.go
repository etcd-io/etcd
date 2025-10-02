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
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestInit(t *testing.T) {
	type want struct {
		min         int64
		max         int64
		historyRevs []int64
	}
	tests := []struct {
		name         string
		capacity     int
		initRev      int64
		eventRevs    []int64
		shouldReinit bool
		reinitRev    int64
		want         want
	}{
		{
			name:         "first init sets only min",
			capacity:     8,
			initRev:      5,
			eventRevs:    nil,
			shouldReinit: false,
			want:         want{min: 5, max: 0, historyRevs: nil},
		},
		{
			name:         "init on empty demux with events",
			capacity:     8,
			initRev:      5,
			eventRevs:    []int64{7, 9, 13},
			shouldReinit: false,
			want:         want{min: 5, max: 13, historyRevs: []int64{7, 9, 13}},
		},
		{
			name:         "continuation at max+1 preserves range and history",
			capacity:     8,
			initRev:      10,
			eventRevs:    []int64{13, 15, 21},
			shouldReinit: true,
			reinitRev:    22,
			want:         want{min: 10, max: 21, historyRevs: []int64{13, 15, 21}},
		},
		{
			name:         "gap from max triggers purge and clears history",
			capacity:     8,
			initRev:      10,
			eventRevs:    []int64{13, 15, 21},
			shouldReinit: true,
			reinitRev:    30,
			want:         want{min: 30, max: 0, historyRevs: nil},
		},
		{
			name:         "idempotent reinit at same revision clears history",
			capacity:     8,
			initRev:      7,
			eventRevs:    []int64{8, 9, 10},
			shouldReinit: true,
			reinitRev:    7,
			want:         want{min: 7, max: 0, historyRevs: nil},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(tt.capacity, 10*time.Millisecond)

			d.Init(tt.initRev)

			if len(tt.eventRevs) > 0 {
				if err := d.Broadcast(respWithEventRevs(tt.eventRevs...)); err != nil {
					t.Fatalf("Broadcast(%v) failed: %v", tt.eventRevs, err)
				}
			}

			if tt.shouldReinit {
				d.Init(tt.reinitRev)
			}

			if d.minRev != tt.want.min || d.maxRev != tt.want.max {
				t.Fatalf("revision range: got(min=%d, max=%d), want(min=%d, max=%d)",
					d.minRev, d.maxRev, tt.want.min, tt.want.max)
			}

			var actualHistoryRevs []int64
			d.history.AscendGreaterOrEqual(0, func(rev int64, events []*clientv3.Event) bool {
				actualHistoryRevs = append(actualHistoryRevs, rev)
				return true
			})

			if diff := cmp.Diff(tt.want.historyRevs, actualHistoryRevs); diff != "" {
				t.Fatalf("history validation failed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBroadcast(t *testing.T) {
	type want struct {
		min         int64
		max         int64
		shouldError bool
	}

	tests := []struct {
		name         string
		capacity     int
		initRev      int64
		initialRevs  []int64
		followupRevs []int64
		want         want
	}{
		{
			name:        "history not full",
			capacity:    2,
			initRev:     1,
			initialRevs: []int64{2},
			want:        want{min: 1, max: 2, shouldError: false},
		},
		{
			name:        "history at exact capacity",
			capacity:    2,
			initRev:     1,
			initialRevs: []int64{2, 3},
			want:        want{min: 1, max: 3, shouldError: false},
		},
		{
			name:        "history overflow with eviction",
			capacity:    2,
			initRev:     1,
			initialRevs: []int64{2, 3, 4},
			want:        want{min: 3, max: 4, shouldError: false},
		},
		{
			name:        "history overflow not continuous",
			capacity:    2,
			initRev:     2,
			initialRevs: []int64{4, 8, 16},
			want:        want{min: 5, max: 16, shouldError: false},
		},
		{
			name:        "empty broadcast is no-op",
			capacity:    8,
			initRev:     10,
			initialRevs: []int64{},
			want:        want{min: 10, max: 0, shouldError: false},
		},
		{
			name:         "revisions below maxRev are rejected",
			capacity:     8,
			initRev:      4,
			initialRevs:  []int64{5, 6},
			followupRevs: []int64{4},
			want:         want{shouldError: true},
		},
		{
			name:         "revisions equal to maxRev are rejected",
			capacity:     8,
			initRev:      4,
			initialRevs:  []int64{5, 6},
			followupRevs: []int64{6},
			want:         want{shouldError: true},
		},
		{
			name:         "revisions above maxRev are accepted",
			capacity:     8,
			initRev:      4,
			initialRevs:  []int64{5, 6},
			followupRevs: []int64{9, 14, 17},
			want:         want{min: 4, max: 17, shouldError: false},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			d := newDemux(tt.capacity, 10*time.Millisecond)
			d.Init(tt.initRev)

			if len(tt.initialRevs) > 0 {
				if err := d.Broadcast(respWithEventRevs(tt.initialRevs...)); err != nil {
					t.Fatalf("unexpected error broadcasting initial revisions %v: %v", tt.initialRevs, err)
				}
			}

			if len(tt.followupRevs) > 0 {
				err := d.Broadcast(respWithEventRevs(tt.followupRevs...))
				if tt.want.shouldError {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
			}

			if d.minRev != tt.want.min || d.maxRev != tt.want.max {
				t.Fatalf("revision range: got(min=%d, max=%d), want(min=%d, max=%d)",
					d.minRev, d.maxRev, tt.want.min, tt.want.max)
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
			d.Init(1)
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
			d.Init(1)
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
