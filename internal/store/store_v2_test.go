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

package store_test

import (
	"testing"

	"github.com/coreos/etcd/internal/store"
	"github.com/coreos/etcd/pkg/testutil"
)

type v2TestStore struct {
	store.Store
}

func (s *v2TestStore) Close() {}

func newTestStore(t *testing.T, ns ...string) StoreCloser {
	return &v2TestStore{store.New(ns...)}
}

// Ensure that the store can recover from a previously saved state.
func TestStoreRecover(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	var eidx uint64 = 4
	s.Create("/foo", true, "", false, store.TTLOptionSet{ExpireTime: store.Permanent})
	s.Create("/foo/x", false, "bar", false, store.TTLOptionSet{ExpireTime: store.Permanent})
	s.Update("/foo/x", "barbar", store.TTLOptionSet{ExpireTime: store.Permanent})
	s.Create("/foo/y", false, "baz", false, store.TTLOptionSet{ExpireTime: store.Permanent})
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
