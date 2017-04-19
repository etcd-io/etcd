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
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/storage/storagepb"
)

func TesttWatchableStoreRev(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer os.Remove(tmpPath)

	for i := 0; i < 3; i++ {
		s.Put([]byte("foo"), []byte("bar"))
		if r := s.Rev(); r != int64(i+1) {
			t.Errorf("#%d: rev = %d, want %d", i, r, i+1)
		}
	}
}

func TestWatchableKVWatchWithUnsynced(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer cleanup(s, tmpPath)

	wa, cancel := s.Watcher([]byte("foo"), true, -1, 0)
	defer cancel()

	s.Put([]byte("foo"), []byte("bar"))
	select {
	case ev := <-wa.Event():
		wev := storagepb.Event{
			Type: storagepb.PUT,
			Kv: &storagepb.KeyValue{
				Key:            []byte("foo"),
				Value:          []byte("bar"),
				CreateRevision: 1,
				ModRevision:    1,
				Version:        1,
			},
		}
		if !reflect.DeepEqual(ev, wev) {
			t.Errorf("watched event = %+v, want %+v", ev, wev)
		}
	case <-time.After(time.Second):
		t.Fatalf("failed to watch the event")
	}

	s.Put([]byte("foo1"), []byte("bar1"))
	select {
	case ev := <-wa.Event():
		wev := storagepb.Event{
			Type: storagepb.PUT,
			Kv: &storagepb.KeyValue{
				Key:            []byte("foo1"),
				Value:          []byte("bar1"),
				CreateRevision: 2,
				ModRevision:    2,
				Version:        1,
			},
		}
		if !reflect.DeepEqual(ev, wev) {
			t.Errorf("watched event = %+v, want %+v", ev, wev)
		}
	case <-time.After(time.Second):
		t.Fatalf("failed to watch the event")
	}

	wa, cancel = s.Watcher([]byte("foo1"), false, -1, 4)
	defer cancel()

	select {
	case ev := <-wa.Event():
		wev := storagepb.Event{
			Type: storagepb.PUT,
			Kv: &storagepb.KeyValue{
				Key:            []byte("foo1"),
				Value:          []byte("bar1"),
				CreateRevision: 2,
				ModRevision:    2,
				Version:        1,
			},
		}
		if !reflect.DeepEqual(ev, wev) {
			t.Errorf("watched event = %+v, want %+v", ev, wev)
		}
	case <-time.After(time.Second):
		t.Fatalf("failed to watch the event")
	}

	s.Put([]byte("foo1"), []byte("bar11"))
	select {
	case ev := <-wa.Event():
		wev := storagepb.Event{
			Type: storagepb.PUT,
			Kv: &storagepb.KeyValue{
				Key:            []byte("foo1"),
				Value:          []byte("bar11"),
				CreateRevision: 2,
				ModRevision:    3,
				Version:        2,
			},
		}
		if !reflect.DeepEqual(ev, wev) {
			t.Errorf("watched event = %+v, want %+v", ev, wev)
		}
	case <-time.After(time.Second):
		t.Fatalf("failed to watch the event")
	}

	select {
	case ev := <-wa.Event():
		if !reflect.DeepEqual(ev, storagepb.Event{}) {
			t.Errorf("watched event = %+v, want %+v", ev, storagepb.Event{})
		}
		if g := wa.Err(); g != ExceedEnd {
			t.Errorf("err = %+v, want %+v", g, ExceedEnd)
		}
	case <-time.After(time.Second):
		t.Fatalf("failed to watch the event")
	}

}
