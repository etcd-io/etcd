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

package cindex

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"go.etcd.io/etcd/server/v3/mvcc/backend"
)

// TestConsistentIndex ensures that LoadConsistentIndex/Save/ConsistentIndex and backend.BatchTx can work well together.
func TestConsistentIndex(t *testing.T) {

	be, tmpPath := backend.NewTmpBackend(t, time.Microsecond, 10)
	defer os.Remove(tmpPath)
	ci := NewConsistentIndex(be.BatchTx())

	tx := be.BatchTx()
	if tx == nil {
		t.Fatal("batch tx is nil")
	}
	tx.Lock()
	tx.UnsafeCreateBucket(metaBucketName)
	tx.Unlock()
	be.ForceCommit()
	r := rand.Uint64()
	ci.SetConsistentIndex(r)
	index := ci.ConsistentIndex()
	if index != r {
		t.Errorf("expected %d,got %d", r, index)
	}
	tx.Lock()
	ci.UnsafeSave(tx)
	tx.Unlock()
	be.ForceCommit()
	be.Close()

	b := backend.NewDefaultBackend(tmpPath)
	ci.SetConsistentIndex(0)
	ci.SetBatchTx(b.BatchTx())
	index = ci.ConsistentIndex()
	if index != r {
		t.Errorf("expected %d,got %d", r, index)
	}

	ci = NewConsistentIndex(b.BatchTx())
	index = ci.ConsistentIndex()
	if index != r {
		t.Errorf("expected %d,got %d", r, index)
	}
	b.Close()

}

func TestFakeConsistentIndex(t *testing.T) {

	r := rand.Uint64()
	ci := NewFakeConsistentIndex(r)
	index := ci.ConsistentIndex()
	if index != r {
		t.Errorf("expected %d,got %d", r, index)
	}
	r = rand.Uint64()
	ci.SetConsistentIndex(r)
	index = ci.ConsistentIndex()
	if index != r {
		t.Errorf("expected %d,got %d", r, index)
	}

}
