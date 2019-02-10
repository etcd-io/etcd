// Copyright 2017 The etcd Authors
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

package v2store

import (
	"testing"
	"time"

	"go.etcd.io/etcd/etcdserver/api/v2error"
	"go.etcd.io/etcd/pkg/testutil"

	"github.com/jonboulle/clockwork"
)

// Ensure that any TTL <= minExpireTime becomes Permanent
func TestMinExpireTime(t *testing.T) {
	s := newStore()
	fc := clockwork.NewFakeClock()
	s.clock = fc
	// FakeClock starts at 0, so minExpireTime should be far in the future.. but just in case
	testutil.AssertTrue(t, minExpireTime.After(fc.Now()), "minExpireTime should be ahead of FakeClock!")
	s.Create("/foo", false, "Y", false, TTLOptionSet{ExpireTime: fc.Now().Add(3 * time.Second)})
	fc.Advance(5 * time.Second)
	// Ensure it hasn't expired
	s.DeleteExpiredKeys(fc.Now())
	var eidx uint64 = 1
	e, err := s.Get("/foo", true, false)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "get")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
}

// Ensure that the store can recursively retrieve a directory listing.
// Note that hidden files should not be returned.
func TestStoreGetDirectory(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc
	s.Create("/foo", true, "", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/bar", false, "X", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/_hidden", false, "*", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/baz", true, "", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/baz/bat", false, "Y", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/baz/_hidden", false, "*", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/baz/ttl", false, "Y", false, TTLOptionSet{ExpireTime: fc.Now().Add(time.Second * 3)})
	var eidx uint64 = 7
	e, err := s.Get("/foo", true, false)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "get")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertEqual(t, len(e.Node.Nodes), 2)
	var bazNodes NodeExterns
	for _, node := range e.Node.Nodes {
		switch node.Key {
		case "/foo/bar":
			testutil.AssertEqual(t, *node.Value, "X")
			testutil.AssertEqual(t, node.Dir, false)
		case "/foo/baz":
			testutil.AssertEqual(t, node.Dir, true)
			testutil.AssertEqual(t, len(node.Nodes), 2)
			bazNodes = node.Nodes
		default:
			t.Errorf("key = %s, not matched", node.Key)
		}
	}
	for _, node := range bazNodes {
		switch node.Key {
		case "/foo/baz/bat":
			testutil.AssertEqual(t, *node.Value, "Y")
			testutil.AssertEqual(t, node.Dir, false)
		case "/foo/baz/ttl":
			testutil.AssertEqual(t, *node.Value, "Y")
			testutil.AssertEqual(t, node.Dir, false)
			testutil.AssertEqual(t, node.TTL, int64(3))
		default:
			t.Errorf("key = %s, not matched", node.Key)
		}
	}
}

// Ensure that the store can update the TTL on a value.
func TestStoreUpdateValueTTL(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	_, err := s.Update("/foo", "baz", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond)})
	testutil.AssertNil(t, err)
	e, _ := s.Get("/foo", false, false)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	e, err = s.Get("/foo", false, false)
	testutil.AssertNil(t, e)
	testutil.AssertEqual(t, err.(*v2error.Error).ErrorCode, v2error.EcodeKeyNotFound)
}

// Ensure that the store can update the TTL on a directory.
func TestStoreUpdateDirTTL(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	var eidx uint64 = 3
	s.Create("/foo", true, "", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/bar", false, "baz", false, TTLOptionSet{ExpireTime: Permanent})
	e, err := s.Update("/foo", "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond)})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.Node.Dir, true)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	e, _ = s.Get("/foo/bar", false, false)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	testutil.AssertEqual(t, e.EtcdIndex, eidx)

	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	e, err = s.Get("/foo/bar", false, false)
	testutil.AssertNil(t, e)
	testutil.AssertEqual(t, err.(*v2error.Error).ErrorCode, v2error.EcodeKeyNotFound)
}

// Ensure that the store can watch for key expiration.
func TestStoreWatchExpire(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	var eidx uint64 = 3
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(400 * time.Millisecond)})
	s.Create("/foofoo", false, "barbarbar", false, TTLOptionSet{ExpireTime: fc.Now().Add(450 * time.Millisecond)})
	s.Create("/foodir", true, "", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond)})

	w, _ := s.Watch("/", true, false, 0)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	c := w.EventChan()
	e := nbselect(c)
	testutil.AssertNil(t, e)
	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	eidx = 4
	e = nbselect(c)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "expire")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	w, _ = s.Watch("/", true, false, 5)
	eidx = 6
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	e = nbselect(w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "expire")
	testutil.AssertEqual(t, e.Node.Key, "/foofoo")
	w, _ = s.Watch("/", true, false, 6)
	e = nbselect(w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "expire")
	testutil.AssertEqual(t, e.Node.Key, "/foodir")
	testutil.AssertEqual(t, e.Node.Dir, true)
}

// Ensure that the store can watch for key expiration when refreshing.
func TestStoreWatchExpireRefresh(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	s.Create("/foofoo", false, "barbarbar", false, TTLOptionSet{ExpireTime: fc.Now().Add(1200 * time.Millisecond), Refresh: true})

	// Make sure we set watch updates when Refresh is true for newly created keys
	w, _ := s.Watch("/", true, false, 0)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	c := w.EventChan()
	e := nbselect(c)
	testutil.AssertNil(t, e)
	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	eidx = 3
	e = nbselect(c)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "expire")
	testutil.AssertEqual(t, e.Node.Key, "/foo")

	s.Update("/foofoo", "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	w, _ = s.Watch("/", true, false, 4)
	fc.Advance(700 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	eidx = 5 // We should skip 4 because a TTL update should occur with no watch notification if set `TTLOptionSet.Refresh` to true
	testutil.AssertEqual(t, w.StartIndex(), eidx-1)
	e = nbselect(w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "expire")
	testutil.AssertEqual(t, e.Node.Key, "/foofoo")
}

// Ensure that the store can watch for key expiration when refreshing with an empty value.
func TestStoreWatchExpireEmptyRefresh(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	var eidx uint64
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	// Should be no-op
	fc.Advance(200 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())

	s.Update("/foo", "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	w, _ := s.Watch("/", true, false, 2)
	fc.Advance(700 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	eidx = 3 // We should skip 2 because a TTL update should occur with no watch notification if set `TTLOptionSet.Refresh` to true
	testutil.AssertEqual(t, w.StartIndex(), eidx-1)
	e := nbselect(w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "expire")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
}

// Update TTL of a key (set TTLOptionSet.Refresh to false) and send notification
func TestStoreWatchNoRefresh(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	var eidx uint64
	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	// Should be no-op
	fc.Advance(200 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())

	// Update key's TTL with setting `TTLOptionSet.Refresh` to false will cause an update event
	s.Update("/foo", "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: false})
	w, _ := s.Watch("/", true, false, 2)
	fc.Advance(700 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	eidx = 2
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	e := nbselect(w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "update")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
}

// Ensure that the store can update the TTL on a value with refresh.
func TestStoreRefresh(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	s.Create("/foo", false, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond)})
	s.Create("/bar", true, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond)})
	_, err := s.Update("/foo", "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	testutil.AssertNil(t, err)

	_, err = s.Set("/foo", false, "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	testutil.AssertNil(t, err)

	_, err = s.Update("/bar", "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	testutil.AssertNil(t, err)

	_, err = s.CompareAndSwap("/foo", "bar", 0, "", TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond), Refresh: true})
	testutil.AssertNil(t, err)
}

// Ensure that the store can recover from a previously saved state that includes an expiring key.
func TestStoreRecoverWithExpiration(t *testing.T) {
	s := newStore()
	s.clock = newFakeClock()

	fc := newFakeClock()

	var eidx uint64 = 4
	s.Create("/foo", true, "", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/x", false, "bar", false, TTLOptionSet{ExpireTime: Permanent})
	s.Create("/foo/y", false, "baz", false, TTLOptionSet{ExpireTime: fc.Now().Add(5 * time.Millisecond)})
	b, err := s.Save()
	testutil.AssertNil(t, err)

	time.Sleep(10 * time.Millisecond)

	s2 := newStore()
	s2.clock = fc

	s2.Recovery(b)

	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())

	e, err := s.Get("/foo/x", false, false)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, *e.Node.Value, "bar")

	e, err = s.Get("/foo/y", false, false)
	testutil.AssertNotNil(t, err)
	testutil.AssertNil(t, e)
}

// Ensure that the store doesn't see expirations of hidden keys.
func TestStoreWatchExpireWithHiddenKey(t *testing.T) {
	s := newStore()
	fc := newFakeClock()
	s.clock = fc

	s.Create("/_foo", false, "bar", false, TTLOptionSet{ExpireTime: fc.Now().Add(500 * time.Millisecond)})
	s.Create("/foofoo", false, "barbarbar", false, TTLOptionSet{ExpireTime: fc.Now().Add(1000 * time.Millisecond)})

	w, _ := s.Watch("/", true, false, 0)
	c := w.EventChan()
	e := nbselect(c)
	testutil.AssertNil(t, e)
	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	e = nbselect(c)
	testutil.AssertNil(t, e)
	fc.Advance(600 * time.Millisecond)
	s.DeleteExpiredKeys(fc.Now())
	e = nbselect(c)
	testutil.AssertEqual(t, e.Action, "expire")
	testutil.AssertEqual(t, e.Node.Key, "/foofoo")
}

// newFakeClock creates a new FakeClock that has been advanced to at least minExpireTime
func newFakeClock() clockwork.FakeClock {
	fc := clockwork.NewFakeClock()
	for minExpireTime.After(fc.Now()) {
		fc.Advance((0x1 << 62) * time.Nanosecond)
	}
	return fc
}

// Performs a non-blocking select on an event channel.
func nbselect(c <-chan *Event) *Event {
	select {
	case e := <-c:
		return e
	default:
		return nil
	}
}
