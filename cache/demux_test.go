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

			d.Broadcast(eventRevs(tt.input...))

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
			d.Register(w, 0)

			d.Broadcast(eventRevs(tt.input...))

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

func eventRevs(revs ...int64) []*clientv3.Event {
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
	return events
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
