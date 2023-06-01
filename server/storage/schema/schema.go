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
	"encoding/json"
	"fmt"

	"github.com/coreos/go-semver/semver"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	serverversion "go.etcd.io/etcd/server/v3/etcdserver/version"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/raft/v3/raftpb"
)

// Validate checks provided backend to confirm that schema used is supported.
func Validate(lg *zap.Logger, tx backend.ReadTx) error {
	tx.RLock()
	defer tx.RUnlock()
	return unsafeValidate(lg, tx)
}

func unsafeValidate(lg *zap.Logger, tx backend.UnsafeReader) error {
	current, err := UnsafeDetectSchemaVersion(lg, tx)
	if err != nil {
		// v3.5 requires a wal snapshot to persist its fields, so we can assign it a schema version.
		lg.Warn("Failed to detect storage schema version. Please wait till wal snapshot before upgrading cluster.")
		return nil
	}
	_, err = newPlan(lg, current, localBinaryVersion())
	return err
}

func localBinaryVersion() semver.Version {
	v := semver.New(version.Version)
	return semver.Version{Major: v.Major, Minor: v.Minor}
}

type WALVersion interface {
	// MinimalEtcdVersion returns minimal etcd version able to interpret WAL log.
	MinimalEtcdVersion() *semver.Version
}

// Migrate updates storage schema to provided target version.
// Downgrading requires that provided WAL doesn't contain unsupported entries.
func Migrate(lg *zap.Logger, tx backend.BatchTx, w WALVersion, target semver.Version) error {
	tx.LockOutsideApply()
	defer tx.Unlock()
	return UnsafeMigrate(lg, tx, w, target)
}

// UnsafeMigrate is non thread-safe version of Migrate.
func UnsafeMigrate(lg *zap.Logger, tx backend.UnsafeReadWriter, w WALVersion, target semver.Version) error {
	current, err := UnsafeDetectSchemaVersion(lg, tx)
	if err != nil {
		return fmt.Errorf("cannot detect storage schema version: %v", err)
	}
	plan, err := newPlan(lg, current, target)
	if err != nil {
		return fmt.Errorf("cannot create migration plan: %v", err)
	}
	if target.LessThan(current) {
		minVersion := w.MinimalEtcdVersion()
		if minVersion != nil && target.LessThan(*minVersion) {
			return fmt.Errorf("cannot downgrade storage, WAL contains newer entries")
		}
	}
	return plan.unsafeExecute(lg, tx)
}

// DetectSchemaVersion returns version of storage schema. Returned value depends on etcd version that created the backend. For
// * v3.6 and newer will return storage version.
// * v3.5 will return it's version if it includes all storage fields added in v3.5 (might require a snapshot).
// * v3.4 will return it's version if it doesn't include all storage fields added in v3.5.
func DetectSchemaVersion(lg *zap.Logger, tx backend.ReadTx) (v semver.Version, err error) {
	tx.RLock()
	defer tx.RUnlock()
	return UnsafeDetectSchemaVersion(lg, tx)
}

// UnsafeDetectSchemaVersion non-threadsafe version of DetectSchemaVersion.
func UnsafeDetectSchemaVersion(lg *zap.Logger, tx backend.UnsafeReader) (v semver.Version, err error) {
	vp := UnsafeReadStorageVersion(tx)
	if vp != nil {
		return *vp, nil
	}

	confstate := UnsafeConfStateFromBackend(lg, tx)
	_, term := UnsafeReadConsistentIndex(tx)
	if confstate == nil && term == 0 {
		// if both confstate and term are missing, assume it's v3.4
		return version.V3_4, nil
	} else if confstate == nil {
		return v, fmt.Errorf("missing confstate information")
	} else if len(confstate.Voters) == 0 && term == 0 {
		// if confstate is empty and term is missing, assume it's v3.5 that was migrated from v3.4 and never started
		return version.V3_5, nil
	} else if term == 0 {
		return v, fmt.Errorf("missing term information")
	}
	return version.V3_5, nil
}

func schemaChangesForVersion(v semver.Version, isUpgrade bool) ([]schemaChange, error) {
	// changes should be taken from higher version
	var higherV = v
	if isUpgrade {
		higherV = semver.Version{Major: v.Major, Minor: v.Minor + 1}
	}

	actions, found := schemaChanges[higherV]
	if !found {
		if isUpgrade {
			return nil, fmt.Errorf("version %q is not supported", higherV.String())
		}
		return nil, fmt.Errorf("version %q is not supported", v.String())
	}
	return actions, nil
}

var (
	// schemaChanges list changes that were introduced in a particular version.
	// schema was introduced in v3.6 as so its changes were not tracked before.
	schemaChanges = map[semver.Version][]schemaChange{
		version.V3_6: {
			// emptyValue is used for v3.6 Step for the first time, in all other version StoragetVersion should be set by migrator.
			addNewField(Meta, MetaStorageVersionName, emptyValue),
		},
		version.V3_5: {
			// UnsafeReadConsistentIndex will fail on []byte(""), use 0 as default
			addNewField(Meta, MetaTermKeyName, emptyTerm),
			// UnsafeConfStateFromBackend will fail on []byte(""), use empty struct as default
			addNewField(Meta, MetaConfStateName, emptyConfState),
			// DowngradeInfoFromBackend will fail on []byte(""), false is better default
			addNewField(Cluster, ClusterDowngradeKeyName, falseDowngradeInfo),
		},
	}

	emptyValue            = []byte("")
	emptyTerm             = make([]byte, 8)
	emptyConfState, _     = json.Marshal(raftpb.ConfState{})
	falseDowngradeInfo, _ = json.Marshal(serverversion.DowngradeInfo{Enabled: false, TargetVersion: ""})
)
