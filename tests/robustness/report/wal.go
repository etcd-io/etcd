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

package report

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/raft/v3/raftpb"
)

func LoadClusterPersistedRequests(lg *zap.Logger, path string) ([]model.EtcdRequest, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	dataDirs := []string{}
	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), "server-") {
			dataDirs = append(dataDirs, filepath.Join(path, file.Name()))
		}
	}
	return PersistedRequests(lg, dataDirs)
}

func PersistedRequestsCluster(lg *zap.Logger, cluster *e2e.EtcdProcessCluster) ([]model.EtcdRequest, error) {
	dataDirs := []string{}
	for _, proc := range cluster.Procs {
		dataDirs = append(dataDirs, memberDataDir(proc))
	}
	return PersistedRequests(lg, dataDirs)
}

func PersistedRequests(lg *zap.Logger, dataDirs []string) ([]model.EtcdRequest, error) {
	return persistedRequests(lg, dataDirs, requestsPersistedInWAL)
}

func persistedRequests(lg *zap.Logger, dataDirs []string, reader persistedRequestReaderFunc) ([]model.EtcdRequest, error) {
	if len(dataDirs) == 0 {
		return nil, errors.New("no data dirs")
	}
	// Allow failure in minority of etcd cluster.
	allowedFailures := len(dataDirs) / 2
	memberRequestHistories := make([][]model.EtcdRequest, 0, len(dataDirs))
	for _, dir := range dataDirs {
		requests, err := reader(lg, dir)
		if err != nil {
			if allowedFailures < 1 {
				return nil, err
			}
			allowedFailures--
			continue
		}
		memberRequestHistories = append(memberRequestHistories, requests)
	}
	// Return empty history if all histories were empty/failed to read.
	if len(memberRequestHistories) == 0 {
		return []model.EtcdRequest{}, nil
	}
	// Each history collects votes from each history that it matches.
	votes := make([]int, len(memberRequestHistories))
	lastDiff := ""
	for i := 0; i < len(memberRequestHistories); i++ {
		for j := 0; j < len(memberRequestHistories); j++ {
			if i == j {
				// history votes for itself
				votes[i]++
				continue
			}
			if i > j {
				// avoid comparing things twice
				continue
			}
			first := memberRequestHistories[i]
			second := memberRequestHistories[j]
			minLength := min(len(first), len(second))
			if diff := cmp.Diff(first[:minLength], second[:minLength]); diff == "" {
				votes[i]++
				votes[j]++
			} else {
				lastDiff = diff
			}
		}
	}
	// Select longest history that has votes from quorum.
	longestHistory := []model.EtcdRequest{}
	quorum := len(dataDirs)/2 + 1
	foundQuorum := false
	for i := 0; i < len(memberRequestHistories); i++ {
		if votes[i] < quorum {
			continue
		}
		// There cannot be incompabible histories supported by quorum
		minLength := min(len(memberRequestHistories[i]), len(longestHistory))
		if diff := cmp.Diff(memberRequestHistories[i][:minLength], longestHistory[:minLength]); diff != "" {
			lastDiff = diff
			foundQuorum = false
			break
		}
		foundQuorum = true
		if len(memberRequestHistories[i]) > len(longestHistory) {
			longestHistory = memberRequestHistories[i]
		}
	}
	if !foundQuorum {
		if lastDiff != "" {
			fmt.Printf("Difference between WAL:\n%s", lastDiff) // zap doesn't nicely writes multiline strings like diff
		}
		return nil, errors.New("unexpected differences between wal entries")
	}
	return longestHistory, nil
}

type persistedRequestReaderFunc = func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error)

func requestsPersistedInWAL(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
	_, ents, err := ReadWAL(lg, dataDir)
	if err != nil {
		return nil, err
	}
	requests := make([]model.EtcdRequest, 0, len(ents))
	for _, ent := range ents {
		if ent.Type != raftpb.EntryNormal || len(ent.Data) == 0 {
			continue
		}
		request, err := parseEntryNormal(ent)
		if err != nil {
			return nil, err
		}
		if request != nil {
			requests = append(requests, *request)
		}
	}
	return requests, nil
}

func ReadWAL(lg *zap.Logger, dataDir string) (state raftpb.HardState, ents []raftpb.Entry, err error) {
	walDir := datadir.ToWALDir(dataDir)
	repaired := false
	for {
		w, err := wal.OpenForRead(lg, walDir, walpb.Snapshot{Index: 0})
		if err != nil {
			return state, nil, fmt.Errorf("failed to open WAL, err: %w", err)
		}
		_, state, ents, err = w.ReadAll()
		w.Close()
		if err != nil {
			if errors.Is(err, wal.ErrSnapshotNotFound) {
				lg.Info("Error occurred when reading WAL entries", zap.Error(err))
				return state, ents, nil
			}
			if errors.Is(err, wal.ErrSliceOutOfRange) {
				return state, nil, fmt.Errorf("failed to read WAL, err: %w", err)
			}
			// we can only repair ErrUnexpectedEOF and we never repair twice.
			if repaired || !errors.Is(err, io.ErrUnexpectedEOF) {
				return state, nil, fmt.Errorf("failed to read WAL, cannot be repaired, err: %w", err)
			}
			if !wal.Repair(lg, walDir) {
				return state, nil, fmt.Errorf("failed to repair WAL, err: %w", err)
			}
			lg.Info("repaired WAL", zap.Error(err))
			repaired = true
			continue
		}
		return state, ents, nil
	}
}

func parseEntryNormal(ent raftpb.Entry) (*model.EtcdRequest, error) {
	var raftReq pb.InternalRaftRequest
	if err := raftReq.Unmarshal(ent.Data); err != nil {
		var r pb.Request
		isV2Entry := pbutil.MaybeUnmarshal(&r, ent.Data)
		if !isV2Entry {
			return nil, err
		}
		return nil, nil
	}
	switch {
	case raftReq.Put != nil:
		op := model.PutOptions{
			Key:     string(raftReq.Put.Key),
			Value:   model.ToValueOrHash(string(raftReq.Put.Value)),
			LeaseID: raftReq.Put.Lease,
		}
		request := model.EtcdRequest{
			Type: model.Txn,
			Txn: &model.TxnRequest{
				OperationsOnSuccess: []model.EtcdOperation{
					{Type: model.PutOperation, Put: op},
				},
			},
		}
		return &request, nil
	case raftReq.DeleteRange != nil:
		op := model.DeleteOptions{Key: string(raftReq.DeleteRange.Key)}
		request := model.EtcdRequest{
			Type: model.Txn,
			Txn: &model.TxnRequest{
				OperationsOnSuccess: []model.EtcdOperation{
					{Type: model.DeleteOperation, Delete: op},
				},
			},
		}
		return &request, nil
	case raftReq.LeaseRevoke != nil:
		return &model.EtcdRequest{
			Type:        model.LeaseRevoke,
			LeaseRevoke: &model.LeaseRevokeRequest{LeaseID: raftReq.LeaseRevoke.ID},
		}, nil
	case raftReq.LeaseGrant != nil:
		return &model.EtcdRequest{
			Type:       model.LeaseGrant,
			LeaseGrant: &model.LeaseGrantRequest{LeaseID: raftReq.LeaseGrant.ID},
		}, nil
	case raftReq.ClusterMemberAttrSet != nil:
		return nil, nil
	case raftReq.ClusterVersionSet != nil:
		return nil, nil
	case raftReq.DowngradeInfoSet != nil:
		return nil, nil
	case raftReq.Compaction != nil:
		request := model.EtcdRequest{
			Type:    model.Compact,
			Compact: &model.CompactRequest{Revision: raftReq.Compaction.Revision},
		}
		return &request, nil
	case raftReq.Txn != nil:
		txn := model.TxnRequest{
			Conditions:          []model.EtcdCondition{},
			OperationsOnSuccess: []model.EtcdOperation{},
			OperationsOnFailure: []model.EtcdOperation{},
		}
		for _, cmp := range raftReq.Txn.Compare {
			switch {
			case cmp.Result == pb.Compare_EQUAL && cmp.Target == pb.Compare_VERSION:
				txn.Conditions = append(txn.Conditions, model.EtcdCondition{
					Key:             string(cmp.Key),
					ExpectedVersion: cmp.GetVersion(),
				})
			case cmp.Result == pb.Compare_EQUAL && cmp.Target == pb.Compare_MOD:
				txn.Conditions = append(txn.Conditions, model.EtcdCondition{
					Key:              string(cmp.Key),
					ExpectedRevision: cmp.GetModRevision(),
				})
			default:
				panic(fmt.Sprintf("unsupported condition: %+v", cmp))
			}
		}
		for _, op := range raftReq.Txn.Success {
			txn.OperationsOnSuccess = append(txn.OperationsOnSuccess, toEtcdOperation(op))
		}
		for _, op := range raftReq.Txn.Failure {
			txn.OperationsOnFailure = append(txn.OperationsOnFailure, toEtcdOperation(op))
		}
		request := model.EtcdRequest{
			Type: model.Txn,
			Txn:  &txn,
		}
		return &request, nil
	default:
		panic(fmt.Sprintf("Unhandled raft request: %+v", raftReq))
	}
}

func toEtcdOperation(op *pb.RequestOp) (operation model.EtcdOperation) {
	switch {
	case op.GetRequestRange() != nil:
		rangeOp := op.GetRequestRange()
		operation = model.EtcdOperation{
			Type: model.RangeOperation,
			Range: model.RangeOptions{
				Start: string(rangeOp.Key),
				End:   string(rangeOp.RangeEnd),
				Limit: rangeOp.Limit,
			},
		}
	case op.GetRequestPut() != nil:
		putOp := op.GetRequestPut()
		operation = model.EtcdOperation{
			Type: model.PutOperation,
			Put: model.PutOptions{
				Key:   string(putOp.Key),
				Value: model.ToValueOrHash(string(putOp.Value)),
			},
		}
	case op.GetRequestDeleteRange() != nil:
		deleteOp := op.GetRequestDeleteRange()
		operation = model.EtcdOperation{
			Type: model.DeleteOperation,
			Delete: model.DeleteOptions{
				Key: string(deleteOp.Key),
			},
		}
	default:
		panic(fmt.Sprintf("Unknown op type %v", op))
	}
	return operation
}
