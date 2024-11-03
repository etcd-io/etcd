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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/etcdserver/api/v2error"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
)

type StoreCloser interface {
	v2store.Store
	Close()
}

func TestNewStoreWithNamespaces(t *testing.T) {
	s := v2store.New("/0", "/1")

	_, err := s.Get("/0", false, false)
	require.NoError(t, err)
	_, err = s.Get("/1", false, false)
	assert.NoError(t, err)
}

// TestStoreGetValue ensures that the store can retrieve an existing value.
func TestStoreGetValue(t *testing.T) {
	s := v2store.New()

	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var eidx uint64 = 1
	e, err := s.Get("/foo", false, false)
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "get", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.Equal(t, "bar", *e.Node.Value)
}

// TestStoreGetSorted ensures that the store can retrieve a directory in sorted order.
func TestStoreGetSorted(t *testing.T) {
	s := v2store.New()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/x", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/z", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y/a", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y/b", false, "0", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var eidx uint64 = 6
	e, err := s.Get("/foo", true, true)
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)

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
	s := v2store.New()

	// Set /foo=""
	var eidx uint64 = 1
	e, err := s.Set("/foo", false, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "set", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "", *e.Node.Value)
	assert.Nil(t, e.Node.Nodes)
	assert.Nil(t, e.Node.Expiration)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(1), e.Node.ModifiedIndex)

	// Set /foo="bar"
	eidx = 2
	e, err = s.Set("/foo", false, "bar", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "set", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "bar", *e.Node.Value)
	assert.Nil(t, e.Node.Nodes)
	assert.Nil(t, e.Node.Expiration)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(2), e.Node.ModifiedIndex)
	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "", *e.PrevNode.Value)
	assert.Equal(t, uint64(1), e.PrevNode.ModifiedIndex)
	// Set /foo="baz" (for testing prevNode)
	eidx = 3
	e, err = s.Set("/foo", false, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "set", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "baz", *e.Node.Value)
	assert.Nil(t, e.Node.Nodes)
	assert.Nil(t, e.Node.Expiration)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(3), e.Node.ModifiedIndex)
	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "bar", *e.PrevNode.Value)
	assert.Equal(t, uint64(2), e.PrevNode.ModifiedIndex)

	// Set /a/b/c/d="efg"
	eidx = 4
	e, err = s.Set("/a/b/c/d", false, "efg", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "/a/b/c/d", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "efg", *e.Node.Value)
	assert.Nil(t, e.Node.Nodes)
	assert.Nil(t, e.Node.Expiration)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(4), e.Node.ModifiedIndex)

	// Set /dir as a directory
	eidx = 5
	e, err = s.Set("/dir", true, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "set", e.Action)
	assert.Equal(t, "/dir", e.Node.Key)
	assert.True(t, e.Node.Dir)
	assert.Nil(t, e.Node.Value)
	assert.Nil(t, e.Node.Nodes)
	assert.Nil(t, e.Node.Expiration)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(5), e.Node.ModifiedIndex)
}

// TestStoreCreateValue ensures that the store can create a new key if it doesn't already exist.
func TestStoreCreateValue(t *testing.T) {
	s := v2store.New()

	// Create /foo=bar
	var eidx uint64 = 1
	e, err := s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "bar", *e.Node.Value)
	assert.Nil(t, e.Node.Nodes)
	assert.Nil(t, e.Node.Expiration)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(1), e.Node.ModifiedIndex)

	// Create /empty=""
	eidx = 2
	e, err = s.Create("/empty", false, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/empty", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "", *e.Node.Value)
	assert.Nil(t, e.Node.Nodes)
	assert.Nil(t, e.Node.Expiration)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(2), e.Node.ModifiedIndex)
}

// TestStoreCreateDirectory ensures that the store can create a new directory if it doesn't already exist.
func TestStoreCreateDirectory(t *testing.T) {
	s := v2store.New()

	var eidx uint64 = 1
	e, err := s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.True(t, e.Node.Dir)
}

// TestStoreCreateFailsIfExists ensure that the store fails to create a key if it already exists.
func TestStoreCreateFailsIfExists(t *testing.T) {
	s := v2store.New()

	// create /foo as dir
	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})

	// create /foo as dir again
	e, _err := s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeNodeExist, err.ErrorCode)
	assert.Equal(t, "Key already exists", err.Message)
	assert.Equal(t, "/foo", err.Cause)
	assert.Equal(t, uint64(1), err.Index)
	assert.Nil(t, e)
}

// TestStoreUpdateValue ensures that the store can update a key if it already exists.
func TestStoreUpdateValue(t *testing.T) {
	s := v2store.New()

	// create /foo=bar
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	// update /foo="bzr"
	var eidx uint64 = 2
	e, err := s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "update", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "baz", *e.Node.Value)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(2), e.Node.ModifiedIndex)
	// check prevNode
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "bar", *e.PrevNode.Value)
	assert.Equal(t, int64(0), e.PrevNode.TTL)
	assert.Equal(t, uint64(1), e.PrevNode.ModifiedIndex)

	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, "baz", *e.Node.Value)
	assert.Equal(t, eidx, e.EtcdIndex)

	// update /foo=""
	eidx = 3
	e, err = s.Update("/foo", "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "update", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.False(t, e.Node.Dir)
	assert.Equal(t, "", *e.Node.Value)
	assert.Equal(t, int64(0), e.Node.TTL)
	assert.Equal(t, uint64(3), e.Node.ModifiedIndex)
	// check prevNode
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "baz", *e.PrevNode.Value)
	assert.Equal(t, int64(0), e.PrevNode.TTL)
	assert.Equal(t, uint64(2), e.PrevNode.ModifiedIndex)

	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "", *e.Node.Value)
}

// TestStoreUpdateFailsIfDirectory ensures that the store cannot update a directory.
func TestStoreUpdateFailsIfDirectory(t *testing.T) {
	s := v2store.New()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeNotFile, err.ErrorCode)
	assert.Equal(t, "Not a file", err.Message)
	assert.Equal(t, "/foo", err.Cause)
	assert.Nil(t, e)
}

// TestStoreDeleteValue ensures that the store can delete a value.
func TestStoreDeleteValue(t *testing.T) {
	s := v2store.New()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.Delete("/foo", false, false)
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "delete", e.Action)
	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "bar", *e.PrevNode.Value)
}

// TestStoreDeleteDirectory ensures that the store can delete a directory if recursive is specified.
func TestStoreDeleteDirectory(t *testing.T) {
	s := v2store.New()

	// create directory /foo
	var eidx uint64 = 2
	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	// delete /foo with dir = true and recursive = false
	// this should succeed, since the directory is empty
	e, err := s.Delete("/foo", true, false)
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "delete", e.Action)
	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.True(t, e.PrevNode.Dir)

	// create directory /foo and directory /foo/bar
	_, err = s.Create("/foo/bar", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	// delete /foo with dir = true and recursive = false
	// this should fail, since the directory is not empty
	_, err = s.Delete("/foo", true, false)
	require.Error(t, err)

	// delete /foo with dir=false and recursive = true
	// this should succeed, since recursive implies dir=true
	// and recursively delete should be able to delete all
	// items under the given directory
	e, err = s.Delete("/foo", false, true)
	require.NoError(t, err)
	assert.Equal(t, "delete", e.Action)
}

// TestStoreDeleteDirectoryFailsIfNonRecursiveAndDir ensures that the
// store cannot delete a directory if both of recursive and dir are not specified.
func TestStoreDeleteDirectoryFailsIfNonRecursiveAndDir(t *testing.T) {
	s := v2store.New()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.Delete("/foo", false, false)
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeNotFile, err.ErrorCode)
	assert.Equal(t, "Not a file", err.Message)
	assert.Nil(t, e)
}

func TestRootRdOnly(t *testing.T) {
	s := v2store.New("/0")

	for _, tt := range []string{"/", "/0"} {
		_, err := s.Set(tt, true, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		require.Error(t, err)

		_, err = s.Delete(tt, true, true)
		require.Error(t, err)

		_, err = s.Create(tt, true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		require.Error(t, err)

		_, err = s.Update(tt, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		require.Error(t, err)

		_, err = s.CompareAndSwap(tt, "", 0, "", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
		require.Error(t, err)
	}
}

func TestStoreCompareAndDeletePrevValue(t *testing.T) {
	s := v2store.New()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndDelete("/foo", "bar", 0)
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "compareAndDelete", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)

	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "bar", *e.PrevNode.Value)
	assert.Equal(t, uint64(1), e.PrevNode.ModifiedIndex)
	assert.Equal(t, uint64(1), e.PrevNode.CreatedIndex)
}

func TestStoreCompareAndDeletePrevValueFailsIfNotMatch(t *testing.T) {
	s := v2store.New()

	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndDelete("/foo", "baz", 0)
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeTestFailed, err.ErrorCode)
	assert.Equal(t, "Compare failed", err.Message)
	assert.Nil(t, e)
	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "bar", *e.Node.Value)
}

func TestStoreCompareAndDeletePrevIndex(t *testing.T) {
	s := v2store.New()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndDelete("/foo", "", 1)
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "compareAndDelete", e.Action)
	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "bar", *e.PrevNode.Value)
	assert.Equal(t, uint64(1), e.PrevNode.ModifiedIndex)
	assert.Equal(t, uint64(1), e.PrevNode.CreatedIndex)
}

func TestStoreCompareAndDeletePrevIndexFailsIfNotMatch(t *testing.T) {
	s := v2store.New()

	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndDelete("/foo", "", 100)
	require.Error(t, _err)
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeTestFailed, err.ErrorCode)
	assert.Equal(t, "Compare failed", err.Message)
	assert.Nil(t, e)
	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "bar", *e.Node.Value)
}

// TestStoreCompareAndDeleteDirectoryFail ensures that the store cannot delete a directory.
func TestStoreCompareAndDeleteDirectoryFail(t *testing.T) {
	s := v2store.New()

	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	_, _err := s.CompareAndDelete("/foo", "", 0)
	require.Error(t, _err)
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeNotFile, err.ErrorCode)
}

// TestStoreCompareAndSwapPrevValue ensures that the store can conditionally
// update a key if it has a previous value.
func TestStoreCompareAndSwapPrevValue(t *testing.T) {
	s := v2store.New()

	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndSwap("/foo", "bar", 0, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "compareAndSwap", e.Action)
	assert.Equal(t, "baz", *e.Node.Value)
	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "bar", *e.PrevNode.Value)
	assert.Equal(t, uint64(1), e.PrevNode.ModifiedIndex)
	assert.Equal(t, uint64(1), e.PrevNode.CreatedIndex)

	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, "baz", *e.Node.Value)
}

// TestStoreCompareAndSwapPrevValueFailsIfNotMatch ensure that the store cannot
// conditionally update a key if it has the wrong previous value.
func TestStoreCompareAndSwapPrevValueFailsIfNotMatch(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndSwap("/foo", "wrong_value", 0, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeTestFailed, err.ErrorCode)
	assert.Equal(t, "Compare failed", err.Message)
	assert.Nil(t, e)
	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, "bar", *e.Node.Value)
	assert.Equal(t, eidx, e.EtcdIndex)
}

// TestStoreCompareAndSwapPrevIndex ensures that the store can conditionally
// update a key if it has a previous index.
func TestStoreCompareAndSwapPrevIndex(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 2
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, err := s.CompareAndSwap("/foo", "", 1, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	require.NoError(t, err)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "compareAndSwap", e.Action)
	assert.Equal(t, "baz", *e.Node.Value)
	// check prevNode
	require.NotNil(t, e.PrevNode)
	assert.Equal(t, "/foo", e.PrevNode.Key)
	assert.Equal(t, "bar", *e.PrevNode.Value)
	assert.Equal(t, uint64(1), e.PrevNode.ModifiedIndex)
	assert.Equal(t, uint64(1), e.PrevNode.CreatedIndex)

	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, "baz", *e.Node.Value)
	assert.Equal(t, eidx, e.EtcdIndex)
}

// TestStoreCompareAndSwapPrevIndexFailsIfNotMatch ensures that the store cannot
// conditionally update a key if it has the wrong previous index.
func TestStoreCompareAndSwapPrevIndexFailsIfNotMatch(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e, _err := s.CompareAndSwap("/foo", "", 100, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	var err *v2error.Error
	require.ErrorAs(t, _err, &err)
	assert.Equal(t, v2error.EcodeTestFailed, err.ErrorCode)
	assert.Equal(t, "Compare failed", err.Message)
	assert.Nil(t, e)
	e, _ = s.Get("/foo", false, false)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "bar", *e.Node.Value)
}

// TestStoreWatchCreate ensures that the store can watch for key creation.
func TestStoreWatchCreate(t *testing.T) {
	s := v2store.New()
	var eidx uint64
	w, _ := s.Watch("/foo", false, false, 0)
	c := w.EventChan()
	assert.Equal(t, eidx, w.StartIndex())
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	eidx = 1
	e := timeoutSelect(t, c)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	select {
	case e = <-w.EventChan():
		assert.Nil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestStoreWatchRecursiveCreate ensures that the store
// can watch for recursive key creation.
func TestStoreWatchRecursiveCreate(t *testing.T) {
	s := v2store.New()
	var eidx uint64
	w, err := s.Watch("/foo", true, false, 0)
	require.NoError(t, err)
	assert.Equal(t, eidx, w.StartIndex())
	eidx = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/foo/bar", e.Node.Key)
}

// TestStoreWatchUpdate ensures that the store can watch for key updates.
func TestStoreWatchUpdate(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", false, false, 0)
	assert.Equal(t, eidx, w.StartIndex())
	eidx = 2
	s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "update", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
}

// TestStoreWatchRecursiveUpdate ensures that the store can watch for recursive key updates.
func TestStoreWatchRecursiveUpdate(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, err := s.Watch("/foo", true, false, 0)
	require.NoError(t, err)
	assert.Equal(t, eidx, w.StartIndex())
	eidx = 2
	s.Update("/foo/bar", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "update", e.Action)
	assert.Equal(t, "/foo/bar", e.Node.Key)
}

// TestStoreWatchDelete ensures that the store can watch for key deletions.
func TestStoreWatchDelete(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", false, false, 0)
	assert.Equal(t, eidx, w.StartIndex())
	eidx = 2
	s.Delete("/foo", false, false)
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "delete", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
}

// TestStoreWatchRecursiveDelete ensures that the store can watch for recursive key deletions.
func TestStoreWatchRecursiveDelete(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, err := s.Watch("/foo", true, false, 0)
	require.NoError(t, err)
	assert.Equal(t, eidx, w.StartIndex())
	eidx = 2
	s.Delete("/foo/bar", false, false)
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "delete", e.Action)
	assert.Equal(t, "/foo/bar", e.Node.Key)
}

// TestStoreWatchCompareAndSwap ensures that the store can watch for CAS updates.
func TestStoreWatchCompareAndSwap(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", false, false, 0)
	assert.Equal(t, eidx, w.StartIndex())
	eidx = 2
	s.CompareAndSwap("/foo", "bar", 0, "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "compareAndSwap", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
}

// TestStoreWatchRecursiveCompareAndSwap ensures that the
// store can watch for recursive CAS updates.
func TestStoreWatchRecursiveCompareAndSwap(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	s.Create("/foo/bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", true, false, 0)
	assert.Equal(t, eidx, w.StartIndex())
	eidx = 2
	s.CompareAndSwap("/foo/bar", "baz", 0, "bat", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "compareAndSwap", e.Action)
	assert.Equal(t, "/foo/bar", e.Node.Key)
}

// TestStoreWatchStream ensures that the store can watch in streaming mode.
func TestStoreWatchStream(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	w, _ := s.Watch("/foo", false, true, 0)
	// first modification
	s.Create("/foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.Equal(t, "bar", *e.Node.Value)
	select {
	case e = <-w.EventChan():
		assert.Nil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
	// second modification
	eidx = 2
	s.Update("/foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e = timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "update", e.Action)
	assert.Equal(t, "/foo", e.Node.Key)
	assert.Equal(t, "baz", *e.Node.Value)
	select {
	case e = <-w.EventChan():
		assert.Nil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestStoreWatchCreateWithHiddenKey ensure that the store can
// watch for hidden keys as long as it's an exact path match.
func TestStoreWatchCreateWithHiddenKey(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	w, _ := s.Watch("/_foo", false, false, 0)
	s.Create("/_foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/_foo", e.Node.Key)
	select {
	case e = <-w.EventChan():
		assert.Nil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestStoreWatchRecursiveCreateWithHiddenKey ensures that the store doesn't
// see hidden key creates without an exact path match in recursive mode.
func TestStoreWatchRecursiveCreateWithHiddenKey(t *testing.T) {
	s := v2store.New()
	w, _ := s.Watch("/foo", true, false, 0)
	s.Create("/foo/_bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := nbselect(w.EventChan())
	assert.Nil(t, e)
	w, _ = s.Watch("/foo", true, false, 0)
	s.Create("/foo/_baz", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	select {
	case e = <-w.EventChan():
		assert.Nil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
	s.Create("/foo/_baz/quux", false, "quux", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	select {
	case e = <-w.EventChan():
		assert.Nil(t, e)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestStoreWatchUpdateWithHiddenKey ensures that the store
// doesn't see hidden key updates.
func TestStoreWatchUpdateWithHiddenKey(t *testing.T) {
	s := v2store.New()
	s.Create("/_foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/_foo", false, false, 0)
	s.Update("/_foo", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, "update", e.Action)
	assert.Equal(t, "/_foo", e.Node.Key)
	e = nbselect(w.EventChan())
	assert.Nil(t, e)
}

// TestStoreWatchRecursiveUpdateWithHiddenKey ensures that the store doesn't
// see hidden key updates without an exact path match in recursive mode.
func TestStoreWatchRecursiveUpdateWithHiddenKey(t *testing.T) {
	s := v2store.New()
	s.Create("/foo/_bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", true, false, 0)
	s.Update("/foo/_bar", "baz", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	e := nbselect(w.EventChan())
	assert.Nil(t, e)
}

// TestStoreWatchDeleteWithHiddenKey ensures that the store can watch for key deletions.
func TestStoreWatchDeleteWithHiddenKey(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 2
	s.Create("/_foo", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/_foo", false, false, 0)
	s.Delete("/_foo", false, false)
	e := timeoutSelect(t, w.EventChan())
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "delete", e.Action)
	assert.Equal(t, "/_foo", e.Node.Key)
	e = nbselect(w.EventChan())
	assert.Nil(t, e)
}

// TestStoreWatchRecursiveDeleteWithHiddenKey ensures that the store doesn't see
// hidden key deletes without an exact path match in recursive mode.
func TestStoreWatchRecursiveDeleteWithHiddenKey(t *testing.T) {
	s := v2store.New()
	s.Create("/foo/_bar", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	w, _ := s.Watch("/foo", true, false, 0)
	s.Delete("/foo/_bar", false, false)
	e := nbselect(w.EventChan())
	assert.Nil(t, e)
}

// TestStoreWatchRecursiveCreateDeeperThanHiddenKey ensures that the store does see
// hidden key creates if watching deeper than a hidden key in recursive mode.
func TestStoreWatchRecursiveCreateDeeperThanHiddenKey(t *testing.T) {
	s := v2store.New()
	var eidx uint64 = 1
	w, _ := s.Watch("/_foo/bar", true, false, 0)
	s.Create("/_foo/bar/baz", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})

	e := timeoutSelect(t, w.EventChan())
	require.NotNil(t, e)
	assert.Equal(t, eidx, e.EtcdIndex)
	assert.Equal(t, "create", e.Action)
	assert.Equal(t, "/_foo/bar/baz", e.Node.Key)
}

// TestStoreWatchSlowConsumer ensures that slow consumers are handled properly.
//
// Since Watcher.EventChan() has a buffer of size 100 we can only queue 100
// event per watcher. If the consumer cannot consume the event on time and
// another event arrives, the channel is closed and event is discarded.
// This test ensures that after closing the channel, the store can continue
// to operate correctly.
func TestStoreWatchSlowConsumer(t *testing.T) {
	s := v2store.New()
	s.Watch("/foo", true, true, 0) // stream must be true
	// Fill watch channel with 100 events
	for i := 1; i <= 100; i++ {
		s.Set("/foo", false, fmt.Sprint(i), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}) // ok
	}
	// assert.Equal(t, s.WatcherHub.count, int64(1))
	s.Set("/foo", false, "101", v2store.TTLOptionSet{ExpireTime: v2store.Permanent}) // ok
	// remove watcher
	// assert.Equal(t, s.WatcherHub.count, int64(0))
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
