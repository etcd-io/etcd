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
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
	"gotest.tools/v3/assert"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/server/v3/storage/backend"
)

// TestRunHashWithTimeout_Success verifies that runHashWithTimeout returns the
// correct hash when the database is available within the timeout.
func TestRunHashWithTimeout_Success(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create and close an empty db so getHash can open it.
	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	b.Close()

	_, err := runHashWithTimeout(dbPath, 5*time.Second)
	assert.NilError(t, err)
}

// TestRunHashWithTimeout_Timeout verifies that runHashWithTimeout returns
// context.DeadlineExceeded when the database lock cannot be acquired within
// the given timeout. This is the core fix for
// https://github.com/etcd-io/etcd/issues/20276.
func TestRunHashWithTimeout_Timeout(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "locked.db")

	// Create the db file.
	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	b.Close()

	// Hold an exclusive bbolt lock so that getHash blocks waiting for it.
	db, err := bolt.Open(dbPath, 0o600, &bolt.Options{})
	assert.NilError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = runHashWithTimeout(dbPath, 200*time.Millisecond)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestRunHashWithTimeout_ZeroTimeout verifies that passing timeout=0 causes
// runHashWithTimeout to wait indefinitely (no deadline), succeeding once the
// db is available. We simulate this with an immediately available db.
func TestRunHashWithTimeout_ZeroTimeout(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_zero.db")

	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	b.Close()

	_, err := runHashWithTimeout(dbPath, 0)
	assert.NilError(t, err)
}
