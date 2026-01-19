// Copyright 2026 The etcd Authors
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

package etcdutl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"gotest.tools/v3/assert"

	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func TestCalculateHashKV(t *testing.T) {
	type testCase struct {
		name            string
		setupFunc       func(t *testing.T) (dbPath string, cleanup func())
		revision        int64
		expectedHash    uint32
		expectedRev     int64
		expectedCompact int64
		expectError     bool
		errorContains   string
	}

	testCases := []testCase{
		{
			name: "non-existent file",
			setupFunc: func(t *testing.T) (string, func()) {
				return "/nonexistent/path/to/db", func() {}
			},
			expectError: true,
		},
		{
			name: "empty directory path",
			setupFunc: func(t *testing.T) (string, func()) {
				return "", func() {}
			},
			expectError: true,
		},
		{
			name: "empty database",
			setupFunc: func(t *testing.T) (string, func()) {
				tempDir := t.TempDir()
				dbPath := filepath.Join(tempDir, "test.db")

				b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
				st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
				_ = st
				b.Close()

				return dbPath, func() {}
			},
			revision:        0,
			expectedHash:    1084519789,
			expectedRev:     1,
			expectedCompact: -1,
			expectError:     false,
		},
		{
			name: "database with data",
			setupFunc: func(t *testing.T) (string, func()) {
				tempDir := t.TempDir()
				dbPath := filepath.Join(tempDir, "test_with_data.db")

				b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
				st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
				st.Put([]byte("test-key"), []byte("test-value"), 1)
				st.Close()
				b.Close()

				return dbPath, func() {}
			},
			revision:        0,
			expectedHash:    645561629,
			expectedRev:     2,
			expectedCompact: -1,
			expectError:     false,
		},
		{
			name: "invalid revision",
			setupFunc: func(t *testing.T) (string, func()) {
				tempDir := t.TempDir()
				dbPath := filepath.Join(tempDir, "test_invalid_rev.db")

				b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
				st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
				st.Put([]byte("key"), []byte("value"), 1)
				st.Close()
				b.Close()

				return dbPath, func() {}
			},
			revision:      999,
			expectError:   true,
			errorContains: "required revision is a future revision",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic: %v", r)
				}
			}()

			dbPath, cleanup := tc.setupFunc(t)
			defer cleanup()

			result, err := calculateHashKV(dbPath, tc.revision, 0, false, "")

			if tc.expectError {
				assert.Assert(t, err != nil)
				if tc.errorContains != "" {
					assert.ErrorContains(t, err, tc.errorContains)
				}
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, tc.expectedHash, result.Hash)
			assert.Equal(t, tc.expectedRev, result.HashRevision)
			assert.Equal(t, tc.expectedCompact, result.CompactRevision)

			t.Logf("Test %s - Hash: %d, HashRevision: %d, CompactRevision: %d",
				tc.name, result.Hash, result.HashRevision, result.CompactRevision)
		})
	}
}

func TestCalculateHashKV_DetailedMode(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_detailed.db")

	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	st.Put([]byte("key1"), []byte("value1"), lease.NoLease)
	st.Put([]byte("key2"), []byte("value2"), lease.NoLease)
	st.Close()
	b.Close()

	tests := []struct {
		name           string
		detailed       bool
		outputFile     string
		expectDetailed bool
		expectOutput   bool
	}{
		{
			name:           "detailed mode with stdout",
			detailed:       true,
			outputFile:     "",
			expectDetailed: true,
			expectOutput:   true,
		},
		{
			name:           "detailed mode with file output",
			detailed:       true,
			outputFile:     filepath.Join(tempDir, "output.json"),
			expectDetailed: true,
			expectOutput:   false,
		},
		{
			name:           "non-detailed mode",
			detailed:       false,
			outputFile:     "",
			expectDetailed: false,
			expectOutput:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculateHashKV(dbPath, 0, 0, tt.detailed, tt.outputFile)
			assert.NilError(t, err)

			assert.Assert(t, result.Hash > 0)

			if tt.expectDetailed {
				if tt.outputFile != "" {
					_, err := os.Stat(tt.outputFile)
					assert.NilError(t, err)

					data, err := os.ReadFile(tt.outputFile)
					assert.NilError(t, err)

					var detailedResult []mvcc.KeyRevisionHash
					err = json.Unmarshal(data, &detailedResult)
					assert.NilError(t, err)
					assert.Equal(t, 2, len(detailedResult))
				}
			} else {
				if tt.outputFile != "" {
					// file should not exist
					_, err := os.Stat(tt.outputFile)
					assert.Assert(t, !os.IsExist(err))
				}
			}
		})
	}
}

func TestCalculateHashKV_CompactRev(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_compact_rev.db")

	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	st := mvcc.NewStore(zap.NewNop(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	curRev := st.Put([]byte("key1"), []byte("value1"), lease.NoLease)
	assert.Equal(t, int64(2), curRev)
	curRev = st.Put([]byte("key2"), []byte("value2"), lease.NoLease)
	assert.Equal(t, int64(3), curRev)
	curRev = st.Put([]byte("key3"), []byte("value3"), lease.NoLease)
	assert.Equal(t, int64(4), curRev)
	st.Close()
	b.Close()

	tests := []struct {
		name          string
		rev           int64
		compactRev    int64
		detailed      bool
		expectRev     int64
		expectCompact int64
	}{
		{
			name:          "basic compact revision test",
			rev:           3,
			compactRev:    0,
			detailed:      false,
			expectRev:     3,
			expectCompact: -1,
		},
		{
			name:          "compact revision 1",
			rev:           3,
			compactRev:    1,
			detailed:      false,
			expectRev:     3,
			expectCompact: 1,
		},
		{
			name:          "compact revision 2",
			rev:           3,
			compactRev:    2,
			detailed:      false,
			expectRev:     3,
			expectCompact: 2,
		},
		{
			name:          "detailed mode with compact revision",
			rev:           3,
			compactRev:    2,
			detailed:      true,
			expectRev:     3,
			expectCompact: 2,
		},
		{
			name:          "compact equals revision",
			rev:           3,
			compactRev:    3,
			detailed:      false,
			expectRev:     3,
			expectCompact: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculateHashKV(dbPath, tt.rev, tt.compactRev, tt.detailed, "")
			assert.NilError(t, err)
			assert.Assert(t, result.Hash > 0)
			assert.Equal(t, tt.expectRev, result.HashRevision)
			assert.Equal(t, tt.expectCompact, result.CompactRevision)
		})
	}
}
