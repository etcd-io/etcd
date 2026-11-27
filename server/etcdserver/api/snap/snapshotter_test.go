// Copyright 2015 The etcd Authors
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

package snap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/raft/v3/raftpb"
)

func TestReleaseSnapDBs(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	snapIndices := []uint64{100, 200, 300, 400}
	for _, index := range snapIndices {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		if err := os.WriteFile(filename, []byte("snap file\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ss := New(zaptest.NewLogger(t), dir)

	if err := ss.ReleaseSnapDBs(&raftpb.Snapshot{Metadata: &raftpb.SnapshotMetadata{Index: new(uint64(300))}}); err != nil {
		t.Fatal(err)
	}

	deleted := []uint64{100, 200}
	for _, index := range deleted {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		if fileutil.Exist(filename) {
			t.Errorf("expected %s (index: %d)  to be deleted, but it still exists", filename, index)
		}
	}

	retained := []uint64{300, 400}
	for _, index := range retained {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		if !fileutil.Exist(filename) {
			t.Errorf("expected %s (index: %d) to be retained, but it no longer exists", filename, index)
		}
	}
}
