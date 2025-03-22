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

package v2store

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

var (
	key, val   = "foo", "bar"
	val1, val2 = "bar1", "bar2"
	expiration = time.Minute
)

func TestNewKVIs(t *testing.T) {
	nd := newTestNode()

	assert.Falsef(t, nd.IsHidden(), "nd.Hidden() = %v, want = false", nd.IsHidden())

	assert.Falsef(t, nd.IsPermanent(), "nd.IsPermanent() = %v, want = false", nd.IsPermanent())

	assert.Falsef(t, nd.IsDir(), "nd.IsDir() = %v, want = false", nd.IsDir())
}

func TestNewKVReadWriteCompare(t *testing.T) {
	nd := newTestNode()

	if v, err := nd.Read(); v != val || err != nil {
		t.Errorf("value = %s and err = %v, want value = %s and err = nil", v, err, val)
	}

	if err := nd.Write(val1, nd.CreatedIndex+1); err != nil {
		t.Errorf("nd.Write error = %v, want = nil", err)
	} else {
		if v, err := nd.Read(); v != val1 || err != nil {
			t.Errorf("value = %s and err = %v, want value = %s and err = nil", v, err, val1)
		}
	}
	if err := nd.Write(val2, nd.CreatedIndex+2); err != nil {
		t.Errorf("nd.Write error = %v, want = nil", err)
	} else {
		if v, err := nd.Read(); v != val2 || err != nil {
			t.Errorf("value = %s and err = %v, want value = %s and err = nil", v, err, val2)
		}
	}

	if ok, which := nd.Compare(val2, 2); !ok || which != 0 {
		t.Errorf("ok = %v and which = %d, want ok = true and which = 0", ok, which)
	}
}

func TestNewKVExpiration(t *testing.T) {
	nd := newTestNode()

	_, ttl := nd.expirationAndTTL(clockwork.NewFakeClock())
	assert.LessOrEqualf(t, ttl, expiration.Nanoseconds(), "ttl = %d, want %d < %d", ttl, ttl, expiration.Nanoseconds())

	newExpiration := time.Hour
	nd.UpdateTTL(time.Now().Add(newExpiration))
	_, ttl = nd.expirationAndTTL(clockwork.NewFakeClock())
	assert.LessOrEqualf(t, ttl, newExpiration.Nanoseconds(), "ttl = %d, want %d < %d", ttl, ttl, newExpiration.Nanoseconds())
	if ns, err := nd.List(); ns != nil || err == nil {
		t.Errorf("nodes = %v and err = %v, want nodes = nil and err != nil", ns, err)
	}

	en := nd.Repr(false, false, clockwork.NewFakeClock())
	assert.Equalf(t, en.Key, nd.Path, "en.Key = %s, want = %s", en.Key, nd.Path)
	assert.Equalf(t, *(en.Value), nd.Value, "*(en.Key) = %s, want = %s", *(en.Value), nd.Value)
}

func TestNewKVListReprCompareClone(t *testing.T) {
	nd := newTestNode()

	if ns, err := nd.List(); ns != nil || err == nil {
		t.Errorf("nodes = %v and err = %v, want nodes = nil and err != nil", ns, err)
	}

	en := nd.Repr(false, false, clockwork.NewFakeClock())
	assert.Equalf(t, en.Key, nd.Path, "en.Key = %s, want = %s", en.Key, nd.Path)
	assert.Equalf(t, *(en.Value), nd.Value, "*(en.Key) = %s, want = %s", *(en.Value), nd.Value)

	cn := nd.Clone()
	assert.Equalf(t, cn.Path, nd.Path, "cn.Path = %s, want = %s", cn.Path, nd.Path)
	assert.Equalf(t, cn.Value, nd.Value, "cn.Value = %s, want = %s", cn.Value, nd.Value)
}

func TestNewKVRemove(t *testing.T) {
	nd := newTestNode()

	if v, err := nd.Read(); v != val || err != nil {
		t.Errorf("value = %s and err = %v, want value = %s and err = nil", v, err, val)
	}

	if err := nd.Write(val1, nd.CreatedIndex+1); err != nil {
		t.Errorf("nd.Write error = %v, want = nil", err)
	} else {
		if v, err := nd.Read(); v != val1 || err != nil {
			t.Errorf("value = %s and err = %v, want value = %s and err = nil", v, err, val1)
		}
	}
	if err := nd.Write(val2, nd.CreatedIndex+2); err != nil {
		t.Errorf("nd.Write error = %v, want = nil", err)
	} else {
		if v, err := nd.Read(); v != val2 || err != nil {
			t.Errorf("value = %s and err = %v, want value = %s and err = nil", v, err, val2)
		}
	}
	err := nd.Remove(false, false, nil)
	assert.Nilf(t, err, "nd.Remove err = %v, want = nil", err)
	// still readable
	v, err := nd.Read()
	if v != val2 || err != nil {
		t.Errorf("value = %s and err = %v, want value = %s and err = nil", v, err, val2)
	}
	assert.Emptyf(t, nd.store.ttlKeyHeap.array, "len(nd.store.ttlKeyHeap.array) = %d, want = 0", len(nd.store.ttlKeyHeap.array))
	assert.Emptyf(t, nd.store.ttlKeyHeap.keyMap, "len(nd.store.ttlKeyHeap.keyMap) = %d, want = 0", len(nd.store.ttlKeyHeap.keyMap))
}

func TestNewDirIs(t *testing.T) {
	nd, _ := newTestNodeDir()
	assert.Falsef(t, nd.IsHidden(), "nd.Hidden() = %v, want = false", nd.IsHidden())

	assert.Falsef(t, nd.IsPermanent(), "nd.IsPermanent() = %v, want = false", nd.IsPermanent())

	assert.Truef(t, nd.IsDir(), "nd.IsDir() = %v, want = true", nd.IsDir())
}

func TestNewDirReadWriteListReprClone(t *testing.T) {
	nd, _ := newTestNodeDir()

	_, err := nd.Read()
	assert.NotNilf(t, err, "err = %v, want err != nil", err)

	err = nd.Write(val, nd.CreatedIndex+1)
	assert.NotNilf(t, err, "err = %v, want err != nil", err)

	if ns, err := nd.List(); ns == nil && err != nil {
		t.Errorf("nodes = %v and err = %v, want nodes = nil and err == nil", ns, err)
	}

	en := nd.Repr(false, false, clockwork.NewFakeClock())
	assert.Equalf(t, en.Key, nd.Path, "en.Key = %s, want = %s", en.Key, nd.Path)

	cn := nd.Clone()
	assert.Equalf(t, cn.Path, nd.Path, "cn.Path = %s, want = %s", cn.Path, nd.Path)
}

func TestNewDirExpirationTTL(t *testing.T) {
	nd, _ := newTestNodeDir()

	_, ttl := nd.expirationAndTTL(clockwork.NewFakeClock())
	assert.LessOrEqualf(t, ttl, expiration.Nanoseconds(), "ttl = %d, want %d < %d", ttl, ttl, expiration.Nanoseconds())

	newExpiration := time.Hour
	nd.UpdateTTL(time.Now().Add(newExpiration))
	_, ttl = nd.expirationAndTTL(clockwork.NewFakeClock())
	assert.LessOrEqualf(t, ttl, newExpiration.Nanoseconds(), "ttl = %d, want %d < %d", ttl, ttl, newExpiration.Nanoseconds())
}

func TestNewDirChild(t *testing.T) {
	nd, child := newTestNodeDir()

	err := nd.Add(child)
	assert.Nilf(t, err, "nd.Add(child) err = %v, want = nil", err)
	assert.NotEmptyf(t, nd.Children, "len(nd.Children) = %d, want = 1", len(nd.Children))
	err = child.Remove(true, true, nil)
	assert.Nilf(t, err, "child.Remove err = %v, want = nil", err)
	assert.Emptyf(t, nd.Children, "len(nd.Children) = %d, want = 0", len(nd.Children))
}

func newTestNode() *node {
	nd := newKV(newStore(), key, val, 0, nil, time.Now().Add(expiration))
	return nd
}

func newTestNodeDir() (*node, *node) {
	s := newStore()
	nd := newDir(s, key, 0, nil, time.Now().Add(expiration))
	cKey, cVal := "hello", "world"
	child := newKV(s, cKey, cVal, 0, nd, time.Now().Add(expiration))
	return nd, child
}
