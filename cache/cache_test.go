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
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
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
			fakeClient := &clientv3.Client{
				Watcher: mw,
				KV:      newKVStub(),
			}
			cache, err := New(fakeClient, "")
			if err != nil {
				t.Fatalf("New cache: %v", err)
			}
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

func TestCacheCompactionResync(t *testing.T) {
	firstSnapshot := &clientv3.GetResponse{
		Header: &pb.ResponseHeader{Revision: 5},
		Kvs: []*mvccpb.KeyValue{
			{Key: []byte("foo"), Value: []byte("old_value"), ModRevision: 5, CreateRevision: 5, Version: 1},
			{Key: []byte("bar"), Value: []byte("old_bar"), ModRevision: 3, CreateRevision: 3, Version: 1},
		},
	}
	secondSnapshot := &clientv3.GetResponse{
		Header: &pb.ResponseHeader{Revision: 20},
		Kvs: []*mvccpb.KeyValue{
			{Key: []byte("foo"), Value: []byte("new_value"), ModRevision: 20, CreateRevision: 5, Version: 2},
			{Key: []byte("baz"), Value: []byte("new_baz"), ModRevision: 18, CreateRevision: 18, Version: 1},
		},
	}
	fakeClient := &clientv3.Client{
		Watcher: newMockWatcher(16),
		KV:      newKVStub(firstSnapshot, secondSnapshot),
	}
	cache, err := New(fakeClient, "")
	if err != nil {
		t.Fatalf("New cache: %v", err)
	}
	defer cache.Close()
	mw := fakeClient.Watcher.(*mockWatcher)

	t.Log("Phase 1: initial getWatch bootstrap")
	mw.triggerCreatedNotify()
	<-mw.registered
	if err = cache.WaitReady(t.Context()); err != nil {
		t.Fatalf("initial WaitReady: %v", err)
	}
	verifySnapshot(t, cache, []*mvccpb.KeyValue{
		{Key: []byte("bar"), Value: []byte("old_bar"), ModRevision: 3, CreateRevision: 3, Version: 1},
		{Key: []byte("foo"), Value: []byte("old_value"), ModRevision: 5, CreateRevision: 5, Version: 1},
	})

	t.Log("Phase 2: simulate compaction")
	mw.errorCompacted(10)

	waitUntil(t, time.Second, 10*time.Millisecond, func() bool { return !cache.Ready() })
	start := time.Now()

	ctxGet, cancelGet := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancelGet()
	snapshot, err := cache.Get(ctxGet, "foo", clientv3.WithSerializable())
	if err != nil {
		t.Fatalf("expected Get() to serve from cached snapshot after compaction, got %v", err)
	}
	if got := snapshot.Header.Revision; got != firstSnapshot.Header.Revision {
		t.Fatalf("expected cached revision %d after compaction, got %d", firstSnapshot.Header.Revision, got)
	}
	if string(snapshot.Kvs[0].Value) != "old_value" {
		t.Fatalf("expected cached value 'old_value' during compaction, got %q", string(snapshot.Kvs[0].Value))
	}

	t.Log("Phase 3: resync after compaction")
	mw.triggerCreatedNotify()
	if err = cache.WaitReady(t.Context()); err != nil {
		t.Fatalf("second WaitReady: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("cache was unready for %v; want:  < 1 s", elapsed)
	}

	expectSnapshotRev := int64(20)
	expectedWatchStart := secondSnapshot.Header.Revision + 1
	if gotWatchStart := mw.lastStartRev; gotWatchStart != expectedWatchStart {
		t.Errorf("Watch started at rev=%d; want %d", gotWatchStart, expectedWatchStart)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err = cache.WaitForRevision(ctx, expectSnapshotRev); err != nil {
		t.Fatalf("cache never reached rev=%d: %v", expectSnapshotRev, err)
	}

	gotSnapshot, err := cache.Get(t.Context(), "foo", clientv3.WithSerializable())
	if err != nil {
		t.Fatalf("Get after resync: %v", err)
	}
	if gotSnapshot.Header.Revision != expectSnapshotRev {
		t.Errorf("unexpected Snapshot revision: got=%d, want=%d", gotSnapshot.Header.Revision, expectSnapshotRev)
	}
	verifySnapshot(t, cache, []*mvccpb.KeyValue{
		{Key: []byte("baz"), Value: []byte("new_baz"), ModRevision: 18, CreateRevision: 18, Version: 1},
		{Key: []byte("foo"), Value: []byte("new_value"), ModRevision: 20, CreateRevision: 5, Version: 2},
	})
}

func waitUntil(t *testing.T, timeout, poll time.Duration, cond func() bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(poll)
	}
	t.Fatalf("condition not satisfied within %s", timeout)
}

type mockWatcher struct {
	responses    chan clientv3.WatchResponse
	registered   chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
	mu           sync.Mutex
	lastStartRev int64
}

func newMockWatcher(buf int) *mockWatcher {
	return &mockWatcher{
		responses:  make(chan clientv3.WatchResponse, buf),
		registered: make(chan struct{}),
	}
}

func (m *mockWatcher) Watch(ctx context.Context, _ string, opts ...clientv3.OpOption) clientv3.WatchChan {
	rev := m.extractRev(opts)
	m.recordStartRev(rev)

	m.signalRegistration()

	out := make(chan clientv3.WatchResponse)
	m.wg.Add(1)
	go m.streamResponses(ctx, out)
	return out
}

func (m *mockWatcher) RequestProgress(_ context.Context) error { return nil }

func (m *mockWatcher) Close() error {
	m.closeOnce.Do(func() { close(m.responses) })
	m.wg.Wait()
	return nil
}

func (m *mockWatcher) triggerCreatedNotify() { m.responses <- clientv3.WatchResponse{} }

func (m *mockWatcher) errorCompacted(compRev int64) {
	m.responses <- clientv3.WatchResponse{
		Canceled:        true,
		CompactRevision: compRev,
	}
}

func (m *mockWatcher) extractRev(opts []clientv3.OpOption) int64 {
	var op clientv3.Op
	for _, o := range opts {
		o(&op)
	}
	return op.Rev()
}

func (m *mockWatcher) recordStartRev(rev int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastStartRev = rev
}

func (m *mockWatcher) signalRegistration() {
	select {
	case <-m.registered:
	default:
		close(m.registered)
	}
}

func (m *mockWatcher) streamResponses(ctx context.Context, out chan<- clientv3.WatchResponse) {
	defer func() {
		close(out)
		m.wg.Done()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-m.responses:
			if !ok {
				return
			}
			out <- resp
			if resp.Canceled {
				return
			}
		}
	}
}

type kvStub struct {
	queued      []*clientv3.GetResponse
	defaultResp *clientv3.GetResponse
}

func newKVStub(resps ...*clientv3.GetResponse) *kvStub {
	queue := append([]*clientv3.GetResponse(nil), resps...)
	return &kvStub{
		queued:      queue,
		defaultResp: &clientv3.GetResponse{Header: &pb.ResponseHeader{Revision: 0}},
	}
}

func (s *kvStub) Get(ctx context.Context, key string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if len(s.queued) > 0 {
		next := s.queued[0]
		s.queued = s.queued[1:]
		return next, nil
	}
	return s.defaultResp, nil
}

func (s *kvStub) Put(ctx context.Context, key, val string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return nil, nil
}

func (s *kvStub) Delete(ctx context.Context, key string, _ ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return nil, nil
}

func (s *kvStub) Compact(ctx context.Context, rev int64, _ ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	return nil, nil
}

func (s *kvStub) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return clientv3.OpResponse{}, nil
}

func (s *kvStub) Txn(ctx context.Context) clientv3.Txn {
	return nil
}

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

func verifySnapshot(t *testing.T, cache *Cache, want []*mvccpb.KeyValue) {
	resp, err := cache.Get(t.Context(), "", clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		t.Fatalf("Get all keys: %v", err)
	}

	if diff := cmp.Diff(want, resp.Kvs); diff != "" {
		t.Fatalf("cache snapshot mismatch (-want +got):\n%s", diff)
	}
}
