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

//go:build !cluster_proxy
// +build !cluster_proxy

package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
)

func TestEtcdCorruptHash(t *testing.T) {
	// oldenv := os.Getenv("EXPECT_DEBUG")
	// defer os.Setenv("EXPECT_DEBUG", oldenv)
	// os.Setenv("EXPECT_DEBUG", "1")

	cfg := newConfigNoTLS()

	// trigger snapshot so that restart member can load peers from disk
	cfg.snapshotCount = 3

	testCtl(t, corruptTest, withQuorum(),
		withCfg(*cfg),
		withInitialCorruptCheck(),
		withCorruptFunc(testutil.CorruptBBolt),
	)
}

func corruptTest(cx ctlCtx) {
	cx.t.Log("putting 10 keys...")
	for i := 0; i < 10; i++ {
		if err := ctlV3Put(cx, fmt.Sprintf("foo%05d", i), fmt.Sprintf("v%05d", i), ""); err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Fatalf("putTest ctlV3Put error (%v)", err)
			}
		}
	}
	// enough time for all nodes sync on the same data
	cx.t.Log("sleeping 3sec to let nodes sync...")
	time.Sleep(3 * time.Second)

	cx.t.Log("connecting clientv3...")
	eps := cx.epc.EndpointsV3()
	cli1, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[1]}, DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer cli1.Close()

	sresp, err := cli1.Status(context.TODO(), eps[0])
	cx.t.Logf("checked status sresp:%v err:%v", sresp, err)
	if err != nil {
		cx.t.Fatal(err)
	}
	id0 := sresp.Header.GetMemberId()

	cx.t.Log("stopping etcd[0]...")
	cx.epc.procs[0].Stop()

	// corrupting first member by modifying backend offline.
	fp := datadir.ToBackendFileName(cx.epc.procs[0].Config().dataDirPath)
	cx.t.Logf("corrupting backend: %v", fp)
	if err = cx.corruptFunc(fp); err != nil {
		cx.t.Fatal(err)
	}

	cx.t.Log("restarting etcd[0]")
	ep := cx.epc.procs[0]
	proc, err := spawnCmd(append([]string{ep.Config().execPath}, ep.Config().args...), cx.envMap)
	if err != nil {
		cx.t.Fatal(err)
	}
	defer proc.Stop()

	cx.t.Log("waiting for etcd[0] failure...")
	// restarting corrupted member should fail
	waitReadyExpectProc(proc, []string{fmt.Sprintf("etcdmain: %016x found data inconsistency with peers", id0)})
}

func TestInPlaceRecovery(t *testing.T) {
	basePort := 20000
	BeforeTest(t)

	// Initialize the cluster.
	epcOld, err := newEtcdProcessCluster(t,
		&etcdProcessClusterConfig{
			clusterSize:      3,
			initialToken:     "old",
			keepDataDir:      false,
			CorruptCheckTime: time.Second,
			basePort:         basePort,
		},
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epcOld.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})
	t.Log("old cluster started.")

	//Put some data into the old cluster, so that after recovering from a blank db, the hash diverges.
	t.Log("putting 10 keys...")

	oldCc := NewEtcdctl(epcOld.EndpointsV3(), clientNonTLS, false, false)
	for i := 0; i < 10; i++ {
		err := oldCc.Put(testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}

	// Create a new cluster config, but with the same port numbers. In this way the new servers can stay in
	// contact with the old ones.
	epcNewConfig := &etcdProcessClusterConfig{
		clusterSize:         3,
		initialToken:        "new",
		keepDataDir:         false,
		CorruptCheckTime:    time.Second,
		basePort:            basePort,
		initialCorruptCheck: true,
	}
	epcNew, err := initEtcdProcessCluster(t, epcNewConfig)
	if err != nil {
		t.Fatalf("could not init etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epcNew.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	newCc := NewEtcdctl(epcNew.EndpointsV3(), clientNonTLS, false, false)
	assert.NoError(t, err)

	wg := sync.WaitGroup{}
	// Rolling recovery of the servers.
	t.Log("rolling updating servers in place...")
	for i := range epcNew.procs {
		oldProc := epcOld.procs[i]
		err = oldProc.Close()
		if err != nil {
			t.Fatalf("could not stop etcd process (%v)", err)
		}
		t.Logf("old cluster server %d: %s stopped.", i, oldProc.Config().name)
		wg.Add(1)
		// Start servers in background to avoid blocking on server start.
		// EtcdProcess.Start waits until etcd becomes healthy, which will not happen here until we restart at least 2 members.
		go func(proc etcdProcess) {
			defer wg.Done()
			err = proc.Start()
			if err != nil {
				t.Errorf("could not start etcd process (%v)", err)
			}
			t.Logf("new cluster server: %s started in-place with blank db.", proc.Config().name)
		}(epcNew.procs[i])
		t.Log("sleeping 5 sec to let nodes do periodical check...")
		time.Sleep(5 * time.Second)
	}
	wg.Wait()
	t.Log("new cluster started.")

	alarmResponse, err := newCc.AlarmList()
	assert.NoError(t, err, "error on alarm list")
	for _, alarm := range alarmResponse.Alarms {
		if alarm.Alarm == etcdserverpb.AlarmType_CORRUPT {
			t.Fatalf("there is no corruption after in-place recovery, but corruption reported.")
		}
	}
	t.Log("no corruption detected.")
}

func TestPeriodicCheckDetectsCorruption(t *testing.T) {
	checkTime := time.Second
	BeforeTest(t)
	epc, err := newEtcdProcessCluster(t, &etcdProcessClusterConfig{
		clusterSize:      3,
		keepDataDir:      true,
		CorruptCheckTime: time.Second,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	cc := NewEtcdctl(epc.EndpointsV3(), clientNonTLS, false, false)

	for i := 0; i < 10; i++ {
		err := cc.Put(testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}

	members, err := cc.MemberList()
	assert.NoError(t, err, "error on member list")
	var memberID uint64
	for _, m := range members.Members {
		if m.Name == epc.procs[0].Config().name {
			memberID = m.ID
		}
	}
	assert.NotZero(t, memberID, "member not found")
	epc.procs[0].Stop()
	err = testutil.CorruptBBolt(datadir.ToBackendFileName(epc.procs[0].Config().dataDirPath))
	assert.NoError(t, err)

	err = epc.procs[0].Restart()
	assert.NoError(t, err)
	time.Sleep(checkTime * 11 / 10)
	alarmResponse, err := cc.AlarmList()
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: 0}}, alarmResponse.Alarms)
}

func TestCompactHashCheckDetectCorruption(t *testing.T) {
	checkTime := time.Second
	BeforeTest(t)
	epc, err := newEtcdProcessCluster(t, &etcdProcessClusterConfig{
		clusterSize:             3,
		keepDataDir:             true,
		CompactHashCheckEnabled: true,
		CompactHashCheckTime:    checkTime,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	cc := NewEtcdctl(epc.EndpointsV3(), clientNonTLS, false, false)

	for i := 0; i < 10; i++ {
		err := cc.Put(testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}
	_, err = cc.MemberList()
	assert.NoError(t, err, "error on member list")

	epc.procs[0].Stop()
	err = testutil.CorruptBBolt(datadir.ToBackendFileName(epc.procs[0].Config().dataDirPath))
	assert.NoError(t, err)

	err = epc.procs[0].Restart()
	assert.NoError(t, err)
	_, err = cc.Compact(5)
	assert.NoError(t, err)
	time.Sleep(checkTime * 11 / 10)
	alarmResponse, err := cc.AlarmList()
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: 0}}, alarmResponse.Alarms)
}

func TestCompactHashCheckDetectCorruptionInterrupt(t *testing.T) {
	checkTime := time.Second
	BeforeTest(t)

	slowCompactionNodeIndex := 1

	// Start a new cluster, with compact hash check enabled.
	t.Log("creating a new cluster with 3 nodes...")

	epc, err := newEtcdProcessCluster(t, &etcdProcessClusterConfig{
		clusterSize:             3,
		keepDataDir:             true,
		CompactHashCheckEnabled: true,
		CompactHashCheckTime:    checkTime,
		logLevel:                "info",
		CompactionBatchLimit:    1,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	// Put 200 identical keys to the cluster, so that the compaction will drop some stale values.
	// We need a relatively big number here to make the compaction takes a non-trivial time, and we can interrupt it.
	t.Log("putting 200 values to the identical key...")
	cc := NewEtcdctl(epc.EndpointsV3(), clientNonTLS, false, false)

	for i := 0; i < 200; i++ {
		err = cc.Put("key", fmt.Sprint(i))
		require.NoError(t, err, "error on put")
	}

	t.Log("compaction started...")
	_, err = cc.Compact(200)

	t.Logf("restart proc %d to interrupt its compaction...", slowCompactionNodeIndex)
	err = epc.procs[slowCompactionNodeIndex].Restart()
	require.NoError(t, err)

	// Wait until the node finished compaction.
	_, err = epc.procs[slowCompactionNodeIndex].Logs().Expect("finished scheduled compaction")
	require.NoError(t, err, "can't get log indicating finished scheduled compaction")

	// Wait for compaction hash check
	time.Sleep(checkTime * 5)

	alarmResponse, err := cc.AlarmList()
	require.NoError(t, err, "error on alarm list")
	for _, alarm := range alarmResponse.Alarms {
		if alarm.Alarm == etcdserverpb.AlarmType_CORRUPT {
			t.Fatal("there should be no corruption after resuming the compaction, but corruption detected")
		}
	}
	t.Log("no corruption detected.")
}
