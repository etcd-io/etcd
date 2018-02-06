// Copyright 2017 The etcd Authors
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

package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/internal/mvcc/mvccpb"

	bolt "github.com/coreos/bbolt"
)

// TODO: test with embedded etcd in integration package

func TestEtcdCorruptHash(t *testing.T) {
	oldenv := os.Getenv("EXPECT_DEBUG")
	defer os.Setenv("EXPECT_DEBUG", oldenv)
	os.Setenv("EXPECT_DEBUG", "1")

	cfg := configNoTLS

	// trigger snapshot so that restart member can load peers from disk
	cfg.snapCount = 3

	testCtl(t, corruptTest, withQuorum(),
		withCfg(cfg),
		withInitialCorruptCheck(),
		withCorruptFunc(corruptHash),
	)
}

func corruptTest(cx ctlCtx) {
	for i := 0; i < 10; i++ {
		if err := ctlV3Put(cx, fmt.Sprintf("foo%05d", i), fmt.Sprintf("v%05d", i), ""); err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Fatalf("putTest ctlV3Put error (%v)", err)
			}
		}
	}
	// enough time for all nodes sync on the same data
	time.Sleep(3 * time.Second)

	eps := cx.epc.EndpointsV3()
	cli1, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[1]}, DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer cli1.Close()

	sresp, err := cli1.Status(context.TODO(), eps[0])
	if err != nil {
		cx.t.Fatal(err)
	}
	id0 := sresp.Header.GetMemberId()

	cx.epc.procs[0].Stop()

	// corrupt first member by modifying backend offline.
	fp := filepath.Join(cx.epc.procs[0].Config().dataDirPath, "member", "snap", "db")
	if err = cx.corruptFunc(fp); err != nil {
		cx.t.Fatal(err)
	}

	ep := cx.epc.procs[0]
	proc, err := spawnCmd(append([]string{ep.Config().execPath}, ep.Config().args...))
	if err != nil {
		cx.t.Fatal(err)
	}
	defer proc.Stop()

	// restarting corrupted member should fail
	waitReadyExpectProc(proc, []string{fmt.Sprintf("etcdmain: %016x found data inconsistency with peers", id0)})
}

func corruptHash(fpath string) error {
	db, derr := bolt.Open(fpath, os.ModePerm, &bolt.Options{})
	if derr != nil {
		return derr
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("key"))
		if b == nil {
			return errors.New("got nil bucket for 'key'")
		}
		keys, vals := [][]byte{}, [][]byte{}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			keys = append(keys, k)
			var kv mvccpb.KeyValue
			if uerr := kv.Unmarshal(v); uerr != nil {
				return uerr
			}
			kv.Key[0]++
			kv.Value[0]++
			v2, v2err := kv.Marshal()
			if v2err != nil {
				return v2err
			}
			vals = append(vals, v2)
		}
		for i := range keys {
			if perr := b.Put(keys[i], vals[i]); perr != nil {
				return perr
			}
		}
		return nil
	})
}
