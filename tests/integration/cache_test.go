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
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	cache "go.etcd.io/etcd/cache/v3"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/stringutil"
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

func TestCacheWithPrefixWatch(t *testing.T) {
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

func TestCacheServerRequestProgress(t *testing.T) {
	if integration.ThroughProxy {
		t.Skip("grpc proxy currently does not support requesting progress notifications")
	}
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	ctx := t.Context()

	c, err := cache.New(client, "/foo")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	t.Cleanup(c.Close)
	if err := c.WaitReady(ctx); err != nil {
		t.Fatalf("cache.WaitReady: %v", err)
	}

	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	watchCh := c.Watch(wctx, "/foo", clientv3.WithPrefix())

	var latestRev int64
	for i := 0; i < 5; i++ {
		resp, err := client.Put(ctx, fmt.Sprintf("/bar/out-%d", i), "v")
		if err != nil {
			t.Fatalf("Put(/bar/out-%d): %v", i, err)
		}
		latestRev = resp.Header.Revision
	}

	if err := client.RequestProgress(ctx); err != nil {
		t.Fatalf("RequestProgress: %v", err)
	}

	var progressRev int64
	select {
	case resp, ok := <-watchCh:
		if !ok || resp.Canceled {
			t.Fatalf("expected active watch (not canceled), got %+v (closed=%v)", resp, !ok)
		}
		if len(resp.Events) != 0 {
			t.Fatalf("expected a progress notification (no events), got %d event(s)", len(resp.Events))
		}
		if !resp.IsProgressNotify() {
			t.Fatalf("expected IsProgressNotify()==true, got false (resp: %+v)", resp)
		}
		progressRev = resp.Header.Revision
		if progressRev < latestRev {
			t.Fatalf("progress revision %d < latest outside-prefix rev %d", progressRev, latestRev)
		}
	case <-wctx.Done():
		t.Fatalf("timed out waiting for progress notification: %v", wctx.Err())
	}
}

func TestCacheRequestProgress(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	ctx := t.Context()

	c, err := cache.New(client, "/foo")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	t.Cleanup(c.Close)
	if err := c.WaitReady(ctx); err != nil {
		t.Fatalf("cache.WaitReady: %v", err)
	}

	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	watchCh := c.Watch(wctx, "/foo", clientv3.WithPrefix())

	// Write some keys under the watched prefix so the cache advances.
	var latestRev int64
	for i := 0; i < 3; i++ {
		resp, err := client.Put(ctx, fmt.Sprintf("/foo/key-%d", i), "v")
		if err != nil {
			t.Fatalf("Put(/foo/key-%d): %v", i, err)
		}
		latestRev = resp.Header.Revision
	}

	// Drain the event responses so the watcher is caught up.
	// Events may be batched into fewer responses, so count total events.
	totalEvents := 0
	for totalEvents < 3 {
		select {
		case resp := <-watchCh:
			if resp.Canceled {
				t.Fatalf("unexpected canceled response: %v", resp.CancelReason)
			}
			totalEvents += len(resp.Events)
		case <-wctx.Done():
			t.Fatalf("timed out waiting for events, got %d/3", totalEvents)
		}
	}

	// Call RequestProgress on the cache — this should deliver a progress
	// notification to the watcher with the cache's current revision.
	if err := c.RequestProgress(ctx); err != nil {
		t.Fatalf("RequestProgress: %v", err)
	}

	select {
	case resp, ok := <-watchCh:
		if !ok {
			t.Fatalf("expected watch response, got closed")
		}
		if !resp.IsProgressNotify() {
			t.Fatalf("expected progress notify, got %d events", len(resp.Events))
		}
		if resp.Header.Revision < latestRev {
			t.Fatalf("progress revision %d < latest rev %d", resp.Header.Revision, latestRev)
		}
	case <-wctx.Done():
		t.Fatalf("timed out waiting for progress notification from RequestProgress")
	}
}

func TestCacheWithoutPrefixGet(t *testing.T) {
	if integration.ThroughProxy {
		t.Skip("grpc proxy currently does not support requesting progress notifications")
	}
	tcs := []struct {
		name                          string
		initialEvents, followupEvents []*clientv3.Event
	}{
		{"watch-early (no pre-events)", nil, TestGetEvents},
		{"watch-mid (partial pre-events)", filterEvents(TestGetEvents, revLessThan(4)), filterEvents(TestGetEvents, revGreaterEqual(4))},
		{"watch-late (all pre-events)", TestGetEvents, nil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			integration.BeforeTest(t)
			clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
			t.Cleanup(func() { clus.Terminate(t) })
			client, kv := clus.Client(0), clus.Client(0).KV

			testGet(t, kv, func() Getter {
				c, err := cache.New(client, "")
				if err != nil {
					t.Fatalf("cache.New: %v", err)
				}
				t.Cleanup(c.Close)
				if err := c.WaitReady(t.Context()); err != nil {
					t.Fatalf("cache not ready: %v", err)
				}
				return c
			}, tc.initialEvents, tc.followupEvents)
		})
	}
}

func TestGet(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })

	client := clus.Client(0)
	kv := client.KV

	testGet(t, kv, func() Getter { return kv }, TestGetEvents, nil)
}

func testGet(t *testing.T, kv clientv3.KV, getReader func() Getter, initialEvents, followupEvents []*clientv3.Event) {
	ctx := t.Context()
	t.Log("Setup")
	baseRev := applyEvents(ctx, t, kv, initialEvents)

	reader := getReader()
	if c, ok := reader.(*cache.Cache); ok {
		if err := c.WaitForRevision(ctx, baseRev); err != nil {
			t.Fatalf("cache never caught up to rev %d: %v", baseRev, err)
		}
	}

	followupRev := applyEvents(ctx, t, kv, followupEvents)
	if c, ok := reader.(*cache.Cache); ok {
		if err := c.WaitForRevision(ctx, followupRev); err != nil {
			t.Fatalf("cache never caught up to rev %d: %v", followupRev, err)
		}
	}

	if followupRev > baseRev {
		baseRev = followupRev
	}

	latestRev := baseRev

	t.Log("Validate")
	for _, tc := range getTestCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			latestRev += advanceRevision(t, kv)

			opts := tc.opts
			if tc.optsFunc != nil {
				opts = append(opts, tc.optsFunc(latestRev)...)
			}

			op := clientv3.OpGet(tc.key, opts...)
			requestedRev := op.Rev()

			resp, err := reader.Get(ctx, tc.key, opts...)
			if tc.expectErr != nil {
				if !errors.Is(err, tc.expectErr) {
					t.Fatalf("expected %v for Get %q; got %v", tc.expectErr, tc.key, err)
				}
				return
			}
			if err != nil {
				if _, ok := reader.(*cache.Cache); ok && requestedRev > 0 && requestedRev < baseRev && errors.Is(err, rpctypes.ErrCompacted) {
					t.Logf("expected ErrCompacted: requestedRev=%d < baseCompleteRev=%d", requestedRev, baseRev)
					return
				}
				t.Fatalf("Get %q failed: %v", tc.key, err)
			}
			if diff := cmp.Diff(tc.wantKVs, resp.Kvs); diff != "" {
				t.Fatalf("unexpected KVs (-want +got):\n%s", diff)
			}
			if op.IsSerializable() {
				if resp.Header.Revision < baseRev {
					t.Fatalf("revision: got %d, want >= %d", resp.Header.Revision, baseRev)
				}
			} else {
				if resp.Header.Revision != latestRev {
					t.Fatalf("revision: got %d, want == %d", resp.Header.Revision, latestRev)
				}
			}
		})
	}
}

var TestGetEvents = []*clientv3.Event{
	Rev2PutFooA, Rev3PutFooB, Rev4PutFooC, Rev5PutFooD, Rev6DeleteFooD, Rev7TxnPutFooA, Rev7TxnPutFooB, Rev8PutFooA,
}

var (
	Rev2PutFooA = &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/a"),
			Value:          []byte("a1"),
			CreateRevision: 2,
			ModRevision:    2,
			Version:        1,
		},
	}
	Rev3PutFooB = &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/b"),
			Value:          []byte("b1"),
			CreateRevision: 3,
			ModRevision:    3,
			Version:        1,
		},
	}
	Rev4PutFooC = &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/c"),
			Value:          []byte("c1"),
			CreateRevision: 4,
			ModRevision:    4,
			Version:        1,
		},
	}
	Rev5PutFooD = &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/d"),
			Value:          []byte("d1"),
			CreateRevision: 5,
			ModRevision:    5,
			Version:        1,
		},
	}
	Rev6DeleteFooD = &clientv3.Event{
		Type: clientv3.EventTypeDelete,
		Kv: &mvccpb.KeyValue{
			Key:         []byte("/foo/d"),
			ModRevision: 6,
		},
	}
	Rev7TxnPutFooA = &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/a"),
			Value:          []byte("a2"),
			CreateRevision: 2,
			ModRevision:    7,
			Version:        2,
		},
	}
	Rev7TxnPutFooB = &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/b"),
			Value:          []byte("b2"),
			CreateRevision: 3,
			ModRevision:    7,
			Version:        2,
		},
	}
	Rev8PutFooA = &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/foo/a"),
			Value:          []byte("a3"),
			CreateRevision: 2,
			ModRevision:    8,
			Version:        3,
		},
	}
)

type getTestCase struct {
	name      string
	key       string
	opts      []clientv3.OpOption
	optsFunc  func(latestRev int64) []clientv3.OpOption
	wantKVs   []*mvccpb.KeyValue
	expectErr error
}

var getTestCases = []getTestCase{
	{
		name:    "single key /foo/a",
		key:     "/foo/a",
		opts:    []clientv3.OpOption{},
		wantKVs: []*mvccpb.KeyValue{Rev8PutFooA.Kv},
	},
	{
		name:    "single key /foo/a at rev=2",
		key:     "/foo/a",
		opts:    []clientv3.OpOption{clientv3.WithRev(2)},
		wantKVs: []*mvccpb.KeyValue{Rev2PutFooA.Kv},
	},
	{
		name:    "single key /foo/a  at rev=7",
		key:     "/foo/a",
		opts:    []clientv3.OpOption{clientv3.WithRev(7)},
		wantKVs: []*mvccpb.KeyValue{Rev7TxnPutFooA.Kv},
	},
	{
		name: "single key /foo/a at rev=latest",
		key:  "/foo/a",
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev)}
		},
		wantKVs: []*mvccpb.KeyValue{Rev8PutFooA.Kv},
	},
	{
		name: "single key /foo/a at rev=latest+1 (future), returns error",
		key:  "/foo/a",
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "non-existing key",
		key:     "/doesnotexist",
		opts:    []clientv3.OpOption{},
		wantKVs: nil,
	},
	{
		name:    "non-existing key at rev=4",
		key:     "/doesnotexist",
		opts:    []clientv3.OpOption{clientv3.WithRev(4)},
		wantKVs: nil,
	},
	{
		name: "non-existing key at rev=latest+1 (future), returns error",
		key:  "/doesnotexist",
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "prefix /foo",
		key:     "/foo",
		opts:    []clientv3.OpOption{clientv3.WithPrefix()},
		wantKVs: []*mvccpb.KeyValue{Rev8PutFooA.Kv, Rev7TxnPutFooB.Kv, Rev4PutFooC.Kv},
	},
	{
		name:    "prefix /foo at rev=5",
		key:     "/foo",
		opts:    []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(5)},
		wantKVs: []*mvccpb.KeyValue{Rev2PutFooA.Kv, Rev3PutFooB.Kv, Rev4PutFooC.Kv, Rev5PutFooD.Kv},
	},
	{
		name:    "prefix /foo/b at rev=4",
		key:     "/foo/b",
		opts:    []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(4)},
		wantKVs: []*mvccpb.KeyValue{Rev3PutFooB.Kv},
	},
	{
		name:    "prefix /foo/b at rev=7",
		key:     "/foo/b",
		opts:    []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(7)},
		wantKVs: []*mvccpb.KeyValue{Rev7TxnPutFooB.Kv},
	},
	{
		name: "prefix /foo at rev=latest+1 (future), returns error",
		key:  "/foo",
		opts: []clientv3.OpOption{clientv3.WithPrefix()},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "range [/foo/a, /foo/c)",
		key:     "/foo/a",
		opts:    []clientv3.OpOption{clientv3.WithRange("/foo/c")},
		wantKVs: []*mvccpb.KeyValue{Rev8PutFooA.Kv, Rev7TxnPutFooB.Kv},
	},
	{
		name:    "range [/foo/a, /foo/d) at rev=5",
		key:     "/foo/a",
		opts:    []clientv3.OpOption{clientv3.WithRange("/foo/d"), clientv3.WithRev(5)},
		wantKVs: []*mvccpb.KeyValue{Rev2PutFooA.Kv, Rev3PutFooB.Kv, Rev4PutFooC.Kv},
	},
	{
		name: "range [/foo/a, /foo/c) at rev=latest+1 (future), returns error",
		key:  "/foo/a",
		opts: []clientv3.OpOption{clientv3.WithRange("/foo/c")},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "fromKey /foo/b",
		key:     "/foo/b",
		opts:    []clientv3.OpOption{clientv3.WithFromKey()},
		wantKVs: []*mvccpb.KeyValue{Rev7TxnPutFooB.Kv, Rev4PutFooC.Kv},
	},
	{
		name:    "fromKey /foo/b at rev=7",
		key:     "/foo/b",
		opts:    []clientv3.OpOption{clientv3.WithFromKey(), clientv3.WithRev(7)},
		wantKVs: []*mvccpb.KeyValue{Rev7TxnPutFooB.Kv, Rev4PutFooC.Kv},
	},
	{
		name: "fromKey /foo/b at rev=latest+1 (future), returns error",
		key:  "/foo/b",
		opts: []clientv3.OpOption{clientv3.WithFromKey()},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "single key /foo/a serializable",
		key:     "/foo/a",
		opts:    []clientv3.OpOption{clientv3.WithSerializable()},
		wantKVs: []*mvccpb.KeyValue{Rev8PutFooA.Kv},
	},
	{
		name: "single key /foo/a serializable at rev=latest+1 (future), returns error",
		key:  "/foo/a",
		opts: []clientv3.OpOption{clientv3.WithSerializable()},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "non-existing key serializable",
		key:     "/doesnotexist",
		opts:    []clientv3.OpOption{clientv3.WithSerializable()},
		wantKVs: nil,
	},
	{
		name: "non-existing key serializable at rev=latest+1 (future), returns error",
		key:  "/doesnotexist",
		opts: []clientv3.OpOption{clientv3.WithSerializable()},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "prefix /foo serializable",
		key:     "/foo",
		opts:    []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSerializable()},
		wantKVs: []*mvccpb.KeyValue{Rev8PutFooA.Kv, Rev7TxnPutFooB.Kv, Rev4PutFooC.Kv},
	},
	{
		name: "prefix /foo serializable at rev=latest+1 (future), returns error",
		key:  "/foo",
		opts: []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSerializable()},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "range [/foo/a, /foo/c) serializable",
		key:     "/foo/a",
		opts:    []clientv3.OpOption{clientv3.WithRange("/foo/c"), clientv3.WithSerializable()},
		wantKVs: []*mvccpb.KeyValue{Rev8PutFooA.Kv, Rev7TxnPutFooB.Kv},
	},
	{
		name: "range [/foo/a, /foo/c) serializable at rev=latest+1 (future), returns error",
		key:  "/foo/a",
		opts: []clientv3.OpOption{clientv3.WithRange("/foo/c"), clientv3.WithSerializable()},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
	{
		name:    "fromKey /foo/b serializable",
		key:     "/foo/b",
		opts:    []clientv3.OpOption{clientv3.WithFromKey(), clientv3.WithSerializable()},
		wantKVs: []*mvccpb.KeyValue{Rev7TxnPutFooB.Kv, Rev4PutFooC.Kv},
	},
	{
		name: "fromKey /foo/b serializable at rev=latest+1 (future), returns error",
		key:  "/foo/b",
		opts: []clientv3.OpOption{clientv3.WithFromKey(), clientv3.WithSerializable()},
		optsFunc: func(latestRev int64) []clientv3.OpOption {
			return []clientv3.OpOption{clientv3.WithRev(latestRev + 1)}
		},
		expectErr: rpctypes.ErrFutureRev,
	},
}

func TestCacheWithPrefixGetInScope(t *testing.T) {
	if integration.ThroughProxy {
		t.Skip("grpc proxy currently does not support requesting progress notifications")
	}
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	cli := clus.Client(0)

	testWithPrefixGet(t, cli, func() Getter {
		c, err := cache.New(cli, "/foo")
		if err != nil {
			t.Fatalf("cache.New: %v", err)
		}
		t.Cleanup(c.Close)
		if err := c.WaitReady(t.Context()); err != nil {
			t.Fatalf("cache.WaitReady: %v", err)
		}
		return c
	})
}

func TestWithPrefixGet(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	cli := clus.Client(0)

	testWithPrefixGet(t, cli, func() Getter { return cli.KV })
}

func testWithPrefixGet(t *testing.T, cli *clientv3.Client, getReader func() Getter) {
	ctx := t.Context()
	seedResp, err := cli.Put(ctx, "/foo/a", "val1")
	if err != nil {
		t.Fatalf("seed put: %v", err)
	}
	seedRev := seedResp.Header.Revision

	reader := getReader()

	for i := 0; i < 5; i++ {
		_, err = cli.Put(ctx, fmt.Sprintf("/bar/x%d", i), fmt.Sprintf("%d", i))
		if err != nil {
			t.Fatalf("advance put: %v", err)
		}
	}

	resp, err := cli.Put(ctx, "/foo/a", "val2")
	if err != nil {
		t.Fatalf("second put: %v", err)
	}
	latestRev := resp.Header.Revision

	if err := cli.RequestProgress(ctx); err != nil {
		t.Fatalf("RequestProgress: %v", err)
	}

	if c, ok := reader.(*cache.Cache); ok {
		if err := c.WaitForRevision(ctx, latestRev); err != nil {
			t.Fatalf("cache didn’t observe progress to rev %d: %v", latestRev, err)
		}
	}

	expectedFooA := &mvccpb.KeyValue{
		Key:            []byte("/foo/a"),
		Value:          []byte("val2"),
		CreateRevision: seedRev,
		ModRevision:    latestRev,
		Version:        2,
	}

	expectedOldFooA := &mvccpb.KeyValue{
		Key:            []byte("/foo/a"),
		Value:          []byte("val1"),
		CreateRevision: seedRev,
		ModRevision:    seedRev,
		Version:        1,
	}

	baseRev := latestRev

	testCases := []struct {
		name     string
		key      string
		opts     []clientv3.OpOption
		optsFunc func(latestRev int64) []clientv3.OpOption
		wantKVs  []*mvccpb.KeyValue
	}{
		{
			name:    "single key within cache prefix",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name: "single key within cache prefix at latest/progress rev",
			key:  "/foo/a",
			optsFunc: func(latestRev int64) []clientv3.OpOption {
				return []clientv3.OpOption{clientv3.WithRev(latestRev)}
			},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "prefix query within cache prefix",
			key:     "/foo",
			opts:    []clientv3.OpOption{clientv3.WithPrefix()},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name: "prefix query within cache prefix at latest/progress rev",
			key:  "/foo",
			opts: []clientv3.OpOption{clientv3.WithPrefix()},
			optsFunc: func(latestRev int64) []clientv3.OpOption {
				return []clientv3.OpOption{clientv3.WithRev(latestRev)}
			},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "range query within cache prefix",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{clientv3.WithRange("/foo/b")},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name: "range query within cache prefix at latest/progress rev",
			key:  "/foo/a",
			opts: []clientv3.OpOption{clientv3.WithRange("/foo/z")},
			optsFunc: func(latestRev int64) []clientv3.OpOption {
				return []clientv3.OpOption{clientv3.WithRev(latestRev)}
			},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "single key within cache prefix serializable",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{clientv3.WithSerializable()},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "single key within cache prefix at base rev serializable",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{clientv3.WithSerializable(), clientv3.WithRev(baseRev)},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "prefix query within cache prefix serializable",
			key:     "/foo",
			opts:    []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSerializable()},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "prefix query within cache prefix at base rev serializable",
			key:     "/foo",
			opts:    []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithSerializable(), clientv3.WithRev(baseRev)},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "range query within cache prefix serializable",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{clientv3.WithRange("/foo/b"), clientv3.WithSerializable()},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "range query within cache prefix at base rev serializable",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{clientv3.WithRange("/foo/z"), clientv3.WithSerializable(), clientv3.WithRev(baseRev)},
			wantKVs: []*mvccpb.KeyValue{expectedFooA},
		},
		{
			name:    "prefix query within cache prefix at intermediate rev serializable",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{clientv3.WithRev(seedRev + 1), clientv3.WithSerializable()},
			wantKVs: []*mvccpb.KeyValue{expectedOldFooA},
		},
		{
			name:    "prefix query within cache prefix at intermediate rev",
			key:     "/foo/a",
			opts:    []clientv3.OpOption{clientv3.WithRev(seedRev + 1)},
			wantKVs: []*mvccpb.KeyValue{expectedOldFooA},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			latestRev += advanceRevision(t, cli)

			opts := tc.opts
			if tc.optsFunc != nil {
				opts = append(opts, tc.optsFunc(latestRev)...)
			}
			op := clientv3.OpGet(tc.key, opts...)
			resp, err := reader.Get(ctx, tc.key, opts...)
			if err != nil {
				t.Fatalf("Get(%q): %v", tc.key, err)
			}

			if diff := cmp.Diff(tc.wantKVs, resp.Kvs); diff != "" {
				t.Errorf("unexpected KVs (-want +got):\n%s", diff)
			}

			if op.IsSerializable() {
				if resp.Header.Revision < baseRev {
					t.Errorf("Header.Revision=%d; want >= %d", resp.Header.Revision, baseRev)
				}
			} else {
				if resp.Header.Revision != latestRev {
					t.Errorf("Header.Revision=%d; want: %d", resp.Header.Revision, latestRev)
				}
			}
		})
	}
}

func TestCacheWithPrefixGetOutOfScope(t *testing.T) {
	if integration.ThroughProxy {
		t.Skip("grpc proxy currently does not support requesting progress notifications")
	}
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	cli := clus.Client(0)
	c, err := cache.New(cli, "/foo")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer c.Close()
	ctx := t.Context()
	if err := c.WaitReady(ctx); err != nil {
		t.Fatalf("cache.WaitReady: %v", err)
	}

	cases := []struct {
		name string
		key  string
		opts []clientv3.OpOption
	}{
		{
			name: "single key outside prefix",
			key:  "/bar/a",
			opts: []clientv3.OpOption{clientv3.WithSerializable()},
		},
		{
			name: "prefix() outside cache prefix",
			key:  "/bar",
			opts: []clientv3.OpOption{clientv3.WithSerializable(), clientv3.WithPrefix()},
		},
		{
			name: "range crossing cache boundary",
			key:  "/foo/a",
			opts: []clientv3.OpOption{clientv3.WithSerializable(), clientv3.WithRange("/zzz")},
		},
		{
			name: "fromKey disallowed with cache prefix",
			key:  "/foo/a",
			opts: []clientv3.OpOption{clientv3.WithSerializable(), clientv3.WithFromKey()},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.Get(ctx, tc.key, tc.opts...)
			if !errors.Is(err, cache.ErrKeyRangeInvalid) {
				t.Fatalf("expected ErrKeyRangeInvalid; got %v", err)
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
			ctx := t.Context()
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

			if err := c.WaitReady(ctx); err != nil {
				t.Fatalf("cache not ready: %v", err)
			}
			ch := c.Watch(ctx, prefix, clientv3.WithPrefix())
			if err := c.WaitForNextResync(ctx); err != nil {
				t.Fatalf("cache not synced: %v", err)
			}

			generateEvents(t, client, prefix, tt.eventCount)
			if err := c.WaitForNextResync(ctx); err != nil {
				t.Fatalf("cache not synced: %v", err)
			}
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

func TestCacheWatchWithPrevKV(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	c, err := cache.New(client, "")
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	t.Cleanup(c.Close)
	ctx := t.Context()
	if err := c.WaitReady(ctx); err != nil {
		t.Fatalf("cache not ready: %v", err)
	}

	// Create /foo/a (rev 2)
	_, err = client.Put(ctx, "/foo/a", "v1")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Create /foo/b (rev 3)
	_, err = client.Put(ctx, "/foo/b", "b1")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := c.WaitForRevision(ctx, 3); err != nil {
		t.Fatalf("WaitForRevision: %v", err)
	}

	watchCh := c.Watch(ctx, "/foo", clientv3.WithPrefix(), clientv3.WithPrevKV())

	// Update /foo/a (rev 4) — should have PrevKv
	_, err = client.Put(ctx, "/foo/a", "v2")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Create /foo/c (rev 5) — create event, no PrevKv
	_, err = client.Put(ctx, "/foo/c", "c1")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Delete /foo/b (rev 6) — should have PrevKv
	_, err = client.Delete(ctx, "/foo/b")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var events []*clientv3.Event
	deadline := time.After(2 * time.Second)
	for len(events) < 3 {
		select {
		case resp := <-watchCh:
			events = append(events, resp.Events...)
		case <-deadline:
			t.Fatalf("timed out waiting for events (%d/3 received)", len(events))
		}
	}

	// Event 1: update /foo/a — PrevKv should have old value "v1"
	if events[0].PrevKv == nil {
		t.Fatal("expected PrevKv for update on /foo/a, got nil")
	}
	if string(events[0].PrevKv.Value) != "v1" {
		t.Fatalf("PrevKv.Value for /foo/a: got %q, want %q", events[0].PrevKv.Value, "v1")
	}

	// Event 2: create /foo/c — PrevKv should be nil
	if events[1].PrevKv != nil {
		t.Fatalf("expected nil PrevKv for create on /foo/c, got %+v", events[1].PrevKv)
	}

	// Event 3: delete /foo/b — PrevKv should have old value "b1"
	if events[2].PrevKv == nil {
		t.Fatal("expected PrevKv for delete on /foo/b, got nil")
	}
	if string(events[2].PrevKv.Value) != "b1" {
		t.Fatalf("PrevKv.Value for /foo/b: got %q, want %q", events[2].PrevKv.Value, "b1")
	}
}

func TestCacheUnsupportedWatchOptions(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	c, err := cache.New(client, "", cache.WithHistoryWindowSize(1))
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer c.Close()
	if err := c.WaitReady(t.Context()); err != nil {
		t.Fatalf("cache not ready: %v", err)
	}

	unsupported := []struct {
		name string
		opt  clientv3.OpOption
	}{
		{"WithFragment", clientv3.WithFragment()},
		{"WithProgressNotify", clientv3.WithProgressNotify()},
		{"WithCreatedNotify", clientv3.WithCreatedNotify()},
		{"WithFilterPut", clientv3.WithFilterPut()},
		{"WithFilterDelete", clientv3.WithFilterDelete()},
	}

	for _, tc := range unsupported {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ch := c.Watch(t.Context(), "foo", tc.opt)

			resp, ok := <-ch
			if !ok {
				t.Fatalf("channel closed without yielding a response")
			}
			if !resp.Canceled {
				t.Errorf("expected Canceled=true, got %+v", resp)
			}
			if !strings.Contains(resp.Err().Error(), cache.ErrUnsupportedRequest.Error()) {
				t.Errorf("expected ErrUnsupportedWatch text %q, got %v",
					cache.ErrUnsupportedRequest.Error(), resp.Err())
			}
		})
	}
}

func TestCacheUnsupportedGetOptions(t *testing.T) {
	if integration.ThroughProxy {
		t.Skip("grpc proxy currently does not support requesting progress notifications")
	}
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	c, err := cache.New(client, "", cache.WithHistoryWindowSize(1))
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	defer c.Close()
	if err := c.WaitReady(t.Context()); err != nil {
		t.Fatalf("cache not ready: %v", err)
	}

	unsupported := []struct {
		name string
		opts []clientv3.OpOption
	}{
		{"WithCountOnly", []clientv3.OpOption{clientv3.WithCountOnly()}},
		{"WithLimit", []clientv3.OpOption{clientv3.WithLimit(1)}},
		{"WithSort", []clientv3.OpOption{clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)}},
		{"WithPrevKV", []clientv3.OpOption{clientv3.WithPrevKV()}},
		{"WithMinModRevision", []clientv3.OpOption{clientv3.WithMinModRev(2)}},
		{"WithMaxModRevision", []clientv3.OpOption{clientv3.WithMaxModRev(10)}},
		{"WithMinCreateRevision", []clientv3.OpOption{clientv3.WithMinCreateRev(3)}},
		{"WithMaxCreateRevision", []clientv3.OpOption{clientv3.WithMaxCreateRev(5)}},
	}

	for _, tc := range unsupported {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.Get(t.Context(), "foo", tc.opts...)
			if !errors.Is(err, cache.ErrUnsupportedRequest) {
				t.Errorf("Get with %s: expected ErrUnsupportedRequest, got %v", tc.name, err)
			}
		})
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

type Getter interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
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

func advanceRevision(t *testing.T, kv clientv3.KV) int64 {
	t.Helper()
	n := rand.Intn(3) + 3
	for i := 0; i < n; i++ {
		_, err := kv.Put(t.Context(), fmt.Sprintf("/bar/%d/%s", i, stringutil.RandString(10)), "v")
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	return int64(n)
}

func applyEvents(ctx context.Context, t *testing.T, kv clientv3.KV, evs []*clientv3.Event) int64 {
	var lastRev int64
	for _, batches := range batchEventsByRevision(evs) {
		lastRev = applyEventBatch(ctx, t, kv, batches)
	}
	return lastRev
}

func batchEventsByRevision(events []*clientv3.Event) [][]*clientv3.Event {
	var batches [][]*clientv3.Event
	if len(events) == 0 {
		return batches
	}
	start := 0
	for end := 1; end < len(events); end++ {
		if events[end].Kv.ModRevision != events[start].Kv.ModRevision {
			batches = append(batches, events[start:end])
			start = end
		}
	}
	batches = append(batches, events[start:])
	return batches
}

func applyEventBatch(ctx context.Context, t *testing.T, kv clientv3.KV, batch []*clientv3.Event) int64 {
	ops := make([]clientv3.Op, 0, len(batch))
	for _, event := range batch {
		switch event.Type {
		case clientv3.EventTypePut:
			ops = append(ops, clientv3.OpPut(string(event.Kv.Key), string(event.Kv.Value)))
		case clientv3.EventTypeDelete:
			ops = append(ops, clientv3.OpDelete(string(event.Kv.Key)))
		default:
			t.Fatalf("unsupported event type: %v", event.Type)
		}
	}
	resp, err := kv.Txn(ctx).Then(ops...).Commit()
	if err != nil {
		t.Fatalf("Txn failed: %v", err)
	}
	return resp.Header.Revision
}

func filterEvents(evs []*clientv3.Event, pred func(int64) bool) []*clientv3.Event {
	var out []*clientv3.Event
	for _, ev := range evs {
		if pred(ev.Kv.ModRevision) {
			out = append(out, ev)
		}
	}
	return out
}

func revLessThan(n int64) func(int64) bool     { return func(r int64) bool { return r < n } }
func revGreaterEqual(n int64) func(int64) bool { return func(r int64) bool { return r >= n } }
