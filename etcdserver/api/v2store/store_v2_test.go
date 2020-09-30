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

// +build !v2v3

package v2store_test

import (
	"testing"

	"go.etcd.io/etcd/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/v3/pkg/testutil"
)

type v2TestStore struct {
	v2store.Store
}

func (s *v2TestStore) Close() {}

func newTestStore(t *testing.T, ns ...string) StoreCloser {
	if len(ns) == 0 {
		t.Logf("new v2 store with no namespace")
	}
	return &v2TestStore{v2store.New(ns...)}
}

// Ensure that the store can recover from a previously saved state.
func TestStoreRecover(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 4
	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/x", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Update("/foo/x", "barbar", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	b, err := s.Save()
	testutil.AssertNil(t, err)

	s2 := newTestStore(t)
	s2.Recovery(b)

	e, err := s.Get("/foo/x", false, false)
	testutil.AssertEqual(t, e.Node.CreatedIndex, uint64(2))
	testutil.AssertEqual(t, e.Node.ModifiedIndex, uint64(3))
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, *e.Node.Value, "barbar")

	e, err = s.Get("/foo/y", false, false)
	testutil.AssertEqual(t, e.EtcdIndex, eidx)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, *e.Node.Value, "baz")
}
