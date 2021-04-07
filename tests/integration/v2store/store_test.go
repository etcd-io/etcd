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

package v2store_test

import (
	"fmt"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2error"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
)

type StoreCloser interface {
	v2store.Store
	Close()
}

func TestNewStoreWithNamespaces(t *testing.T) {
	s := newTestStore(t, "/0", "/1")
	defer s.Close()

	_, err := s.Get("/0", false, false)
	testutil.AssertNil(t, err)
	_, err = s.Get("/1", false, false)
	testutil.AssertNil(t, err)
}

// Ensure that the store can retrieve an existing value.
func TestStoreGetValue(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var eidx uint64 = 1
	e, err := s.Get("/foo", false, false)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "get")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertEqual(t, *e.Node.Value, "bar")
}

// Ensure that the store can retrieve a directory in sorted order.
func TestStoreGetSorted(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/x", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/z", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y/a", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y/b", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var eidx uint64 = 6
	e, err := s.Get("/foo", true, true)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)

	var yNodes v2store.NodeExterns
	sortedStrings := []string{"/foo/x", "/foo/y", "/foo/z"}
	for i := range e.Node.Nodes {
		node := e.Node.Nodes[i]
		if node.Key != sortedStrings[i] {
			t.Errorf("expect key = %s, got key = %s", sortedStrings[i], node.Key)
		}
		if node.Key == "/foo/y" {
			yNodes = node.Nodes
		}
	}

	sortedStrings = []string{"/foo/y/a", "/foo/y/b"}
	for i := range yNodes {
		node := yNodes[i]
		if node.Key != sortedStrings[i] {
			t.Errorf("expect key = %s, got key = %s", sortedStrings[i], node.Key)
		}
	}
}

func TestSet(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Set /foo=""
	var eidx uint64 = 1
	e, err := s.Set("/foo", false, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "set")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "")
	testutil.AssertNil(t, e.Node.Nodes)
	testutil.AssertNil(t, e.Node.Expiration)
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(1))

	// Set /foo="bar"
	eidx = 2
	e, err = s.Set("/foo", false, "bar", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "set")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "bar")
	testutil.AssertNil(t, e.Node.Nodes)
	testutil.AssertNil(t, e.Node.Expiration)
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(2))
	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "")
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(1))
	// Set /foo="baz" (for testing prevNode)
	eidx = 3
	e, err = s.Set("/foo", false, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "set")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	testutil.AssertNil(t, e.Node.Nodes)
	testutil.AssertNil(t, e.Node.Expiration)
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(3))
	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(2))

	// Set /a/b/c/d="efg"
	eidx = 4
	e, err = s.Set("/a/b/c/d", false, "efg", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Node.Key, "/a/b/c/d")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "efg")
	testutil.AssertNil(t, e.Node.Nodes)
	testutil.AssertNil(t, e.Node.Expiration)
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(4))

	// Set /dir as a directory
	eidx = 5
	e, err = s.Set("/dir", true, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "set")
	testutil.AssertEqual(t, e.Node.Key, "/dir")
	testutil.AssertTrue(t, e.Node.Dir)
	testutil.AssertNil(t, e.Node.Value)
	testutil.AssertNil(t, e.Node.Nodes)
	testutil.AssertNil(t, e.Node.Expiration)
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(5))
}

// Ensure that the store can create a new key if it doesn't already exist.
func TestStoreCreateValue(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create /foo=bar
	var eidx uint64 = 1
	e, err := s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "bar")
	testutil.AssertNil(t, e.Node.Nodes)
	testutil.AssertNil(t, e.Node.Expiration)
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(1))

	// Create /empty=""
	eidx = 2
	e, err = s.Create("/empty", false, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/empty")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "")
	testutil.AssertNil(t, e.Node.Nodes)
	testutil.AssertNil(t, e.Node.Expiration)
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(2))

}

// Ensure that the store can create a new directory if it doesn't already exist.
func TestStoreCreateDirectory(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var eidx uint64 = 1
	e, err := s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertTrue(t, e.Node.Dir)
}

// Ensure that the store fails to create a key if it already exists.
func TestStoreCreateFailsIfExists(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// create /foo as dir
	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})

	// create /foo as dir again
	e, _err := s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeNodeExist)
	testutil.AssertEqual(t, err.Message, "Key already exists")
	testutil.AssertEqual(t, err.Cause, "/foo")
	testutil.AssertEqual(t, err.Index, uint64(1))
	testutil.AssertNil(t, e)
}

// Ensure that the store can update a key if it already exists.
func TestStoreUpdateValue(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// create /foo=bar
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	// update /foo="bzr"
	var eidx uint64 = 2
	e, err := s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "update")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(2))
	// check prevNode
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
	testutil.AssertEqual(t, e.PrevNode.TTL, int64(0))
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(1))

	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	testutil.AssertEqual(t, e.EtcdIndex, eidx)

	// update /foo=""
	eidx = 3
	e, err = s.Update("/foo", "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "update")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertFalse(t, e.Node.Dir)
	testutil.AssertEqual(t, *e.Node.Value, "")
	testutil.AssertEqual(t, e.Node.TTL, int64(0))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(3))
	// check prevNode
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "baz")
	testutil.AssertEqual(t, e.PrevNode.TTL, int64(0))
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(2))

	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, *e.Node.Value, "")
}

// Ensure that the store cannot update a directory.
func TestStoreUpdateFailsIfDirectory(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeNotFile)
	testutil.AssertEqual(t, err.Message, "Not a file")
	testutil.AssertEqual(t, err.Cause, "/foo")
	testutil.AssertNil(t, e)
}

// Ensure that the store can delete a value.
func TestStoreDeleteValue(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.Delete("/foo", false, false)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "delete")
	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
}

// Ensure that the store can delete a directory if recursive is specified.
func TestStoreDeleteDirectory(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// create directory /foo
	var eidx uint64 = 2
	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	// delete /foo with dir = true and recursive = false
	// this should succeed, since the directory is empty
	e, err := s.Delete("/foo", true, false)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "delete")
	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, e.PrevNode.Dir, true)

	// create directory /foo and directory /foo/bar
	_, err = s.Create("/foo/bar", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	// delete /foo with dir = true and recursive = false
	// this should fail, since the directory is not empty
	_, err = s.Delete("/foo", true, false)
	testutil.AssertNotNil(t, err)

	// delete /foo with dir=false and recursive = true
	// this should succeed, since recursive implies dir=true
	// and recursively delete should be able to delete all
	// items under the given directory
	e, err = s.Delete("/foo", false, true)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.Action, "delete")

}

// Ensure that the store cannot delete a directory if both of recursive
// and dir are not specified.
func TestStoreDeleteDirectoryFailsIfNonRecursiveAndDir(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.Delete("/foo", false, false)
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeNotFile)
	testutil.AssertEqual(t, err.Message, "Not a file")
	testutil.AssertNil(t, e)
}

func TestRootRdOnly(t *testing.T) {
	s := newTestStore(t, "/0")
	defer s.Close()

	for _, tt := range []string{"/", "/0"} {
		_, err := s.Set(tt, true, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		testutil.AssertNotNil(t, err)

		_, err = s.Delete(tt, true, true)
		testutil.AssertNotNil(t, err)

		_, err = s.Create(tt, true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		testutil.AssertNotNil(t, err)

		_, err = s.Update(tt, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		testutil.AssertNotNil(t, err)

		_, err = s.CompareAndSwap(tt, "", 0, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		testutil.AssertNotNil(t, err)
	}
}

func TestStoreCompareAndDeletePrevValue(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndDelete("/foo", "bar", 0)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "compareAndDelete")
	testutil.AssertEqual(t, e.Node.Key, "/foo")

	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(1))
	testutil.AssertEqual(t, e.PrevNode.CreatedIndex, uint64(1))
}

func TestStoreCompareAndDeletePrevValueFailsIfNotMatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndDelete("/foo", "baz", 0)
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeTestFailed)
	testutil.AssertEqual(t, err.Message, "Compare failed")
	testutil.AssertNil(t, e)
	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, *e.Node.Value, "bar")
}

func TestStoreCompareAndDeletePrevIndex(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndDelete("/foo", "", 1)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "compareAndDelete")
	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(1))
	testutil.AssertEqual(t, e.PrevNode.CreatedIndex, uint64(1))
}

func TestStoreCompareAndDeletePrevIndexFailsIfNotMatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndDelete("/foo", "", 100)
	testutil.AssertNotNil(t, _err)
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeTestFailed)
	testutil.AssertEqual(t, err.Message, "Compare failed")
	testutil.AssertNil(t, e)
	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, *e.Node.Value, "bar")
}

// Ensure that the store cannot delete a directory.
func TestStoreCompareAndDeleteDirectoryFail(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	_, _err := s.CompareAndDelete("/foo", "", 0)
	testutil.AssertNotNil(t, _err)
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeNotFile)
}

// Ensure that the store can conditionally update a key if it has a previous value.
func TestStoreCompareAndSwapPrevValue(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndSwap("/foo", "bar", 0, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "compareAndSwap")
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(1))
	testutil.AssertEqual(t, e.PrevNode.CreatedIndex, uint64(1))

	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
}

// Ensure that the store cannot conditionally update a key if it has the wrong previous value.
func TestStoreCompareAndSwapPrevValueFailsIfNotMatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndSwap("/foo", "wrong_value", 0, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeTestFailed)
	testutil.AssertEqual(t, err.Message, "Compare failed")
	testutil.AssertNil(t, e)
	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, *e.Node.Value, "bar")
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
}

// Ensure that the store can conditionally update a key if it has a previous index.
func TestStoreCompareAndSwapPrevIndex(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndSwap("/foo", "", 1, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "compareAndSwap")
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	// check prevNode
	testutil.AssertNotNil(t, e.PrevNode)
	testutil.AssertEqual(t, e.PrevNode.Key, "/foo")
	testutil.AssertEqual(t, *e.PrevNode.Value, "bar")
	testutil.AssertEqual(t, e.PrevNode.ModifiedIndex, uint64(1))
	testutil.AssertEqual(t, e.PrevNode.CreatedIndex, uint64(1))

	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
}

// Ensure that the store cannot conditionally update a key if it has the wrong previous index.
func TestStoreCompareAndSwapPrevIndexFailsIfNotMatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndSwap("/foo", "", 100, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	err := _err.(*v2error.Error)
	testutil.AssertEqual(t, err.ErrorCode, v2error.EcodeTestFailed)
	testutil.AssertEqual(t, err.Message, "Compare failed")
	testutil.AssertNil(t, e)
	e, _ = s.Get("/foo", false, false)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, *e.Node.Value, "bar")
}

// Ensure that the store can watch for key creation.
func TestStoreWatchCreate(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 0
	w, _ := s.Watch("/foo", false, false, 0)
	c := w.EventChan()
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	eidx = 1
	e := timeoutSelect(t, c)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	select {
	case e = <-w.EventChan():
		testutil.AssertNil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// Ensure that the store can watch for recursive key creation.
func TestStoreWatchRecursiveCreate(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 0
	w, err := s.Watch("/foo", true, false, 0)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	eidx = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/foo/bar")
}

// Ensure that the store can watch for key updates.
func TestStoreWatchUpdate(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", false, false, 0)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	eidx = 2
	s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "update")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
}

// Ensure that the store can watch for recursive key updates.
func TestStoreWatchRecursiveUpdate(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, err := s.Watch("/foo", true, false, 0)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	eidx = 2
	s.Update("/foo/bar", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "update")
	testutil.AssertEqual(t, e.Node.Key, "/foo/bar")
}

// Ensure that the store can watch for key deletions.
func TestStoreWatchDelete(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", false, false, 0)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	eidx = 2
	s.Delete("/foo", false, false)
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "delete")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
}

// Ensure that the store can watch for recursive key deletions.
func TestStoreWatchRecursiveDelete(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, err := s.Watch("/foo", true, false, 0)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	eidx = 2
	s.Delete("/foo/bar", false, false)
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "delete")
	testutil.AssertEqual(t, e.Node.Key, "/foo/bar")
}

// Ensure that the store can watch for CAS updates.
func TestStoreWatchCompareAndSwap(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", false, false, 0)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	eidx = 2
	s.CompareAndSwap("/foo", "bar", 0, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "compareAndSwap")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
}

// Ensure that the store can watch for recursive CAS updates.
func TestStoreWatchRecursiveCompareAndSwap(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", true, false, 0)
	testutil.AssertEqual(t, w.StartIndex(), eidx)
	eidx = 2
	s.CompareAndSwap("/foo/bar", "baz", 0, "bat", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "compareAndSwap")
	testutil.AssertEqual(t, e.Node.Key, "/foo/bar")
}

// Ensure that the store can watch in streaming mode.
func TestStoreWatchStream(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	w, _ := s.Watch("/foo", false, true, 0)
	// first modification
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertEqual(t, *e.Node.Value, "bar")
	select {
	case e = <-w.EventChan():
		testutil.AssertNil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
	// second modification
	eidx = 2
	s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e = timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "update")
	testutil.AssertEqual(t, e.Node.Key, "/foo")
	testutil.AssertEqual(t, *e.Node.Value, "baz")
	select {
	case e = <-w.EventChan():
		testutil.AssertNil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// Ensure that the store can watch for hidden keys as long as it's an exact path match.
func TestStoreWatchCreateWithHiddenKey(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	w, _ := s.Watch("/_foo", false, false, 0)
	s.Create("/_foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/_foo")
	select {
	case e = <-w.EventChan():
		testutil.AssertNil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// Ensure that the store doesn't see hidden key creates without an exact path match in recursive mode.
func TestStoreWatchRecursiveCreateWithHiddenKey(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	w, _ := s.Watch("/foo", true, false, 0)
	s.Create("/foo/_bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := nbselect(w.EventChan())
	testutil.AssertNil(t, e)
	w, _ = s.Watch("/foo", true, false, 0)
	s.Create("/foo/_baz", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	select {
	case e = <-w.EventChan():
		testutil.AssertNil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
	s.Create("/foo/_baz/quux", false, "quux", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	select {
	case e = <-w.EventChan():
		testutil.AssertNil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// Ensure that the store doesn't see hidden key updates.
func TestStoreWatchUpdateWithHiddenKey(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	s.Create("/_foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/_foo", false, false, 0)
	s.Update("/_foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.Action, "update")
	testutil.AssertEqual(t, e.Node.Key, "/_foo")
	e = nbselect(w.EventChan())
	testutil.AssertNil(t, e)
}

// Ensure that the store doesn't see hidden key updates without an exact path match in recursive mode.
func TestStoreWatchRecursiveUpdateWithHiddenKey(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	s.Create("/foo/_bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", true, false, 0)
	s.Update("/foo/_bar", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := nbselect(w.EventChan())
	testutil.AssertNil(t, e)
}

// Ensure that the store can watch for key deletions.
func TestStoreWatchDeleteWithHiddenKey(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 2
	s.Create("/_foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/_foo", false, false, 0)
	s.Delete("/_foo", false, false)
	e := timeoutSelect(t, w.EventChan())
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "delete")
	testutil.AssertEqual(t, e.Node.Key, "/_foo")
	e = nbselect(w.EventChan())
	testutil.AssertNil(t, e)
}

// Ensure that the store doesn't see hidden key deletes without an exact path match in recursive mode.
func TestStoreWatchRecursiveDeleteWithHiddenKey(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	s.Create("/foo/_bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", true, false, 0)
	s.Delete("/foo/_bar", false, false)
	e := nbselect(w.EventChan())
	testutil.AssertNil(t, e)
}

// Ensure that the store does see hidden key creates if watching deeper than a hidden key in recursive mode.
func TestStoreWatchRecursiveCreateDeeperThanHiddenKey(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 1
	w, _ := s.Watch("/_foo/bar", true, false, 0)
	s.Create("/_foo/bar/baz", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})

	e := timeoutSelect(t, w.EventChan())
	testutil.AssertNotNil(t, e)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertEqual(t, e.Action, "create")
	testutil.AssertEqual(t, e.Node.Key, "/_foo/bar/baz")
}

// Ensure that slow consumers are handled properly.
//
// Since Watcher.EventChan() has a buffer of size 100 we can only queue 100
// event per watcher. If the consumer cannot consume the event on time and
// another event arrives, the channel is closed and event is discarded.
// This test ensures that after closing the channel, the store can continue
// to operate correctly.
func TestStoreWatchSlowConsumer(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	s.Watch("/foo", true, true, 0) // stream must be true
	// Fill watch channel with 100 events
	for i := 1; i <= 100; i++ {
		s.Set("/foo", false, fmt.Sprint(i), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}) // ok
	}
	// testutil.AssertEqual(t, s.WatcherHub.count, int64(1))
	s.Set("/foo", false, "101", v2store.TTLOptionSet{ExpireTime: v2store.Permanent}) // ok
	// remove watcher
	// testutil.AssertEqual(t, s.WatcherHub.count, int64(0))
	s.Set("/foo", false, "102", v2store.TTLOptionSet{ExpireTime: v2store.Permanent}) // must not panic
}

// Performs a non-blocking select on an event channel.
func nbselect(c <-chan *v2store.Event) *v2store.Event {
	select {
	case e := <-c:
		return e
	default:
		return nil
	}
}

// Performs a non-blocking select on an event channel.
func timeoutSelect(t *testing.T, c <-chan *v2store.Event) *v2store.Event {
	select {
	case e := <-c:
		return e
	case <-time.After(time.Second):
		t.Errorf("timed out waiting on event")
		return nil
	}
}
