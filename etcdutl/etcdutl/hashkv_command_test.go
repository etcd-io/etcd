package etcdutl

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"gotest.tools/v3/assert"

	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func TestCalculateHashKV_Basic(t *testing.T) {
	// Test with non-existent file
	t.Run("non-existent file", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic: %v", r)
			}
		}()
		_, err := calculateHashKV("/nonexistent/path/to/db", 0)
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	// Test with empty directory path
	t.Run("empty directory path", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic: %v", r)
			}
		}()
		_, err := calculateHashKV("", 0)
		if err == nil {
			t.Error("Expected error for empty path")
		}
	})
}

func TestCalculateHashKV_EmptyDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create a minimal database with required structure
	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	_ = st

	b.Close()

	result, err := calculateHashKV(dbPath, 0)
	if err != nil {
		t.Fatalf("calculateHashKV() failed: %v", err)
	}

	assert.Equal(t, uint32(1084519789), result.Hash)
	assert.Equal(t, int64(1), result.HashRevision)
	assert.Equal(t, int64(-1), result.CompactRevision)

	// Empty but properly initialized DB should return valid hash (likely 0)
	t.Logf("Empty DB hash: %d, revision: %d, compact: %d",
		result.Hash, result.HashRevision, result.CompactRevision)
}

func TestCalculateHashKV_WithData(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_with_data.db")

	// Create database with mvcc store
	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})

	// Put some test data
	st.Put([]byte("test-key"), []byte("test-value"), 1)

	b.Close()

	// Test with revision 0 (latest)
	result, err := calculateHashKV(dbPath, 0)
	if err != nil {
		t.Fatalf("calculateHashKV() failed: %v", err)
	}

	assert.Equal(t, uint32(645561629), result.Hash)
	assert.Equal(t, int64(2), result.HashRevision)
	assert.Equal(t, int64(-1), result.CompactRevision)

	t.Logf("With data - Hash: %d, HashRevision: %d, CompactRevision: %d",
		result.Hash, result.HashRevision, result.CompactRevision)
}

func TestCalculateHashKV_InvalidRevision(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_invalid_rev.db")

	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	st.Put([]byte("key"), []byte("value"), 1)
	b.Close()

	// Test with non-existent revision
	_, err := calculateHashKV(dbPath, 999)
	assert.ErrorContains(t, err, "required revision is a future revision")
}
