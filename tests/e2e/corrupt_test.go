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
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/v3"
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
	e2e.WaitReadyExpectProc(context.TODO(), proc, []string{fmt.Sprintf("etcdmain: %016x found data inconsistency with peers", id0)})
}

func TestPeriodicCheckDetectsCorruption(t *testing.T) {
	checkTime := time.Second
	e2e.BeforeTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	epc, err := e2e.NewEtcdProcessCluster(ctx, t, &e2e.EtcdProcessClusterConfig{
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

	cc := e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())

	for i := 0; i < 10; i++ {
		err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i), config.PutOptions{})
		assert.NoError(t, err, "error on put")
	}

	members, err := cc.MemberList(ctx)
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
	epc, err := e2e.NewEtcdProcessCluster(ctx, t, &e2e.EtcdProcessClusterConfig{
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

	cc := e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())

	for i := 0; i < 10; i++ {
		err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i), config.PutOptions{})
		assert.NoError(t, err, "error on put")
	}
	members, err := cc.MemberList(ctx)
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
