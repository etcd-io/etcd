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

package etcdutl

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
)

// FlockTimeout is the duration to wait to obtain a file lock on db file.
var FlockTimeout time.Duration

func GetLogger() *zap.Logger {
	config := logutil.DefaultZapLoggerConfig
	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	lg, err := config.Build()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}
	return lg
}

func getLatestWALSnap(lg *zap.Logger, dataDir string) (walpb.Snapshot, error) {
	walPath := datadir.ToWALDir(dataDir)
	walSnaps, err := wal.ValidSnapshotEntries(lg, walPath)
	if err != nil {
		return walpb.Snapshot{}, err
	}

	if len(walSnaps) > 0 {
		lastIdx := len(walSnaps) - 1
		return walSnaps[lastIdx], nil
	}
	return walpb.Snapshot{}, nil
}

// SimpleLessor is a simplified implementation of Lessor interface.
// Used by etcdutl tools to simulate Lessor behavior without full lease management
type SimpleLessor struct {
	LeaseSet map[lease.LeaseID]struct{}
}

var _ lease.Lessor = (*SimpleLessor)(nil)

func (sl *SimpleLessor) SetRangeDeleter(dr lease.RangeDeleter) {}

func (sl *SimpleLessor) SetCheckpointer(cp lease.Checkpointer) {}

func (sl *SimpleLessor) Grant(id lease.LeaseID, ttl int64) (*lease.Lease, error) {
	sl.LeaseSet[id] = struct{}{}
	return nil, nil
}

func (sl *SimpleLessor) Revoke(id lease.LeaseID) error { return nil }

func (sl *SimpleLessor) Checkpoint(id lease.LeaseID, remainingTTL int64) error { return nil }

func (sl *SimpleLessor) Attach(id lease.LeaseID, items []lease.LeaseItem) error { return nil }

func (sl *SimpleLessor) GetLease(item lease.LeaseItem) lease.LeaseID            { return 0 }
func (sl *SimpleLessor) Detach(id lease.LeaseID, items []lease.LeaseItem) error { return nil }

func (sl *SimpleLessor) Promote(extend time.Duration) {}

func (sl *SimpleLessor) Demote() {}

func (sl *SimpleLessor) Renew(id lease.LeaseID) (int64, error) { return 10, nil }

func (sl *SimpleLessor) Lookup(id lease.LeaseID) *lease.Lease {
	if _, ok := sl.LeaseSet[id]; ok {
		return &lease.Lease{ID: id}
	}
	return nil
}

func (sl *SimpleLessor) Leases() []*lease.Lease { return nil }

func (sl *SimpleLessor) ExpiredLeasesC() <-chan []*lease.Lease { return nil }

func (sl *SimpleLessor) Recover(b backend.Backend, rd lease.RangeDeleter) {}

func (sl *SimpleLessor) Stop() {}
