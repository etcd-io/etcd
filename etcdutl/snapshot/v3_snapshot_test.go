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
	"context"
	"errors"
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

	assert.Equal(t, uint32(0x62132b4d), status.Hash)
	assert.Equal(t, int64(11), status.Revision)
}

// TestSnapshotStatusCorruptRevision tests if snapshot status command fails when there is an unexpected revision in "key" bucket.
func TestSnapshotStatusCorruptRevision(t *testing.T) {
	dbpath := createDB(t, insertKeys(t, 1, 0))

	db, err := bbolt.Open(dbpath, 0600, nil)
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

	db, err := bbolt.Open(dbpath, 0666, nil)
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

	db, err := bbolt.Open(dbpath, 0666, nil)
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
			_, err := srv.Put(context.TODO(), &req)
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
