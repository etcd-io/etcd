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
	"fmt"
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
			rb := newRingBuffer(tt.capacity)
			for _, r := range tt.revs {
				batch, err := makeEventBatch(r, "k", 1)
				if err != nil {
					t.Fatalf("makeEventBatch(%d, k, 1) failed: %v", r, err)
				}
				rb.Append(batch)
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
		minRev           int64
		wantFilteredRevs []int64
		wantLatestRev    int64
	}{
		{
			name:             "no_filter",
			capacity:         5,
			revs:             []int64{1, 2, 3},
			minRev:           0,
			wantFilteredRevs: []int64{1, 2, 3},
			wantLatestRev:    3,
		},
		{
			name:             "partial_match",
			capacity:         5,
			revs:             []int64{10, 11, 12, 13},
			minRev:           12,
			wantFilteredRevs: []int64{12, 13},
			wantLatestRev:    13,
		},
		{
			name:             "filter_when_full",
			capacity:         3,
			revs:             []int64{20, 21, 22, 23, 24},
			minRev:           23,
			wantFilteredRevs: []int64{23, 24},
			wantLatestRev:    24,
		},
		{
			name:             "none_match",
			capacity:         4,
			revs:             []int64{30, 31},
			minRev:           100,
			wantFilteredRevs: []int64{},
			wantLatestRev:    31,
		},
		{
			name:             "empty_buffer",
			capacity:         3,
			revs:             nil,
			minRev:           0,
			wantFilteredRevs: []int64{},
			wantLatestRev:    0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rb := newRingBuffer(tt.capacity)
			for _, r := range tt.revs {
				batch, err := makeEventBatch(r, "k", 11)
				if err != nil {
					t.Fatalf("makeEventBatch(%d, k, 11) failed: %v", r, err)
				}
				rb.Append(batch)
			}

			gotBatches := rb.Filter(tt.minRev)
			gotRevs := make([]int64, len(gotBatches))
			for i, b := range gotBatches {
				gotRevs[i] = b[0].Kv.ModRevision
			}

			if diff := cmp.Diff(tt.wantFilteredRevs, gotRevs); diff != "" {
				t.Fatalf("Filter() revisions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAtomicOrdered(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		inputs   []struct {
			rev  int64
			key  string
			size int
		}
		wantRev  []int64
		wantSize []int
	}{
		{
			name:     "unfiltered",
			capacity: 5,
			inputs: []struct {
				rev  int64
				key  string
				size int
			}{
				{5, "a", 1},
				{10, "b", 3},
				{15, "c", 7},
				{20, "d", 11},
			},
			wantRev:  []int64{5, 10, 15, 20},
			wantSize: []int{1, 3, 7, 11},
		},
		{
			name:     "across_wrap",
			capacity: 3,
			inputs: []struct {
				rev  int64
				key  string
				size int
			}{
				{1, "a", 2},
				{2, "b", 1},
				{3, "c", 3},
				{4, "d", 7},
			},
			wantRev:  []int64{2, 3, 4},
			wantSize: []int{1, 3, 7},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rb := newRingBuffer(tt.capacity)
			for _, in := range tt.inputs {
				batch, err := makeEventBatch(in.rev, in.key, in.size)
				if err != nil {
					t.Fatalf("makeEventBatch(%d, k, 1) failed: %v", in.rev, err)
				}
				rb.Append(batch)
			}

			gotBatches := rb.Filter(0)
			if len(gotBatches) != len(tt.wantRev) {
				t.Fatalf("len(got) = %d, want %d", len(gotBatches), len(tt.wantRev))
			}
			for i, b := range gotBatches {
				if b[0].Kv.ModRevision != tt.wantRev[i] {
					t.Errorf("at idx %d: rev = %d, want %d", i, b[0].Kv.ModRevision, tt.wantRev[i])
				}
				if batchSize := len(b); batchSize != tt.wantSize[i] {
					t.Errorf("at rev %d: events.len = %d, want %d", b[0].Kv.ModRevision, batchSize, tt.wantSize[i])
				}
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
				batch, err := makeEventBatch(r, "k", 1)
				if err != nil {
					t.Fatalf("makeEventBatch(%d, k, 1) failed: %v", r, err)
				}
				rb.Append(batch)
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

			batches := rb.Filter(0)
			if len(batches) != 0 {
				t.Fatalf("Filter() len(events)=%d, want=%d", len(batches), 0)
			}
		})
	}
}

func makeEventBatch(rev int64, key string, batchSize int) ([]*clientv3.Event, error) {
	if batchSize < 0 {
		return nil, fmt.Errorf("invalid batchSize %d", batchSize)
	}
	events := make([]*clientv3.Event, batchSize)
	for i := range events {
		events[i] = &clientv3.Event{
			Kv: &mvccpb.KeyValue{
				Key:         []byte(fmt.Sprintf("%s-%d", key, i)),
				ModRevision: rev,
			},
		}
	}
	return events, nil
}
