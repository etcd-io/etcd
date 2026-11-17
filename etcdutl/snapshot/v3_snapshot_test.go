// Copyright 2018 The etcd Authors
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

package snapshot

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// TestSnapshotStatus is the happy case.
// It inserts pre-defined number of keys and asserts the output hash of status command.
// The expected hash value must not be changed.
// If it changes, there must be some backwards incompatible change introduced.
func TestSnapshotStatus(t *testing.T) {
	dbpath := createDB(t, insertKeys(t, 10, 100))

	status, err := NewV3(zap.NewNop()).Status(dbpath)
	require.NoError(t, err)

	assert.Equal(t, uint32(0x4b84b1be), status.Hash)
	assert.Equal(t, int64(11), status.Revision)
}

// TestSnapshotStatusCorruptRevision tests if snapshot status command fails when there is an unexpected revision in "key" bucket.
func TestSnapshotStatusCorruptRevision(t *testing.T) {
	dbpath := createDB(t, insertKeys(t, 1, 0))

	db, err := bbolt.Open(dbpath, 0o600, nil)
	require.NoError(t, err)
	defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("key"))
		if b == nil {
			return errors.New("key bucket not found")
		}
		return b.Put([]byte("0"), []byte{})
	})
	require.NoError(t, err)
	db.Close()

	_, err = NewV3(zap.NewNop()).Status(dbpath)
	require.ErrorContains(t, err, "invalid revision length")
}

// TestSnapshotStatusNegativeRevisionMain tests if snapshot status command fails when main revision number is negative.
func TestSnapshotStatusNegativeRevisionMain(t *testing.T) {
	dbpath := createDB(t, insertKeys(t, 1, 0))

	db, err := bbolt.Open(dbpath, 0o666, nil)
	require.NoError(t, err)
	defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(schema.Key.Name())
		if b == nil {
			return errors.New("key bucket not found")
		}
		bytes := mvcc.NewRevBytes()
		mvcc.RevToBytes(mvcc.Revision{Main: -1}, bytes)
		return b.Put(bytes, []byte{})
	})
	require.NoError(t, err)
	db.Close()

	_, err = NewV3(zap.NewNop()).Status(dbpath)
	require.ErrorContains(t, err, "negative revision")
}

// TestSnapshotStatusNegativeRevisionSub tests if snapshot status command fails when sub revision number is negative.
func TestSnapshotStatusNegativeRevisionSub(t *testing.T) {
	dbpath := createDB(t, insertKeys(t, 1, 0))

	db, err := bbolt.Open(dbpath, 0o666, nil)
	require.NoError(t, err)
	defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("key"))
		if b == nil {
			return errors.New("key bucket not found")
		}
		bytes := mvcc.NewRevBytes()
		mvcc.RevToBytes(mvcc.Revision{Sub: -1}, bytes)
		return b.Put(bytes, []byte{})
	})
	require.NoError(t, err)
	db.Close()

	_, err = NewV3(zap.NewNop()).Status(dbpath)
	require.ErrorContains(t, err, "negative revision")
}

// TestSnapshotStatusTotalKey tests if snapshot status command correctly reports total number of valid keys.
func TestSnapshotStatusTotalKey(t *testing.T) {
	cases := []struct {
		name     string
		prepare  func(srv *etcdserver.EtcdServer)
		expected int
	}{
		{
			name: "duplicate keys",
			prepare: func(srv *etcdserver.EtcdServer) {
				keys := []string{"key1", "key2", "key1"}
				val := make([]byte, len(keys))
				for _, key := range keys {
					for i := 0; i < 3; i++ {
						req := etcdserverpb.PutRequest{
							Key:   []byte(key),
							Value: val,
						}
						_, err := srv.Put(t.Context(), &req)
						require.NoError(t, err)
					}
				}
			},
			expected: 2,
		},
		{
			name: "mixed revisions",
			prepare: func(srv *etcdserver.EtcdServer) {
				// key1: create -> put -> delete
				key := []byte("key1")
				for i := 0; i < 3; i++ {
					if i < 2 {
						_, err := srv.Put(t.Context(), &etcdserverpb.PutRequest{Key: key, Value: []byte(strconv.Itoa(i))})
						require.NoError(t, err)
					} else {
						_, err := srv.DeleteRange(t.Context(), &etcdserverpb.DeleteRangeRequest{Key: key})
						require.NoError(t, err)
					}
				}
			},
			expected: 0,
		},
		{
			name: "ignored tombstones",
			prepare: func(srv *etcdserver.EtcdServer) {
				// key1: create -> delete -> re-create -> delete
				key := []byte("key1")
				for i := 0; i < 2; i++ {
					_, err := srv.Put(t.Context(), &etcdserverpb.PutRequest{Key: key, Value: make([]byte, 1)})
					require.NoError(t, err)
					_, err = srv.DeleteRange(t.Context(), &etcdserverpb.DeleteRangeRequest{Key: key})
					require.NoError(t, err)
				}
			},
			expected: 0,
		},
		{
			name: "restored keys",
			prepare: func(srv *etcdserver.EtcdServer) {
				// key1: create -> delete -> re-create -> delete -> re-create
				key := []byte("key1")
				for i := 0; i < 5; i++ {
					if i%2 == 0 {
						_, err := srv.Put(t.Context(), &etcdserverpb.PutRequest{Key: key, Value: make([]byte, 1)})
						require.NoError(t, err)
					} else {
						_, err := srv.DeleteRange(t.Context(), &etcdserverpb.DeleteRangeRequest{Key: key})
						require.NoError(t, err)
					}
				}
			},
			expected: 1,
		},
		{
			name: "mixed deletions",
			prepare: func(srv *etcdserver.EtcdServer) {
				// Put("key1") -> Put("key2")-> Delete("key1")
				_, err := srv.Put(t.Context(), &etcdserverpb.PutRequest{Key: []byte("key1"), Value: make([]byte, 1)})
				require.NoError(t, err)
				_, err = srv.Put(t.Context(), &etcdserverpb.PutRequest{Key: []byte("key2"), Value: make([]byte, 1)})
				require.NoError(t, err)
				_, err = srv.DeleteRange(t.Context(), &etcdserverpb.DeleteRangeRequest{Key: []byte("key1")})
				require.NoError(t, err)
			},
			expected: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dbpath := createDB(t, tc.prepare)

			status, err := NewV3(zap.NewNop()).Status(dbpath)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, status.TotalKey)
		})
	}
}

func TestSnapshotRestoreRevisionBump(t *testing.T) {
	snapshotPath := createDB(t, insertKeys(t, 3, 0))
	status, err := NewV3(zap.NewNop()).Status(snapshotPath)
	require.NoError(t, err)
	require.Equal(t, int64(4), status.Revision)

	tests := []struct {
		name    string
		bump    uint64
		wantErr string
	}{
		{
			name:    "max uint64 bump",
			bump:    math.MaxUint64,
			wantErr: "exceeds maximum revision bump",
		},
		{
			name:    "bump does not fit in int64",
			bump:    uint64(math.MaxInt64) + 1,
			wantErr: "exceeds maximum revision bump",
		},
		{
			name:    "bump overflows latest revision",
			bump:    uint64(math.MaxInt64-status.Revision) + 1,
			wantErr: "would overflow latest revision",
		},
		{
			name: "valid bump",
			bump: 2,
		},
		{
			name: "bump to max int64",
			bump: uint64(math.MaxInt64 - status.Revision),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, markCompacted := range []bool{false, true} {
				t.Run("markCompacted="+strconv.FormatBool(markCompacted), func(t *testing.T) {
					outputDir := t.TempDir()
					outputDataDir := filepath.Join(outputDir, "restore")
					err := NewV3(zap.NewNop()).Restore(RestoreConfig{
						SnapshotPath:   snapshotPath,
						Name:           "default",
						OutputDataDir:  outputDataDir,
						OutputWALDir:   filepath.Join(outputDir, "wal"),
						PeerURLs:       []string{"http://localhost:2380"},
						InitialCluster: "default=http://localhost:2380",
						SkipHashCheck:  true,
						RevisionBump:   tt.bump,
						MarkCompacted:  markCompacted,
					})
					if tt.wantErr != "" {
						assert.ErrorContains(t, err, tt.wantErr)
						entries, readErr := os.ReadDir(outputDir)
						require.NoError(t, readErr)
						assert.Empty(t, entries, "invalid revision bump must not write to the destination")
						return
					}

					require.NoError(t, err)
					restoredStatus, err := NewV3(zap.NewNop()).Status(filepath.Join(outputDataDir, "member", "snap", "db"))
					require.NoError(t, err)
					expectedRevision := status.Revision
					if markCompacted {
						expectedRevision += int64(tt.bump)
					}
					require.Equal(t, expectedRevision, restoredStatus.Revision)
				})
			}
		})
	}
}

// insertKeys insert `numKeys` number of keys of `valueSize` size into a running etcd server.
func insertKeys(t *testing.T, numKeys, valueSize int) func(*etcdserver.EtcdServer) {
	t.Helper()
	return func(srv *etcdserver.EtcdServer) {
		val := make([]byte, valueSize)
		for i := 0; i < numKeys; i++ {
			req := etcdserverpb.PutRequest{
				Key:   []byte(strconv.Itoa(i)),
				Value: val,
			}
			_, err := srv.Put(t.Context(), &req)
			require.NoError(t, err)
		}
	}
}

// createDB creates a bbolt database file by running an embedded etcd server.
// While the server is running, `generateContent` function is called to insert values.
// It returns the path of bbolt database.
func createDB(t *testing.T, generateContent func(*etcdserver.EtcdServer)) string {
	t.Helper()

	cfg := embed.NewConfig()
	cfg.BackendBatchLimit = 1
	cfg.LogLevel = "fatal"
	cfg.Dir = t.TempDir()

	etcd, err := embed.StartEtcd(cfg)
	require.NoError(t, err)
	defer etcd.Close()

	select {
	case <-etcd.Server.ReadyNotify():
	case <-time.After(10 * time.Second):
		t.FailNow()
	}

	generateContent(etcd.Server)

	return filepath.Join(cfg.Dir, "member", "snap", "db")
}
