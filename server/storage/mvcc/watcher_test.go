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
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

// resetNextWatchIDForTesting resets the global watch ID counter and registry.
// This should only be used in tests.
func resetNextWatchIDForTesting() {
	nextWatchIDMutex.Lock()
	defer nextWatchIDMutex.Unlock()
	nextWatchID = 0
	assignedWatchIDs = make(map[WatchID]int)
}

// TestWatcherWatchID tests that each watcher provides unique watchID,
// and the watched event attaches the correct watchID.
func TestWatcherWatchID(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	w := s.NewWatchStream()
	defer w.Close()

	idm := make(map[WatchID]struct{})

	for i := 0; i < 10; i++ {
		id, _ := w.Watch(t.Context(), 0, []byte("foo"), nil, 0)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		s.Put([]byte("foo"), []byte("bar"), lease.NoLease)

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Error(err)
		}
	}

	s.Put([]byte("foo2"), []byte("bar"), lease.NoLease)

	// unsynced watchers
	for i := 10; i < 20; i++ {
		id, _ := w.Watch(t.Context(), 0, []byte("foo2"), nil, 1)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Error(err)
		}
	}
}

func TestWatcherRequestsCustomID(t *testing.T) {
	resetNextWatchIDForTesting()
	t.Cleanup(resetNextWatchIDForTesting)

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	w := s.NewWatchStream()
	defer w.Close()

	// - Request specifically ID #1
	// - Try to duplicate it, get an error
	// - Make sure the auto-assignment skips over things we manually assigned

	tt := []struct {
		givenID     WatchID
		expectedID  WatchID
		expectedErr error
	}{
		{1, 1, nil},
		{1, 0, ErrWatcherDuplicateID},
		{0, 0, nil},
		{0, 2, nil},
	}

	for i, tcase := range tt {
		id, err := w.Watch(t.Context(), tcase.givenID, []byte("foo"), nil, 0)
		if tcase.expectedErr != nil || err != nil {
			if !errors.Is(err, tcase.expectedErr) {
				t.Errorf("expected get error %q in test case %q, got %q", tcase.expectedErr, i, err)
			}
		} else if tcase.expectedID != id {
			t.Errorf("expected to create ID %d, got %d in test case %d", tcase.expectedID, id, i)
		}
	}
}

// TestWatcherWatchPrefix tests if Watch operation correctly watches
// and returns events with matching prefixes.
func TestWatcherWatchPrefix(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	w := s.NewWatchStream()
	defer w.Close()

	idm := make(map[WatchID]struct{})

	val := []byte("bar")
	keyWatch, keyEnd, keyPut := []byte("foo"), []byte("fop"), []byte("foobar")

	for i := 0; i < 10; i++ {
		id, _ := w.Watch(t.Context(), 0, keyWatch, keyEnd, 0)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: unexpected duplicated id %x", i, id)
		}
		idm[id] = struct{}{}

		s.Put(keyPut, val, lease.NoLease)

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Errorf("#%d: unexpected cancel error %v", i, err)
		}

		if len(resp.Events) != 1 {
			t.Errorf("#%d: len(resp.Events) got = %d, want = 1", i, len(resp.Events))
		}
		if len(resp.Events) == 1 {
			if !bytes.Equal(resp.Events[0].Kv.Key, keyPut) {
				t.Errorf("#%d: resp.Events got = %s, want = %s", i, resp.Events[0].Kv.Key, keyPut)
			}
		}
	}

	keyWatch1, keyEnd1, keyPut1 := []byte("foo1"), []byte("foo2"), []byte("foo1bar")
	s.Put(keyPut1, val, lease.NoLease)

	// unsynced watchers
	for i := 10; i < 15; i++ {
		id, _ := w.Watch(t.Context(), 0, keyWatch1, keyEnd1, 1)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Error(err)
		}

		if len(resp.Events) != 1 {
			t.Errorf("#%d: len(resp.Events) got = %d, want = 1", i, len(resp.Events))
		}
		if len(resp.Events) == 1 {
			if !bytes.Equal(resp.Events[0].Kv.Key, keyPut1) {
				t.Errorf("#%d: resp.Events got = %s, want = %s", i, resp.Events[0].Kv.Key, keyPut1)
			}
		}
	}
}

// TestWatcherWatchWrongRange ensures that watcher with wrong 'end' range
// does not create watcher, which panics when canceling in range tree.
func TestWatcherWatchWrongRange(t *testing.T) {
	resetNextWatchIDForTesting()
	t.Cleanup(resetNextWatchIDForTesting)

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	w := s.NewWatchStream()
	defer w.Close()

	if _, err := w.Watch(t.Context(), 0, []byte("foa"), []byte("foa"), 1); !errors.Is(err, ErrEmptyWatcherRange) {
		t.Fatalf("key == end range given; expected ErrEmptyWatcherRange, got %+v", err)
	}
	if _, err := w.Watch(t.Context(), 0, []byte("fob"), []byte("foa"), 1); !errors.Is(err, ErrEmptyWatcherRange) {
		t.Fatalf("key > end range given; expected ErrEmptyWatcherRange, got %+v", err)
	}
	// watch request with 'WithFromKey' has empty-byte range end
	if id, _ := w.Watch(t.Context(), 0, []byte("foo"), []byte{}, 1); id != 0 {
		t.Fatalf("\x00 is range given; id expected 0, got %d", id)
	}
}

func TestWatchDeleteRange(t *testing.T) {
	b, tmpPath := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})

	defer func() {
		b.Close()
		s.Close()
		os.Remove(tmpPath)
	}()

	testKeyPrefix := []byte("foo")

	for i := 0; i < 3; i++ {
		s.Put([]byte(fmt.Sprintf("%s_%d", testKeyPrefix, i)), []byte("bar"), lease.NoLease)
	}

	w := s.NewWatchStream()
	from, to := testKeyPrefix, []byte(fmt.Sprintf("%s_%d", testKeyPrefix, 99))
	w.Watch(t.Context(), 0, from, to, 0)

	s.DeleteRange(from, to)

	we := []mvccpb.Event{
		{Type: mvccpb.DELETE, Kv: &mvccpb.KeyValue{Key: []byte("foo_0"), ModRevision: 5}},
		{Type: mvccpb.DELETE, Kv: &mvccpb.KeyValue{Key: []byte("foo_1"), ModRevision: 5}},
		{Type: mvccpb.DELETE, Kv: &mvccpb.KeyValue{Key: []byte("foo_2"), ModRevision: 5}},
	}

	select {
	case r := <-w.Chan():
		if !reflect.DeepEqual(r.Events, we) {
			t.Errorf("event = %v, want %v", r.Events, we)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("failed to receive event after 10 seconds!")
	}
}

// TestWatchStreamCancelWatcherByID ensures cancel calls the cancel func of the watcher
// with given id inside watchStream.
func TestWatchStreamCancelWatcherByID(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	w := s.NewWatchStream()
	defer w.Close()

	id, _ := w.Watch(t.Context(), 0, []byte("foo"), nil, 0)

	tests := []struct {
		cancelID WatchID
		werr     error
	}{
		// no error should be returned when cancel the created watcher.
		{id, nil},
		// not exist error should be returned when cancel again.
		{id, ErrWatcherNotExist},
		// not exist error should be returned when cancel a bad id.
		{id + 1, ErrWatcherNotExist},
	}

	for i, tt := range tests {
		gerr := w.Cancel(tt.cancelID)

		if !errors.Is(gerr, tt.werr) {
			t.Errorf("#%d: err = %v, want %v", i, gerr, tt.werr)
		}
	}

	if l := len(w.(*watchStream).cancels); l != 0 {
		t.Errorf("cancels = %d, want 0", l)
	}
}

func TestWatcherRequestProgressBadId(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := newWatchableStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})

	defer cleanup(s, b)
	w := s.NewWatchStream()
	badID := WatchID(1000)
	w.RequestProgress(badID)
	select {
	case resp := <-w.Chan():
		t.Fatalf("unexpected %+v", resp)
	default:
	}
}

func TestWatcherRequestProgress(t *testing.T) {
	testKey := []byte("foo")
	notTestKey := []byte("bad")
	testValue := []byte("bar")
	tcs := []struct {
		name                     string
		startRev                 int64
		expectProgressBeforeSync bool
		expectProgressAfterSync  bool
	}{
		{
			name:                     "Zero revision",
			startRev:                 0,
			expectProgressBeforeSync: true,
			expectProgressAfterSync:  true,
		},
		{
			name:                    "Old revision",
			startRev:                1,
			expectProgressAfterSync: true,
		},
		{
			name:                    "Current revision",
			startRev:                2,
			expectProgressAfterSync: true,
		},
		{
			name:     "Current revision plus one",
			startRev: 3,
		},
		{
			name:     "Current revision plus two",
			startRev: 4,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := betesting.NewDefaultTmpBackend(t)
			s := newWatchableStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})

			defer cleanup(s, b)

			s.Put(testKey, testValue, lease.NoLease)

			w := s.NewWatchStream()

			id, _ := w.Watch(t.Context(), 0, notTestKey, nil, tc.startRev)
			w.RequestProgress(id)
			asssertProgressSent(t, w, id, tc.expectProgressBeforeSync)
			s.syncWatchers([]mvccpb.Event{})
			w.RequestProgress(id)
			asssertProgressSent(t, w, id, tc.expectProgressAfterSync)
		})
	}
}

func asssertProgressSent(t *testing.T, stream WatchStream, id WatchID, expectProgress bool) {
	select {
	case resp := <-stream.Chan():
		if expectProgress {
			wrs := WatchResponse{WatchID: id, Revision: 2}
			if !reflect.DeepEqual(resp, wrs) {
				t.Fatalf("got %+v, expect %+v", resp, wrs)
			}
		} else {
			t.Fatalf("unexpected response %+v", resp)
		}
	default:
		if expectProgress {
			t.Fatalf("failed to receive progress")
		}
	}
}

func TestWatcherRequestProgressAll(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := newWatchableStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})

	defer cleanup(s, b)

	testKey := []byte("foo")
	notTestKey := []byte("bad")
	testValue := []byte("bar")
	s.Put(testKey, testValue, lease.NoLease)

	// Create watch stream with watcher. We will not actually get
	// any notifications on it specifically, but there needs to be
	// at least one Watch for progress notifications to get
	// generated.
	w := s.NewWatchStream()
	w.Watch(t.Context(), 0, notTestKey, nil, 1)

	w.RequestProgressAll()
	select {
	case resp := <-w.Chan():
		t.Fatalf("unexpected %+v", resp)
	default:
	}

	s.syncWatchers([]mvccpb.Event{})

	w.RequestProgressAll()
	wrs := WatchResponse{WatchID: clientv3.InvalidWatchID, Revision: 2}
	select {
	case resp := <-w.Chan():
		if !reflect.DeepEqual(resp, wrs) {
			t.Fatalf("got %+v, expect %+v", resp, wrs)
		}
	case <-time.After(time.Second):
		t.Fatal("failed to receive progress")
	}
}

func TestWatcherWatchWithFilter(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	w := s.NewWatchStream()
	defer w.Close()

	filterPut := func(e mvccpb.Event) bool {
		return e.Type == mvccpb.PUT
	}

	w.Watch(t.Context(), 0, []byte("foo"), nil, 0, filterPut)
	done := make(chan struct{}, 1)

	go func() {
		<-w.Chan()
		done <- struct{}{}
	}()

	s.Put([]byte("foo"), []byte("bar"), 0)

	select {
	case <-done:
		t.Fatal("failed to filter put request")
	case <-time.After(100 * time.Millisecond):
	}

	s.DeleteRange([]byte("foo"), nil)

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("failed to receive delete request")
	}
}

// TestWatchIDUniquenessAcrossStreams verifies auto-assigned watch IDs are unique across multiple streams.
func TestWatchIDUniquenessAcrossStreams(t *testing.T) {
	resetNextWatchIDForTesting()
	t.Cleanup(resetNextWatchIDForTesting)

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	// Create multiple streams
	numStreams := 5
	watchersPerStream := 10
	streams := make([]WatchStream, numStreams)
	allIDs := make(map[WatchID]struct{})

	for i := 0; i < numStreams; i++ {
		streams[i] = s.NewWatchStream()
		defer streams[i].Close()
	}

	// Create watchers across all streams with auto-assigned IDs
	for i := 0; i < numStreams; i++ {
		for j := 0; j < watchersPerStream; j++ {
			id, err := streams[i].Watch(t.Context(), 0, []byte("foo"), nil, 0)
			require.NoError(t, err)

			// Verify ID is unique across all streams
			if _, exists := allIDs[id]; exists {
				t.Errorf("duplicate watch ID %d found across streams", id)
			}
			allIDs[id] = struct{}{}
		}
	}

	// Verify we got the expected number of unique IDs
	expectedTotal := numStreams * watchersPerStream
	assert.Lenf(t, allIDs, expectedTotal, "expected %d unique IDs", expectedTotal)
}

// TestUserSpecifiedIDsCanCollideAcrossStreams verifies backward compatibility -
// user-specified IDs can be duplicated across different streams.
func TestUserSpecifiedIDsCanCollideAcrossStreams(t *testing.T) {
	resetNextWatchIDForTesting()
	t.Cleanup(resetNextWatchIDForTesting)

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	stream1 := s.NewWatchStream()
	defer stream1.Close()
	stream2 := s.NewWatchStream()
	defer stream2.Close()

	// Both streams can use the same user-specified ID
	customID := WatchID(100)

	id1, err := stream1.Watch(t.Context(), customID, []byte("foo"), nil, 0)
	require.NoError(t, err)
	assert.Equalf(t, customID, id1, "stream1 should be able to use customID for its watchID")

	id2, err := stream2.Watch(t.Context(), customID, []byte("foo"), nil, 0)
	require.NoError(t, err)
	assert.Equalf(t, customID, id2, "stream2 should be able to use customID for its watchID")

	// Same ID on same stream should still fail
	_, err = stream1.Watch(t.Context(), customID, []byte("bar"), nil, 0)
	assert.ErrorIs(t, err, ErrWatcherDuplicateID)
}

// TestAutoAssignSkipsUserSpecifiedIDsFromOtherStreams verifies auto-assignment
// skips IDs that are in use by user-specified watches on other streams.
func TestAutoAssignSkipsUserSpecifiedIDsFromOtherStreams(t *testing.T) {
	resetNextWatchIDForTesting()
	t.Cleanup(resetNextWatchIDForTesting)

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	stream1 := s.NewWatchStream()
	defer stream1.Close()
	stream2 := s.NewWatchStream()
	defer stream2.Close()

	// Stream1 uses auto-assign, gets ID 0
	id1, err := stream1.Watch(t.Context(), 0, []byte("foo"), nil, 0)
	require.NoError(t, err)
	assert.Equalf(t, WatchID(0), id1, "first auto-assigned ID should be 0")

	// Stream2 manually specifies ID 1
	id2, err := stream2.Watch(t.Context(), WatchID(1), []byte("foo"), nil, 0)
	require.NoError(t, err)
	assert.Equalf(t, WatchID(1), id2, "stream2 should be able to use custom ID 1")

	// Stream1 auto-assigns again - should skip ID 1 and get ID 2
	id3, err := stream1.Watch(t.Context(), 0, []byte("bar"), nil, 0)
	require.NoError(t, err)
	assert.Equalf(t, WatchID(2), id3, "auto-assigned ID should skip ID 1 which is used by stream2")
}

// TestWatchIDReferenceCountingOnCancel verifies reference counting -
// IDs become available only when all streams using them are cancelled.
func TestWatchIDReferenceCountingOnCancel(t *testing.T) {
	resetNextWatchIDForTesting()
	t.Cleanup(resetNextWatchIDForTesting)

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	stream1 := s.NewWatchStream()
	defer stream1.Close()
	stream2 := s.NewWatchStream()
	defer stream2.Close()
	stream3 := s.NewWatchStream()
	defer stream3.Close()

	// Both streams use same user-specified ID
	customID := WatchID(1)

	_, err := stream1.Watch(t.Context(), customID, []byte("foo"), nil, 0)
	require.NoError(t, err)
	_, err = stream2.Watch(t.Context(), customID, []byte("foo"), nil, 0)
	require.NoError(t, err)
	require.Equalf(t, 2, assignedWatchIDs[WatchID(1)], "ID 1 should have ref count 2 after being used by stream1 and stream2")

	// Cancel on stream1 - ID should still be in use by stream2
	err = stream1.Cancel(customID)
	require.NoError(t, err)
	require.Equalf(t, 1, assignedWatchIDs[WatchID(1)], "ID 1 should have ref count 1 after being cancelled on stream1")

	// Stream3 auto-assigns - should skip ID 1 since stream2 still uses it
	// First fill up ID 0
	_, err = stream3.Watch(t.Context(), 0, []byte("foo"), nil, 0)
	require.NoError(t, err)

	// Next auto-assign should skip 1 and get 2
	id, err := stream3.Watch(t.Context(), 0, []byte("foo"), nil, 0)
	require.NoError(t, err)
	assert.Equalf(t, WatchID(2), id, "should skip ID 1 still in use by stream2")

	// Cancel on stream2 - ID 1 is now fully released
	err = stream2.Cancel(customID)
	require.NoError(t, err)
	_, ok := assignedWatchIDs[WatchID(1)]
	require.Falsef(t, ok, "ID 1 should have been released")
}

// TestConcurrentWatchIDAssignmentAcrossStreams is a stress test to verify
// no duplicate IDs under concurrent access from multiple streams.
func TestConcurrentWatchIDAssignmentAcrossStreams(t *testing.T) {
	resetNextWatchIDForTesting()
	t.Cleanup(resetNextWatchIDForTesting)

	b, _ := betesting.NewDefaultTmpBackend(t)
	s := New(zaptest.NewLogger(t), b, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, b)

	numStreams := 10
	watchersPerStream := 50

	var mu sync.Mutex
	allIDs := make(map[WatchID]struct{})
	var wg sync.WaitGroup

	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stream := s.NewWatchStream()
			defer stream.Close()

			for j := 0; j < watchersPerStream; j++ {
				id, err := stream.Watch(t.Context(), 0, []byte("foo"), nil, 0)
				assert.NoError(t, err)

				mu.Lock()
				if _, exists := allIDs[id]; exists {
					t.Errorf("duplicate watch ID %d found", id)
				}
				allIDs[id] = struct{}{}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	expectedTotal := numStreams * watchersPerStream
	assert.Lenf(t, allIDs, expectedTotal, "expected %d unique IDs", expectedTotal)
}
