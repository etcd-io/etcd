// Copyright 2023 The etcd Authors
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
	"math/rand"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestV3CurlMaintenanceAlarmMissiongAlarm(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlMaintenanceAlarmMissiongAlarm, withApiPrefix(p), withCfg(*e2e.NewConfigNoTLS()))
	}
}

func testV3CurlMaintenanceAlarmMissiongAlarm(cx ctlCtx) {
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/maintenance/alarm"),
		Value:    `{"action": "ACTIVATE"}`,
	}); err != nil {
		cx.t.Fatalf("failed post maintenance alarm (%s) (%v)", cx.apiPrefix, err)
	}
}

func TestV3CurlMaintenanceStatus(t *testing.T) {
	testCtl(t, testV3CurlMaintenanceStatus, withCfg(*e2e.NewConfigNoTLS()))
}

func testV3CurlMaintenanceStatus(cx ctlCtx) {
	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[rand.Intn(clus.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: "/v3/maintenance/status",
		Value:    "{}",
	})
	resp, err := runCommandAndReadJsonOutput(args)
	require.NoError(cx.t, err)

	requiredFields := []string{"version", "dbSize", "leader", "raftIndex", "raftTerm", "raftAppliedIndex", "dbSizeInUse", "storageVersion"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			cx.t.Fatalf("Field %q not found in (%v)", field, resp)
		}
	}

	actualVersion, _ := resp["version"]
	require.Equal(cx.t, version.Version, actualVersion)
}

func TestV3CurlMaintenanceDefragment(t *testing.T) {
	testCtl(t, testV3CurlMaintenanceDefragment, withCfg(*e2e.NewConfigNoTLS()))
}

func testV3CurlMaintenanceDefragment(cx ctlCtx) {
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/maintenance/defragment",
		Value:    "{}",
		Expected: expect.ExpectedResponse{
			Value: "{}",
		},
	}); err != nil {
		cx.t.Fatalf("failed post maintenance defragment request: (%v)", err)
	}
}

func TestV3CurlMaintenanceHash(t *testing.T) {
	testCtl(t, testV3CurlMaintenanceHash, withCfg(*e2e.NewConfigNoTLS()))
}

func testV3CurlMaintenanceHash(cx ctlCtx) {
	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[rand.Intn(clus.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: "/v3/maintenance/hash",
		Value:    "{}",
	})
	resp, err := runCommandAndReadJsonOutput(args)
	require.NoError(cx.t, err)

	requiredFields := []string{"header", "hash"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			cx.t.Fatalf("Field %q not found in (%v)", field, resp)
		}
	}
}

func TestV3CurlMaintenanceHashKV(t *testing.T) {
	testCtl(t, testV3CurlMaintenanceHashKV, withCfg(*e2e.NewConfigNoTLS()))
}

func testV3CurlMaintenanceHashKV(cx ctlCtx) {
	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[rand.Intn(clus.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: "/v3/maintenance/hashkv",
		Value:    "{}",
	})
	resp, err := runCommandAndReadJsonOutput(args)
	require.NoError(cx.t, err)

	requiredFields := []string{"header", "hash", "compact_revision", "hash_revision"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			cx.t.Fatalf("Field %q not found in (%v)", field, resp)
		}
	}
}

func TestV3CurlMaintenanceSnapshot(t *testing.T) {
	testCtl(t, testV3CurlMaintenanceSnapshot, withCfg(*e2e.NewConfigNoTLS()))
}

func testV3CurlMaintenanceSnapshot(cx ctlCtx) {
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/maintenance/snapshot",
		Value:    "{}",
		Expected: expect.ExpectedResponse{
			Value: `"result":{"blob":`,
		},
	}); err != nil {
		cx.t.Fatalf("failed post maintenance snapshot request: (%v)", err)
	}
}

func TestV3CurlMaintenanceMoveleader(t *testing.T) {
	testCtl(t, testV3CurlMaintenanceMoveleader, withCfg(*e2e.NewConfigNoTLS()))
}

func testV3CurlMaintenanceMoveleader(cx ctlCtx) {
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/maintenance/transfer-leadership",
		Value:    `{"targetID": 123}`,
		Expected: expect.ExpectedResponse{
			Value: `"message":"etcdserver: bad leader transferee"`,
		},
	}); err != nil {
		cx.t.Fatalf("failed post maintenance moveleader request: (%v)", err)
	}
}

func TestV3CurlMaintenanceDowngrade(t *testing.T) {
	testCtl(t, testV3CurlMaintenanceDowngrade, withCfg(*e2e.NewConfigNoTLS()))
}

func testV3CurlMaintenanceDowngrade(cx ctlCtx) {
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/maintenance/downgrade",
		Value:    `{"action": 0, "version": "3.0"}`,
		Expected: expect.ExpectedResponse{
			Value: `"message":"etcdserver: invalid downgrade target version"`,
		},
	}); err != nil {
		cx.t.Fatalf("failed post maintenance downgrade request: (%v)", err)
	}
}
