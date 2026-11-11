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

package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func TestMergeClientReports(t *testing.T) {
	op1 := porcupine.Operation{ClientId: 0, Call: 1, Return: 2}
	op2 := porcupine.Operation{ClientId: 0, Call: 3, Return: 4}
	op3 := porcupine.Operation{ClientId: 1, Call: 5, Return: 6}

	watch1 := model.WatchOperation{Request: model.WatchRequest{Key: "k1"}}
	watch2 := model.WatchOperation{Request: model.WatchRequest{Key: "k2"}}

	t.Run("both empty", func(t *testing.T) {
		merged := mergeClientReports(nil, nil)
		require.Empty(t, merged)
	})

	t.Run("existing empty, new non-empty", func(t *testing.T) {
		newReports := []ClientReport{{ClientID: 1, KeyValue: []porcupine.Operation{op3}}}
		merged := mergeClientReports(nil, newReports)
		require.Equal(t, newReports, merged)
	})

	t.Run("existing non-empty, new empty", func(t *testing.T) {
		existing := []ClientReport{{ClientID: 0, KeyValue: []porcupine.Operation{op1}}}
		merged := mergeClientReports(existing, nil)
		require.Equal(t, existing, merged)
	})

	t.Run("disjoint client IDs preserved and sorted", func(t *testing.T) {
		existing := []ClientReport{{ClientID: 2, KeyValue: []porcupine.Operation{op3}}}
		newReports := []ClientReport{{ClientID: 0, KeyValue: []porcupine.Operation{op1}}}
		merged := mergeClientReports(existing, newReports)
		require.Len(t, merged, 2)
		require.Equal(t, 0, merged[0].ClientID)
		require.Equal(t, 2, merged[1].ClientID)
	})

	t.Run("overlapping client IDs merged", func(t *testing.T) {
		existing := []ClientReport{{ClientID: 0, KeyValue: []porcupine.Operation{op1}, Watch: []model.WatchOperation{watch1}}}
		newReports := []ClientReport{{ClientID: 0, KeyValue: []porcupine.Operation{op2}, Watch: []model.WatchOperation{watch2}}}
		merged := mergeClientReports(existing, newReports)
		require.Len(t, merged, 1)
		require.Equal(t, 0, merged[0].ClientID)
		require.Equal(t, []porcupine.Operation{op1, op2}, merged[0].KeyValue)
		require.Equal(t, []model.WatchOperation{watch1, watch2}, merged[0].Watch)
	})
}

func TestSaveEtcdDataPreservesAndMergesClientReports(t *testing.T) {
	lg := zaptest.NewLogger(t)
	reportDir := t.TempDir()

	// 1. Write an initial client report (ClientID: 0) as written by entrypoint step
	h0 := model.NewAppendableHistory(identity.NewIDProvider())
	baseTime := time.Now()
	start := time.Since(baseTime)
	stop := start + time.Millisecond
	h0.AppendRange("key", "", 0, 0, false, start, stop, &clientv3.GetResponse{
		Header: &etcdserverpb.ResponseHeader{Revision: 1},
		Count:  0,
		Kvs:    []*mvccpb.KeyValue{},
	}, nil)

	initialReports := []ClientReport{
		{
			ClientID: 0,
			KeyValue: h0.Operations(),
		},
	}
	err := PersistClientReports(lg, reportDir, initialReports)
	require.NoError(t, err)

	// 2. Set up dummy server data dir to test server data persistence
	serverDir := t.TempDir()
	err = os.WriteFile(filepath.Join(serverDir, "dummy.wal"), []byte("wal-data"), 0o600)
	require.NoError(t, err)

	serverDataPaths := map[string]string{
		"server-1": serverDir,
	}

	// 3. Create TestReport and simulate traffic client reports (ClientID: 1)
	r, err := NewTestReport(lg, reportDir, serverDataPaths, &TrafficDetail{ExpectUniqueRevision: true})
	require.NoError(t, err)

	h1 := model.NewAppendableHistory(identity.NewIDProvider())
	start = time.Since(baseTime)
	stop = start + time.Millisecond
	h1.AppendPut("key", "val", start, stop, &clientv3.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	trafficReports := []ClientReport{
		{
			ClientID: 1,
			KeyValue: h1.Operations(),
		},
	}
	r.SetClientReports(trafficReports)

	// 4. SaveEtcdData should preserve initial ClientID 0 and merge with ClientID 1
	err = r.SaveEtcdData()
	require.NoError(t, err)

	loadedReports, err := LoadClientReports(reportDir)
	require.NoError(t, err)
	require.Len(t, loadedReports, 2)
	require.Equal(t, 0, loadedReports[0].ClientID)
	require.Equal(t, 1, loadedReports[1].ClientID)

	if diff := cmp.Diff(initialReports[0].KeyValue, loadedReports[0].KeyValue, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Client 0 reports mismatch: %s", diff)
	}
	if diff := cmp.Diff(trafficReports[0].KeyValue, loadedReports[1].KeyValue, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Client 1 reports mismatch: %s", diff)
	}

	// Verify server data was preserved
	serverWalContent, err := os.ReadFile(filepath.Join(reportDir, "server-server-1", "dummy.wal"))
	require.NoError(t, err)
	require.Equal(t, []byte("wal-data"), serverWalContent)

	// Verify traffic detail was persisted
	td, err := LoadTrafficDetail(reportDir)
	require.NoError(t, err)
	require.True(t, td.ExpectUniqueRevision)

	// Repeated call should be idempotent
	err = r.SaveEtcdData()
	require.NoError(t, err)
}

func TestSaveEtcdDataWithoutPriorReports(t *testing.T) {
	lg := zaptest.NewLogger(t)
	reportDir := filepath.Join(t.TempDir(), "nonexistent_dir")

	r, err := NewTestReport(lg, reportDir, nil, nil)
	require.NoError(t, err)

	h := model.NewAppendableHistory(identity.NewIDProvider())
	baseTime := time.Now()
	start := time.Since(baseTime)
	stop := start + time.Millisecond
	h.AppendPut("foo", "bar", start, stop, &clientv3.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: 2}}, nil)

	reports := []ClientReport{
		{
			ClientID: 1,
			KeyValue: h.Operations(),
		},
	}
	r.SetClientReports(reports)

	err = r.SaveEtcdData()
	require.NoError(t, err)

	loaded, err := LoadClientReports(reportDir)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, 1, loaded[0].ClientID)
}
