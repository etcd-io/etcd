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

package testing

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3/raftpb"
)

func NewTmpWAL(t testing.TB, reqs []etcdserverpb.InternalRaftRequest) (*wal.WAL, string) {
	t.Helper()
	dir, err := os.MkdirTemp(t.TempDir(), "etcd_wal_test")
	if err != nil {
		panic(err)
	}
	tmpPath := filepath.Join(dir, "wal")
	lg := zaptest.NewLogger(t)
	w, err := wal.Create(lg, tmpPath, nil)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("Failed to close WAL: %v", err)
	}
	if len(reqs) != 0 {
		w, err = wal.Open(lg, tmpPath, walpb.Snapshot{})
		if err != nil {
			t.Fatalf("Failed to open WAL: %v", err)
		}

		var state raftpb.HardState
		_, state, _, err = w.ReadAll()
		if err != nil {
			t.Fatalf("Failed to read WAL: %v", err)
		}
		var entries []raftpb.Entry
		for _, req := range reqs {
			entries = append(entries, raftpb.Entry{
				Term:  1,
				Index: 1,
				Type:  raftpb.EntryNormal,
				Data:  pbutil.MustMarshal(&req),
			})
		}
		err = w.Save(state, entries)
		if err != nil {
			t.Fatalf("Failed to save WAL: %v", err)
		}
		err = w.Close()
		if err != nil {
			t.Fatalf("Failed to close WAL: %v", err)
		}
	}

	w, err = wal.OpenForRead(lg, tmpPath, walpb.Snapshot{})
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	return w, tmpPath
}

func Reopen(t testing.TB, walPath string) *wal.WAL {
	t.Helper()
	lg := zaptest.NewLogger(t)
	w, err := wal.OpenForRead(lg, walPath, walpb.Snapshot{})
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	return w
}
