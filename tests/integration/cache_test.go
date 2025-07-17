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

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	cache "go.etcd.io/etcd/cache/v3"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestCacheWithoutPrefixWatch(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	c, err := cache.New(client, "", cache.WithHistoryWindowSize(32))
	if err != nil {
		t.Fatalf("New(...): %v", err)
	}
	t.Cleanup(c.Close)
	if err := c.WaitReady(t.Context()); err != nil {
		t.Fatalf("cache not ready: %v", err)
	}
	testWatch(t, client.KV, c)
}

func TestWatch(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	testWatch(t, client.KV, client.Watcher)
}

func testWatch(t *testing.T, kv clientv3.KV, watcher Watcher) {
	ctx := t.Context()
	rev2PutFooA := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/a"),
			Value:          []byte("1"),
			CreateRevision: 2,
			ModRevision:    2,
			Version:        1,
		},
	}
	rev3PutFooB := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/b"),
			Value:          []byte("2"),
			CreateRevision: 3,
			ModRevision:    3,
			Version:        1,
		},
	}
	rev4DeleteFooA := &clientv3.Event{
		Type: clientv3.EventTypeDelete,
		Kv: &mvccpb.KeyValue{
			Key:         []byte("/foo/a"),
			ModRevision: 4,
		},
	}
	rev5PutFooA := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/a"),
			Value:          []byte("3"),
			CreateRevision: 5,
			ModRevision:    5,
			Version:        1,
		},
	}
	rev5DeleteFooB := &clientv3.Event{
		Type: clientv3.EventTypeDelete,
		Kv: &mvccpb.KeyValue{
			Key:         []byte("/foo/b"),
			ModRevision: 5,
		},
	}
	rev6PutFooC := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/c"),
			Value:          []byte("x"),
			CreateRevision: 6,
			ModRevision:    6,
			Version:        1,
		},
	}
	rev7PutFooBar := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/bar"),
			Value:          []byte("y"),
			CreateRevision: 7,
			ModRevision:    7,
			Version:        1,
		},
	}
	rev8PutFooBaz := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/baz"),
			Value:          []byte("z"),
			CreateRevision: 8,
			ModRevision:    8,
			Version:        1,
		},
	}
	rev9PutFooYoo := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/yoo"),
			Value:          []byte("1"),
			CreateRevision: 9,
			ModRevision:    9,
			Version:        1,
		},
	}
	rev10PutZoo := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/zoo"),
			Value:          []byte("1"),
			CreateRevision: 10,
			ModRevision:    10,
			Version:        1,
		},
	}
	rev11PutFooFuture := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/future"),
			Value:          []byte("42"),
			CreateRevision: 11,
			ModRevision:    11,
			Version:        1,
		},
	}
	rev12PutFooTx1 := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/tx1"),
			Value:          []byte("a"),
			CreateRevision: 12,
			ModRevision:    12,
			Version:        1,
		},
	}
	rev12DeleteFooFuture := &clientv3.Event{
		Type: clientv3.EventTypeDelete,
		Kv: &mvccpb.KeyValue{
			Key:         []byte("/foo/future"),
			ModRevision: 12,
		},
	}
	rev12PutFooTx2 := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/tx2"),
			Value:          []byte("b"),
			CreateRevision: 12,
			ModRevision:    12,
			Version:        1,
		},
	}

	tcs := []struct {
		name       string
		key        string
		opts       []clientv3.OpOption
		wantEvents []*clientv3.Event
	}{
		{
			name:       "Watch single key existing /foo/c",
			key:        "/foo/c",
			opts:       []clientv3.OpOption{clientv3.WithRev(2)},
			wantEvents: []*clientv3.Event{rev6PutFooC},
		},
		{
			name:       "Watch single key non‑existent /doesnotexist",
			key:        "/doesnotexist",
			opts:       []clientv3.OpOption{clientv3.WithRev(2)},
			wantEvents: nil,
		},
		{
			name:       "Watch range empty",
			key:        "",
			opts:       []clientv3.OpOption{clientv3.WithRange(""), clientv3.WithRev(2)},
			wantEvents: nil,
		},
		{
			name:       "Watch range [/foo/a, /foo/b)",
			key:        "/foo/a",
			opts:       []clientv3.OpOption{clientv3.WithRange("/foo/b"), clientv3.WithRev(2)},
			wantEvents: []*clientv3.Event{rev2PutFooA, rev4DeleteFooA, rev5PutFooA},
		},
		{
			name:       "Watch with prefix /foo/b",
			key:        "/foo/b",
			opts:       []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(2)},
			wantEvents: []*clientv3.Event{rev3PutFooB, rev5DeleteFooB, rev7PutFooBar, rev8PutFooBaz},
		},
		{
			name:       "Watch with prefix non-existent /doesnotexist",
			key:        "/doesnotexist",
			opts:       []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(2)},
			wantEvents: nil,
		},
		{
			name:       "Watch with prefix empty string",
			key:        "",
			opts:       []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(2)},
			wantEvents: []*clientv3.Event{rev2PutFooA, rev3PutFooB, rev4DeleteFooA, rev5PutFooA, rev5DeleteFooB, rev6PutFooC, rev7PutFooBar, rev8PutFooBaz, rev9PutFooYoo, rev10PutZoo, rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2},
		},
		{
			name:       "Watch from key /foo/b",
			key:        "/foo/b",
			opts:       []clientv3.OpOption{clientv3.WithFromKey(), clientv3.WithRev(2)},
			wantEvents: []*clientv3.Event{rev3PutFooB, rev5DeleteFooB, rev6PutFooC, rev7PutFooBar, rev8PutFooBaz, rev9PutFooYoo, rev10PutZoo, rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2},
		},
		{
			name:       "Watch from empty key",
			key:        "",
			opts:       []clientv3.OpOption{clientv3.WithFromKey(), clientv3.WithRev(2)},
			wantEvents: []*clientv3.Event{rev2PutFooA, rev3PutFooB, rev4DeleteFooA, rev5PutFooA, rev5DeleteFooB, rev6PutFooC, rev7PutFooBar, rev8PutFooBaz, rev9PutFooYoo, rev10PutZoo, rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2},
		},
		{
			name:       "Watch from non-existent key /doesnotexist",
			key:        "/doesnotexist",
			opts:       []clientv3.OpOption{clientv3.WithFromKey(), clientv3.WithRev(2)},
			wantEvents: []*clientv3.Event{rev2PutFooA, rev3PutFooB, rev4DeleteFooA, rev5PutFooA, rev5DeleteFooB, rev6PutFooC, rev7PutFooBar, rev8PutFooBaz, rev9PutFooYoo, rev10PutZoo, rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2},
		},
		{
			name:       "Watch from rev 4 with single key /foo/a",
			key:        "/foo/a",
			opts:       []clientv3.OpOption{clientv3.WithRev(4)},
			wantEvents: []*clientv3.Event{rev4DeleteFooA, rev5PutFooA},
		},
		{
			name:       "Watch from rev 6 with single key /foo/a",
			key:        "/foo/a",
			opts:       []clientv3.OpOption{clientv3.WithRev(6)},
			wantEvents: nil,
		},
		{
			name: "Watch from rev 5 with prefix /foo",
			key:  "/foo",
			opts: []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(5)},
			wantEvents: []*clientv3.Event{
				rev5PutFooA, rev5DeleteFooB, rev6PutFooC, rev7PutFooBar, rev8PutFooBaz, rev9PutFooYoo, rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2,
			},
		},
		{
			name: "Watch from rev 10 with prefix /foo",
			key:  "/foo",
			opts: []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(10)},
			wantEvents: []*clientv3.Event{
				rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2,
			},
		},
		{
			name: "Watch from rev 4 with range [/foo/a, /foo/c)",
			key:  "/foo/a",
			opts: []clientv3.OpOption{clientv3.WithRange("/foo/c"), clientv3.WithRev(4)},
			wantEvents: []*clientv3.Event{
				rev4DeleteFooA, rev5PutFooA, rev5DeleteFooB, rev7PutFooBar, rev8PutFooBaz,
			},
		},
		{
			name:       "Latest‑revision watcher for /foo",
			key:        "/foo",
			opts:       []clientv3.OpOption{clientv3.WithPrefix()},
			wantEvents: []*clientv3.Event{rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2},
		},
		{
			name:       "Watch from rev 11 with single key /foo/future",
			key:        "/foo",
			opts:       []clientv3.OpOption{clientv3.WithRev(11), clientv3.WithPrefix()},
			wantEvents: []*clientv3.Event{rev11PutFooFuture, rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2},
		},
		{
			name:       "Watch from rev 12 with txn prefix /foo",
			key:        "/foo",
			opts:       []clientv3.OpOption{clientv3.WithRev(12), clientv3.WithPrefix()},
			wantEvents: []*clientv3.Event{rev12PutFooTx1, rev12DeleteFooFuture, rev12PutFooTx2},
		},
	}

	t.Log("Write the first batch of events rev 2-10")
	if _, err := kv.Put(ctx, string(rev2PutFooA.Kv.Key), string(rev2PutFooA.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Put(ctx, string(rev3PutFooB.Kv.Key), string(rev3PutFooB.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Delete(ctx, string(rev4DeleteFooA.Kv.Key)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := kv.Txn(ctx).Then(clientv3.OpPut(string(rev5PutFooA.Kv.Key), string(rev5PutFooA.Kv.Value)), clientv3.OpDelete(string(rev5DeleteFooB.Kv.Key))).Commit(); err != nil {
		t.Fatalf("Txn: %v", err)
	}
	if _, err := kv.Put(ctx, string(rev6PutFooC.Kv.Key), string(rev6PutFooC.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Put(ctx, string(rev7PutFooBar.Kv.Key), string(rev7PutFooBar.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Put(ctx, string(rev8PutFooBaz.Kv.Key), string(rev8PutFooBaz.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Put(ctx, string(rev9PutFooYoo.Kv.Key), string(rev9PutFooYoo.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Put(ctx, string(rev10PutZoo.Kv.Key), string(rev10PutZoo.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	t.Log("Open watches")
	watches := make([]clientv3.WatchChan, len(tcs))
	for i, tc := range tcs {
		watches[i] = watcher.Watch(ctx, tc.key, tc.opts...)
	}
	time.Sleep(50 * time.Millisecond)

	t.Log("Write the second batch of events rev 11‑12")
	if _, err := kv.Put(ctx, string(rev11PutFooFuture.Kv.Key), string(rev11PutFooFuture.Kv.Value)); err != nil {
		t.Fatalf("Put /foo/future: %v", err)
	}
	if _, err := kv.Txn(ctx).Then(
		clientv3.OpPut(string(rev12PutFooTx1.Kv.Key), string(rev12PutFooTx1.Kv.Value)),
		clientv3.OpDelete(string(rev12DeleteFooFuture.Kv.Key)),
		clientv3.OpPut(string(rev12PutFooTx2.Kv.Key), string(rev12PutFooTx2.Kv.Value)),
	).Commit(); err != nil {
		t.Fatalf("Txn rev12: %v", err)
	}

	t.Log("Validate")
	for i, tc := range tcs {
		i, tc := i, tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			events, _ := collectAndAssertAtomicEvents(t, watches[i])
			if diff := cmp.Diff(tc.wantEvents, events); diff != "" {
				t.Errorf("unexpected events (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCacheWithPrefix(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	ctx := t.Context()

	tests := []struct {
		name           string
		key            string
		opts           []clientv3.OpOption
		expectCanceled bool
	}{
		{
			name:           "single key within prefix",
			key:            "/foo/a",
			opts:           nil,
			expectCanceled: false,
		},
		{
			name:           "single key outside prefix returns error",
			key:            "/bar/a",
			opts:           nil,
			expectCanceled: true,
		},
		{
			name:           "prefix() within cache prefix",
			key:            "/foo",
			opts:           []clientv3.OpOption{clientv3.WithPrefix()},
			expectCanceled: false,
		},
		{
			name:           "prefix() outside cache prefix returns error",
			key:            "/bar",
			opts:           []clientv3.OpOption{clientv3.WithPrefix()},
			expectCanceled: true,
		},
		{
			name:           "range within prefix",
			key:            "/foo/a",
			opts:           []clientv3.OpOption{clientv3.WithRange("/foo/b")},
			expectCanceled: false,
		},
		{
			name:           "range crosses cache prefix boundary returns error",
			key:            "/foo/a",
			opts:           []clientv3.OpOption{clientv3.WithRange("/zzz")},
			expectCanceled: true,
		},
		{
			name:           "fromKey not allowed when cache has prefix returns error",
			key:            "/foo/a",
			opts:           []clientv3.OpOption{clientv3.WithFromKey()},
			expectCanceled: true,
		},
	}

	const testPutKey = "/foo/a"

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c, err := cache.New(client, "/foo")
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			defer c.Close()
			if err := c.WaitReady(ctx); err != nil {
				t.Fatal(err)
			}

			watchCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			ch := c.Watch(watchCtx, tc.key, tc.opts...)

			if !tc.expectCanceled {
				if _, err := client.Put(ctx, testPutKey, "val"); err != nil {
					t.Fatalf("Put(%q): %v", testPutKey, err)
				}
			}

			select {
			case resp, ok := <-ch:
				if tc.expectCanceled {
					if !ok || !resp.Canceled {
						t.Fatalf("expected canceled watch, got %+v (closed=%v)", resp, !ok)
					}
					return
				}

				if !ok || resp.Canceled {
					t.Fatalf("expected active watch (not canceled), got %+v (closed=%v)", resp, !ok)
				}
				if len(resp.Events) == 0 {
					t.Fatalf("watch returned no events, expected at least the test event")
				}
				if string(resp.Events[0].Kv.Key) != testPutKey {
					t.Fatalf("got event for key %q, want %q", resp.Events[0].Kv.Key, testPutKey)
				}
			case <-watchCtx.Done():
				if tc.expectCanceled {
					t.Fatalf("watch did not cancel within timeout")
				} else {
					t.Fatalf("active watch did not deliver event within timeout")
				}
			}
		})
	}
}

func TestCacheLaggingWatcher(t *testing.T) {
	const prefix = "/test/"
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	tests := []struct {
		name                string
		window              int
		eventCount          int
		wantExactEventCount int
		wantAtMaxEventCount int
		wantClosed          bool
	}{
		{
			name:                "all event fit",
			window:              10,
			eventCount:          9,
			wantExactEventCount: 9,
			wantClosed:          false,
		},
		{
			name:                "events fill window",
			window:              10,
			eventCount:          10,
			wantExactEventCount: 10,
			wantClosed:          false,
		},
		{
			name:                "event fill pipeline",
			window:              10,
			eventCount:          11,
			wantExactEventCount: 11,
			wantClosed:          false,
		},
		{
			name:                "pipeline overflow",
			window:              10,
			eventCount:          12,
			wantAtMaxEventCount: 1, // Either 0 or 1.
			wantClosed:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := cache.New(
				client, prefix,
				cache.WithHistoryWindowSize(tt.window),
				cache.WithPerWatcherBufferSize(0),
				cache.WithResyncInterval(10*time.Millisecond),
			)
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			defer c.Close()

			if err := c.WaitReady(t.Context()); err != nil {
				t.Fatalf("cache not ready: %v", err)
			}
			ch := c.Watch(t.Context(), prefix, clientv3.WithPrefix())

			generateEvents(t, client, prefix, tt.eventCount)
			gotEvents, ok := collectAndAssertAtomicEvents(t, ch)
			closed := !ok

			if tt.wantExactEventCount != 0 && tt.wantExactEventCount != len(gotEvents) {
				t.Errorf("gotEvents=%v, wantEvents=%v", len(gotEvents), tt.wantExactEventCount)
			}
			if tt.wantAtMaxEventCount != 0 && len(gotEvents) > tt.wantAtMaxEventCount {
				t.Errorf("gotEvents=%v, wantEvents<%v", len(gotEvents), tt.wantAtMaxEventCount)
			}
			if closed != tt.wantClosed {
				t.Errorf("closed=%v, wantClosed=%v", closed, tt.wantClosed)
			}
		})
	}
}

func TestCacheWatchOldRevisionCompacted(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)
	ctx := t.Context()

	const window = 4
	c, err := cache.New(client, "",
		cache.WithHistoryWindowSize(window),
		cache.WithPerWatcherBufferSize(0),
		cache.WithResyncInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("New(...): %v", err)
	}
	defer c.Close()
	if err = c.WaitReady(ctx); err != nil {
		t.Fatalf("cache not ready: %v", err)
	}

	putResp, err := client.Put(ctx, "/old/key", "0")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	oldRev := putResp.Header.Revision

	for i := 1; i < window*2; i++ {
		if _, err = client.Put(ctx, fmt.Sprintf("/overflow/%d", i), "x"); err != nil {
			t.Fatalf("Put overflow: %v", err)
		}
	}

	watchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ch := c.Watch(watchCtx, "/old/key", clientv3.WithRev(oldRev))

	select {
	case resp, ok := <-ch:
		if !ok || !resp.Canceled {
			t.Fatalf("expected canceled watch, got %+v closed=%v", resp, ok)
		}
		if resp.Err() == nil || (!errors.Is(resp.Err(), rpctypes.ErrCompacted) &&
			!errors.Is(resp.Err(), cache.ErrUnsupportedWatch)) {
			t.Fatalf("expected ErrCompacted (or temporary ErrUnsupportedWatch), got %v", resp.Err())
		}
	case <-watchCtx.Done():
		t.Fatal("watch did not cancel with compaction error within timeout")
	}
}

func generateEvents(t *testing.T, client *clientv3.Client, prefix string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		if _, err := client.Put(t.Context(), key, fmt.Sprintf("%d", i)); err != nil {
			t.Fatalf("Put(%q): %v", key, err)
		}
	}
}

type Watcher interface {
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
}

func collectAndAssertAtomicEvents(t *testing.T, watch clientv3.WatchChan) (events []*clientv3.Event, ok bool) {
	deadline := time.After(time.Second)
	var lastRevision int64

	for {
		select {
		case resp, ok := <-watch:
			if !ok {
				return events, false
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
		case <-deadline:
			return events, true
		case <-time.After(100 * time.Millisecond):
			return events, true
		}
	}
}
