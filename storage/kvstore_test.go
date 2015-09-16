// Copyright 2015 CoreOS, Inc.
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

package storage

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/storage/backend"
	"github.com/coreos/etcd/storage/storagepb"
)

func TestStoreRev(t *testing.T) {
	s := newStore(tmpPath)
	defer os.Remove(tmpPath)

	for i := 0; i < 3; i++ {
		s.Put([]byte("foo"), []byte("bar"))
		if r := s.Rev(); r != int64(i+1) {
			t.Errorf("#%d: rev = %d, want %d", i, r, i+1)
		}
	}
}

func TestStorePut(t *testing.T) {
	tests := []struct {
		rev revision
		r   indexGetResp

		wrev    revision
		wev     storagepb.Event
		wputrev revision
	}{
		{
			revision{1, 0},
			indexGetResp{revision{}, revision{}, 0, ErrRevisionNotFound},
			revision{1, 1},
			storagepb.Event{
				Type: storagepb.PUT,
				Kv: &storagepb.KeyValue{
					Key:            []byte("foo"),
					Value:          []byte("bar"),
					CreateRevision: 2,
					ModRevision:    2,
					Version:        1,
				},
			},
			revision{2, 0},
		},
		{
			revision{1, 1},
			indexGetResp{revision{2, 0}, revision{2, 0}, 1, nil},
			revision{1, 2},
			storagepb.Event{
				Type: storagepb.PUT,
				Kv: &storagepb.KeyValue{
					Key:            []byte("foo"),
					Value:          []byte("bar"),
					CreateRevision: 2,
					ModRevision:    2,
					Version:        2,
				},
			},
			revision{2, 1},
		},
		{
			revision{2, 0},
			indexGetResp{revision{2, 1}, revision{2, 0}, 2, nil},
			revision{2, 1},
			storagepb.Event{
				Type: storagepb.PUT,
				Kv: &storagepb.KeyValue{
					Key:            []byte("foo"),
					Value:          []byte("bar"),
					CreateRevision: 2,
					ModRevision:    3,
					Version:        3,
				},
			},
			revision{3, 0},
		},
	}
	for i, tt := range tests {
		s, b, index := newFakeStore()
		s.currentRev = tt.rev
		index.indexGetRespc <- tt.r

		s.put([]byte("foo"), []byte("bar"))

		data, err := tt.wev.Marshal()
		if err != nil {
			t.Errorf("#%d: marshal err = %v, want nil", i, err)
		}
		wact := []testutil.Action{
			{"put", []interface{}{keyBucketName, newTestBytes(tt.wputrev), data}},
		}
		if g := b.tx.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: tx action = %+v, want %+v", i, g, wact)
		}
		wact = []testutil.Action{
			{"get", []interface{}{[]byte("foo"), tt.wputrev.main}},
			{"put", []interface{}{[]byte("foo"), tt.wputrev}},
		}
		if g := index.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: index action = %+v, want %+v", i, g, wact)
		}
		if s.currentRev != tt.wrev {
			t.Errorf("#%d: rev = %+v, want %+v", i, s.currentRev, tt.wrev)
		}
	}
}

func TestStoreRange(t *testing.T) {
	ev := storagepb.Event{
		Type: storagepb.PUT,
		Kv: &storagepb.KeyValue{
			Key:            []byte("foo"),
			Value:          []byte("bar"),
			CreateRevision: 1,
			ModRevision:    2,
			Version:        1,
		},
	}
	evb, err := ev.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	currev := revision{1, 1}
	wrev := int64(2)

	tests := []struct {
		idxr indexRangeResp
		r    rangeResp
	}{
		{
			indexRangeResp{[][]byte{[]byte("foo")}, []revision{{2, 0}}},
			rangeResp{[][]byte{newTestBytes(revision{2, 0})}, [][]byte{evb}},
		},
		{
			indexRangeResp{[][]byte{[]byte("foo"), []byte("foo1")}, []revision{{2, 0}, {3, 0}}},
			rangeResp{[][]byte{newTestBytes(revision{2, 0})}, [][]byte{evb}},
		},
	}
	for i, tt := range tests {
		s, b, index := newFakeStore()
		s.currentRev = currev
		b.tx.rangeRespc <- tt.r
		index.indexRangeRespc <- tt.idxr

		kvs, rev, err := s.rangeKeys([]byte("foo"), []byte("goo"), 1, 0)
		if err != nil {
			t.Errorf("#%d: err = %v, want nil", i, err)
		}
		if w := []storagepb.KeyValue{*ev.Kv}; !reflect.DeepEqual(kvs, w) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, kvs, w)
		}
		if rev != wrev {
			t.Errorf("#%d: rev = %d, want %d", i, rev, wrev)
		}

		wact := []testutil.Action{
			{"range", []interface{}{keyBucketName, newTestBytes(tt.idxr.revs[0]), []byte(nil), int64(0)}},
		}
		if g := b.tx.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: tx action = %+v, want %+v", i, g, wact)
		}
		wact = []testutil.Action{
			{"range", []interface{}{[]byte("foo"), []byte("goo"), wrev}},
		}
		if g := index.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: index action = %+v, want %+v", i, g, wact)
		}
		if s.currentRev != currev {
			t.Errorf("#%d: current rev = %+v, want %+v", i, s.currentRev, currev)
		}
	}
}

func TestStoreDeleteRange(t *testing.T) {
	tests := []struct {
		rev revision
		r   indexRangeResp

		wrev    revision
		wrrev   int64
		wdelrev revision
	}{
		{
			revision{2, 0},
			indexRangeResp{[][]byte{[]byte("foo")}, []revision{{2, 0}}},
			revision{2, 1},
			2,
			revision{3, 0},
		},
		{
			revision{2, 1},
			indexRangeResp{[][]byte{[]byte("foo")}, []revision{{2, 0}}},
			revision{2, 2},
			3,
			revision{3, 1},
		},
	}
	for i, tt := range tests {
		s, b, index := newFakeStore()
		s.currentRev = tt.rev
		index.indexRangeRespc <- tt.r

		n := s.deleteRange([]byte("foo"), []byte("goo"))
		if n != 1 {
			t.Errorf("#%d: n = %d, want 1", i, n)
		}

		data, err := (&storagepb.Event{
			Type: storagepb.DELETE,
			Kv: &storagepb.KeyValue{
				Key: []byte("foo"),
			},
		}).Marshal()
		if err != nil {
			t.Errorf("#%d: marshal err = %v, want nil", i, err)
		}
		wact := []testutil.Action{
			{"put", []interface{}{keyBucketName, newTestBytes(tt.wdelrev), data}},
		}
		if g := b.tx.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: tx action = %+v, want %+v", i, g, wact)
		}
		wact = []testutil.Action{
			{"range", []interface{}{[]byte("foo"), []byte("goo"), tt.wrrev}},
			{"tombstone", []interface{}{[]byte("foo"), tt.wdelrev}},
		}
		if g := index.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: index action = %+v, want %+v", i, g, wact)
		}
		if s.currentRev != tt.wrev {
			t.Errorf("#%d: rev = %+v, want %+v", i, s.currentRev, tt.wrev)
		}
	}
}

func TestStoreRangeEvents(t *testing.T) {
	ev := storagepb.Event{
		Type: storagepb.PUT,
		Kv: &storagepb.KeyValue{
			Key:            []byte("foo"),
			Value:          []byte("bar"),
			CreateRevision: 1,
			ModRevision:    2,
			Version:        1,
		},
	}
	evb, err := ev.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	currev := revision{2, 0}

	tests := []struct {
		idxr indexRangeEventsResp
		r    rangeResp
	}{
		{
			indexRangeEventsResp{[]revision{{2, 0}}},
			rangeResp{[][]byte{newTestBytes(revision{2, 0})}, [][]byte{evb}},
		},
		{
			indexRangeEventsResp{[]revision{{2, 0}, {3, 0}}},
			rangeResp{[][]byte{newTestBytes(revision{2, 0})}, [][]byte{evb}},
		},
	}
	for i, tt := range tests {
		s, b, index := newFakeStore()
		s.currentRev = currev
		index.indexRangeEventsRespc <- tt.idxr
		b.tx.rangeRespc <- tt.r

		evs, _, err := s.RangeEvents([]byte("foo"), []byte("goo"), 1, 1, 4)
		if err != nil {
			t.Errorf("#%d: err = %v, want nil", i, err)
		}
		if w := []storagepb.Event{ev}; !reflect.DeepEqual(evs, w) {
			t.Errorf("#%d: evs = %+v, want %+v", i, evs, w)
		}

		wact := []testutil.Action{
			{"rangeEvents", []interface{}{[]byte("foo"), []byte("goo"), int64(1)}},
		}
		if g := index.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: index action = %+v, want %+v", i, g, wact)
		}
		wact = []testutil.Action{
			{"range", []interface{}{keyBucketName, newTestBytes(tt.idxr.revs[0]), []byte(nil), int64(0)}},
		}
		if g := b.tx.Action(); !reflect.DeepEqual(g, wact) {
			t.Errorf("#%d: tx action = %+v, want %+v", i, g, wact)
		}
		if s.currentRev != currev {
			t.Errorf("#%d: current rev = %+v, want %+v", i, s.currentRev, currev)
		}
	}
}

func TestStoreCompact(t *testing.T) {
	s, b, index := newFakeStore()
	s.currentRev = revision{3, 0}
	index.indexCompactRespc <- map[revision]struct{}{revision{1, 0}: {}}
	b.tx.rangeRespc <- rangeResp{[][]byte{newTestBytes(revision{1, 0}), newTestBytes(revision{2, 0})}, nil}

	s.Compact(3)
	s.wg.Wait()

	if s.compactMainRev != 3 {
		t.Errorf("compact main rev = %d, want 3", s.compactMainRev)
	}
	end := make([]byte, 8)
	binary.BigEndian.PutUint64(end, uint64(4))
	wact := []testutil.Action{
		{"put", []interface{}{metaBucketName, scheduledCompactKeyName, newTestBytes(revision{3, 0})}},
		{"range", []interface{}{keyBucketName, make([]byte, 17), end, int64(10000)}},
		{"delete", []interface{}{keyBucketName, newTestBytes(revision{2, 0})}},
		{"put", []interface{}{metaBucketName, finishedCompactKeyName, newTestBytes(revision{3, 0})}},
	}
	if g := b.tx.Action(); !reflect.DeepEqual(g, wact) {
		t.Errorf("tx actions = %+v, want %+v", g, wact)
	}
	wact = []testutil.Action{
		{"compact", []interface{}{int64(3)}},
	}
	if g := index.Action(); !reflect.DeepEqual(g, wact) {
		t.Errorf("index action = %+v, want %+v", g, wact)
	}
}

func TestStoreRestore(t *testing.T) {
	s, b, index := newFakeStore()

	putev := storagepb.Event{
		Type: storagepb.PUT,
		Kv: &storagepb.KeyValue{
			Key:            []byte("foo"),
			Value:          []byte("bar"),
			CreateRevision: 3,
			ModRevision:    3,
			Version:        1,
		},
	}
	putevb, err := putev.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	delev := storagepb.Event{
		Type: storagepb.DELETE,
		Kv: &storagepb.KeyValue{
			Key: []byte("foo"),
		},
	}
	delevb, err := delev.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	b.tx.rangeRespc <- rangeResp{[][]byte{finishedCompactKeyName}, [][]byte{newTestBytes(revision{2, 0})}}
	b.tx.rangeRespc <- rangeResp{[][]byte{newTestBytes(revision{3, 0}), newTestBytes(revision{4, 0})}, [][]byte{putevb, delevb}}
	b.tx.rangeRespc <- rangeResp{[][]byte{scheduledCompactKeyName}, [][]byte{newTestBytes(revision{2, 0})}}

	s.Restore()

	if s.compactMainRev != 2 {
		t.Errorf("compact rev = %d, want 4", s.compactMainRev)
	}
	wrev := revision{4, 0}
	if !reflect.DeepEqual(s.currentRev, wrev) {
		t.Errorf("current rev = %v, want %v", s.currentRev, wrev)
	}
	wact := []testutil.Action{
		{"range", []interface{}{metaBucketName, finishedCompactKeyName, []byte(nil), int64(0)}},
		{"range", []interface{}{keyBucketName, newTestBytes(revision{}), newTestBytes(revision{math.MaxInt64, math.MaxInt64}), int64(0)}},
		{"range", []interface{}{metaBucketName, scheduledCompactKeyName, []byte(nil), int64(0)}},
	}
	if g := b.tx.Action(); !reflect.DeepEqual(g, wact) {
		t.Errorf("tx actions = %+v, want %+v", g, wact)
	}
	wact = []testutil.Action{
		{"restore", []interface{}{[]byte("foo"), revision{3, 0}, revision{3, 0}, int64(1)}},
		{"tombstone", []interface{}{[]byte("foo"), revision{4, 0}}},
	}
	if g := index.Action(); !reflect.DeepEqual(g, wact) {
		t.Errorf("index action = %+v, want %+v", g, wact)
	}
}

// tests end parameter works well
func TestStoreRangeEventsEnd(t *testing.T) {
	s := newStore(tmpPath)
	defer cleanup(s, tmpPath)

	s.Put([]byte("foo"), []byte("bar"))
	s.Put([]byte("foo1"), []byte("bar1"))
	s.Put([]byte("foo2"), []byte("bar2"))
	evs := []storagepb.Event{
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 1, ModRevision: 1, Version: 1},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo1"), Value: []byte("bar1"), CreateRevision: 2, ModRevision: 2, Version: 1},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo2"), Value: []byte("bar2"), CreateRevision: 3, ModRevision: 3, Version: 1},
		},
	}

	tests := []struct {
		key, end []byte
		wevs     []storagepb.Event
	}{
		// get no keys
		{
			[]byte("doo"), []byte("foo"),
			nil,
		},
		// get no keys when key == end
		{
			[]byte("foo"), []byte("foo"),
			nil,
		},
		// get no keys when ranging single key
		{
			[]byte("doo"), nil,
			nil,
		},
		// get all keys
		{
			[]byte("foo"), []byte("foo3"),
			evs,
		},
		// get partial keys
		{
			[]byte("foo"), []byte("foo1"),
			evs[:1],
		},
		// get single key
		{
			[]byte("foo"), nil,
			evs[:1],
		},
	}

	for i, tt := range tests {
		evs, rev, err := s.RangeEvents(tt.key, tt.end, 0, 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		if rev != 4 {
			t.Errorf("#%d: rev = %d, want %d", i, rev, 4)
		}
		if !reflect.DeepEqual(evs, tt.wevs) {
			t.Errorf("#%d: evs = %+v, want %+v", i, evs, tt.wevs)
		}
	}
}

func TestStoreRangeEventsRev(t *testing.T) {
	s := newStore(tmpPath)
	defer cleanup(s, tmpPath)

	s.Put([]byte("foo"), []byte("bar"))
	s.DeleteRange([]byte("foo"), nil)
	s.Put([]byte("foo"), []byte("bar"))
	s.Put([]byte("unrelated"), []byte("unrelated"))
	evs := []storagepb.Event{
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 1, ModRevision: 1, Version: 1},
		},
		{
			Type: storagepb.DELETE,
			Kv:   &storagepb.KeyValue{Key: []byte("foo")},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 3, ModRevision: 3, Version: 1},
		},
	}

	tests := []struct {
		start int64
		end   int64
		wevs  []storagepb.Event
		wnext int64
	}{
		{1, 1, nil, 1},
		{1, 2, evs[:1], 2},
		{1, 3, evs[:2], 3},
		{1, 4, evs, 5},
		{1, 5, evs, 5},
		{1, 10, evs, 5},
		{3, 4, evs[2:], 5},
		{0, 10, evs, 5},
		{1, 0, evs, 5},
		{0, 0, evs, 5},
	}

	for i, tt := range tests {
		evs, next, err := s.RangeEvents([]byte("foo"), nil, 0, tt.start, tt.end)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(evs, tt.wevs) {
			t.Errorf("#%d: evs = %+v, want %+v", i, evs, tt.wevs)
		}
		if next != tt.wnext {
			t.Errorf("#%d: next = %d, want %d", i, next, tt.wnext)
		}
	}
}

func TestStoreRangeEventsBad(t *testing.T) {
	s := newStore(tmpPath)
	defer cleanup(s, tmpPath)

	s.Put([]byte("foo"), []byte("bar"))
	s.Put([]byte("foo"), []byte("bar1"))
	s.Put([]byte("foo"), []byte("bar2"))
	if err := s.Compact(3); err != nil {
		t.Fatalf("compact error (%v)", err)
	}

	tests := []struct {
		rev  int64
		werr error
	}{
		{1, ErrCompacted},
		{2, ErrCompacted},
		{3, ErrCompacted},
		{4, ErrFutureRev},
		{10, ErrFutureRev},
	}
	for i, tt := range tests {
		_, _, err := s.RangeEvents([]byte("foo"), nil, 0, tt.rev, 100)
		if err != tt.werr {
			t.Errorf("#%d: error = %v, want %v", i, err, tt.werr)
		}
	}
}

func TestStoreRangeEventsLimit(t *testing.T) {
	s := newStore(tmpPath)
	defer cleanup(s, tmpPath)

	s.Put([]byte("foo"), []byte("bar"))
	s.DeleteRange([]byte("foo"), nil)
	s.Put([]byte("foo"), []byte("bar"))
	evs := []storagepb.Event{
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 1, ModRevision: 1, Version: 1},
		},
		{
			Type: storagepb.DELETE,
			Kv:   &storagepb.KeyValue{Key: []byte("foo")},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 3, ModRevision: 3, Version: 1},
		},
	}

	tests := []struct {
		limit int64
		wevs  []storagepb.Event
	}{
		// no limit
		{-1, evs},
		// no limit
		{0, evs},
		{1, evs[:1]},
		{2, evs[:2]},
		{3, evs},
		{100, evs},
	}
	for i, tt := range tests {
		evs, _, err := s.RangeEvents([]byte("foo"), nil, tt.limit, 1, 100)
		if err != nil {
			t.Fatalf("#%d: range error (%v)", i, err)
		}
		if !reflect.DeepEqual(evs, tt.wevs) {
			t.Errorf("#%d: evs = %+v, want %+v", i, evs, tt.wevs)
		}
	}
}

func TestRestoreContinueUnfinishedCompaction(t *testing.T) {
	s0 := newStore(tmpPath)
	defer os.Remove(tmpPath)

	s0.Put([]byte("foo"), []byte("bar"))
	s0.Put([]byte("foo"), []byte("bar1"))
	s0.Put([]byte("foo"), []byte("bar2"))

	// write scheduled compaction, but not do compaction
	rbytes := newRevBytes()
	revToBytes(revision{main: 2}, rbytes)
	tx := s0.b.BatchTx()
	tx.Lock()
	tx.UnsafePut(metaBucketName, scheduledCompactKeyName, rbytes)
	tx.Unlock()

	s0.Close()

	s1 := newStore(tmpPath)
	s1.Restore()

	// wait for scheduled compaction to be finished
	time.Sleep(100 * time.Millisecond)

	if _, _, err := s1.Range([]byte("foo"), nil, 0, 2); err != ErrCompacted {
		t.Errorf("range on compacted rev error = %v, want %v", err, ErrCompacted)
	}
	// check the key in backend is deleted
	revbytes := newRevBytes()
	// TODO: compact should delete main=2 key too
	revToBytes(revision{main: 1}, revbytes)
	tx = s1.b.BatchTx()
	tx.Lock()
	ks, _ := tx.UnsafeRange(keyBucketName, revbytes, nil, 0)
	if len(ks) != 0 {
		t.Errorf("key for rev %+v still exists, want deleted", bytesToRev(revbytes))
	}
	tx.Unlock()
}

func BenchmarkStorePut(b *testing.B) {
	s := newStore(tmpPath)
	defer os.Remove(tmpPath)

	// prepare keys
	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = make([]byte, 64)
		rand.Read(keys[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[i], []byte("foo"))
	}
}

func newTestBytes(rev revision) []byte {
	bytes := newRevBytes()
	revToBytes(rev, bytes)
	return bytes
}

func newFakeStore() (*store, *fakeBackend, *fakeIndex) {
	b := &fakeBackend{&fakeBatchTx{rangeRespc: make(chan rangeResp, 5)}}
	index := &fakeIndex{
		indexGetRespc:         make(chan indexGetResp, 1),
		indexRangeRespc:       make(chan indexRangeResp, 1),
		indexRangeEventsRespc: make(chan indexRangeEventsResp, 1),
		indexCompactRespc:     make(chan map[revision]struct{}, 1),
	}
	return &store{
		b:              b,
		kvindex:        index,
		currentRev:     revision{},
		compactMainRev: -1,
	}, b, index
}

type rangeResp struct {
	keys [][]byte
	vals [][]byte
}

type fakeBatchTx struct {
	testutil.Recorder
	rangeRespc chan rangeResp
}

func (b *fakeBatchTx) Lock()                          {}
func (b *fakeBatchTx) Unlock()                        {}
func (b *fakeBatchTx) UnsafeCreateBucket(name []byte) {}
func (b *fakeBatchTx) UnsafePut(bucketName []byte, key []byte, value []byte) {
	b.Recorder.Record(testutil.Action{Name: "put", Params: []interface{}{bucketName, key, value}})
}
func (b *fakeBatchTx) UnsafeRange(bucketName []byte, key, endKey []byte, limit int64) (keys [][]byte, vals [][]byte) {
	b.Recorder.Record(testutil.Action{Name: "range", Params: []interface{}{bucketName, key, endKey, limit}})
	r := <-b.rangeRespc
	return r.keys, r.vals
}
func (b *fakeBatchTx) UnsafeDelete(bucketName []byte, key []byte) {
	b.Recorder.Record(testutil.Action{Name: "delete", Params: []interface{}{bucketName, key}})
}
func (b *fakeBatchTx) Commit()        {}
func (b *fakeBatchTx) CommitAndStop() {}

type fakeBackend struct {
	tx *fakeBatchTx
}

func (b *fakeBackend) BatchTx() backend.BatchTx   { return b.tx }
func (b *fakeBackend) Hash() (uint32, error)      { return 0, nil }
func (b *fakeBackend) Snapshot() backend.Snapshot { return nil }
func (b *fakeBackend) ForceCommit()               {}
func (b *fakeBackend) Close() error               { return nil }

type indexGetResp struct {
	rev     revision
	created revision
	ver     int64
	err     error
}

type indexRangeResp struct {
	keys [][]byte
	revs []revision
}

type indexRangeEventsResp struct {
	revs []revision
}

type fakeIndex struct {
	testutil.Recorder
	indexGetRespc         chan indexGetResp
	indexRangeRespc       chan indexRangeResp
	indexRangeEventsRespc chan indexRangeEventsResp
	indexCompactRespc     chan map[revision]struct{}
}

func (i *fakeIndex) Get(key []byte, atRev int64) (rev, created revision, ver int64, err error) {
	i.Recorder.Record(testutil.Action{Name: "get", Params: []interface{}{key, atRev}})
	r := <-i.indexGetRespc
	return r.rev, r.created, r.ver, r.err
}
func (i *fakeIndex) Range(key, end []byte, atRev int64) ([][]byte, []revision) {
	i.Recorder.Record(testutil.Action{Name: "range", Params: []interface{}{key, end, atRev}})
	r := <-i.indexRangeRespc
	return r.keys, r.revs
}
func (i *fakeIndex) Put(key []byte, rev revision) {
	i.Recorder.Record(testutil.Action{Name: "put", Params: []interface{}{key, rev}})
}
func (i *fakeIndex) Restore(key []byte, created, modified revision, ver int64) {
	i.Recorder.Record(testutil.Action{Name: "restore", Params: []interface{}{key, created, modified, ver}})
}
func (i *fakeIndex) Tombstone(key []byte, rev revision) error {
	i.Recorder.Record(testutil.Action{Name: "tombstone", Params: []interface{}{key, rev}})
	return nil
}
func (i *fakeIndex) RangeEvents(key, end []byte, rev int64) []revision {
	i.Recorder.Record(testutil.Action{Name: "rangeEvents", Params: []interface{}{key, end, rev}})
	r := <-i.indexRangeEventsRespc
	return r.revs
}
func (i *fakeIndex) Compact(rev int64) map[revision]struct{} {
	i.Recorder.Record(testutil.Action{Name: "compact", Params: []interface{}{rev}})
	return <-i.indexCompactRespc
}
func (i *fakeIndex) Equal(b index) bool { return false }
