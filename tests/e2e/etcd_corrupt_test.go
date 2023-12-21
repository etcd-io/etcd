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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"go.etcd.io/etcd/pkg/testutil"

	bolt "go.etcd.io/bbolt"
)

// TODO: test with embedded etcd in integration package

func TestEtcdCorruptHash(t *testing.T) {
	// oldenv := os.Getenv("EXPECT_DEBUG")
	// defer os.Setenv("EXPECT_DEBUG", oldenv)
	// os.Setenv("EXPECT_DEBUG", "1")

	cfg := configNoTLS

	// trigger snapshot so that restart member can load peers from disk
	cfg.snapshotCount = 3

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

func TestInPlaceRecovery(t *testing.T) {
	defer testutil.AfterTest(t)

	basePort := 20000

	// Initialize the cluster.
	cfgOld := etcdProcessClusterConfig{
		clusterSize:      3,
		initialToken:     "old",
		keepDataDir:      false,
		clientTLS:        clientNonTLS,
		corruptCheckTime: time.Second,
		basePort:         basePort,
	}
	epcOld, err := newEtcdProcessCluster(t, &cfgOld)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epcOld.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})
	t.Log("Old cluster started.")

	//Put some data into the old cluster, so that after recovering from a blank db, the hash diverges.
	t.Log("putting 10 keys...")
	oldEtcdctl := NewEtcdctl(epcOld.EndpointsV3(), cfgOld.clientTLS, false, false)
	for i := 0; i < 10; i++ {
		err := oldEtcdctl.Put(fmt.Sprintf("%d", i), fmt.Sprintf("%d", i))
		assert.NoError(t, err, "error on put")
	}

	// Create a new cluster config, but with the same port numbers. In this way the new servers can stay in
	// contact with the old ones.
	cfgNew := etcdProcessClusterConfig{
		clusterSize:         3,
		initialToken:        "new",
		keepDataDir:         false,
		clientTLS:           clientNonTLS,
		initialCorruptCheck: true,
		corruptCheckTime:    time.Second,
		basePort:            basePort,
	}
	epcNew, err := initEtcdProcessCluster(&cfgNew)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epcNew.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})
	t.Log("New cluster initialized.")

	newEtcdctl := NewEtcdctl(epcNew.EndpointsV3(), cfgNew.clientTLS, false, false)
	// Rolling recovery of the servers.
	var wg sync.WaitGroup
	t.Log("rolling updating servers in place...")
	for i, newProc := range epcNew.procs {
		oldProc := epcOld.procs[i]
		err = oldProc.Close()
		if err != nil {
			t.Fatalf("could not stop etcd process (%v)", err)
		}
		t.Logf("old cluster server %d: %s stopped.", i, oldProc.Config().name)

		wg.Add(1)
		go func(proc etcdProcess) {
			defer wg.Done()
			perr := proc.Start()
			if perr != nil {
				t.Fatalf("failed to start etcd process: %v", perr)
				return
			}
			t.Logf("new etcd server %q started in-place with blank db", proc.Config().name)
		}(newProc)
		t.Log("sleeping 5 sec to let nodes do periodical check...")
		time.Sleep(5 * time.Second)
	}
	wg.Wait()
	t.Log("new cluster started.")

	alarmResponse, err := newEtcdctl.AlarmList()
	assert.NoError(t, err, "error on alarm list")
	for _, alarm := range alarmResponse.Alarms {
		if alarm.Alarm == etcdserverpb.AlarmType_CORRUPT {
			t.Fatalf("there is no corruption after in-place recovery, but corruption reported.")
		}
	}
	t.Log("no corruption detected.")
}
