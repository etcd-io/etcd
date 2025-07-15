// Copyright 2025 The etcd Authors
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
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/testutils"
)

func TestForceNewCluster_MemberCount(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := context.Background()

	epc, promotedMembers := mustCreateNewClusterByPromotingMembers(t, e2e.CurrentVersion, 3)
	require.Len(t, promotedMembers, 2)

	// Wait for the backend TXN to sync/commit the data to disk, to ensure
	// the consistent-index is persisted. Another way is to issue a snapshot
	// command to forcibly commit the backend TXN.
	time.Sleep(time.Second)

	t.Log("Killing all the members")
	require.NoError(t, epc.Kill())
	require.NoError(t, epc.Wait(ctx))

	m := epc.Procs[0]
	t.Logf("Forcibly create a one-member cluster with member: %s", m.Config().Name)
	m.Config().Args = append(m.Config().Args, "--force-new-cluster")
	require.NoError(t, m.Start())

	t.Log("Online checking the member count")
	mresp, merr := m.Etcdctl(e2e.ClientNonTLS, false, false).MemberList()
	require.NoError(t, merr)
	require.Len(t, mresp.Members, 1)

	t.Log("Closing the member")
	require.NoError(t, m.Close())
	require.NoError(t, m.Wait(ctx))

	t.Log("Offline checking the member count")
	members := mustReadMembersFromBoltDB(t, m.Config().DataDirPath)
	require.Len(t, members, 1)
}

func mustCreateNewClusterByPromotingMembers(t *testing.T, clusterVersion e2e.ClusterVersion, clusterSize int) (*e2e.EtcdProcessCluster, []*etcdserverpb.Member) {
	require.GreaterOrEqualf(t, clusterSize, 1, "clusterSize must be at least 1")

	t.Logf("Creating new etcd cluster - version: %s, clusterSize: %v", clusterVersion, clusterSize)
	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize: 1,
		KeepDataDir: true,
	})
	require.NoErrorf(t, err, "failed to start first etcd process")
	defer func() {
		if t.Failed() {
			epc.Close()
		}
	}()

	var promotedMembers []*etcdserverpb.Member
	for i := 1; i < clusterSize; i++ {
		var (
			memberID uint64
			aerr     error
		)

		// NOTE: New promoted member needs time to get connected.
		t.Logf("[%d] Adding new member as learner", i)
		testutils.ExecuteWithTimeout(t, 1*time.Minute, func() {
			for {
				memberID, aerr = epc.StartNewProc(nil, true, t)
				if aerr != nil {
					if strings.Contains(aerr.Error(), "etcdserver: unhealthy cluster") {
						time.Sleep(1 * time.Second)
						continue
					}
				}
				break
			}
		})
		require.NoError(t, aerr)

		t.Logf("[%d] Promoting member (%x)", i, memberID)
		etcdctl := epc.Procs[0].Etcdctl(e2e.ClientNonTLS, false, false)
		resp, merr := etcdctl.MemberPromote(memberID)
		require.NoError(t, merr)

		for _, m := range resp.Members {
			if m.ID == memberID {
				promotedMembers = append(promotedMembers, m)
			}
		}
	}

	t.Log("Ensure all members are voting members from user perspective")
	ensureAllMembersAreVotingMembers(t, epc.Etcdctl())

	return epc, promotedMembers
}

func mustReadMembersFromBoltDB(t *testing.T, dataDir string) []*membership.Member {
	dbPath := datadir.ToBackendFileName(dataDir)

	db, err := bbolt.Open(dbPath, 0o400, &bbolt.Options{ReadOnly: true})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	var members []*membership.Member
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(buckets.Members.Name())
		_ = b.ForEach(func(k, v []byte) error {
			m := membership.Member{}
			err := json.Unmarshal(v, &m)
			require.NoError(t, err)
			members = append(members, &m)
			return nil
		})
		return nil
	})

	return members
}
