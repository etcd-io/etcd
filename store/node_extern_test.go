package store

import (
	"testing"
	"time"
)
import "github.com/coreos/etcd/Godeps/_workspace/src/github.com/stretchr/testify/assert"

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
		val  = "some_data"
		valp = &val
		exp  = time.Unix(12345, 67890)
		expp = &exp
	)

	eNode = &NodeExtern{
		Key:           key,
		TTL:           ttl,
		CreatedIndex:  ci,
		ModifiedIndex: mi,
		Value:         valp,
		Expiration:    expp,
	}

	gNode := eNode.Clone()
	// Check the clone is as expected
	assert.Equal(t, gNode.Key, key)
	assert.Equal(t, gNode.TTL, ttl)
	assert.Equal(t, gNode.CreatedIndex, ci)
	assert.Equal(t, gNode.ModifiedIndex, mi)
	// values should be the same
	assert.Equal(t, *gNode.Value, val)
	assert.Equal(t, *gNode.Expiration, exp)
	// but pointers should differ
	if gNode.Value == eNode.Value {
		t.Fatalf("expected value pointers to differ, but got same!")
	}
	if gNode.Expiration == eNode.Expiration {
		t.Fatalf("expected expiration pointers to differ, but got same!")
	}
	// Original should be the same
	assert.Equal(t, eNode.Key, key)
	assert.Equal(t, eNode.TTL, ttl)
	assert.Equal(t, eNode.CreatedIndex, ci)
	assert.Equal(t, eNode.ModifiedIndex, mi)
	assert.Equal(t, eNode.Value, valp)
	assert.Equal(t, eNode.Expiration, expp)
	// Change the clone and ensure the original is not affected
	gNode.Key = "/baz"
	gNode.TTL = 0
	assert.Equal(t, eNode.Key, key)
	assert.Equal(t, eNode.TTL, ttl)
	assert.Equal(t, eNode.CreatedIndex, ci)
	assert.Equal(t, eNode.ModifiedIndex, mi)
	// Change the original and ensure the clone is not affected
	eNode.Key = "/wuf"
	assert.Equal(t, eNode.Key, "/wuf")
	assert.Equal(t, gNode.Key, "/baz")
}
