// Copyright 2016 The etcd Authors
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestCtlV3MemberList(t *testing.T)        { testCtl(t, memberListTest) }
func TestCtlV3MemberListWithHex(t *testing.T) { testCtl(t, memberListWithHexTest) }
func TestCtlV3MemberListSerializable(t *testing.T) {
	cfg := e2e.NewConfig(
		e2e.WithClusterSize(1),
	)
	testCtl(t, memberListSerializableTest, withCfg(*cfg))
}

func TestCtlV3MemberAdd(t *testing.T)          { testCtl(t, memberAddTest) }
func TestCtlV3MemberAddAsLearner(t *testing.T) { testCtl(t, memberAddAsLearnerTest) }

func TestCtlV3MemberUpdate(t *testing.T) { testCtl(t, memberUpdateTest) }

func TestCtlV3MemberPromoteWithAuthFromLeader(t *testing.T) {
	testCtl(t, memberPromoteWithAuth(false), withTestTimeout(30*time.Second))
}

func TestCtlV3MemberPromoteWithAuthFromFollower(t *testing.T) {
	testCtl(t, memberPromoteWithAuth(true), withTestTimeout(30*time.Second))
}

func TestCtlV3MemberUpdateNoTLS(t *testing.T) {
	testCtl(t, memberUpdateTest, withCfg(*e2e.NewConfigNoTLS()))
}

func TestCtlV3MemberUpdateClientTLS(t *testing.T) {
	testCtl(t, memberUpdateTest, withCfg(*e2e.NewConfigClientTLS()))
}

func TestCtlV3MemberUpdateClientAutoTLS(t *testing.T) {
	testCtl(t, memberUpdateTest, withCfg(*e2e.NewConfigClientAutoTLS()))
}

func TestCtlV3MemberUpdatePeerTLS(t *testing.T) {
	testCtl(t, memberUpdateTest, withCfg(*e2e.NewConfigPeerTLS()))
}

// TestCtlV3ConsistentMemberList requires the gofailpoint to be enabled.
// If you execute this case locally, please do not forget to execute
// `make gofail-enable`.
func TestCtlV3ConsistentMemberList(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := context.Background()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithEnvVars(map[string]string{"GOFAIL_FAILPOINTS": `beforeApplyOneConfChange=sleep("2s")`}),
	)
	require.NoErrorf(t, err, "failed to start etcd cluster")
	defer func() {
		derr := epc.Close()
		require.NoErrorf(t, derr, "failed to close etcd cluster")
	}()

	t.Log("Adding and then removing a learner")
	resp, err := epc.Etcdctl().MemberAddAsLearner(ctx, "newLearner", []string{fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)})
	require.NoError(t, err)
	_, err = epc.Etcdctl().MemberRemove(ctx, resp.Member.ID)
	require.NoError(t, err)
	t.Logf("Added and then removed a learner with ID: %x", resp.Member.ID)

	t.Log("Restarting the etcd process to ensure all data is persisted")
	err = epc.Procs[0].Restart(ctx)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)
	stopc := make(chan struct{}, 2)

	t.Log("Starting a goroutine to repeatedly restart etcdserver")
	go func() {
		defer func() {
			stopc <- struct{}{}
			wg.Done()
		}()
		for i := 0; i < 3; i++ {
			select {
			case <-stopc:
				return
			default:
			}

			merr := epc.Procs[0].Restart(ctx)
			assert.NoError(t, merr)
			epc.WaitLeader(t)

			time.Sleep(100 * time.Millisecond)
		}
	}()

	t.Log("Starting a goroutine to repeated check the member list")
	count := 0
	go func() {
		defer func() {
			stopc <- struct{}{}
			wg.Done()
		}()

		for {
			select {
			case <-stopc:
				return
			default:
			}

			mresp, merr := epc.Etcdctl().MemberList(ctx, true)
			if merr != nil {
				continue
			}

			count++
			assert.Len(t, mresp.Members, 1)
		}
	}()

	wg.Wait()
	assert.Positive(t, count)
	t.Logf("Checked the member list %d times", count)
}

func memberListTest(cx ctlCtx) {
	if err := ctlV3MemberList(cx); err != nil {
		cx.t.Fatalf("memberListTest ctlV3MemberList error (%v)", err)
	}
}

func memberListSerializableTest(cx ctlCtx) {
	resp, err := getMemberList(cx, false)
	require.NoError(cx.t, err)
	require.Len(cx.t, resp.Members, 1)

	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	err = ctlV3MemberAdd(cx, peerURL, false)
	require.NoError(cx.t, err)

	resp, err = getMemberList(cx, true)
	require.NoError(cx.t, err)
	require.Len(cx.t, resp.Members, 2)
}

func ctlV3MemberList(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "list")
	lines := make([]expect.ExpectedResponse, cx.cfg.ClusterSize)
	for i := range lines {
		lines[i] = expect.ExpectedResponse{Value: "started"}
	}
	return e2e.SpawnWithExpects(cmdArgs, cx.envMap, lines...)
}

func getMemberList(cx ctlCtx, serializable bool) (etcdserverpb.MemberListResponse, error) {
	cmdArgs := append(cx.PrefixArgs(), "--write-out", "json", "member", "list")
	if serializable {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}

	proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		return etcdserverpb.MemberListResponse{}, err
	}
	var txt string
	txt, err = proc.Expect("members")
	if err != nil {
		return etcdserverpb.MemberListResponse{}, err
	}
	if err = proc.Close(); err != nil {
		return etcdserverpb.MemberListResponse{}, err
	}

	resp := etcdserverpb.MemberListResponse{}
	dec := json.NewDecoder(strings.NewReader(txt))
	if err := dec.Decode(&resp); errors.Is(err, io.EOF) {
		return etcdserverpb.MemberListResponse{}, err
	}
	return resp, nil
}

func memberListWithHexTest(cx ctlCtx) {
	resp, err := getMemberList(cx, false)
	if err != nil {
		cx.t.Fatalf("getMemberList error (%v)", err)
	}

	cmdArgs := append(cx.PrefixArgs(), "--write-out", "json", "--hex", "member", "list")

	proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		cx.t.Fatalf("memberListWithHexTest error (%v)", err)
	}
	var txt string
	txt, err = proc.Expect("members")
	if err != nil {
		cx.t.Fatalf("memberListWithHexTest error (%v)", err)
	}
	if err = proc.Close(); err != nil {
		cx.t.Fatalf("memberListWithHexTest error (%v)", err)
	}
	hexResp := etcdserverpb.MemberListResponse{}
	dec := json.NewDecoder(strings.NewReader(txt))
	if err := dec.Decode(&hexResp); errors.Is(err, io.EOF) {
		cx.t.Fatalf("memberListWithHexTest error (%v)", err)
	}
	num := len(resp.Members)
	hexNum := len(hexResp.Members)
	if num != hexNum {
		cx.t.Fatalf("member number,expected %d,got %d", num, hexNum)
	}
	if num == 0 {
		cx.t.Fatal("member number is 0")
	}

	if resp.Header.RaftTerm != hexResp.Header.RaftTerm {
		cx.t.Fatalf("Unexpected raft_term, expected %d, got %d", resp.Header.RaftTerm, hexResp.Header.RaftTerm)
	}

	for i := 0; i < num; i++ {
		if resp.Members[i].Name != hexResp.Members[i].Name {
			cx.t.Fatalf("Unexpected member name,expected %v, got %v", resp.Members[i].Name, hexResp.Members[i].Name)
		}
		if !reflect.DeepEqual(resp.Members[i].PeerURLs, hexResp.Members[i].PeerURLs) {
			cx.t.Fatalf("Unexpected member peerURLs, expected %v, got %v", resp.Members[i].PeerURLs, hexResp.Members[i].PeerURLs)
		}
		if !reflect.DeepEqual(resp.Members[i].ClientURLs, hexResp.Members[i].ClientURLs) {
			cx.t.Fatalf("Unexpected member clientURLs, expected %v, got %v", resp.Members[i].ClientURLs, hexResp.Members[i].ClientURLs)
		}
	}
}

func memberAddTest(cx ctlCtx) {
	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	require.NoError(cx.t, ctlV3MemberAdd(cx, peerURL, false))
}

func memberAddAsLearnerTest(cx ctlCtx) {
	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	require.NoError(cx.t, ctlV3MemberAdd(cx, peerURL, true))
}

func memberPromoteWithAuth(fromFollower bool) func(cx ctlCtx) {
	return func(cx ctlCtx) {
		ctx := context.Background()

		// Add a regular member
		_, err := cx.epc.StartNewProc(ctx, nil, cx.t, false)
		require.NoError(cx.t, err)

		var learnerID uint64
		var addErr error
		for {
			// Add a learner once the cluster is healthy
			learnerID, addErr = cx.epc.StartNewProc(ctx, nil, cx.t, true)
			if addErr != nil {
				if strings.Contains(addErr.Error(), "etcdserver: unhealthy cluster") {
					time.Sleep(1 * time.Second)
					continue
				}
			}
			break
		}
		require.NoError(cx.t, addErr)

		leaderIdx := cx.epc.WaitLeader(cx.t)
		followerIdx := (leaderIdx + 1) % len(cx.epc.Procs)

		require.NoError(cx.t, authEnable(cx))
		cx.user, cx.pass = "root", "root"

		if fromFollower {
			_, err = cx.epc.Procs[followerIdx].
				Etcdctl(e2e.WithAuth("root", "root")).
				MemberPromote(ctx, learnerID)
		} else {
			_, err = cx.epc.Procs[leaderIdx].
				Etcdctl(e2e.WithAuth("root", "root")).
				MemberPromote(ctx, learnerID)
		}

		require.NoError(cx.t, err)
	}
}

func ctlV3MemberAdd(cx ctlCtx, peerURL string, isLearner bool) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "add", "newmember", fmt.Sprintf("--peer-urls=%s", peerURL))
	asLearner := " "
	if isLearner {
		cmdArgs = append(cmdArgs, "--learner")
		asLearner = " as learner "
	}
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: fmt.Sprintf(" added%sto cluster ", asLearner)})
}

func memberUpdateTest(cx ctlCtx) {
	mr, err := getMemberList(cx, false)
	require.NoError(cx.t, err)

	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	memberID := fmt.Sprintf("%x", mr.Members[0].ID)
	require.NoError(cx.t, ctlV3MemberUpdate(cx, memberID, peerURL))
}

func ctlV3MemberUpdate(cx ctlCtx, memberID, peerURL string) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "update", memberID, fmt.Sprintf("--peer-urls=%s", peerURL))
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: " updated in cluster "})
}

func TestRemoveNonExistingMember(t *testing.T) {
	e2e.BeforeTest(t)
	ctx := context.Background()

	cfg := e2e.ConfigStandalone(*e2e.NewConfig())
	epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(cfg))
	require.NoError(t, err)
	defer epc.Close()
	c := epc.Etcdctl()

	_, err = c.MemberRemove(ctx, 1)
	require.Error(t, err)

	// Ensure that membership is properly bootstrapped.
	assert.NoError(t, epc.Restart(ctx))
}

// TestClusterUpgradeWithPromotedLearner tests whether etcd can automatically
// fix the issue caused by https://github.com/etcd-io/etcd/issues/19557
// when upgrading from 3.5 to 3.6.
func TestClusterUpgradeWithPromotedLearner(t *testing.T) {
	testCases := []struct {
		name                  string
		snapshotCount         uint64
		writeToV3StoreSuccess bool
	}{
		{
			name:          "create snapshot after learner promotion which is not saved to v3store",
			snapshotCount: 10,
		},
		{
			name:          "no snapshot and learner promotion is not saved to v3store",
			snapshotCount: 0,
		},
		{
			name:                  "no snapshot and learner promotion is saved to v3store",
			snapshotCount:         0,
			writeToV3StoreSuccess: true,
		},
	}

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)

			ctx := context.Background()

			epc, promotedMembers := mustCreateNewClusterByPromotingMembers(t, e2e.LastVersion, 2, e2e.WithSnapshotCount(tc.snapshotCount))
			defer func() {
				require.NoError(t, epc.Close())
			}()
			require.Len(t, promotedMembers, 1)
			t.Logf("Promoted member: %+v", promotedMembers[0])

			if tc.snapshotCount != 0 {
				t.Logf("Write %d keys to trigger a snapshot", tc.snapshotCount)
				for i := 0; i < int(tc.snapshotCount); i++ {
					err := epc.Etcdctl().Put(ctx, fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i), config.PutOptions{})
					require.NoError(t, err)
				}
			}

			if tc.writeToV3StoreSuccess {
				t.Log("Skip manually changing the already promoted member to a learner in v3store")
			} else {
				t.Logf("Stopping all members")
				require.NoError(t, epc.Stop())

				t.Log("Manually changing the already promoted member to a learner again in the v3store of all members to simulate the issue https://github.com/etcd-io/etcd/issues/19557")
				promotedMembers[0].IsLearner = true
				for i := 0; i < len(epc.Procs); i++ {
					mustSaveMemberIntoBbolt(t, epc.Procs[i].Config().DataDirPath, promotedMembers[0])
				}
			}

			t.Log("Upgrading the cluster")
			currentVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
			require.NoError(t, err)
			lastVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.EtcdLastRelease)
			require.NoError(t, err)
			err = e2e.DowngradeUpgradeMembers(t, nil, epc, len(epc.Procs), false, lastVersion, currentVersion)
			require.NoError(t, err)

			t.Log("Check the expected log message")
			for _, proc := range epc.Procs {
				if tc.snapshotCount != 0 {
					e2e.AssertProcessLogs(t, proc, "Syncing member in v3store")
				} else {
					if tc.writeToV3StoreSuccess {
						e2e.AssertProcessLogs(t, proc, "ignore already promoted member")
					} else {
						e2e.AssertProcessLogs(t, proc, "Forcibly apply member promotion request")
					}
				}
			}

			t.Log("Checking all members are ready to serve client requests")
			for i := 0; i < len(epc.Procs); i++ {
				e2e.AssertProcessLogs(t, epc.Procs[i], e2e.EtcdServerReadyLines[0])
			}

			t.Log("Ensure all members in v3store are voting members again")
			for i := 0; i < len(epc.Procs); i++ {
				t.Logf("Stopping the member: %d", i)
				require.NoError(t, epc.Procs[i].Stop())

				t.Logf("Checking all members in member's backend store: %d", i)
				ensureAllMembersFromV3StoreAreVotingMembers(t, epc.Procs[i].Config().DataDirPath)

				t.Logf("Starting the member again: %d", i)
				require.NoError(t, epc.Procs[i].Start(ctx))
			}
		})
	}
}

func mustCreateNewClusterByPromotingMembers(t *testing.T, clusterVersion e2e.ClusterVersion, clusterSize int, opts ...e2e.EPClusterOption) (*e2e.EtcdProcessCluster, []*etcdserverpb.Member) {
	require.GreaterOrEqualf(t, clusterSize, 1, "clusterSize must be at least 1")

	ctx := context.Background()

	t.Logf("Creating new etcd cluster - version: %s, clusterSize: %v", clusterVersion, clusterSize)
	opts = append(opts, e2e.WithVersion(clusterVersion), e2e.WithClusterSize(1))
	epc, err := e2e.NewEtcdProcessCluster(ctx, t, opts...)
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
				memberID, aerr = epc.StartNewProc(ctx, nil, t, true)
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
		etcdctl := epc.Procs[0].Etcdctl()
		resp, merr := etcdctl.MemberPromote(ctx, memberID)
		require.NoError(t, merr)

		for _, m := range resp.Members {
			if m.ID == memberID {
				promotedMembers = append(promotedMembers, m)
			}
		}
	}

	t.Log("Ensure all members are voting members from user perspective")
	ensureAllMembersAreVotingMembers(t, epc)

	return epc, promotedMembers
}

func mustSaveMemberIntoBbolt(t *testing.T, dataDir string, protoMember *etcdserverpb.Member) {
	dbPath := datadir.ToBackendFileName(dataDir)
	db, err := bbolt.Open(dbPath, 0o666, nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	m := &membership.Member{
		ID: types.ID(protoMember.ID),
		RaftAttributes: membership.RaftAttributes{
			PeerURLs:  protoMember.PeerURLs,
			IsLearner: protoMember.IsLearner,
		},
		Attributes: membership.Attributes{
			Name:       protoMember.Name,
			ClientURLs: protoMember.ClientURLs,
		},
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(schema.Members.Name())

		mkey := []byte(m.ID.String())
		mvalue, jerr := json.Marshal(m)
		require.NoError(t, jerr)

		return b.Put(mkey, mvalue)
	})
	require.NoError(t, err)
}

func ensureAllMembersAreVotingMembers(t *testing.T, epc *e2e.EtcdProcessCluster) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	memberListResp, err := epc.Etcdctl().MemberList(ctx, false)
	require.NoError(t, err)
	for _, m := range memberListResp.Members {
		require.False(t, m.IsLearner)
	}
}

func ensureAllMembersFromV3StoreAreVotingMembers(t *testing.T, dataDir string) {
	dbPath := datadir.ToBackendFileName(dataDir)
	db, err := bbolt.Open(dbPath, 0o400, &bbolt.Options{ReadOnly: true})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	var members []membership.Member
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(schema.Members.Name())
		_ = b.ForEach(func(k, v []byte) error {
			m := membership.Member{}
			jerr := json.Unmarshal(v, &m)
			require.NoError(t, jerr)
			members = append(members, m)
			return nil
		})
		return nil
	})

	for _, m := range members {
		require.Falsef(t, m.IsLearner, "member is still learner: %+v", m)
	}
}
