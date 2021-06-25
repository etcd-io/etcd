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
	"fmt"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
)

var (
	V3_5 = semver.Version{Major: 3, Minor: 5}
	V3_6 = semver.Version{Major: 3, Minor: 6}
)

// UpdateStorageVersion updates storage version.
func UpdateStorageVersion(lg *zap.Logger, tx backend.BatchTx) error {
	tx.Lock()
	defer tx.Unlock()
	v, err := detectStorageVersion(lg, tx)
	if err != nil {
		return fmt.Errorf("cannot determine storage version: %w", err)
	}
	switch *v {
	case V3_5:
		lg.Warn("setting storage version", zap.String("storage-version", V3_6.String()))
		// All meta keys introduced in v3.6 should be filled in here.
		unsafeSetStorageVersion(tx, &V3_6)
	case V3_6:
	default:
		lg.Warn("unknown storage version", zap.String("storage-version", v.String()))
	}
	return nil
}

func detectStorageVersion(lg *zap.Logger, tx backend.ReadTx) (*semver.Version, error) {
	v := unsafeReadStorageVersion(tx)
	if v != nil {
		return v, nil
	}
	_, cfs := tx.UnsafeRange(buckets.Meta, buckets.MetaConfStateName, nil, 0)
	if len(cfs) == 0 {
		return nil, fmt.Errorf("missing %q key", buckets.MetaConfStateName)
	}
	_, ts := tx.UnsafeRange(buckets.Meta, buckets.MetaTermKeyName, nil, 0)
	if len(ts) == 0 {
		return nil, fmt.Errorf("missing %q key", buckets.MetaTermKeyName)
	}
	copied := V3_5
	return &copied, nil
}

// ReadStorageVersion loads storage version from given backend transaction.
// Populated since v3.6
func ReadStorageVersion(tx backend.ReadTx) *semver.Version {
	tx.Lock()
	defer tx.Unlock()
	return unsafeReadStorageVersion(tx)
}

// unsafeReadStorageVersion loads storage version from given backend transaction.
// Populated since v3.6
func unsafeReadStorageVersion(tx backend.ReadTx) *semver.Version {
	_, vs := tx.UnsafeRange(buckets.Meta, buckets.MetaStorageVersionName, nil, 1)
	if len(vs) == 0 {
		return nil
	}
	v, err := semver.NewVersion(string(vs[0]))
	if err != nil {
		return nil
	}
	return v
}

// ReadStorageVersionFromSnapshot loads storage version from given bbolt transaction.
// Populated since v3.6
func ReadStorageVersionFromSnapshot(tx *bbolt.Tx) *semver.Version {
	v := tx.Bucket(buckets.Meta.Name()).Get(buckets.MetaStorageVersionName)
	version, err := semver.NewVersion(string(v))
	if err != nil {
		return nil
	}
	return version
}

// unsafeSetStorageVersion updates etcd storage version in backend.
// Populated since v3.6
func unsafeSetStorageVersion(tx backend.BatchTx, v *semver.Version) {
	sv := semver.Version{Major: v.Major, Minor: v.Minor}
	tx.UnsafePut(buckets.Meta, buckets.MetaStorageVersionName, []byte(sv.String()))
}
