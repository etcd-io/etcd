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
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t)
	require.NoError(t, err)
	defer epc.Close()

	memberIdx := rand.Int() % len(epc.Procs)
	member := epc.Procs[memberIdx]
	memberName := member.Config().Name
	var endpoints []string
	for i := 1; i < len(epc.Procs); i++ {
		endpoints = append(endpoints, epc.Procs[(memberIdx+i)%len(epc.Procs)].EndpointsGRPC()...)
	}
	cc, err := e2e.NewEtcdctl(epc.Cfg.Client, endpoints)
	require.NoError(t, err)

	memberID, found, err := getMemberIDByName(ctx, cc, memberName)
	require.NoError(t, err)
	require.Truef(t, found, "Member not found")

	// Need to wait health interval for cluster to accept member changes
	time.Sleep(etcdserver.HealthInterval)

	t.Logf("Removing member %s", memberName)
	_, err = cc.MemberRemove(ctx, memberID)
	require.NoError(t, err)
	_, found, err = getMemberIDByName(ctx, cc, memberName)
	require.NoError(t, err)
	require.Falsef(t, found, "Expected member to be removed")
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
	removedMemberPeerURL := member.Config().PeerURL.String()
	_, err = cc.MemberAdd(ctx, memberName, []string{removedMemberPeerURL})
	require.NoError(t, err)
	err = patchArgs(member.Config().Args, "initial-cluster-state", "existing")
	require.NoError(t, err)

	// Sleep 100ms to bypass the known issue https://github.com/etcd-io/etcd/issues/16687.
	time.Sleep(100 * time.Millisecond)
	t.Logf("Starting member %s", memberName)
	err = member.Start(ctx)
	require.NoError(t, err)
	testutils.ExecuteUntil(ctx, t, func() {
		for {
			_, found, err := getMemberIDByName(ctx, cc, memberName)
			if err != nil || !found {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
	})
}

func TestMemberReplaceWithLearner(t *testing.T) {
	e2e.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t)
	require.NoError(t, err)
	defer epc.Close()

	memberIdx := rand.Int() % len(epc.Procs)
	member := epc.Procs[memberIdx]
	memberName := member.Config().Name
	var endpoints []string
	for i := 1; i < len(epc.Procs); i++ {
		endpoints = append(endpoints, epc.Procs[(memberIdx+i)%len(epc.Procs)].EndpointsGRPC()...)
	}
	cc, err := e2e.NewEtcdctl(epc.Cfg.Client, endpoints)
	require.NoError(t, err)

	memberID, found, err := getMemberIDByName(ctx, cc, memberName)
	require.NoError(t, err)
	require.Truef(t, found, "Member not found")

	// Need to wait health interval for cluster to accept member changes
	time.Sleep(etcdserver.HealthInterval)

	t.Logf("Removing member %s", memberName)
	_, err = cc.MemberRemove(ctx, memberID)
	require.NoError(t, err)
	_, found, err = getMemberIDByName(ctx, cc, memberName)
	require.NoError(t, err)
	require.Falsef(t, found, "Expected member to be removed")
	for member.IsRunning() {
		err = member.Wait(ctx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			t.Fatalf("member didn't exit as expected: %v", err)
		}
	}

	t.Logf("Removing member %s data", memberName)
	err = os.RemoveAll(member.Config().DataDirPath)
	require.NoError(t, err)

	t.Logf("Adding member %s back as Learner", memberName)
	removedMemberPeerURL := member.Config().PeerURL.String()
	_, err = cc.MemberAddAsLearner(ctx, memberName, []string{removedMemberPeerURL})
	require.NoError(t, err)

	err = patchArgs(member.Config().Args, "initial-cluster-state", "existing")
	require.NoError(t, err)

	// Sleep 100ms to bypass the known issue https://github.com/etcd-io/etcd/issues/16687.
	time.Sleep(100 * time.Millisecond)

	t.Logf("Starting member %s", memberName)
	err = member.Start(ctx)
	require.NoError(t, err)
	var learnMemberID uint64
	testutils.ExecuteUntil(ctx, t, func() {
		for {
			learnMemberID, found, err = getMemberIDByName(ctx, cc, memberName)
			if err != nil || !found {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
	})

	learnMemberID, found, err = getMemberIDByName(ctx, cc, memberName)
	require.NoError(t, err)
	require.Truef(t, found, "Member not found")

	_, err = cc.MemberPromote(ctx, learnMemberID)
	require.NoError(t, err)
}
