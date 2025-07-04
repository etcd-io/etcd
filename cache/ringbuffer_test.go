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

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestPeekLatestAndOldest(t *testing.T) {
	tests := []struct {
		name          string
		capacity      int
		revs          []int64
		wantLatestRev int64
		wantOldestRev int64
	}{
		{
			name:          "empty_buffer",
			capacity:      4,
			revs:          nil,
			wantLatestRev: 0,
			wantOldestRev: 0,
		},
		{
			name:          "single_element",
			capacity:      8,
			revs:          []int64{1},
			wantLatestRev: 1,
			wantOldestRev: 1,
		},
		{
			name:          "ascending_fill",
			capacity:      4,
			revs:          []int64{1, 2, 3, 4},
			wantLatestRev: 4,
			wantOldestRev: 1,
		},
		{
			name:          "overwrite_when_full",
			capacity:      3,
			revs:          []int64{5, 6, 7, 8},
			wantLatestRev: 8,
			wantOldestRev: 6,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rb := newRingBuffer(tt.capacity)
			for _, r := range tt.revs {
				rb.Append(event(r, "k"))
			}

			latestRev := rb.PeekLatest()
			oldestRev := rb.PeekOldest()

			gotLatestRev := latestRev
			gotOldestRev := oldestRev

			if tt.wantLatestRev != gotLatestRev {
				t.Fatalf("PeekLatest()=%d, want=%d", gotLatestRev, tt.wantLatestRev)
			}
			if tt.wantOldestRev != gotOldestRev {
				t.Fatalf("PeekOldest()=%d, want=%d", gotOldestRev, tt.wantOldestRev)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name             string
		capacity         int
		revs             []int64
		predicate        EntryPredicate
		wantFilteredRevs []int64
		wantLatestRev    int64
	}{
		{
			name:             "no_filter",
			capacity:         5,
			revs:             []int64{1, 2, 3},
			predicate:        AfterRev(0),
			wantFilteredRevs: []int64{1, 2, 3},
			wantLatestRev:    3,
		},
		{
			name:             "partial_match",
			capacity:         5,
			revs:             []int64{10, 11, 12, 13},
			predicate:        AfterRev(12),
			wantFilteredRevs: []int64{12, 13},
			wantLatestRev:    13,
		},
		{
			name:             "filter_when_full",
			capacity:         3,
			revs:             []int64{20, 21, 22, 23, 24},
			predicate:        AfterRev(23),
			wantFilteredRevs: []int64{23, 24},
			wantLatestRev:    24,
		},
		{
			name:             "none_match",
			capacity:         4,
			revs:             []int64{30, 31},
			predicate:        AfterRev(100),
			wantFilteredRevs: []int64{},
			wantLatestRev:    31,
		},
		{
			name:             "nil_predicate",
			capacity:         4,
			revs:             []int64{1, 2, 3},
			predicate:        nil,
			wantFilteredRevs: []int64{1, 2, 3},
			wantLatestRev:    3,
		},
		{
			name:             "empty_buffer_nil_predicate",
			capacity:         3,
			revs:             nil,
			predicate:        nil,
			wantFilteredRevs: []int64{},
			wantLatestRev:    0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rb := newRingBuffer(tt.capacity)
			for _, r := range tt.revs {
				rb.Append(event(r, "k"))
			}

			gotEvents := rb.Filter(tt.predicate)
			gotRevs := make([]int64, len(gotEvents))
			for i, ev := range gotEvents {
				gotRevs[i] = ev.Kv.ModRevision
			}

			if diff := cmp.Diff(tt.wantFilteredRevs, gotRevs); diff != "" {
				t.Fatalf("Filter() revisions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRebaseHistory(t *testing.T) {
	tests := []struct {
		name string
		revs []int64
	}{
		{
			name: "rebase_empty_buffer",
			revs: nil,
		},
		{
			name: "rebase_after_data",
			revs: []int64{7, 8, 9},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rb := newRingBuffer(4)
			for _, r := range tt.revs {
				rb.Append(event(r, "k"))
			}

			rb.RebaseHistory()

			oldestRev := rb.PeekOldest()

			latestRev := rb.PeekLatest()

			if oldestRev != 0 {
				t.Fatalf("PeekOldest()=%d, want=%d", oldestRev, 0)
			}
			if latestRev != 0 {
				t.Fatalf("PeekLatest()=%d, want=%d", latestRev, 0)
			}

			events := rb.Filter(nil)
			if len(events) != 0 {
				t.Fatalf("Filter() len(events)=%d, want=%d", len(events), 0)
			}
		})
	}
}

func event(rev int64, key string) *clientv3.Event {
	return &clientv3.Event{
		Kv: &mvccpb.KeyValue{
			Key:         []byte(key),
			ModRevision: rev,
		},
	}
}
