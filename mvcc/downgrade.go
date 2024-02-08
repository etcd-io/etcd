// Copyright 2024 The etcd Authors
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

package mvcc

import (
	"encoding/binary"
	"encoding/json"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/mvcc/backend"
	"go.etcd.io/etcd/raft/raftpb"
	"go.etcd.io/etcd/version"
	"go.uber.org/zap"
)

var (
	// meta bucket keys since 3.5
	confStateKeyName = []byte("confState")
	termKeyName      = []byte("term")
	// meta bucket keys since 3.6
	storageVersionKeyName = []byte("storageVersion")
)

// unsafeDowngradeMetaBucket delete 3.5 specific keys to make backend fully compatible with 3.4
func unsafeDowngradeMetaBucket(lg *zap.Logger, tx backend.BatchTx) {
	if lg != nil {
		lg.Info(
			"downgrade meta bucket",
			zap.ByteStrings("remove-keys", [][]byte{confStateKeyName, termKeyName}),
		)
	} else {
		plog.Printf("downgrade meta bucket: remove keys [%s, %s]", string(confStateKeyName), string(termKeyName))
	}
	tx.UnsafeDelete(metaBucketName, confStateKeyName)
	tx.UnsafeDelete(metaBucketName, termKeyName)
}

// unsafeReadStorageVersion loads storage version from given backend transaction.
// Populated since v3.6
func unsafeReadStorageVersion(tx backend.ReadTx) *semver.Version {
	_, vs := tx.UnsafeRange(metaBucketName, storageVersionKeyName, nil, 0)
	if len(vs) == 0 {
		return nil
	}
	v, err := semver.NewVersion(string(vs[0]))
	if err != nil {
		return nil
	}
	return v
}

// unsafeReadTerm loads term from given transaction.
// returns 0 if the data are not found.
// Term is persisted since v3.5.
func unsafeReadTerm(tx backend.ReadTx) uint64 {
	_, ts := tx.UnsafeRange(metaBucketName, termKeyName, nil, 0)
	if len(ts) == 0 {
		return 0
	}
	return binary.BigEndian.Uint64(ts[0])
}

// unsafeConfStateFromBackend retrieves ConfState from the backend.
// Returns nil if confState in backend is not persisted (e.g. backend writen by <v3.5).
func unsafeConfStateFromBackend(lg *zap.Logger, tx backend.ReadTx) *raftpb.ConfState {
	keys, vals := tx.UnsafeRange(metaBucketName, confStateKeyName, nil, 0)
	if len(keys) == 0 {
		return nil
	}

	if len(keys) != 1 {
		if lg != nil {
			lg.Panic(
				"unexpected number of key: "+string(confStateKeyName)+" when getting cluster version from backend",
				zap.Int("number-of-key", len(keys)),
			)
		} else {
			plog.Panicf("unexpected number of key: %s when getting cluster version from backend, number-of-key=%d",
				string(confStateKeyName), len(keys))
		}
	}
	var confState raftpb.ConfState
	if err := json.Unmarshal(vals[0], &confState); err != nil {
		if lg != nil {
			lg.Panic(
				"cannot unmarshal confState json retrieved from the backend",
				zap.ByteString("conf-state-json", vals[0]),
				zap.Error(err),
			)
		} else {
			plog.Panicf("cannot unmarshal confState json retrieved from the backend, err: %s", err)
		}
	}
	return &confState
}

// UnsafeDetectSchemaVersion returns version of storage schema. Returned value depends on etcd version that created the backend. For
// * v3.6 and newer will return storage version.
// * v3.5 will return it's version if it includes any storage fields added in v3.5.
// * v3.4 will return it's version if it doesn't include any storage fields added in v3.5.
// Note that data schema older than 3.4 will also return v3.4, which means the data will be read as it is and will not be modified
// by the downgrade code.
func UnsafeDetectSchemaVersion(lg *zap.Logger, tx backend.ReadTx) semver.Version {
	vp := unsafeReadStorageVersion(tx)
	if vp != nil {
		return *vp
	}

	confstate := unsafeConfStateFromBackend(lg, tx)
	term := unsafeReadTerm(tx)
	if confstate == nil && term == 0 {
		// if both confstate and term are missing, assume it's v3.4
		return version.V3_4
	}
	return version.V3_5
}
