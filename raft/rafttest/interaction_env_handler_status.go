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
	"fmt"
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft/tracker"
)

func (env *InteractionEnv) handleStatus(t *testing.T, d datadriven.TestData) error {
	idx := firstAsNodeIdx(t, d)
	return env.Status(idx)
}

// Status pretty-prints the raft status for the node at the given index to the output
// buffer.
func (env *InteractionEnv) Status(idx int) error {
	// TODO(tbg): actually print the full status.
	st := env.Nodes[idx].Status()
	m := tracker.ProgressMap{}
	for id, pr := range st.Progress {
		pr := pr // loop-local copy
		m[id] = &pr
	}
	fmt.Fprint(env.Output, m)
	return nil
}
