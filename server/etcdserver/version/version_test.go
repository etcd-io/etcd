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

package version

import (
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	betesting "go.etcd.io/etcd/server/v3/mvcc/backend/testing"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
)

func TestUpdateStorageVersion(t *testing.T) {
	tcs := []struct {
		name             string
		version          string
		metaKeys         [][]byte
		expectVersion    *semver.Version
		expectError      bool
		expectedErrorMsg string
		expectedMetaKeys [][]byte
	}{
		{
			name:             `Backend before 3.6 without "confState" should be rejected`,
			version:          "",
			expectVersion:    nil,
			expectError:      true,
			expectedErrorMsg: `cannot determine storage version: missing "confState" key`,
		},
		{
			name:             `Backend before 3.6 without "term" should be rejected`,
			version:          "",
			metaKeys:         [][]byte{buckets.MetaConfStateName},
			expectVersion:    nil,
			expectError:      true,
			expectedErrorMsg: `cannot determine storage version: missing "term" key`,
			expectedMetaKeys: [][]byte{buckets.MetaConfStateName},
		},
		{
			name:             "Backend with 3.5 with all metadata keys should be upgraded to v3.6",
			version:          "",
			metaKeys:         [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName},
			expectVersion:    &semver.Version{Major: 3, Minor: 6},
			expectedMetaKeys: [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName, buckets.MetaStorageVersionName},
		},
		{
			name:             "Backend in 3.6.0 should be skipped",
			version:          "3.6.0",
			metaKeys:         [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName, buckets.MetaStorageVersionName},
			expectVersion:    &semver.Version{Major: 3, Minor: 6},
			expectedMetaKeys: [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName, buckets.MetaStorageVersionName},
		},
		{
			name:             "Backend with current version should be skipped",
			version:          version.Version,
			metaKeys:         [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName, buckets.MetaStorageVersionName},
			expectVersion:    &semver.Version{Major: 3, Minor: 6},
			expectedMetaKeys: [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName, buckets.MetaStorageVersionName},
		},
		{
			name:             "Backend in 3.7.0 should be skipped",
			version:          "3.7.0",
			metaKeys:         [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName, buckets.MetaStorageVersionName, []byte("future-key")},
			expectVersion:    &semver.Version{Major: 3, Minor: 7},
			expectedMetaKeys: [][]byte{buckets.MetaTermKeyName, buckets.MetaConfStateName, buckets.MetaStorageVersionName, []byte("future-key")},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			tx.UnsafeCreateBucket(buckets.Meta)
			for _, k := range tc.metaKeys {
				tx.UnsafePut(buckets.Meta, k, []byte{})
			}
			if tc.version != "" {
				unsafeSetStorageVersion(tx, semver.New(tc.version))
			}
			tx.Unlock()
			be.ForceCommit()
			be.Close()

			b := backend.NewDefaultBackend(tmpPath)
			defer b.Close()
			err := UpdateStorageVersion(zap.NewNop(), b.BatchTx())
			if (err != nil) != tc.expectError {
				t.Errorf("UpgradeStorage(...) = %+v, expected error: %v", err, tc.expectError)
			}
			if err != nil && err.Error() != tc.expectedErrorMsg {
				t.Errorf("UpgradeStorage(...) = %q, expected error message: %q", err, tc.expectedErrorMsg)
			}
			v := unsafeReadStorageVersion(b.BatchTx())
			assert.Equal(t, tc.expectVersion, v)
			keys, _ := b.BatchTx().UnsafeRange(buckets.Meta, []byte("a"), []byte("z"), 0)
			assert.ElementsMatch(t, tc.expectedMetaKeys, keys)
		})
	}
}

// TestVersion ensures that unsafeSetStorageVersion/unsafeReadStorageVersion work well together.
func TestVersion(t *testing.T) {
	tcs := []struct {
		version       string
		expectVersion string
	}{
		{
			version:       "3.5.0",
			expectVersion: "3.5.0",
		},
		{
			version:       "3.5.0-alpha",
			expectVersion: "3.5.0",
		},
		{
			version:       "3.5.0-beta.0",
			expectVersion: "3.5.0",
		},
		{
			version:       "3.5.0-rc.1",
			expectVersion: "3.5.0",
		},
		{
			version:       "3.5.1",
			expectVersion: "3.5.0",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.version, func(t *testing.T) {
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			tx.UnsafeCreateBucket(buckets.Meta)
			unsafeSetStorageVersion(tx, semver.New(tc.version))
			tx.Unlock()
			be.ForceCommit()
			be.Close()

			b := backend.NewDefaultBackend(tmpPath)
			defer b.Close()
			v := unsafeReadStorageVersion(b.BatchTx())

			assert.Equal(t, tc.expectVersion, v.String())
		})
	}
}

// TestVersionSnapshot ensures that unsafeSetStorageVersion/unsafeReadStorageVersionFromSnapshot work well together.
func TestVersionSnapshot(t *testing.T) {
	tcs := []struct {
		version       string
		expectVersion string
	}{
		{
			version:       "3.5.0",
			expectVersion: "3.5.0",
		},
		{
			version:       "3.5.0-alpha",
			expectVersion: "3.5.0",
		},
		{
			version:       "3.5.0-beta.0",
			expectVersion: "3.5.0",
		},
		{
			version:       "3.5.0-rc.1",
			expectVersion: "3.5.0",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.version, func(t *testing.T) {
			be, tmpPath := betesting.NewTmpBackend(t, time.Microsecond, 10)
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			tx.UnsafeCreateBucket(buckets.Meta)
			unsafeSetStorageVersion(tx, semver.New(tc.version))
			tx.Unlock()
			be.ForceCommit()
			be.Close()
			db, err := bolt.Open(tmpPath, 0400, &bolt.Options{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			var ver *semver.Version
			if err = db.View(func(tx *bolt.Tx) error {
				ver = ReadStorageVersionFromSnapshot(tx)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tc.expectVersion, ver.String())

		})
	}
}
