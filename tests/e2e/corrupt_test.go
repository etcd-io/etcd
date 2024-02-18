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
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestEtcdCorruptHash(t *testing.T) {
	// oldenv := os.Getenv("EXPECT_DEBUG")
	// defer os.Setenv("EXPECT_DEBUG", oldenv)
	// os.Setenv("EXPECT_DEBUG", "1")

	cfg := e2e.NewConfigNoTLS()

	// trigger snapshot so that restart member can load peers from disk
	cfg.SnapshotCount = 3

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
	cx.epc.Procs[0].Stop()

	// corrupting first member by modifying backend offline.
	fp := datadir.ToBackendFileName(cx.epc.Procs[0].Config().DataDirPath)
	cx.t.Logf("corrupting backend: %v", fp)
	if err = cx.corruptFunc(fp); err != nil {
		cx.t.Fatal(err)
	}

	cx.t.Log("restarting etcd[0]")
	ep := cx.epc.Procs[0]
	proc, err := e2e.SpawnCmd(append([]string{ep.Config().ExecPath}, ep.Config().Args...), cx.envMap)
	if err != nil {
		cx.t.Fatal(err)
	}
	defer proc.Stop()

	cx.t.Log("waiting for etcd[0] failure...")
	// restarting corrupted member should fail
	e2e.WaitReadyExpectProc(proc, []string{fmt.Sprintf("etcdmain: %016x found data inconsistency with peers", id0)})
}

func TestInPlaceRecovery(t *testing.T) {
	basePort := 20000
	e2e.BeforeTest(t)

	// Initialize the cluster.
	epcOld, err := e2e.NewEtcdProcessCluster(t,
		&e2e.EtcdProcessClusterConfig{
			ClusterSize:      3,
			InitialToken:     "old",
			KeepDataDir:      false,
			CorruptCheckTime: time.Second,
			BasePort:         basePort,
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

	oldCc := e2e.NewEtcdctl(epcOld.EndpointsV3(), e2e.ClientNonTLS, false, false)
	for i := 0; i < 10; i++ {
		err := oldCc.Put(testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}

	// Create a new cluster config, but with the same port numbers. In this way the new servers can stay in
	// contact with the old ones.
	epcNewConfig := &e2e.EtcdProcessClusterConfig{
		ClusterSize:         3,
		InitialToken:        "new",
		KeepDataDir:         false,
		CorruptCheckTime:    time.Second,
		BasePort:            basePort,
		InitialCorruptCheck: true,
	}
	epcNew, err := e2e.InitEtcdProcessCluster(t, epcNewConfig)
	if err != nil {
		t.Fatalf("could not init etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epcNew.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	newCc := e2e.NewEtcdctl(epcNew.EndpointsV3(), e2e.ClientNonTLS, false, false)
	assert.NoError(t, err)

	wg := sync.WaitGroup{}
	// Rolling recovery of the servers.
	t.Log("rolling updating servers in place...")
	for i := range epcNew.Procs {
		oldProc := epcOld.Procs[i]
		err = oldProc.Close()
		if err != nil {
			t.Fatalf("could not stop etcd process (%v)", err)
		}
		t.Logf("old cluster server %d: %s stopped.", i, oldProc.Config().Name)
		wg.Add(1)
		// Start servers in background to avoid blocking on server start.
		// EtcdProcess.Start waits until etcd becomes healthy, which will not happen here until we restart at least 2 members.
		go func(proc e2e.EtcdProcess) {
			defer wg.Done()
			err = proc.Start()
			if err != nil {
				t.Errorf("could not start etcd process (%v)", err)
			}
			t.Logf("new cluster server: %s started in-place with blank db.", proc.Config().Name)
		}(epcNew.Procs[i])
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
	e2e.BeforeTest(t)
	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize:      3,
		KeepDataDir:      true,
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

	cc := e2e.NewEtcdctl(epc.EndpointsV3(), e2e.ClientNonTLS, false, false)

	for i := 0; i < 10; i++ {
		err := cc.Put(testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}

	_, found, err := getMemberIdByName(context.Background(), cc, epc.Procs[0].Config().Name)
	assert.NoError(t, err, "error on member list")
	assert.Equal(t, found, true, "member not found")

	epc.Procs[0].Stop()
	err = testutil.CorruptBBolt(datadir.ToBackendFileName(epc.Procs[0].Config().DataDirPath))
	assert.NoError(t, err)

	err = epc.Procs[0].Restart()
	assert.NoError(t, err)
	time.Sleep(checkTime * 11 / 10)
	alarmResponse, err := cc.AlarmList()
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: 0}}, alarmResponse.Alarms)
}

func TestCompactHashCheckDetectCorruption(t *testing.T) {
	checkTime := time.Second
	e2e.BeforeTest(t)
	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize:             3,
		KeepDataDir:             true,
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

	cc := e2e.NewEtcdctl(epc.EndpointsV3(), e2e.ClientNonTLS, false, false)

	for i := 0; i < 10; i++ {
		err := cc.Put(testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}
	_, err = cc.MemberList()
	assert.NoError(t, err, "error on member list")

	epc.Procs[0].Stop()
	err = testutil.CorruptBBolt(datadir.ToBackendFileName(epc.Procs[0].Config().DataDirPath))
	assert.NoError(t, err)

	err = epc.Procs[0].Restart()
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
	e2e.BeforeTest(t)

	slowCompactionNodeIndex := 1

	// Start a new cluster, with compact hash check enabled.
	t.Log("creating a new cluster with 3 nodes...")

	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize:             3,
		KeepDataDir:             true,
		CompactHashCheckEnabled: true,
		CompactHashCheckTime:    checkTime,
		LogLevel:                "info",
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
	cc := e2e.NewEtcdctl(epc.EndpointsV3(), e2e.ClientNonTLS, false, false)

	for i := 0; i < 200; i++ {
		err = cc.Put("key", fmt.Sprint(i))
		require.NoError(t, err, "error on put")
	}

	t.Log("compaction started...")
	_, err = cc.Compact(200)

	t.Logf("restart proc %d to interrupt its compaction...", slowCompactionNodeIndex)
	err = epc.Procs[slowCompactionNodeIndex].Restart()
	require.NoError(t, err)

	// Wait until the node finished compaction.
	_, err = epc.Procs[slowCompactionNodeIndex].Logs().Expect("finished scheduled compaction")
	require.NoError(t, err, "can't get log indicating finished scheduled compaction")

	// Wait until the leader finished compaction hash check.
	leaderIndex := epc.WaitLeader(t)
	_, err = epc.Procs[leaderIndex].Logs().Expect("finished compaction hash check")
	require.NoError(t, err, "can't get log indicating finished compaction hash check")

	alarmResponse, err := cc.AlarmList()
	require.NoError(t, err, "error on alarm list")
	for _, alarm := range alarmResponse.Alarms {
		if alarm.Alarm == etcdserverpb.AlarmType_CORRUPT {
			t.Fatal("there should be no corruption after resuming the compaction, but corruption detected")
		}
	}
	t.Log("no corruption detected.")
}
