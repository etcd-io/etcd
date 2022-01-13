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

package v2store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

// Ensure that the store can recover from a previously saved state.
func TestStoreRecover(t *testing.T) {
	integration2.BeforeTest(t)
	s := v2store.New()
	var eidx uint64 = 4
	s.Create("/foo", true, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/x", false, "bar", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Update("/foo/x", "barbar", v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	s.Create("/foo/y", false, "baz", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	b, err := s.Save()
	testutil.AssertNil(t, err)

	s2 := v2store.New()
	s2.Recovery(b)

	e, err := s.Get("/foo/x", false, false)
	assert.Equal(t, e.Node.CreatedIndex, uint64(2))
	assert.Equal(t, e.Node.ModifiedIndex, uint64(3))
	assert.Equal(t, e.EtcdIndex, eidx)
	testutil.AssertNil(t, err)
	assert.Equal(t, *e.Node.Value, "barbar")

	e, err = s.Get("/foo/y", false, false)
	assert.Equal(t, e.EtcdIndex, eidx)
	testutil.AssertNil(t, err)
	assert.Equal(t, *e.Node.Value, "baz")
}
