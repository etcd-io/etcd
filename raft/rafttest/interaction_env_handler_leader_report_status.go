// Copyright 2022 The etcd Authors
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

package rafttest

import (
	"strconv"
	"testing"

	"github.com/cockroachdb/datadriven"
	pb "go.etcd.io/etcd/raft/v3/raftpb"
)

func (env *InteractionEnv) handleLeaderReportStatus(t *testing.T, d datadriven.TestData) error {
	idx := firstAsNodeIdx(t, d)
	logIndex, err := strconv.Atoi(d.Input)
	if err != nil {
		t.Fatalf("Invalid input for the report-status command: %s, error: %v", d.Input, err)
	}
	return env.ReportStatus(idx, logIndex)
}

func (env *InteractionEnv) ReportStatus(idx int, index int) error {
	rn := env.Nodes[idx].RawNode
	return rn.Step(pb.Message{From: rn.Status().ID, To: rn.Status().ID, Type: pb.MsgAppResp, Index: uint64(index)})
}
