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

package correctness

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// RevocableModelLeaseID is pre-granted in setupTestStorage for deterministic LeaseRevoke scenarios.
const RevocableModelLeaseID = int64(999)

func TestStorageCorrectness(t *testing.T) {
	s := setupFuzzTestStorage(t)
	report := SimulateTraffic(t, s, DefaultTrafficConfig)
	Validate(t, s, report)
}

func TestStorageModelAgainstBackend(t *testing.T) {
	for _, tc := range commonModelScenarios {
		t.Run(tc.name, func(t *testing.T) {
			s := setupTestStorage(t)
			testStorageModelAgainstBackend(t, s, tc.operations)
		})
	}
}

func testStorageModelAgainstBackend(t *testing.T, s *TestStorage, operations []modelTestOp) {
	modelState := StoragePorcupineModel.Init()
	for opIdx, op := range operations {
		actualRes := ExecuteOp(s, op.req)

		ok, newState := StoragePorcupineModel.Step(modelState, op.req, actualRes)
		if !ok {
			t.Fatalf("Model rejected actual response on op #%d: %s. State: %s",
				opIdx, StoragePorcupineModel.DescribeOperation(op.req, actualRes), StoragePorcupineModel.DescribeState(modelState))
		}
		modelState = newState

		diffOpts := []cmp.Option{
			cmpopts.IgnoreUnexported(mvccpb.KeyValue{}),
			cmpopts.EquateErrors(),
			cmp.Comparer(bytes.Equal),
		}
		if op.res.Hash == 0 {
			diffOpts = append(diffOpts, cmpopts.IgnoreFields(StorageResponse{}, "Hash"))
		}
		if diff := cmp.Diff(op.res, actualRes, diffOpts...); diff != "" {
			t.Fatalf("Op #%d (%s) response mismatch (-want +got):\n%s",
				opIdx, StoragePorcupineModel.DescribeOperation(op.req, actualRes), diff)
		}
	}
}

func setupTestStorage(t *testing.T) *TestStorage {
	t.Helper()
	lg := zap.NewNop()
	bcfg := backend.DefaultBackendConfig(lg)
	bcfg.Path = filepath.Join(t.TempDir(), "bbolt.db")
	return setupTestStorageWithConfigs(t, bcfg, mvcc.StoreConfig{})
}

func setupFuzzTestStorage(t *testing.T) *TestStorage {
	t.Helper()
	lg := zap.NewNop()
	bcfg := backend.DefaultBackendConfig(lg)
	bcfg.Path = filepath.Join(t.TempDir(), "bbolt.db")
	// Lower BatchLimit and enable NonBlockingDefrag so batch auto-flushes and
	// LockForSafeRangeDelete compaction/defrag exclusion trigger during a 2s fuzz run.
	bcfg.BatchLimit = 25
	bcfg.BatchInterval = 25 * time.Millisecond
	bcfg.NonBlockingDefrag = true

	scfg := mvcc.StoreConfig{
		CompactionBatchLimit:    10,
		CompactionSleepInterval: time.Millisecond,
	}
	return setupTestStorageWithConfigs(t, bcfg, scfg)
}

func setupTestStorageWithConfigs(t *testing.T, bcfg backend.BackendConfig, scfg mvcc.StoreConfig) *TestStorage {
	t.Helper()
	lg := zap.NewNop()
	be := backend.New(bcfg)

	tx := be.BatchTx()
	tx.Lock()
	for _, b := range schema.AllBuckets {
		tx.UnsafeCreateBucket(b)
	}
	tx.Unlock()
	be.ForceCommit()

	le := lease.NewLessor(lg, be, nil, lease.LessorConfig{MinLeaseTTL: 3600})
	st := mvcc.New(lg, be, le, scfg)

	for _, lid := range append(append([]int64(nil), DefaultLeaseIDs...), RevocableModelLeaseID) {
		_, err := le.Grant(lease.LeaseID(lid), 3600)
		require.NoError(t, err)
	}
	be.ForceCommit()

	t.Cleanup(func() {
		_ = st.Close()
		le.Stop()
		_ = be.Close()
	})
	return &TestStorage{
		Store:   st,
		Backend: be,
		Lessor:  le,
	}
}
