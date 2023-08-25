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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
	"go.etcd.io/etcd/tests/v3/framework/config"
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
	eps := cx.epc.EndpointsGRPC()
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
	e2e.WaitReadyExpectProc(context.TODO(), proc, []string{fmt.Sprintf("etcdmain: %016x found data inconsistency with peers", id0)})
}

func TestInPlaceRecovery(t *testing.T) {
	basePort := 20000
	e2e.BeforeTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the cluster.
	epcOld, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithInitialClusterToken("old"),
		e2e.WithKeepDataDir(false),
		e2e.WithCorruptCheckTime(time.Second),
		e2e.WithBasePort(basePort),
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
	oldCc, err := e2e.NewEtcdctl(epcOld.Cfg.Client, epcOld.EndpointsGRPC())
	assert.NoError(t, err)
	for i := 0; i < 10; i++ {
		err := oldCc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i), config.PutOptions{})
		assert.NoError(t, err, "error on put")
	}

	// Create a new cluster config, but with the same port numbers. In this way the new servers can stay in
	// contact with the old ones.
	epcNewConfig := e2e.NewConfig(
		e2e.WithInitialClusterToken("new"),
		e2e.WithKeepDataDir(false),
		e2e.WithCorruptCheckTime(time.Second),
		e2e.WithBasePort(basePort),
		e2e.WithInitialCorruptCheck(true),
	)
	epcNew, err := e2e.InitEtcdProcessCluster(t, epcNewConfig)
	if err != nil {
		t.Fatalf("could not init etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epcNew.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	newCc, err := e2e.NewEtcdctl(epcNew.Cfg.Client, epcNew.EndpointsGRPC())
	assert.NoError(t, err)

	// Rolling recovery of the servers.
	t.Log("rolling updating servers in place...")
	for i, newProc := range epcNew.Procs {
		oldProc := epcOld.Procs[i]
		err = oldProc.Close()
		if err != nil {
			t.Fatalf("could not stop etcd process (%v)", err)
		}
		t.Logf("old cluster server %d: %s stopped.", i, oldProc.Config().Name)
		err = newProc.Start(ctx)
		if err != nil {
			t.Fatalf("could not start etcd process (%v)", err)
		}
		t.Logf("new cluster server %d: %s started in-place with blank db.", i, newProc.Config().Name)
		t.Log("sleeping 5 sec to let nodes do periodical check...")
		time.Sleep(5 * time.Second)
	}
	t.Log("new cluster started.")

	alarmResponse, err := newCc.AlarmList(ctx)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithKeepDataDir(true),
		e2e.WithCorruptCheckTime(time.Second),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	cc := epc.Etcdctl()
	for i := 0; i < 10; i++ {
		err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i), config.PutOptions{})
		assert.NoError(t, err, "error on put")
	}

	members, err := cc.MemberList(ctx, false)
	assert.NoError(t, err, "error on member list")
	var memberID uint64
	for _, m := range members.Members {
		if m.Name == epc.Procs[0].Config().Name {
			memberID = m.ID
		}
	}
	assert.NotZero(t, memberID, "member not found")
	epc.Procs[0].Stop()
	err = testutil.CorruptBBolt(datadir.ToBackendFileName(epc.Procs[0].Config().DataDirPath))
	assert.NoError(t, err)

	err = epc.Procs[0].Restart(context.TODO())
	assert.NoError(t, err)
	time.Sleep(checkTime * 11 / 10)
	alarmResponse, err := cc.AlarmList(ctx)
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: memberID}}, alarmResponse.Alarms)
}

func TestCompactHashCheckDetectCorruption(t *testing.T) {
	checkTime := time.Second
	e2e.BeforeTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithKeepDataDir(true),
		e2e.WithCompactHashCheckEnabled(true),
		e2e.WithCompactHashCheckTime(checkTime),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	cc := epc.Etcdctl()
	for i := 0; i < 10; i++ {
		err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i), config.PutOptions{})
		assert.NoError(t, err, "error on put")
	}
	members, err := cc.MemberList(ctx, false)
	assert.NoError(t, err, "error on member list")
	var memberID uint64
	for _, m := range members.Members {
		if m.Name == epc.Procs[0].Config().Name {
			memberID = m.ID
		}
	}

	epc.Procs[0].Stop()
	err = testutil.CorruptBBolt(datadir.ToBackendFileName(epc.Procs[0].Config().DataDirPath))
	assert.NoError(t, err)

	err = epc.Procs[0].Restart(ctx)
	assert.NoError(t, err)
	_, err = cc.Compact(ctx, 5, config.CompactOption{})
	assert.NoError(t, err)
	time.Sleep(checkTime * 11 / 10)
	alarmResponse, err := cc.AlarmList(ctx)
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: memberID}}, alarmResponse.Alarms)
}

func TestCompactHashCheckDetectCorruptionInterrupt(t *testing.T) {
	checkTime := time.Second
	e2e.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	slowCompactionNodeIndex := 1

	// Start a new cluster, with compact hash check enabled.
	t.Log("creating a new cluster with 3 nodes...")

	dataDirPath := t.TempDir()
	cfg := e2e.NewConfig(
		e2e.WithKeepDataDir(true),
		e2e.WithCompactHashCheckEnabled(true),
		e2e.WithCompactHashCheckTime(checkTime),
		e2e.WithClusterSize(3),
		e2e.WithDataDirPath(dataDirPath),
		e2e.WithLogLevel("info"),
	)
	epc, err := e2e.InitEtcdProcessCluster(t, cfg)
	require.NoError(t, err)

	// Assign a node a very slow compaction speed, so that its compaction can be interrupted.
	err = epc.UpdateProcOptions(slowCompactionNodeIndex, t,
		e2e.WithCompactionBatchLimit(1),
		e2e.WithCompactionSleepInterval(1*time.Hour),
	)
	require.NoError(t, err)

	epc, err = e2e.StartEtcdProcessCluster(ctx, epc, cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	// Put 10 identical keys to the cluster, so that the compaction will drop some stale values.
	t.Log("putting 10 values to the identical key...")
	cc := epc.Etcdctl()
	for i := 0; i < 10; i++ {
		err := cc.Put(ctx, "key", fmt.Sprint(i), config.PutOptions{})
		require.NoError(t, err, "error on put")
	}

	t.Log("compaction started...")
	_, err = cc.Compact(ctx, 5, config.CompactOption{})

	err = epc.Procs[slowCompactionNodeIndex].Close()
	require.NoError(t, err)

	err = epc.UpdateProcOptions(slowCompactionNodeIndex, t)
	require.NoError(t, err)

	t.Logf("restart proc %d to interrupt its compaction...", slowCompactionNodeIndex)
	err = epc.Procs[slowCompactionNodeIndex].Restart(ctx)

	// Wait until the node finished compaction and the leader finished compaction hash check
	_, err = epc.Procs[slowCompactionNodeIndex].Logs().ExpectWithContext(ctx, expect.ExpectedResponse{Value: "finished scheduled compaction"})
	require.NoError(t, err, "can't get log indicating finished scheduled compaction")

	leaderIndex := epc.WaitLeader(t)
	_, err = epc.Procs[leaderIndex].Logs().ExpectWithContext(ctx, expect.ExpectedResponse{Value: "finished compaction hash check"})
	require.NoError(t, err, "can't get log indicating finished compaction hash check")

	alarmResponse, err := cc.AlarmList(ctx)
	require.NoError(t, err, "error on alarm list")
	for _, alarm := range alarmResponse.Alarms {
		if alarm.Alarm == etcdserverpb.AlarmType_CORRUPT {
			t.Fatal("there should be no corruption after resuming the compaction, but corruption detected")
		}
	}
	t.Log("no corruption detected.")
}
