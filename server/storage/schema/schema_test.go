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
	"fmt"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.uber.org/zap"
)

var (
	V3_7 = semver.Version{Major: 3, Minor: 7}
)

func TestMigrate(t *testing.T) {
	tcs := []struct {
		name    string
		version semver.Version
		// Overrides which keys should be set (default based on version)
		overrideKeys  func(tx backend.BatchTx)
		targetVersion semver.Version

		expectVersion  *semver.Version
		expectError    bool
		expectErrorMsg string
	}{
		// As storage version field was added in v3.6, for v3.5 we will not set it.
		// For storage to be considered v3.5 it have both confstate and term key set.
		{
			name:           `Upgrading v3.5 to v3.6 should be rejected if confstate is not set`,
			version:        V3_5,
			overrideKeys:   func(tx backend.BatchTx) {},
			targetVersion:  V3_6,
			expectVersion:  nil,
			expectError:    true,
			expectErrorMsg: `cannot determine storage version: missing confstate information`,
		},
		{
			name:    `Upgrading v3.5 to v3.6 should be rejected if term is not set`,
			version: V3_5,
			overrideKeys: func(tx backend.BatchTx) {
				MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
			},
			targetVersion:  V3_6,
			expectVersion:  nil,
			expectError:    true,
			expectErrorMsg: `cannot determine storage version: missing term information`,
		},
		{
			name:          `Upgrading v3.5 to v3.6 should be succeed all required fields are set`,
			version:       V3_5,
			targetVersion: V3_6,
			expectVersion: &V3_6,
		},
		{
			name:          `Migrate on same v3.5 version passes and doesn't set storage version'`,
			version:       V3_5,
			targetVersion: V3_5,
			expectVersion: nil,
		},
		{
			name:          `Migrate on same v3.6 version passes`,
			version:       V3_6,
			targetVersion: V3_6,
			expectVersion: &V3_6,
		},
		{
			name:          `Migrate on same v3.7 version passes`,
			version:       V3_7,
			targetVersion: V3_7,
			expectVersion: &V3_7,
		},
		{
			name:           "Upgrading 3.6 to v3.7 is not supported",
			version:        V3_6,
			targetVersion:  V3_7,
			expectVersion:  &V3_6,
			expectError:    true,
			expectErrorMsg: `cannot create migration plan: version "3.7.0" is not supported`,
		},
		{
			name:           "Downgrading v3.7 to v3.6 is not supported",
			version:        V3_7,
			targetVersion:  V3_6,
			expectVersion:  &V3_7,
			expectError:    true,
			expectErrorMsg: `cannot create migration plan: downgrades are not yet supported`,
		},
		{
			name:           "Downgrading v3.6 to v3.5 is not supported",
			version:        V3_6,
			targetVersion:  V3_5,
			expectVersion:  &V3_6,
			expectError:    true,
			expectErrorMsg: `cannot create migration plan: downgrades are not yet supported`,
		},
		{
			name:           "Downgrading v3.5 to v3.4 is not supported",
			version:        V3_5,
			targetVersion:  V3_4,
			expectVersion:  nil,
			expectError:    true,
			expectErrorMsg: `cannot create migration plan: downgrades are not yet supported`,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zap.NewNop()
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			UnsafeCreateMetaBucket(tx)
			if tc.overrideKeys != nil {
				tc.overrideKeys(tx)
			} else {
				setupKeys(t, tx, tc.version)
			}
			tx.Unlock()
			be.ForceCommit()
			be.Close()

			b := backend.NewDefaultBackend(tmpPath)
			defer b.Close()
			err := Migrate(lg, b.BatchTx(), tc.targetVersion)
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
			initialVersion: V3_5,
			state: map[string]string{
				"confState":        `{"auto_leave":false}`,
				"consistent_index": "\x00\x00\x00\x00\x00\x00\x00\x01",
				"term":             "\x00\x00\x00\x00\x00\x00\x00\x01",
			},
		},
		{
			initialVersion: V3_6,
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
			be, _ := betesting.NewTmpBackend(t, time.Microsecond, 10)
			defer be.Close()
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			UnsafeCreateMetaBucket(tx)
			setupKeys(t, tx, tc.initialVersion)
			assertBucketState(t, tx, Meta, tc.state)
			tx.Unlock()

			// Upgrade to current version
			tx.Lock()
			err := testUnsafeMigrate(lg, be.BatchTx(), currentVersion)
			if err != nil {
				t.Errorf("Migrate(lg, tx, %q) returned error %+v", currentVersion, err)
			}
			assert.Equal(t, &currentVersion, UnsafeReadStorageVersion(tx))
			tx.Unlock()

			// Downgrade back to initial version
			tx.Lock()
			err = testUnsafeMigrate(lg, be.BatchTx(), tc.initialVersion)
			if err != nil {
				t.Errorf("Migrate(lg, tx, %q) returned error %+v", tc.initialVersion, err)
			}
			tx.Unlock()

			// Assert that all changes were revered
			tx.Lock()
			assertBucketState(t, tx, Meta, tc.state)
			tx.Unlock()
		})
	}
}

// Does the same as UnsafeMigrate but skips version checks
// TODO(serathius): Use UnsafeMigrate when downgrades are implemented
func testUnsafeMigrate(lg *zap.Logger, tx backend.BatchTx, target semver.Version) error {
	current, err := UnsafeDetectSchemaVersion(lg, tx)
	if err != nil {
		return fmt.Errorf("cannot determine storage version: %w", err)
	}
	plan, err := buildPlan(current, target)
	if err != nil {
		return fmt.Errorf("cannot create migration plan: %w", err)
	}
	return plan.unsafeExecute(lg, tx)
}

func setupKeys(t *testing.T, tx backend.BatchTx, ver semver.Version) {
	t.Helper()
	switch ver {
	case V3_4:
	case V3_5:
		MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
		UnsafeUpdateConsistentIndex(tx, 1, 1, false)
	case V3_6:
		MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
		UnsafeUpdateConsistentIndex(tx, 1, 1, false)
		UnsafeSetStorageVersion(tx, &V3_6)
	case V3_7:
		MustUnsafeSaveConfStateToBackend(zap.NewNop(), tx, &raftpb.ConfState{})
		UnsafeUpdateConsistentIndex(tx, 1, 1, false)
		UnsafeSetStorageVersion(tx, &V3_7)
		tx.UnsafePut(Meta, []byte("future-key"), []byte(""))
	default:
		t.Fatalf("Unsupported storage version")
	}
}
