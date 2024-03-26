// Copyright 2015 The etcd Authors
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

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestPauseMember(t *testing.T) {
	integration.BeforeTest(t)

	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 5})
	defer c.Terminate(t)

	for i := 0; i < 5; i++ {
		c.Members[i].Pause()
		membs := append([]*integration.Member{}, c.Members[:i]...)
		membs = append(membs, c.Members[i+1:]...)
		c.WaitMembersForLeader(t, membs)
		clusterMustProgress(t, membs)
		c.Members[i].Resume()
	}
	c.WaitMembersForLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

func TestRestartMember(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true})
	defer c.Terminate(t)

	for i := 0; i < 3; i++ {
		c.Members[i].Stop(t)
		membs := append([]*integration.Member{}, c.Members[:i]...)
		membs = append(membs, c.Members[i+1:]...)
		c.WaitMembersForLeader(t, membs)
		clusterMustProgress(t, membs)
		err := c.Members[i].Restart(t)
		if err != nil {
			t.Fatal(err)
		}
	}
	c.WaitMembersForLeader(t, c.Members)
	clusterMustProgress(t, c.Members)
}

func TestLaunchDuplicateMemberShouldFail(t *testing.T) {
	integration.BeforeTest(t)
	size := 3
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: size})
	m := c.Members[0].Clone(t)
	m.DataDir = t.TempDir()
	defer c.Terminate(t)

	if err := m.Launch(); err == nil {
		t.Errorf("unexpect successful launch")
	} else {
		t.Logf("launch failed as expected: %v", err)
		assert.Contains(t, err.Error(), "has already been bootstrapped")
	}
}

func TestSnapshotAndRestartMember(t *testing.T) {
	integration.BeforeTest(t)
	m := integration.MustNewMember(t, integration.MemberConfig{Name: "snapAndRestartTest", UseBridge: true})
	m.SnapshotCount = 100
	m.Launch()
	defer m.Terminate(t)
	defer m.Client.Close()
	m.WaitOK(t)

	var err error
	for i := 0; i < 120; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
		key := fmt.Sprintf("foo%d", i)
		_, err = m.Client.Put(ctx, "/"+key, "bar")
		if err != nil {
			t.Fatalf("#%d: create on %s error: %v", i, m.URL(), err)
		}
		cancel()
	}
	m.Stop(t)
	m.Restart(t)

	m.WaitOK(t)
	for i := 0; i < 120; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
		key := fmt.Sprintf("foo%d", i)
		resp, err := m.Client.Get(ctx, "/"+key)
		if err != nil {
			t.Fatalf("#%d: get on %s error: %v", i, m.URL(), err)
		}
		cancel()

		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Value) != "bar" {
			t.Errorf("#%d: got = %v, want %v", i, resp.Kvs[0], "bar")
		}
	}
}

func TestRemoveMember(t *testing.T) {
	integration.BeforeTest(t)
	c := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true, BackendBatchInterval: 1000 * time.Second})
	defer c.Terminate(t)
	// membership changes additionally require cluster to be stable for etcdserver.HealthInterval
	time.Sleep(etcdserver.HealthInterval)

	err := c.RemoveMember(t, c.Client(2), uint64(c.Members[0].ID()))
	require.NoError(t, err)

	checkMemberCount(t, c.Members[0], 2)
	checkMemberCount(t, c.Members[1], 2)
}

func checkMemberCount(t *testing.T, m *integration.Member, expectedMemberCount int) {
	be := schema.NewMembershipBackend(m.Logger, m.Server.Backend())
	membersFromBackend, _ := be.MustReadMembersFromBackend()
	if len(membersFromBackend) != expectedMemberCount {
		t.Errorf("Expect member count read from backend=%d, got %d", expectedMemberCount, len(membersFromBackend))
	}
	membersResp, err := m.Client.MemberList(context.Background())
	require.NoError(t, err)
	if len(membersResp.Members) != expectedMemberCount {
		t.Errorf("Expect len(MemberList)=%d, got %d", expectedMemberCount, len(membersResp.Members))
	}
}
