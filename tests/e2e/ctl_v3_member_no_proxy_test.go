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

//go:build !cluster_proxy

package e2e

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestMemberReplace(t *testing.T) {
	e2e.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize:      3,
		KeepDataDir:      true,
		CorruptCheckTime: time.Second,
	})
	require.NoError(t, err)
	defer epc.Close()

	memberIdx := rand.Int() % len(epc.Procs)
	member := epc.Procs[memberIdx]
	memberName := member.Config().Name
	var endpoints []string
	for i := 1; i < len(epc.Procs); i++ {
		endpoints = append(endpoints, epc.Procs[(memberIdx+i)%len(epc.Procs)].EndpointsGRPC()...)
	}
	cc := NewEtcdctl(endpoints, e2e.ClientNonTLS, false, false)

	memberID, found, err := getMemberIdByName(ctx, cc, memberName)
	require.NoError(t, err)
	require.Equal(t, found, true, "Member not found")

	// Need to wait health interval for cluster to accept member changes
	time.Sleep(etcdserver.HealthInterval)

	t.Logf("Removing member %s", memberName)
	_, err = cc.MemberRemove(memberID)
	require.NoError(t, err)
	_, found, err = getMemberIdByName(ctx, cc, memberName)
	require.NoError(t, err)
	require.Equal(t, found, false, "Expected member to be removed")
	for member.IsRunning() {
		member.Close()
		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("Removing member %s data", memberName)
	err = os.RemoveAll(member.Config().DataDirPath)
	require.NoError(t, err)

	t.Logf("Adding member %s back", memberName)
	removedMemberPeerUrl := member.Config().Purl.String()
	_, err = cc.MemberAdd(memberName, []string{removedMemberPeerUrl})
	require.NoError(t, err)
	member.Config().Args = patchArgs(member.Config().Args, "initial-cluster-state", "existing")
	require.NoError(t, err)

	// Sleep 100ms to bypass the known issue https://github.com/etcd-io/etcd/issues/16687.
	time.Sleep(100 * time.Millisecond)
	t.Logf("Starting member %s", memberName)
	err = member.Start()
	require.NoError(t, err)
	e2e.ExecuteUntil(ctx, t, func() {
		for {
			_, found, err := getMemberIdByName(ctx, cc, memberName)
			if err != nil || !found {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
	})
}
