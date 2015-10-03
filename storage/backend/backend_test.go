// Copyright 2015 CoreOS, Inc.
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

package backend

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/boltdb/bolt"
	"github.com/coreos/etcd/pkg/testutil"
)

var tmpPath string

func init() {
	dir, err := ioutil.TempDir(os.TempDir(), "etcd_backend_test")
	if err != nil {
		log.Fatal(err)
	}
	tmpPath = path.Join(dir, "database")
}

func TestBackendClose(t *testing.T) {
	b := newBackend(tmpPath, time.Hour, 10000)
	defer os.Remove(tmpPath)

	// check close could work
	done := make(chan struct{})
	go func() {
		err := b.Close()
		if err != nil {
			t.Errorf("close error = %v, want nil", err)
		}
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("failed to close database in 1s")
	}
}

func TestBackendSnapshot(t *testing.T) {
	b := New(tmpPath, time.Hour, 10000)
	defer cleanup(b, tmpPath)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket([]byte("test"))
	tx.UnsafePut([]byte("test"), []byte("foo"), []byte("bar"))
	tx.Unlock()
	b.ForceCommit()

	// write snapshot to a new file
	f, err := ioutil.TempFile(os.TempDir(), "etcd_backend_test")
	if err != nil {
		t.Fatal(err)
	}
	snap := b.Snapshot()
	defer snap.Close()
	if _, err := snap.WriteTo(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// bootstrap new backend from the snapshot
	nb := New(f.Name(), time.Hour, 10000)
	defer cleanup(nb, f.Name())

	newTx := b.BatchTx()
	newTx.Lock()
	ks, _ := newTx.UnsafeRange([]byte("test"), []byte("foo"), []byte("goo"), 0)
	if len(ks) != 1 {
		t.Errorf("len(kvs) = %d, want 1", len(ks))
	}
	newTx.Unlock()
}

func TestBackendBatchIntervalCommit(t *testing.T) {
	// start backend with super short batch interval
	b := newBackend(tmpPath, time.Nanosecond, 10000)
	defer cleanup(b, tmpPath)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket([]byte("test"))
	tx.UnsafePut([]byte("test"), []byte("foo"), []byte("bar"))
	tx.Unlock()

	// give time for batch interval commit to happen
	time.Sleep(time.Nanosecond)
	testutil.WaitSchedule()
	// give time for commit to finish, including possible disk IO
	time.Sleep(50 * time.Millisecond)

	// check whether put happens via db view
	b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("test"))
		if bucket == nil {
			t.Errorf("bucket test does not exit")
			return nil
		}
		v := bucket.Get([]byte("foo"))
		if v == nil {
			t.Errorf("foo key failed to written in backend")
		}
		return nil
	})
}

func cleanup(b Backend, path string) {
	b.Close()
	os.Remove(path)
}
