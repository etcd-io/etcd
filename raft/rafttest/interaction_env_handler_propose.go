// Copyright 2019 The etcd Authors
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

func (env *InteractionEnv) handlePropose(t *testing.T, d datadriven.TestData) error {
	idx := firstAsNodeIdx(t, d)
	if len(d.CmdArgs) != 2 || len(d.CmdArgs[1].Vals) > 0 {
		t.Fatalf("expected exactly one key with no vals: %+v", d.CmdArgs[1:])
	}
	return env.Propose(idx, []byte(d.CmdArgs[1].Key))
}

// Propose a regular entry.
func (env *InteractionEnv) Propose(idx int, data []byte) error {
	return env.Nodes[idx].Propose(data)
}
