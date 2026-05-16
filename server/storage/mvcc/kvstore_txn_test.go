// Copyright 2025 The etcd Authors
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

package mvcc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

// TestUnmarshalKVSkipValue verifies that unmarshalKVSkipValue correctly decodes
// all KeyValue fields except Value, which must always be nil (never allocated).
func TestUnmarshalKVSkipValue(t *testing.T) {
	original := &mvccpb.KeyValue{
		Key:            []byte("test-key"),
		CreateRevision: 42,
		ModRevision:    77,
		Version:        3,
		Value:          []byte("large-value-bytes-that-should-not-be-allocated"),
		Lease:          99,
	}

	b, err := proto.Marshal(original)
	require.NoError(t, err)

	got := &mvccpb.KeyValue{}
	require.NoError(t, unmarshalKVSkipValue(b, got))

	assert.Equal(t, original.Key, got.Key, "Key must be preserved")
	assert.Equal(t, original.CreateRevision, got.CreateRevision, "CreateRevision must be preserved")
	assert.Equal(t, original.ModRevision, got.ModRevision, "ModRevision must be preserved")
	assert.Equal(t, original.Version, got.Version, "Version must be preserved")
	assert.Nil(t, got.Value, "Value must be nil (never allocated)")
	assert.Equal(t, original.Lease, got.Lease, "Lease must be preserved")
}

// TestUnmarshalKVSkipValueEmptyValue confirms that a KV with no value field
// (e.g. a deletion tombstone) is decoded correctly.
func TestUnmarshalKVSkipValueEmptyValue(t *testing.T) {
	original := &mvccpb.KeyValue{
		Key:         []byte("tombstone-key"),
		ModRevision: 10,
	}

	b, err := proto.Marshal(original)
	require.NoError(t, err)

	got := &mvccpb.KeyValue{}
	require.NoError(t, unmarshalKVSkipValue(b, got))

	assert.Equal(t, original.Key, got.Key)
	assert.Equal(t, original.ModRevision, got.ModRevision)
	assert.Nil(t, got.Value)
}

// TestRangeKeysOnlyDoesNotLoadValues is an end-to-end test verifying that
// a range query with RangeOptions.KeysOnly=true returns KVs with nil Value
// while still populating all metadata fields correctly.
func TestRangeKeysOnlyDoesNotLoadValues(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend(t)
	s := NewStore(zaptest.NewLogger(t), be, &lease.FakeLessor{}, StoreConfig{})
	defer cleanup(s, be)

	// Store several keys with distinct large values.
	keys := [][]byte{
		[]byte("key1"),
		[]byte("key2"),
		[]byte("key3"),
	}
	vals := [][]byte{
		[]byte("value-for-key1"),
		[]byte("value-for-key2"),
		[]byte("value-for-key3"),
	}
	for i, k := range keys {
		s.Put(k, vals[i], lease.NoLease)
	}
	s.Commit()

	// Range with KeysOnly=true: values must not be loaded.
	rr, err := s.Range(t.Context(), []byte("key1"), []byte("key4"), RangeOptions{KeysOnly: true})
	require.NoError(t, err)
	require.Len(t, rr.KVs, len(keys))

	for i, kv := range rr.KVs {
		assert.Equal(t, keys[i], kv.Key, "key %d: Key must match", i)
		assert.Nil(t, kv.Value, "key %d: Value must be nil for KeysOnly range", i)
		assert.Greater(t, kv.ModRevision, int64(0), "key %d: ModRevision must be populated", i)
		assert.Greater(t, kv.CreateRevision, int64(0), "key %d: CreateRevision must be populated", i)
		assert.Greater(t, kv.Version, int64(0), "key %d: Version must be populated", i)
	}

	// Range without KeysOnly: values must be present.
	rr2, err := s.Range(t.Context(), []byte("key1"), []byte("key4"), RangeOptions{})
	require.NoError(t, err)
	require.Len(t, rr2.KVs, len(keys))

	for i, kv := range rr2.KVs {
		assert.Equal(t, vals[i], kv.Value, "key %d: Value must be present for non-KeysOnly range", i)
	}
}
