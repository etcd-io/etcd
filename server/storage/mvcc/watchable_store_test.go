// Copyright 2015 The etcd Authors
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

package mvcc

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

func TestWatch(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue, lease.NoLease)

	w := s.NewWatchStream()
	defer w.Close()

	w.Watch(0, testKey, nil, 0)
	if !s.(*watchableStore).synced.contains(string(testKey)) {
		// the key must have had an entry in synced
		t.Errorf("existence = false, want true")
	}
}

func TestNewWatcherCancel(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue, lease.NoLease)

	w := s.NewWatchStream()
	defer w.Close()

	wt, _ := w.Watch(0, testKey, nil, 0)
	if err := w.Cancel(wt); err != nil {
		t.Error(err)
	}

	if s.(*watchableStore).synced.contains(string(testKey)) {
		// the key shoud have been deleted
		t.Errorf("existence = true, want false")
	}
}

func TestNewWatcherCountGauge(t *testing.T) {
	expectWatchGauge := func(watchers int) {
		expected := fmt.Sprintf(`# HELP etcd_debugging_mvcc_watcher_total Total number of watchers.
# TYPE etcd_debugging_mvcc_watcher_total gauge
etcd_debugging_mvcc_watcher_total %d
`, watchers)
		err := testutil.CollectAndCompare(watcherGauge, strings.NewReader(expected), "etcd_debugging_mvcc_watcher_total")
		if err != nil {
			t.Error(err)
		}
	}

	t.Run("regular watch", func(t *testing.T) {
		b, _ := betesting.NewDefaultTmpBackend(t)
		s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
		defer cleanup(s, b)

		// watcherGauge is a package variable and its value may change depending on
		// the execution of other tests
		initialGaugeState := int(testutil.ToFloat64(watcherGauge))

		testKey := []byte("foo")
		testValue := []byte("bar")
		s.Put(testKey, testValue, lease.NoLease)

		// we expect the gauge state to still be in its initial state
		expectWatchGauge(initialGaugeState)

		w := s.NewWatchStream()
		defer w.Close()

		wt, _ := w.Watch(0, testKey, nil, 0)

		// after creating watch, the gauge state should have increased
		expectWatchGauge(initialGaugeState + 1)

		if err := w.Cancel(wt); err != nil {
			t.Error(err)
		}

		// after cancelling watch, the gauge state should have decreased
		expectWatchGauge(initialGaugeState)

		w.Cancel(wt)

		// cancelling the watch twice shouldn't decrement the counter twice
		expectWatchGauge(initialGaugeState)
	})

	t.Run("compacted watch", func(t *testing.T) {
		b, _ := betesting.NewDefaultTmpBackend(t)
		s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
		defer cleanup(s, b)

		// watcherGauge is a package variable and its value may change depending on
		// the execution of other tests
		initialGaugeState := int(testutil.ToFloat64(watcherGauge))

		testKey := []byte("foo")
		testValue := []byte("bar")

		s.Put(testKey, testValue, lease.NoLease)
		rev := s.Put(testKey, testValue, lease.NoLease)

		// compact up to the revision of the key we just put
		_, err := s.Compact(traceutil.TODO(), rev)
		if err != nil {
			t.Error(err)
		}

		// we expect the gauge state to still be in its initial state
		expectWatchGauge(initialGaugeState)

		w := s.NewWatchStream()
		defer w.Close()

		wt, _ := w.Watch(0, testKey, nil, rev-1)

		// wait for the watcher to be marked as compacted
		select {
		case resp := <-w.Chan():
			if resp.CompactRevision == 0 {
				t.Errorf("resp.Compacted = %v, want %v", resp.CompactRevision, rev)
			}
		case <-time.After(time.Second):
			t.Fatalf("failed to receive response (timeout)")
		}

		// after creating watch, the gauge state should have increased
		expectWatchGauge(initialGaugeState + 1)

		if err := w.Cancel(wt); err != nil {
			t.Error(err)
		}

		// after cancelling watch, the gauge state should have decreased
		expectWatchGauge(initialGaugeState)

		w.Cancel(wt)

		// cancelling the watch twice shouldn't decrement the counter twice
		expectWatchGauge(initialGaugeState)
	})

	t.Run("compacted watch, close/cancel race", func(t *testing.T) {
		b, _ := betesting.NewDefaultTmpBackend(t)
		s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
		defer cleanup(s, b)

		// watcherGauge is a package variable and its value may change depending on
		// the execution of other tests
		initialGaugeState := int(testutil.ToFloat64(watcherGauge))

		testKey := []byte("foo")
		testValue := []byte("bar")

		s.Put(testKey, testValue, lease.NoLease)
		rev := s.Put(testKey, testValue, lease.NoLease)

		// compact up to the revision of the key we just put
		_, err := s.Compact(traceutil.TODO(), rev)
		if err != nil {
			t.Error(err)
		}

		// we expect the gauge state to still be in its initial state
		expectWatchGauge(initialGaugeState)

		w := s.NewWatchStream()

		wt, _ := w.Watch(0, testKey, nil, rev-1)

		// wait for the watcher to be marked as compacted
		select {
		case resp := <-w.Chan():
			if resp.CompactRevision == 0 {
				t.Errorf("resp.Compacted = %v, want %v", resp.CompactRevision, rev)
			}
		case <-time.After(time.Second):
			t.Fatalf("failed to receive response (timeout)")
		}

		// after creating watch, the gauge state should have increased
		expectWatchGauge(initialGaugeState + 1)

		// now race cancelling and closing the watcher and watch stream.
		// in rare scenarios the watcher cancel function can be invoked
		// multiple times, leading to a potentially negative gauge state,
		// see: https://github.com/etcd-io/etcd/issues/19577
		wg := sync.WaitGroup{}
		wg.Add(2)

		go func() {
			w.Cancel(wt)
			wg.Done()
		}()

		go func() {
			w.Close()
			wg.Done()
		}()

		wg.Wait()

		// the gauge should be decremented to its original state
		expectWatchGauge(initialGaugeState)
	})
}

// TestCancelUnsynced tests if running CancelFunc removes watchers from unsynced.
func TestCancelUnsynced(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)

	// manually create watchableStore instead of newWatchableStore
	// because newWatchableStore automatically calls syncWatchers
	// method to sync watchers in unsynced map. We want to keep watchers
	// in unsynced to test if syncWatchers works as expected.
	s := newWatchableStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	// Put a key so that we can spawn watchers on that key.
	// (testKey in this test). This increases the rev to 1,
	// and later we can we set the watcher's startRev to 1,
	// and force watchers to be in unsynced.
	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue, lease.NoLease)

	w := s.NewWatchStream()
	defer w.Close()

	// arbitrary number for watchers
	watcherN := 100

	// create watcherN of watch ids to cancel
	watchIDs := make([]WatchID, watcherN)
	for i := 0; i < watcherN; i++ {
		// use 1 to keep watchers in unsynced
		watchIDs[i], _ = w.Watch(0, testKey, nil, 1)
	}

	for _, idx := range watchIDs {
		if err := w.Cancel(idx); err != nil {
			t.Error(err)
		}
	}

	// After running CancelFunc
	//
	// unsynced should be empty
	// because cancel removes watcher from unsynced
	if size := s.unsynced.size(); size != 0 {
		t.Errorf("unsynced size = %d, want 0", size)
	}
}

// TestSyncWatchers populates unsynced watcher map and tests syncWatchers
// method to see if it correctly sends events to channel of unsynced watchers
// and moves these watchers to synced.
func TestSyncWatchers(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := newWatchableStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue, lease.NoLease)
	w := s.NewWatchStream()
	defer w.Close()
	watcherN := 100
	for i := 0; i < watcherN; i++ {
		_, err := w.Watch(0, testKey, nil, 1)
		require.NoError(t, err)
	}

	assert.Empty(t, s.synced.watcherSetByKey(string(testKey)))
	assert.Len(t, s.unsynced.watcherSetByKey(string(testKey)), watcherN)
	s.syncWatchers([]mvccpb.Event{})
	assert.Len(t, s.synced.watcherSetByKey(string(testKey)), watcherN)
	assert.Empty(t, s.unsynced.watcherSetByKey(string(testKey)))

	require.Len(t, w.(*watchStream).ch, watcherN)
	for i := 0; i < watcherN; i++ {
		events := (<-w.(*watchStream).ch).Events
		assert.Len(t, events, 1)
		assert.Equal(t, []mvccpb.Event{
			{
				Type: mvccpb.PUT,
				Kv: &mvccpb.KeyValue{
					Key:            testKey,
					CreateRevision: 2,
					ModRevision:    2,
					Version:        1,
					Value:          testValue,
				},
			},
		}, events)
	}
}

func TestRangeEvents(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	lg := zaptest.NewLogger(t)
	s := NewStore(lg, b, &lease.FakeLessor{}, StoreConfig{})

	defer cleanup(s, b)

	foo1 := []byte("foo1")
	foo2 := []byte("foo2")
	foo3 := []byte("foo3")
	value := []byte("bar")
	s.Put(foo1, value, lease.NoLease)
	s.Put(foo2, value, lease.NoLease)
	s.Put(foo3, value, lease.NoLease)
	s.DeleteRange(foo1, foo3) // Deletes "foo1" and "foo2" generating 2 events

	expectEvents := []mvccpb.Event{
		{
			Type: mvccpb.PUT,
			Kv: &mvccpb.KeyValue{
				Key:            foo1,
				CreateRevision: 2,
				ModRevision:    2,
				Version:        1,
				Value:          value,
			},
		},
		{
			Type: mvccpb.PUT,
			Kv: &mvccpb.KeyValue{
				Key:            foo2,
				CreateRevision: 3,
				ModRevision:    3,
				Version:        1,
				Value:          value,
			},
		},
		{
			Type: mvccpb.PUT,
			Kv: &mvccpb.KeyValue{
				Key:            foo3,
				CreateRevision: 4,
				ModRevision:    4,
				Version:        1,
				Value:          value,
			},
		},
		{
			Type: mvccpb.DELETE,
			Kv: &mvccpb.KeyValue{
				Key:         foo1,
				ModRevision: 5,
			},
		},
		{
			Type: mvccpb.DELETE,
			Kv: &mvccpb.KeyValue{
				Key:         foo2,
				ModRevision: 5,
			},
		},
	}

	tcs := []struct {
		minRev       int64
		maxRev       int64
		expectEvents []mvccpb.Event
	}{
		// maxRev, top to bottom
		{minRev: 2, maxRev: 6, expectEvents: expectEvents[0:5]},
		{minRev: 2, maxRev: 5, expectEvents: expectEvents[0:3]},
		{minRev: 2, maxRev: 4, expectEvents: expectEvents[0:2]},
		{minRev: 2, maxRev: 3, expectEvents: expectEvents[0:1]},
		{minRev: 2, maxRev: 2, expectEvents: expectEvents[0:0]},

		// minRev, bottom to top
		{minRev: 2, maxRev: 6, expectEvents: expectEvents[0:5]},
		{minRev: 3, maxRev: 6, expectEvents: expectEvents[1:5]},
		{minRev: 4, maxRev: 6, expectEvents: expectEvents[2:5]},
		{minRev: 5, maxRev: 6, expectEvents: expectEvents[3:5]},
		{minRev: 6, maxRev: 6, expectEvents: expectEvents[0:0]},

		// Moving window algorithm, first increase maxRev, then increase minRev, repeat.
		{minRev: 2, maxRev: 2, expectEvents: expectEvents[0:0]},
		{minRev: 2, maxRev: 3, expectEvents: expectEvents[0:1]},
		{minRev: 2, maxRev: 4, expectEvents: expectEvents[0:2]},
		{minRev: 3, maxRev: 4, expectEvents: expectEvents[1:2]},
		{minRev: 3, maxRev: 5, expectEvents: expectEvents[1:3]},
		{minRev: 4, maxRev: 5, expectEvents: expectEvents[2:3]},
		{minRev: 4, maxRev: 6, expectEvents: expectEvents[2:5]},
		{minRev: 5, maxRev: 6, expectEvents: expectEvents[3:5]},
		{minRev: 6, maxRev: 6, expectEvents: expectEvents[5:5]},
	}
	// reuse the evs to test rangeEventsWithReuse
	var evs []mvccpb.Event
	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d rangeEvents(%d, %d)", i, tc.minRev, tc.maxRev), func(t *testing.T) {
			assert.ElementsMatch(t, tc.expectEvents, rangeEvents(lg, b, tc.minRev, tc.maxRev))
			evs = rangeEventsWithReuse(lg, b, evs, tc.minRev, tc.maxRev)
			assert.ElementsMatch(t, tc.expectEvents, evs)
		})
	}
}

// TestWatchCompacted tests a watcher that watches on a compacted revision.
func TestWatchCompacted(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	testKey := []byte("foo")
	testValue := []byte("bar")

	maxRev := 10
	compactRev := int64(5)
	for i := 0; i < maxRev; i++ {
		s.Put(testKey, testValue, lease.NoLease)
	}
	_, err := s.Compact(traceutil.TODO(), compactRev)
	if err != nil {
		t.Fatalf("failed to compact kv (%v)", err)
	}

	w := s.NewWatchStream()
	defer w.Close()

	wt, _ := w.Watch(0, testKey, nil, compactRev-1)
	select {
	case resp := <-w.Chan():
		if resp.WatchID != wt {
			t.Errorf("resp.WatchID = %x, want %x", resp.WatchID, wt)
		}
		if resp.CompactRevision == 0 {
			t.Errorf("resp.Compacted = %v, want %v", resp.CompactRevision, compactRev)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("failed to receive response (timeout)")
	}
}

func TestWatchNoEventLossOnCompact(t *testing.T) {
	oldChanBufLen, oldMaxWatchersPerSync := chanBufLen, maxWatchersPerSync

	b, _ := betesting.NewDefaultTmpBackend(t)
	lg := zaptest.NewLogger(t)
	s := New(lg, b, &lease.FakeLessor{}, StoreConfig{})

	defer func() {
		cleanup(s, b)
		chanBufLen, maxWatchersPerSync = oldChanBufLen, oldMaxWatchersPerSync
	}()

	chanBufLen, maxWatchersPerSync = 1, 4
	testKey, testValue := []byte("foo"), []byte("bar")

	maxRev := 10
	compactRev := int64(5)
	for i := 0; i < maxRev; i++ {
		s.Put(testKey, testValue, lease.NoLease)
	}
	_, err := s.Compact(traceutil.TODO(), compactRev)
	require.NoErrorf(t, err, "failed to compact kv (%v)", err)

	w := s.NewWatchStream()
	defer w.Close()

	watchers := map[WatchID]int64{
		0: 1,
		1: 1, // create unsyncd watchers with startRev < compactRev
		2: 6, // create unsyncd watchers with compactRev < startRev < currentRev
	}
	for id, startRev := range watchers {
		_, err := w.Watch(id, testKey, nil, startRev)
		require.NoError(t, err)
	}
	// fill up w.Chan() with 1 buf via 2 compacted watch response
	sImpl, ok := s.(*watchableStore)
	require.Truef(t, ok, "TestWatchNoEventLossOnCompact: needs a WatchableKV implementation")
	sImpl.syncWatchers([]mvccpb.Event{})

	for len(watchers) > 0 {
		resp := <-w.Chan()
		if resp.CompactRevision != 0 {
			require.Equal(t, resp.CompactRevision, compactRev)
			require.Contains(t, watchers, resp.WatchID)
			delete(watchers, resp.WatchID)
			continue
		}
		nextRev := watchers[resp.WatchID]
		for _, ev := range resp.Events {
			require.Equalf(t, nextRev, ev.Kv.ModRevision, "got event revision %d but want %d for watcher with watch ID %d", ev.Kv.ModRevision, nextRev, resp.WatchID)
			nextRev++
		}
		if nextRev == sImpl.rev()+1 {
			delete(watchers, resp.WatchID)
		}
	}
}

func TestWatchFutureRev(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	testKey := []byte("foo")
	testValue := []byte("bar")

	w := s.NewWatchStream()
	defer w.Close()

	wrev := int64(10)
	w.Watch(0, testKey, nil, wrev)

	for i := 0; i < 10; i++ {
		rev := s.Put(testKey, testValue, lease.NoLease)
		if rev >= wrev {
			break
		}
	}

	select {
	case resp := <-w.Chan():
		if resp.Revision != wrev {
			t.Fatalf("rev = %d, want %d", resp.Revision, wrev)
		}
		if len(resp.Events) != 1 {
			t.Fatalf("failed to get events from the response")
		}
		if resp.Events[0].Kv.ModRevision != wrev {
			t.Fatalf("kv.rev = %d, want %d", resp.Events[0].Kv.ModRevision, wrev)
		}
	case <-time.After(time.Second):
		t.Fatal("failed to receive event in 1 second.")
	}
}

func TestWatchRestore(t *testing.T) {
	test := func(delay time.Duration) func(t *testing.T) {
		return func(t *testing.T) {
			b, _ := betesting.NewDefaultTmpBackend(t)
			s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
			defer cleanup(s, b)

			testKey := []byte("foo")
			testValue := []byte("bar")
			w := s.NewWatchStream()
			defer w.Close()
			w.Watch(0, testKey, nil, 1)

			time.Sleep(delay)
			wantRev := s.Put(testKey, testValue, lease.NoLease)

			s.Restore(b)
			events := readEventsForSecond(w.Chan())
			if len(events) != 1 {
				t.Errorf("Expected only one event, got %d", len(events))
			}
			if events[0].Kv.ModRevision != wantRev {
				t.Errorf("Expected revision to match, got %d, want %d", events[0].Kv.ModRevision, wantRev)
			}
		}
	}

	t.Run("Normal", test(0))
	t.Run("RunSyncWatchLoopBeforeRestore", test(time.Millisecond*120)) // longer than default waitDuration
}

func readEventsForSecond(ws <-chan WatchResponse) (events []mvccpb.Event) {
	for {
		select {
		case resp := <-ws:
			events = append(events, resp.Events...)
		case <-time.After(time.Second):
			return events
		}
	}
}

// TestWatchRestoreSyncedWatcher tests such a case that:
//  1. watcher is created with a future revision "math.MaxInt64 - 2"
//  2. watcher with a future revision is added to "synced" watcher group
//  3. restore/overwrite storage with snapshot of a higher lasat revision
//  4. restore operation moves "synced" to "unsynced" watcher group
//  5. choose the watcher from step 1, without panic
func TestWatchRestoreSyncedWatcher(t *testing.T) {
	b1, _ := betesting.NewDefaultTmpBackend(t)
	s1 := New(zaptest.NewLogger(t), b1, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s1, b1)

	b2, _ := betesting.NewDefaultTmpBackend(t)
	s2 := New(zaptest.NewLogger(t), b2, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s2, b2)

	testKey, testValue := []byte("foo"), []byte("bar")
	rev := s1.Put(testKey, testValue, lease.NoLease)
	startRev := rev + 2

	// create a watcher with a future revision
	// add to "synced" watcher group (startRev > s.store.currentRev)
	w1 := s1.NewWatchStream()
	defer w1.Close()

	w1.Watch(0, testKey, nil, startRev)

	// make "s2" ends up with a higher last revision
	s2.Put(testKey, testValue, lease.NoLease)
	s2.Put(testKey, testValue, lease.NoLease)

	// overwrite storage with higher revisions
	if err := s1.Restore(b2); err != nil {
		t.Fatal(err)
	}

	// wait for next "syncWatchersLoop" iteration
	// and the unsynced watcher should be chosen
	time.Sleep(2 * time.Second)

	// trigger events for "startRev"
	s1.Put(testKey, testValue, lease.NoLease)

	select {
	case resp := <-w1.Chan():
		if resp.Revision != startRev {
			t.Fatalf("resp.Revision expect %d, got %d", startRev, resp.Revision)
		}
		if len(resp.Events) != 1 {
			t.Fatalf("len(resp.Events) expect 1, got %d", len(resp.Events))
		}
		if resp.Events[0].Kv.ModRevision != startRev {
			t.Fatalf("resp.Events[0].Kv.ModRevision expect %d, got %d", startRev, resp.Events[0].Kv.ModRevision)
		}
	case <-time.After(time.Second):
		t.Fatal("failed to receive event in 1 second")
	}
}

// TestWatchBatchUnsynced tests batching on unsynced watchers
func TestWatchBatchUnsynced(t *testing.T) {
	tcs := []struct {
		name                  string
		revisions             int
		watchBatchMaxRevs     int
		eventsPerRevision     int
		expectRevisionBatches [][]int64
	}{
		{
			name:              "3 revisions, 4 revs per batch, 1 events per revision",
			revisions:         12,
			watchBatchMaxRevs: 4,
			eventsPerRevision: 1,
			expectRevisionBatches: [][]int64{
				{2, 3, 4, 5},
				{6, 7, 8, 9},
				{10, 11, 12, 13},
			},
		},
		{
			name:              "3 revisions, 4 revs per batch, 3 events per revision",
			revisions:         12,
			watchBatchMaxRevs: 4,
			eventsPerRevision: 3,
			expectRevisionBatches: [][]int64{
				{2, 2, 2, 3, 3, 3, 4, 4, 4, 5, 5, 5},
				{6, 6, 6, 7, 7, 7, 8, 8, 8, 9, 9, 9},
				{10, 10, 10, 11, 11, 11, 12, 12, 12, 13, 13, 13},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := betesting.NewDefaultTmpBackend(t)
			s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
			oldMaxRevs := watchBatchMaxRevs
			defer func() {
				watchBatchMaxRevs = oldMaxRevs
				cleanup(s, b)
			}()
			watchBatchMaxRevs = tc.watchBatchMaxRevs

			v := []byte("foo")
			for i := 0; i < tc.revisions; i++ {
				txn := s.Write(traceutil.TODO())
				for j := 0; j < tc.eventsPerRevision; j++ {
					txn.Put(v, v, lease.NoLease)
				}
				txn.End()
			}

			w := s.NewWatchStream()
			defer w.Close()

			w.Watch(0, v, nil, 1)
			var revisionBatches [][]int64
			eventCount := 0
			for eventCount < tc.revisions*tc.eventsPerRevision {
				var revisions []int64
				for _, e := range (<-w.Chan()).Events {
					revisions = append(revisions, e.Kv.ModRevision)
					eventCount++
				}
				revisionBatches = append(revisionBatches, revisions)
			}
			assert.Equal(t, tc.expectRevisionBatches, revisionBatches)

			sImpl, ok := s.(*watchableStore)
			require.Truef(t, ok, "TestWatchBatchUnsynced: needs a WatchableKV implementation")

			sImpl.store.revMu.Lock()
			defer sImpl.store.revMu.Unlock()
			assert.Equal(t, 1, sImpl.synced.size())
			assert.Equal(t, 0, sImpl.unsynced.size())
		})
	}
}

func TestNewMapwatcherToEventMap(t *testing.T) {
	k0, k1, k2 := []byte("foo0"), []byte("foo1"), []byte("foo2")
	v0, v1, v2 := []byte("bar0"), []byte("bar1"), []byte("bar2")

	ws := []*watcher{{key: k0}, {key: k1}, {key: k2}}

	evs := []mvccpb.Event{
		{
			Type: mvccpb.PUT,
			Kv:   &mvccpb.KeyValue{Key: k0, Value: v0},
		},
		{
			Type: mvccpb.PUT,
			Kv:   &mvccpb.KeyValue{Key: k1, Value: v1},
		},
		{
			Type: mvccpb.PUT,
			Kv:   &mvccpb.KeyValue{Key: k2, Value: v2},
		},
	}

	tests := []struct {
		sync []*watcher
		evs  []mvccpb.Event

		wwe map[*watcher][]mvccpb.Event
	}{
		// no watcher in sync, some events should return empty wwe
		{
			nil,
			evs,
			map[*watcher][]mvccpb.Event{},
		},

		// one watcher in sync, one event that does not match the key of that
		// watcher should return empty wwe
		{
			[]*watcher{ws[2]},
			evs[:1],
			map[*watcher][]mvccpb.Event{},
		},

		// one watcher in sync, one event that matches the key of that
		// watcher should return wwe with that matching watcher
		{
			[]*watcher{ws[1]},
			evs[1:2],
			map[*watcher][]mvccpb.Event{
				ws[1]: evs[1:2],
			},
		},

		// two watchers in sync that watches two different keys, one event
		// that matches the key of only one of the watcher should return wwe
		// with the matching watcher
		{
			[]*watcher{ws[0], ws[2]},
			evs[2:],
			map[*watcher][]mvccpb.Event{
				ws[2]: evs[2:],
			},
		},

		// two watchers in sync that watches the same key, two events that
		// match the keys should return wwe with those two watchers
		{
			[]*watcher{ws[0], ws[1]},
			evs[:2],
			map[*watcher][]mvccpb.Event{
				ws[0]: evs[:1],
				ws[1]: evs[1:2],
			},
		},
	}

	for i, tt := range tests {
		wg := newWatcherGroup()
		for _, w := range tt.sync {
			wg.add(w)
		}

		gwe := newWatcherBatch(&wg, tt.evs)
		if len(gwe) != len(tt.wwe) {
			t.Errorf("#%d: len(gwe) got = %d, want = %d", i, len(gwe), len(tt.wwe))
		}
		// compare gwe and tt.wwe
		for w, eb := range gwe {
			if len(eb.evs) != len(tt.wwe[w]) {
				t.Errorf("#%d: len(eb.evs) got = %d, want = %d", i, len(eb.evs), len(tt.wwe[w]))
			}
			if !reflect.DeepEqual(eb.evs, tt.wwe[w]) {
				t.Errorf("#%d: reflect.DeepEqual events got = %v, want = true", i, false)
			}
		}
	}
}

// TestWatchVictims tests that watchable store delivers watch events
// when the watch channel is temporarily clogged with too many events.
func TestWatchVictims(t *testing.T) {
	oldChanBufLen, oldMaxWatchersPerSync := chanBufLen, maxWatchersPerSync

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})

	defer func() {
		cleanup(s, b)
		chanBufLen, maxWatchersPerSync = oldChanBufLen, oldMaxWatchersPerSync
	}()

	chanBufLen, maxWatchersPerSync = 1, 2
	numPuts := chanBufLen * 64
	testKey, testValue := []byte("foo"), []byte("bar")

	var wg sync.WaitGroup
	numWatches := maxWatchersPerSync * 128
	errc := make(chan error, numWatches)
	wg.Add(numWatches)
	for i := 0; i < numWatches; i++ {
		go func() {
			w := s.NewWatchStream()
			w.Watch(0, testKey, nil, 1)
			defer func() {
				w.Close()
				wg.Done()
			}()
			tc := time.After(10 * time.Second)
			evs, nextRev := 0, int64(2)
			for evs < numPuts {
				select {
				case <-tc:
					errc <- fmt.Errorf("time out")
					return
				case wr := <-w.Chan():
					evs += len(wr.Events)
					for _, ev := range wr.Events {
						if ev.Kv.ModRevision != nextRev {
							errc <- fmt.Errorf("expected rev=%d, got %d", nextRev, ev.Kv.ModRevision)
							return
						}
						nextRev++
					}
					time.Sleep(time.Millisecond)
				}
			}
			if evs != numPuts {
				errc <- fmt.Errorf("expected %d events, got %d", numPuts, evs)
				return
			}
			select {
			case <-w.Chan():
				errc <- fmt.Errorf("unexpected response")
			default:
			}
		}()
		time.Sleep(time.Millisecond)
	}

	var wgPut sync.WaitGroup
	wgPut.Add(numPuts)
	for i := 0; i < numPuts; i++ {
		go func() {
			defer wgPut.Done()
			s.Put(testKey, testValue, lease.NoLease)
		}()
	}
	wgPut.Wait()

	wg.Wait()
	select {
	case err := <-errc:
		t.Fatal(err)
	default:
	}
}

// TestStressWatchCancelClose tests closing a watch stream while
// canceling its watches.
func TestStressWatchCancelClose(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	testKey, testValue := []byte("foo"), []byte("bar")
	var wg sync.WaitGroup
	readyc := make(chan struct{})
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			w := s.NewWatchStream()
			ids := make([]WatchID, 10)
			for i := range ids {
				ids[i], _ = w.Watch(0, testKey, nil, 0)
			}
			<-readyc
			wg.Add(1 + len(ids)/2)
			for i := range ids[:len(ids)/2] {
				go func(n int) {
					defer wg.Done()
					w.Cancel(ids[n])
				}(i)
			}
			go func() {
				defer wg.Done()
				w.Close()
			}()
		}()
	}

	close(readyc)
	for i := 0; i < 100; i++ {
		s.Put(testKey, testValue, lease.NoLease)
	}

	wg.Wait()
}
