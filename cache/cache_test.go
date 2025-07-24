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
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestCacheWatchAtomicOrderedDelivery(t *testing.T) {
	tests := []struct {
		name        string
		sentBatches [][]*clientv3.Event
		wantBatch   []*clientv3.Event
	}{
		{
			name: "single_event",
			sentBatches: [][]*clientv3.Event{
				{event(mvccpb.PUT, "/a", 5)},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/a", 5),
			},
		},
		{
			name: "same_revision_batch",
			sentBatches: [][]*clientv3.Event{
				{
					event(mvccpb.PUT, "/a", 10),
					event(mvccpb.PUT, "/b", 10),
				},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/a", 10),
				event(mvccpb.PUT, "/b", 10),
			},
		},
		{
			name: "mixed_revisions_in_single_response",
			sentBatches: [][]*clientv3.Event{
				{
					event(mvccpb.PUT, "/a", 11),
					event(mvccpb.PUT, "/b", 11),
					event(mvccpb.PUT, "/c", 12),
				},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/a", 11),
				event(mvccpb.PUT, "/b", 11),
				event(mvccpb.PUT, "/c", 12),
			},
		},
		{
			name: "mixed_event_types_same_revision",
			sentBatches: [][]*clientv3.Event{
				{
					event(mvccpb.PUT, "/x", 5),
					event(mvccpb.PUT, "/y", 6),
					event(mvccpb.DELETE, "/x", 6),
				},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/x", 5),
				event(mvccpb.PUT, "/y", 6),
				event(mvccpb.DELETE, "/x", 6),
			},
		},
		{
			name: "all_events_in_one_response",
			sentBatches: [][]*clientv3.Event{
				{
					event(mvccpb.PUT, "/a", 2),
					event(mvccpb.PUT, "/b", 2),
					event(mvccpb.PUT, "/c", 3),
					event(mvccpb.PUT, "/d", 4),
					event(mvccpb.PUT, "/e", 4),
					event(mvccpb.PUT, "/f", 5),
					event(mvccpb.PUT, "/g", 6),
					event(mvccpb.PUT, "/h", 6),
					event(mvccpb.PUT, "/i", 7),
					event(mvccpb.PUT, "/j", 7),
				},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/a", 2),
				event(mvccpb.PUT, "/b", 2),
				event(mvccpb.PUT, "/c", 3),
				event(mvccpb.PUT, "/d", 4),
				event(mvccpb.PUT, "/e", 4),
				event(mvccpb.PUT, "/f", 5),
				event(mvccpb.PUT, "/g", 6),
				event(mvccpb.PUT, "/h", 6),
				event(mvccpb.PUT, "/i", 7),
				event(mvccpb.PUT, "/j", 7),
			},
		},
		{
			name: "one_revision_group_per_response",
			sentBatches: [][]*clientv3.Event{
				{event(mvccpb.PUT, "/a", 2), event(mvccpb.PUT, "/b", 2)},
				{event(mvccpb.PUT, "/c", 3)},
				{event(mvccpb.PUT, "/d", 4), event(mvccpb.PUT, "/e", 4)},
				{event(mvccpb.PUT, "/f", 5)},
				{event(mvccpb.PUT, "/g", 6), event(mvccpb.PUT, "/h", 6)},
				{event(mvccpb.PUT, "/i", 7), event(mvccpb.PUT, "/j", 7)},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/a", 2),
				event(mvccpb.PUT, "/b", 2),
				event(mvccpb.PUT, "/c", 3),
				event(mvccpb.PUT, "/d", 4),
				event(mvccpb.PUT, "/e", 4),
				event(mvccpb.PUT, "/f", 5),
				event(mvccpb.PUT, "/g", 6),
				event(mvccpb.PUT, "/h", 6),
				event(mvccpb.PUT, "/i", 7),
				event(mvccpb.PUT, "/j", 7),
			},
		},
		{
			name: "two_revision_groups_per_response",
			sentBatches: [][]*clientv3.Event{
				{event(mvccpb.PUT, "/a", 2), event(mvccpb.PUT, "/b", 2), event(mvccpb.PUT, "/c", 3)},
				{event(mvccpb.PUT, "/d", 4), event(mvccpb.PUT, "/e", 4), event(mvccpb.PUT, "/f", 5)},
				{event(mvccpb.PUT, "/g", 6), event(mvccpb.PUT, "/h", 6)},
				{event(mvccpb.PUT, "/i", 7), event(mvccpb.PUT, "/j", 7)},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/a", 2),
				event(mvccpb.PUT, "/b", 2),
				event(mvccpb.PUT, "/c", 3),
				event(mvccpb.PUT, "/d", 4),
				event(mvccpb.PUT, "/e", 4),
				event(mvccpb.PUT, "/f", 5),
				event(mvccpb.PUT, "/g", 6),
				event(mvccpb.PUT, "/h", 6),
				event(mvccpb.PUT, "/i", 7),
				event(mvccpb.PUT, "/j", 7),
			},
		},
		{
			name: "three_revision_groups_per_response",
			sentBatches: [][]*clientv3.Event{
				{
					event(mvccpb.PUT, "/a", 2), event(mvccpb.PUT, "/b", 2),
					event(mvccpb.PUT, "/c", 3),
					event(mvccpb.PUT, "/d", 4), event(mvccpb.PUT, "/e", 4),
				},
				{
					event(mvccpb.PUT, "/f", 5),
					event(mvccpb.PUT, "/g", 6), event(mvccpb.PUT, "/h", 6),
					event(mvccpb.PUT, "/i", 7), event(mvccpb.PUT, "/j", 7),
				},
			},
			wantBatch: []*clientv3.Event{
				event(mvccpb.PUT, "/a", 2),
				event(mvccpb.PUT, "/b", 2),
				event(mvccpb.PUT, "/c", 3),
				event(mvccpb.PUT, "/d", 4),
				event(mvccpb.PUT, "/e", 4),
				event(mvccpb.PUT, "/f", 5),
				event(mvccpb.PUT, "/g", 6),
				event(mvccpb.PUT, "/h", 6),
				event(mvccpb.PUT, "/i", 7),
				event(mvccpb.PUT, "/j", 7),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := newMockWatcher(16)
			cache, err := New(mw, "")
			if err != nil {
				t.Fatalf("New cache: %v", err)
			}
			defer cache.Close()

			mw.responses <- clientv3.WatchResponse{}
			<-mw.registered

			ctxWait, cancelWait := context.WithTimeout(t.Context(), time.Second)
			if err := cache.WaitReady(ctxWait); err != nil {
				t.Fatalf("cache did not become Ready(): %v", err)
			}
			cancelWait()

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			watchCh := cache.Watch(ctx, "", clientv3.WithPrefix())

			for _, batch := range tt.sentBatches {
				mw.responses <- clientv3.WatchResponse{Events: batch}
			}
			close(mw.responses)

			got := collectAndAssertAtomicEvents(ctx, t, watchCh, len(tt.wantBatch))

			if diff := cmp.Diff(tt.wantBatch, got); diff != "" {
				t.Fatalf("event mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateWatchRange(t *testing.T) {
	type tc struct {
		name        string
		watchKey    string
		opts        []clientv3.OpOption
		cachePrefix string
		wantErr     bool
	}

	tests := []tc{
		{
			name:        "single key",
			watchKey:    "/a",
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "prefix single key",
			watchKey:    "/foo/a",
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "single key outside prefix returns error",
			watchKey:    "/z",
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "explicit range",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/b")},
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "exact prefix range",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/b")},
			cachePrefix: "/a",
			wantErr:     false,
		},
		{
			name:        "prefix subrange",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithRange("/foo/a")},
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "reverse range returns error",
			watchKey:    "/b",
			opts:        []clientv3.OpOption{clientv3.WithRange("/a")},
			cachePrefix: "",
			wantErr:     true,
		},
		{
			name:        "empty range returns error",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithRange("/foo")},
			cachePrefix: "",
			wantErr:     true,
		},
		{
			name:        "range starting below cache prefix returns error",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/foo")},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "range encompassing cache prefix returns error",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/z")},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "range crossing prefixEnd returns error",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithRange("/z")},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "empty prefix",
			watchKey:    "",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "empty prefix with cachePrefix returns error",
			watchKey:    "",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "prefix watch matches cachePrefix exactly",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "prefix watch inside cachePrefix",
			watchKey:    "/foo/bar",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "prefix starting below cachePrefix returns error",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "prefix starting above shard prefixEnd returns error",
			watchKey:    "/fop",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "fromKey open‑ended",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "fromKey starting at prefix start",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "fromKey starting below prefixEnd",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "fromKey starting above prefixEnd returns error",
			watchKey:    "/fop",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			dummyCache := &Cache{prefix: c.cachePrefix}
			op := clientv3.OpGet(c.watchKey, c.opts...)
			err := dummyCache.validateRange([]byte(c.watchKey), op.RangeBytes())
			if gotErr := err != nil; gotErr != c.wantErr {
				t.Fatalf("validateWatchRange(%q, %q, %v) err=%v, wantErr=%v",
					c.cachePrefix, c.watchKey, c.opts, err, c.wantErr)
			}
		})
	}
}

type mockWatcher struct {
	responses  chan clientv3.WatchResponse
	registered chan struct{}
}

func newMockWatcher(buf int) *mockWatcher {
	return &mockWatcher{
		responses:  make(chan clientv3.WatchResponse, buf),
		registered: make(chan struct{}),
	}
}

func (m *mockWatcher) Watch(_ context.Context, _ string, _ ...clientv3.OpOption) clientv3.WatchChan {
	close(m.registered)
	return m.responses
}

func (m *mockWatcher) RequestProgress(_ context.Context) error { return nil }

func (m *mockWatcher) Close() error { close(m.responses); return nil }

func event(eventType mvccpb.Event_EventType, key string, rev int64) *clientv3.Event {
	return &clientv3.Event{
		Type: eventType,
		Kv: &mvccpb.KeyValue{
			Key:            []byte(key),
			ModRevision:    rev,
			CreateRevision: rev,
			Version:        1,
		},
	}
}

func collectAndAssertAtomicEvents(ctx context.Context, t *testing.T, watchCh clientv3.WatchChan, wantCount int) []*clientv3.Event {
	t.Helper()
	var events []*clientv3.Event
	var lastRevision int64
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for events (%d/%d received)",
				len(events), wantCount)

		case resp, ok := <-watchCh:
			if !ok {
				return events
			}
			if len(resp.Events) != 0 && resp.Events[0].Kv.ModRevision == lastRevision {
				t.Fatalf("same revision found as in previous response: %d", lastRevision)
			}
			for _, ev := range resp.Events {
				if ev.Kv.ModRevision < lastRevision {
					t.Fatalf("revision went backwards: last %d, now %d", lastRevision, ev.Kv.ModRevision)
				}
				events = append(events, ev)
				lastRevision = ev.Kv.ModRevision
			}
			if wantCount != 0 && len(events) >= wantCount {
				return events
			}
		}
	}
}
