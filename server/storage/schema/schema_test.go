// Copyright 2021 The etcd Authors
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

package schema

import (
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/wal"
	waltesting "go.etcd.io/etcd/server/v3/storage/wal/testing"
	"go.etcd.io/raft/v3/raftpb"
)

func TestValidate(t *testing.T) {
	tcs := []struct {
		name    string
		version semver.Version
		// Overrides which keys should be set (default based on version)
		overrideKeys   func(tx backend.UnsafeReadWriter)
		expectError    bool
		expectErrorMsg string
	}{
		// As storage version field was added in v3.6, for v3.5 we will not set it.
		// For storage to be considered v3.5 it have both confstate and term key set.
		{
			name:    `V3.4 schema is correct`,
			version: version.V3_4,
		},
		{
			name:         `V3.5 schema without confstate and term fields is correct`,
			version:      version.V3_5,
			overrideKeys: func(tx backend.UnsafeReadWriter) {},
		},
		{
			name:    `V3.5 schema without term field is correct`,
			version: version.V3_5,
			overrideKeys: func(tx backend.UnsafeReadWriter) {
				MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
			},
		},
		{
			name:    `V3.5 schema with all fields is correct`,
			version: version.V3_5,
			overrideKeys: func(tx backend.UnsafeReadWriter) {
				MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
				UnsafeUpdateConsistentIndex(tx, 1, 1)
			},
		},
		{
			name:    `V3.6 schema is correct`,
			version: version.V3_6,
		},
		{
			name:           `V3.7 schema is unknown and should return error`,
			version:        version.V3_7,
			expectError:    true,
			expectErrorMsg: `version "3.7.0" is not supported`,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zap.NewNop()
			dataPath := setupBackendData(t, tc.version, tc.overrideKeys)

			b := backend.NewDefaultBackend(lg, dataPath)
			defer b.Close()
			err := Validate(lg, b.ReadTx())
			if (err != nil) != tc.expectError {
				t.Errorf("Validate(lg, tx) = %+v, expected error: %v", err, tc.expectError)
			}
			if err != nil && err.Error() != tc.expectErrorMsg {
				t.Errorf("Validate(lg, tx) = %q, expected error message: %q", err, tc.expectErrorMsg)
			}
		})
	}
}

func TestMigrate(t *testing.T) {
	tcs := []struct {
		name    string
		version semver.Version
		// Overrides which keys should be set (default based on version)
		overrideKeys  func(tx backend.UnsafeReadWriter)
		targetVersion semver.Version
		walEntries    []etcdserverpb.InternalRaftRequest

		expectVersion  *semver.Version
		expectError    bool
		expectErrorMsg string
	}{
		// As storage version field was added in v3.6, for v3.5 we will not set it.
		// For storage to be considered v3.5 it have both confstate and term key set.
		{
			name:           `Upgrading v3.5 to v3.6 should be rejected if confstate is not set`,
			version:        version.V3_5,
			overrideKeys:   func(tx backend.UnsafeReadWriter) {},
			targetVersion:  version.V3_6,
			expectVersion:  nil,
			expectError:    true,
			expectErrorMsg: `cannot detect storage schema version: missing confstate information`,
		},
		{
			name:    `Upgrading v3.5 to v3.6 should be rejected if term is not set`,
			version: version.V3_5,
			overrideKeys: func(tx backend.UnsafeReadWriter) {
				MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
			},
			targetVersion:  version.V3_6,
			expectVersion:  nil,
			expectError:    true,
			expectErrorMsg: `cannot detect storage schema version: missing term information`,
		},
		{
			name:          `Upgrading v3.5 to v3.6 should succeed; all required fields are set`,
			version:       version.V3_5,
			targetVersion: version.V3_6,
			expectVersion: &version.V3_6,
		},
		{
			name:          `Migrate on same v3.5 version passes and doesn't set storage version'`,
			version:       version.V3_5,
			targetVersion: version.V3_5,
			expectVersion: nil,
		},
		{
			name:          `Migrate on same v3.6 version passes`,
			version:       version.V3_6,
			targetVersion: version.V3_6,
			expectVersion: &version.V3_6,
		},
		{
			name:          `Migrate on same v3.7 version passes`,
			version:       version.V3_7,
			targetVersion: version.V3_7,
			expectVersion: &version.V3_7,
		},
		{
			name:           "Upgrading 3.6 to v3.7 is not supported",
			version:        version.V3_6,
			targetVersion:  version.V3_7,
			expectVersion:  &version.V3_6,
			expectError:    true,
			expectErrorMsg: `cannot create migration plan: version "3.7.0" is not supported`,
		},
		{
			name:           "Downgrading v3.7 to v3.6 is not supported",
			version:        version.V3_7,
			targetVersion:  version.V3_6,
			expectVersion:  &version.V3_7,
			expectError:    true,
			expectErrorMsg: `cannot create migration plan: version "3.7.0" is not supported`,
		},
		{
			name:          "Downgrading v3.6 to v3.5 works as there are no v3.6 wal entries",
			version:       version.V3_6,
			targetVersion: version.V3_5,
			walEntries: []etcdserverpb.InternalRaftRequest{
				{Range: &etcdserverpb.RangeRequest{Key: []byte("\x00"), RangeEnd: []byte("\xff")}},
			},
			expectVersion: nil,
		},
		{
			name:          "Downgrading v3.6 to v3.5 fails if there are newer WAL entries",
			version:       version.V3_6,
			targetVersion: version.V3_5,
			walEntries: []etcdserverpb.InternalRaftRequest{
				{ClusterVersionSet: &membershippb.ClusterVersionSetRequest{Ver: "3.6.0"}},
			},
			expectVersion:  &version.V3_6,
			expectError:    true,
			expectErrorMsg: "cannot downgrade storage, WAL contains newer entries",
		},
		{
			name:           "Downgrading v3.5 to v3.4 is not supported as schema was introduced in v3.6",
			version:        version.V3_5,
			targetVersion:  version.V3_4,
			expectVersion:  nil,
			expectError:    true,
			expectErrorMsg: `cannot create migration plan: version "3.5.0" is not supported`,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zap.NewNop()
			dataPath := setupBackendData(t, tc.version, tc.overrideKeys)
			w, _ := waltesting.NewTmpWAL(t, tc.walEntries)
			defer w.Close()
			walVersion, err := wal.ReadWALVersion(w)
			if err != nil {
				t.Fatal(err)
			}

			b := backend.NewDefaultBackend(lg, dataPath)
			defer b.Close()

			err = Migrate(lg, b.BatchTx(), walVersion, tc.targetVersion)
			if (err != nil) != tc.expectError {
				t.Errorf("Migrate(lg, tx, %q) = %+v, expected error: %v", tc.targetVersion, err, tc.expectError)
			}
			if err != nil && err.Error() != tc.expectErrorMsg {
				t.Errorf("Migrate(lg, tx, %q) = %q, expected error message: %q", tc.targetVersion, err, tc.expectErrorMsg)
			}
			v := UnsafeReadStorageVersion(b.BatchTx())
			assert.Equal(t, tc.expectVersion, v)
		})
	}
}

func TestMigrateIsReversible(t *testing.T) {
	tcs := []struct {
		initialVersion semver.Version
		state          map[string]string
	}{
		{
			initialVersion: version.V3_5,
			state: map[string]string{
				"confState":        `{"auto_leave":false}`,
				"consistent_index": "\x00\x00\x00\x00\x00\x00\x00\x01",
				"term":             "\x00\x00\x00\x00\x00\x00\x00\x01",
			},
		},
		{
			initialVersion: version.V3_6,
			state: map[string]string{
				"confState":        `{"auto_leave":false}`,
				"consistent_index": "\x00\x00\x00\x00\x00\x00\x00\x01",
				"term":             "\x00\x00\x00\x00\x00\x00\x00\x01",
				"storageVersion":   "3.6.0",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.initialVersion.String(), func(t *testing.T) {
			lg := zap.NewNop()
			dataPath := setupBackendData(t, tc.initialVersion, nil)

			be := backend.NewDefaultBackend(lg, dataPath)
			defer be.Close()
			tx := be.BatchTx()
			tx.Lock()
			defer tx.Unlock()
			assertBucketState(t, tx, Meta, tc.state)
			w, walPath := waltesting.NewTmpWAL(t, nil)
			walVersion, err := wal.ReadWALVersion(w)
			if err != nil {
				t.Fatal(err)
			}

			// Upgrade to current version
			ver := localBinaryVersion()
			err = UnsafeMigrate(lg, tx, walVersion, ver)
			if err != nil {
				t.Errorf("Migrate(lg, tx, %q) returned error %+v", ver, err)
			}
			assert.Equal(t, &ver, UnsafeReadStorageVersion(tx))

			// Downgrade back to initial version
			w.Close()
			w = waltesting.Reopen(t, walPath)
			defer w.Close()
			walVersion, err = wal.ReadWALVersion(w)
			if err != nil {
				t.Fatal(err)
			}
			err = UnsafeMigrate(lg, tx, walVersion, tc.initialVersion)
			if err != nil {
				t.Errorf("Migrate(lg, tx, %q) returned error %+v", tc.initialVersion, err)
			}

			// Assert that all changes were revered
			assertBucketState(t, tx, Meta, tc.state)
		})
	}
}

func setupBackendData(t *testing.T, ver semver.Version, overrideKeys func(tx backend.UnsafeReadWriter)) string {
	t.Helper()
	be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
	tx := be.BatchTx()
	if tx == nil {
		t.Fatal("batch tx is nil")
	}
	tx.Lock()
	UnsafeCreateMetaBucket(tx)
	if overrideKeys != nil {
		overrideKeys(tx)
	} else {
		switch ver {
		case version.V3_4:
		case version.V3_5:
			MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
			UnsafeUpdateConsistentIndex(tx, 1, 1)
		case version.V3_6:
			MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
			UnsafeUpdateConsistentIndex(tx, 1, 1)
			UnsafeSetStorageVersion(tx, &version.V3_6)
		case version.V3_7:
			MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
			UnsafeUpdateConsistentIndex(tx, 1, 1)
			UnsafeSetStorageVersion(tx, &version.V3_7)
			tx.UnsafePut(Meta, []byte("future-key"), []byte(""))
		default:
			t.Fatalf("Unsupported storage version")
		}
	}
	tx.Unlock()
	be.ForceCommit()
	be.Close()
	return tmpPath
}
