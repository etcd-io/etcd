// Copyright 2025 The etcd Authors
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

package apply

import (
	"go.uber.org/zap"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/pkg/v3/wait"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/raft/v3/raftpb"
)

func Apply(lg *zap.Logger, e *raftpb.Entry, uberApply UberApplier, w wait.Wait, shouldApplyV3 membership.ShouldApplyV3) (ar *Result, id uint64) {
	var raftReq pb.InternalRaftRequest
	pbutil.MustUnmarshal(&raftReq, e.Data)
	lg.Debug("Apply", zap.Stringer("raftReq", &raftReq))

	id = raftReq.ID
	if id == 0 {
		if raftReq.Header == nil {
			lg.Panic("Apply, could not find a header")
		}
		id = raftReq.Header.ID
	}

	needResult := w.IsRegistered(id)
	if needResult || !noSideEffect(&raftReq) {
		if !needResult && raftReq.Txn != nil {
			removeNeedlessRangeReqs(raftReq.Txn)
		}
		return uberApply.Apply(&raftReq, shouldApplyV3), id
	}
	return nil, id
}

func noSideEffect(r *pb.InternalRaftRequest) bool {
	return r.Range != nil || r.AuthUserGet != nil || r.AuthRoleGet != nil || r.AuthStatus != nil
}

func removeNeedlessRangeReqs(txn *pb.TxnRequest) {
	// removeNeedlessRangeReqs mutates follower RangeRequests to minimize I/O
	// while preserving revision validation. Deleting reads entirely can cause
	// split-brain if a read targets a compacted revision (see #18667).
	// Using a non-existent proxy key with CountOnly=true eliminates disk access.
	neuter := func(ops []*pb.RequestOp) {
		for i := 0; i < len(ops); i++ {
			if rangeOp, ok := ops[i].Request.(*pb.RequestOp_RequestRange); ok {
				// Preserve RequestRange.Revision for compaction validation.
				// Mutate search parameters to a proxy key to avoid Boltdb disk I/O.
				rangeOp.RequestRange.Key = []byte("\x00")
				rangeOp.RequestRange.RangeEnd = nil
				rangeOp.RequestRange.Limit = 1
				rangeOp.RequestRange.KeysOnly = true
				rangeOp.RequestRange.CountOnly = true
			}
		}
	}

	neuter(txn.Success)
	neuter(txn.Failure)
}
