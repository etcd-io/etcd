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

package rafttest

import (
	"testing"

	"github.com/cockroachdb/datadriven"
)

func (env *InteractionEnv) handleTransferLeadership(t *testing.T, d datadriven.TestData) error {
	var from, to uint64
	d.ScanArgs(t, "from", &from)
	d.ScanArgs(t, "to", &to)
	if from == 0 || from > uint64(len(env.Nodes)) {
		t.Fatalf(`expected valid "from" argument`)
	}
	if to == 0 || to > uint64(len(env.Nodes)) {
		t.Fatalf(`expected valid "to" argument`)
	}
	return env.transferLeadership(from, to)
}

// Initiate leadership transfer.
func (env *InteractionEnv) transferLeadership(from, to uint64) error {
	fromIdx := from - 1
	env.Nodes[fromIdx].TransferLeader(to)
	return nil
}
