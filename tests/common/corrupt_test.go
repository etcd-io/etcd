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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestEtcdCorruptHash(t *testing.T) {
	testRunner.BeforeTest(t)

	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t)
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			corruptTest(t, cc, ctx)
		})
	}
}

func corruptTest(t *testing.T, c interfaces.Client, ctx context.Context) {
	t.Helper()
	t.Log("putting 10 keys...")

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%05d", i)
		value := fmt.Sprintf("val-%05d", i)
		require.NoErrorf(t, c.Put(ctx, key, value, config.PutOptions{}), "could not put key %q", key)
	}

	t.Log("sleeping 3sec to let nodes sync")
	time.Sleep(3 * time.Second)

	t.Log("connecting to clientv3")
	// I don't know how to convert this part
	// eps := cx.epc.EndpointsGRPC()
	// cli1, err := clientv3.New(clientv3.Config{Endpoints: []string{eps[1]}, DialTimeout: 3 * time.Second})
	// require.NoError(cx.t, err)
	// defer cli1.Close()

	// sresp, err := cli1.Status(context.TODO(), eps[0])
	// t.Logf("checked status sresp:%v err:%v", sresp, err)
	// require.NoError(cx.t, err)
	// id0 := sresp.Header.GetMemberId()

	// t.Log("stopping etcd[0]...")
	// cx.epc.Procs[0].Stop()

	// // corrupting first member by modifying backend offline.
	// fp := datadir.ToBackendFileName(cx.epc.Procs[0].Config().DataDirPath)
	// t.Logf("corrupting backend: %v", fp)
	// err = cx.corruptFunc(fp)
	// require.NoError(cx.t, err)

	// t.Log("restarting etcd[0]")
	// ep := cx.epc.Procs[0]
	// proc, err := e2e.SpawnCmd(append([]string{ep.Config().ExecPath}, ep.Config().Args...), cx.envMap)
	// require.NoError(cx.t, err)
	// defer proc.Stop()

	// t.Log("waiting for etcd[0] failure...")
	// // restarting corrupted member should fail
	// e2e.WaitReadyExpectProc(context.TODO(), proc, []string{fmt.Sprintf("etcdmain: %016x found data inconsistency with peers", id0)})

}

func TestInPlaceRecovery(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {

		})
	}
}

func TestPeriodicCheckDetectsCorruption(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {

		})
	}
}

func TestCompactHashCheckDetectCorruption(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			testCompactHashCheckDetectCorruption(t, false)
		})
	}
}

func TestCompactHashCheckDetectCorruptionWithFeatureGate(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			testCompactHashCheckDetectCorruption(t, true)
		})
	}
}

func testCompactHashCheckDetectCorruption(t *testing.T, useFeatureGate bool) {
	t.Helper()
	testRunner.BeforeTest(t)
	checkTime := time.Second
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cfg := config.NewClusterConfig()

	// not sure what to do for this
	// opts := []e2e.EPClusterOption{e2e.WithKeepDataDir(true), e2e.WithCompactHashCheckTime(checkTime)}

	if useFeatureGate {
		opts = append(opts, e2e.WithServerFeatureGate("CompactHashCheck", true))
	} else {
		opts = append(opts, e2e.WithCompactHashCheckEnabled(true))
	}

	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
	defer clus.Close()

}
