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

package raft_test

import (
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft/v3/rafttest"
)

func TestInteraction(t *testing.T) {
	// NB: if this test fails, run `go test ./raft -rewrite` and inspect the
	// diff. Only commit the changes if you understand what caused them and if
	// they are desired.
	datadriven.Walk(t, "testdata", func(t *testing.T, path string) {
		env := rafttest.NewInteractionEnv(nil)
		datadriven.RunTest(t, path, func(t *testing.T, d *datadriven.TestData) string {
			return env.Handle(t, *d)
		})
	})
}
