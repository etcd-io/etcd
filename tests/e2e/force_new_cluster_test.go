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
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestForceNewCluster verified that etcd works as expected when --force-new-cluster.
// Refer to discussion in https://github.com/etcd-io/etcd/issues/20009.
func TestForceNewCluster(t *testing.T) {
	e2e.BeforeTest(t)

	testCases := []struct {
		name      string
		snapcount int
	}{
		{
			name:      "create a snapshot after promotion",
			snapcount: 10,
		},
		{
			name:      "no snapshot after promotion",
			snapcount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			epc, promotedMembers := mustCreateNewClusterByPromotingMembers(t, e2e.CurrentVersion, 5,
				e2e.WithKeepDataDir(true), e2e.WithSnapshotCount(uint64(tc.snapcount)))
			require.Len(t, promotedMembers, 4)

			for i := 0; i < tc.snapcount; i++ {
				err := epc.Etcdctl().Put(ctx, "foo", "bar", config.PutOptions{})
				require.NoError(t, err)
			}

			require.NoError(t, epc.Close())

			m := epc.Procs[0]
			t.Logf("Forcibly create a one-member cluster with member: %s", m.Config().Name)
			m.Config().Args = append(m.Config().Args, "--force-new-cluster")
			require.NoError(t, m.Start(ctx))

			t.Log("Restarting the member")
			require.NoError(t, m.Restart(ctx))

			t.Log("Closing the member")
			require.NoError(t, m.Close())
		})
	}
}

func TestForceNewCluster_MemberCount(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := context.Background()

	epc, promotedMembers := mustCreateNewClusterByPromotingMembers(t, e2e.CurrentVersion, 3, e2e.WithKeepDataDir(true))
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
	require.NoError(t, m.Start(ctx))

	t.Log("Online checking the member count")
	mresp, merr := m.Etcdctl().MemberList(ctx, false)
	require.NoError(t, merr)
	require.Len(t, mresp.Members, 1)

	t.Log("Closing the member")
	require.NoError(t, m.Close())
	require.NoError(t, m.Wait(ctx))

	t.Log("Offline checking the member count")
	members := mustReadMembersFromBoltDB(t, m.Config().DataDirPath)
	require.Len(t, members, 1)
}

// TestForceNewCluster_AddLearner_MemberCount verifies that `--force-new-cluster`
// should always be able to clean up all other members, including learners.
func TestForceNewCluster_AddLearner_MemberCount(t *testing.T) {
	e2e.BeforeTest(t)

	testCases := []struct {
		name      string
		snapcount int
	}{
		{
			name:      "no snapshot after adding learner",
			snapcount: 0,
		},
		{
			name:      "create a snapshot after adding learner",
			snapcount: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := e2e.NewConfig(e2e.WithClusterSize(3))
			epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg), e2e.WithSnapshotCount(uint64(tc.snapcount)), e2e.WithKeepDataDir(true))
			require.NoError(t, err)

			t.Log("Adding a learner member")
			testutils.ExecuteWithTimeout(t, 1*time.Minute, func() {
				for {
					_, aerr := epc.StartNewProc(t.Context(), nil, t, true)
					if aerr != nil {
						if strings.Contains(aerr.Error(), "etcdserver: unhealthy cluster") {
							time.Sleep(1 * time.Second)
							continue
						}
					}
					break
				}
			})

			for i := 0; i < tc.snapcount; i++ {
				werr := epc.Etcdctl().Put(t.Context(), "foo", "bar", config.PutOptions{})
				require.NoError(t, werr)
			}
			require.NoError(t, epc.Close())

			m := epc.Procs[0]
			t.Logf("Forcibly create a one-member cluster with member: %s", m.Config().Name)
			m.Config().Args = append(m.Config().Args, "--force-new-cluster")
			require.NoError(t, m.Start(t.Context()))

			t.Log("Restarting the member")
			require.NoError(t, m.Restart(t.Context()))
			defer func() {
				t.Log("Closing the member")
				require.NoError(t, m.Close())
			}()

			t.Log("Checking member count")
			resp, merr := m.Etcdctl().MemberList(t.Context(), false)
			require.NoError(t, merr)
			require.Len(t, resp.Members, 1)
		})
	}
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
		b := tx.Bucket(schema.Members.Name())
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
