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
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNodeExternClone(t *testing.T) {
	var eNode *NodeExtern
	if g := eNode.Clone(); g != nil {
		t.Fatalf("nil.Clone=%v, want nil", g)
	}

	const (
		key string = "/foo/bar"
		ttl int64  = 123456789
		ci  uint64 = 123
		mi  uint64 = 321
	)
	var (
		val    = "some_data"
		valp   = &val
		exp    = time.Unix(12345, 67890)
		expp   = &exp
		child  = NodeExtern{}
		childp = &child
		childs = []*NodeExtern{childp}
	)

	eNode = &NodeExtern{
		Key:           key,
		TTL:           ttl,
		CreatedIndex:  ci,
		ModifiedIndex: mi,
		Value:         valp,
		Expiration:    expp,
		Nodes:         childs,
	}

	gNode := eNode.Clone()
	// Check the clone is as expected
	assert.Equal(t, key, gNode.Key)
	assert.Equal(t, ttl, gNode.TTL)
	assert.Equal(t, ci, gNode.CreatedIndex)
	assert.Equal(t, mi, gNode.ModifiedIndex)
	// values should be the same
	assert.Equal(t, val, *gNode.Value)
	assert.Equal(t, exp, *gNode.Expiration)
	assert.Len(t, gNode.Nodes, len(childs))
	assert.Equal(t, child, *gNode.Nodes[0])
	// but pointers should differ
	if gNode.Value == eNode.Value {
		t.Fatalf("expected value pointers to differ, but got same!")
	}
	if gNode.Expiration == eNode.Expiration {
		t.Fatalf("expected expiration pointers to differ, but got same!")
	}
	if sameSlice(gNode.Nodes, eNode.Nodes) {
		t.Fatalf("expected nodes pointers to differ, but got same!")
	}
	// Original should be the same
	assert.Equal(t, key, eNode.Key)
	assert.Equal(t, ttl, eNode.TTL)
	assert.Equal(t, ci, eNode.CreatedIndex)
	assert.Equal(t, mi, eNode.ModifiedIndex)
	assert.Equal(t, valp, eNode.Value)
	assert.Equal(t, expp, eNode.Expiration)
	if !sameSlice(eNode.Nodes, childs) {
		t.Fatalf("expected nodes pointer to same, but got different!")
	}
	// Change the clone and ensure the original is not affected
	gNode.Key = "/baz"
	gNode.TTL = 0
	gNode.Nodes[0].Key = "uno"
	assert.Equal(t, key, eNode.Key)
	assert.Equal(t, ttl, eNode.TTL)
	assert.Equal(t, ci, eNode.CreatedIndex)
	assert.Equal(t, mi, eNode.ModifiedIndex)
	assert.Equal(t, child, *eNode.Nodes[0])
	// Change the original and ensure the clone is not affected
	eNode.Key = "/wuf"
	assert.Equal(t, "/wuf", eNode.Key)
	assert.Equal(t, "/baz", gNode.Key)
}

func sameSlice(a, b []*NodeExtern) bool {
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	return va.Len() == vb.Len() && va.Pointer() == vb.Pointer()
}
