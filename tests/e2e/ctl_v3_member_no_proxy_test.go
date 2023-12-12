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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestMemberReplace(t *testing.T) {
	e2e.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t)
	require.NoError(t, err)
	defer epc.Close()

	memberId := rand.Int() % len(epc.Procs)
	member := epc.Procs[memberId]
	memberName := member.Config().Name
	var endpoints []string
	for i := 1; i < len(epc.Procs); i++ {
		endpoints = append(endpoints, epc.Procs[(memberId+i)%len(epc.Procs)].EndpointsGRPC()...)
	}
	cc, err := e2e.NewEtcdctl(epc.Cfg.Client, endpoints)
	require.NoError(t, err)

	c := epc.Etcdctl()
	memberID, found, err := getMemberIdByName(ctx, c, memberName)
	require.NoError(t, err)
	require.Equal(t, found, true, "Member not found")

	// Need to wait health interval for cluster to accept member changes
	time.Sleep(etcdserver.HealthInterval)

	t.Logf("Removing member %s", memberName)
	_, err = c.MemberRemove(ctx, memberID)
	require.NoError(t, err)
	_, found, err = getMemberIdByName(ctx, c, memberName)
	require.NoError(t, err)
	require.Equal(t, found, false, "Expected member to be removed")
	for member.IsRunning() {
		err = member.Wait(ctx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			t.Fatalf("member didn't exit as expected: %v", err)
		}
	}

	t.Logf("Removing member %s data", memberName)
	err = os.RemoveAll(member.Config().DataDirPath)
	require.NoError(t, err)

	t.Logf("Adding member %s back", memberName)
	removedMemberPeerUrl := member.Config().PeerURL.String()
	_, err = cc.MemberAdd(ctx, memberName, []string{removedMemberPeerUrl})
	require.NoError(t, err)
	err = patchArgs(member.Config().Args, "initial-cluster-state", "existing")
	require.NoError(t, err)

	t.Logf("Starting member %s", memberName)
	err = member.Start(ctx)
	require.NoError(t, err)
	testutils.ExecuteUntil(ctx, t, func() {
		for {
			_, found, err := getMemberIdByName(ctx, c, memberName)
			if err != nil || !found {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
	})
}
