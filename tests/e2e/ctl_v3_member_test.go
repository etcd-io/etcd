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
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3MemberList(t *testing.T)        { testCtl(t, memberListTest) }
func TestCtlV3MemberListWithHex(t *testing.T) { testCtl(t, memberListWithHexTest) }
func TestCtlV3MemberListNoTLS(t *testing.T) {
	testCtl(t, memberListTest, withCfg(*e2e.NewConfigNoTLS()))
}
func TestCtlV3MemberListClientTLS(t *testing.T) {
	testCtl(t, memberListTest, withCfg(*e2e.NewConfigClientTLS()))
}
func TestCtlV3MemberListClientAutoTLS(t *testing.T) {
	testCtl(t, memberListTest, withCfg(*e2e.NewConfigClientAutoTLS()))
}
func TestCtlV3MemberListPeerTLS(t *testing.T) {
	testCtl(t, memberListTest, withCfg(*e2e.NewConfigPeerTLS()))
}
func TestCtlV3MemberRemove(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig())
}
func TestCtlV3MemberRemoveNoTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(*e2e.NewConfigNoTLS()))
}
func TestCtlV3MemberRemoveClientTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(*e2e.NewConfigClientTLS()))
}
func TestCtlV3MemberRemoveClientAutoTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(
		// default ClusterSize is 1
		e2e.EtcdProcessClusterConfig{
			ClusterSize:     3,
			IsClientAutoTLS: true,
			ClientTLS:       e2e.ClientTLS,
			InitialToken:    "new",
		}))
}
func TestCtlV3MemberRemovePeerTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(*e2e.NewConfigPeerTLS()))
}
func TestCtlV3MemberAdd(t *testing.T)      { testCtl(t, memberAddTest) }
func TestCtlV3MemberAddNoTLS(t *testing.T) { testCtl(t, memberAddTest, withCfg(*e2e.NewConfigNoTLS())) }
func TestCtlV3MemberAddClientTLS(t *testing.T) {
	testCtl(t, memberAddTest, withCfg(*e2e.NewConfigClientTLS()))
}
func TestCtlV3MemberAddClientAutoTLS(t *testing.T) {
	testCtl(t, memberAddTest, withCfg(*e2e.NewConfigClientAutoTLS()))
}
func TestCtlV3MemberAddPeerTLS(t *testing.T) {
	testCtl(t, memberAddTest, withCfg(*e2e.NewConfigPeerTLS()))
}
func TestCtlV3MemberAddForLearner(t *testing.T) { testCtl(t, memberAddForLearnerTest) }
func TestCtlV3MemberUpdate(t *testing.T)        { testCtl(t, memberUpdateTest) }
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

func memberListTest(cx ctlCtx) {
	if err := ctlV3MemberList(cx); err != nil {
		cx.t.Fatalf("memberListTest ctlV3MemberList error (%v)", err)
	}
}

func ctlV3MemberList(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "list")
	lines := make([]string, cx.cfg.ClusterSize)
	for i := range lines {
		lines[i] = "started"
	}
	return e2e.SpawnWithExpects(cmdArgs, cx.envMap, lines...)
}

func getMemberList(cx ctlCtx) (etcdserverpb.MemberListResponse, error) {
	cmdArgs := append(cx.PrefixArgs(), "--write-out", "json", "member", "list")

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
	if err := dec.Decode(&resp); err == io.EOF {
		return etcdserverpb.MemberListResponse{}, err
	}
	return resp, nil
}

func memberListWithHexTest(cx ctlCtx) {
	resp, err := getMemberList(cx)
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
	if err := dec.Decode(&hexResp); err == io.EOF {
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
			cx.t.Fatalf("Unexpected member clientURLS, expected %v, got %v", resp.Members[i].ClientURLs, hexResp.Members[i].ClientURLs)
		}
	}
}

func memberRemoveTest(cx ctlCtx) {
	ep, memIDToRemove, clusterID := cx.memberToRemove()
	if err := ctlV3MemberRemove(cx, ep, memIDToRemove, clusterID); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3MemberRemove(cx ctlCtx, ep, memberID, clusterID string) error {
	cmdArgs := append(cx.prefixArgs([]string{ep}), "member", "remove", memberID)
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, fmt.Sprintf("%s removed from cluster %s", memberID, clusterID))
}

func memberAddTest(cx ctlCtx) {
	if err := ctlV3MemberAdd(cx, fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11), false); err != nil {
		cx.t.Fatal(err)
	}
}

func memberAddForLearnerTest(cx ctlCtx) {
	if err := ctlV3MemberAdd(cx, fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11), true); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3MemberAdd(cx ctlCtx, peerURL string, isLearner bool) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "add", "newmember", fmt.Sprintf("--peer-urls=%s", peerURL))
	if isLearner {
		cmdArgs = append(cmdArgs, "--learner")
	}
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, " added to cluster ")
}

func memberUpdateTest(cx ctlCtx) {
	mr, err := getMemberList(cx)
	if err != nil {
		cx.t.Fatal(err)
	}

	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	memberID := fmt.Sprintf("%x", mr.Members[0].ID)
	if err = ctlV3MemberUpdate(cx, memberID, peerURL); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3MemberUpdate(cx ctlCtx, memberID, peerURL string) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "update", memberID, fmt.Sprintf("--peer-urls=%s", peerURL))
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, " updated in cluster ")
}

func TestCtlV3PromotingLearner(t *testing.T) {
	e2e.BeforeTest(t)

	t.Log("Create a single node etcd cluster")
	cfg := e2e.NewConfigNoTLS()
	cfg.BasePeerScheme = "unix"
	cfg.ClusterSize = 1

	epc, err := e2e.NewEtcdProcessCluster(t, cfg)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		derr := epc.Close()
		require.NoError(t, derr, "failed to close etcd cluster: %v", derr)
	}()

	t.Log("Add and start a learner")
	learnerID, err := epc.StartNewProc(nil, true, t)
	require.NoError(t, err)

	t.Log("Write a key to ensure the cluster is healthy so far")
	etcdctl := epc.Procs[0].Etcdctl(e2e.ClientNonTLS, false, false)
	err = etcdctl.Put("foo", "bar")
	require.NoError(t, err)

	t.Logf("Promoting the learner %x", learnerID)
	_, err = etcdctl.MemberPromote(learnerID)
	require.NoError(t, err)
}

func TestIssue19557FixMemberDataAfterUpgrade(t *testing.T) {
	e2e.BeforeTest(t)

	v3518Binary := downloadReleaseBinary(t, "v3.5.18")

	t.Log("Create a single node etcd cluster with v3.5.18")
	cfg := e2e.NewConfigNoTLS()
	cfg.BasePeerScheme = "unix"
	cfg.ClusterSize = 1
	cfg.ExecPath = v3518Binary
	cfg.KeepDataDir = true
	cfg.BackendBatchLimit = 1
	cfg.SnapshotCount = 10
	epc, err := e2e.NewEtcdProcessCluster(t, cfg)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		derr := epc.Close()
		require.NoError(t, derr, "failed to close etcd cluster")
	}()

	t.Log("Add and start a learner")
	learnerID, err := epc.StartNewProc(nil, true, t)
	require.NoError(t, err)

	t.Logf("Promoting the learner %x", learnerID)
	etcdctl := epc.Procs[0].Etcdctl(e2e.ClientNonTLS, false, false)
	_, err = etcdctl.MemberPromote(learnerID)
	require.NoError(t, err)

	t.Logf("Adding 10 key/value to trigger snapshot")
	etcdctl = epc.Procs[1].Etcdctl(e2e.ClientNonTLS, false, false)
	for i := 0; i < 10; i++ {
		err = etcdctl.Put("foo", "bar")
		require.NoError(t, err, "failed to ensure cluster is healthy")
	}

	t.Logf("Stopping the cluster with %s", e2e.BinPath)
	for _, proc := range epc.Procs {
		proc.WithStopSignal(syscall.SIGTERM)
	}
	require.NoError(t, epc.Stop(), "failed to close etcd cluster")
	for _, proc := range epc.Procs {
		t.Logf("Checking %s v3-backend", proc.Config().Name)
		dbPath := datadir.ToBackendFileName(proc.Config().DataDirPath)
		mInfo := membershipInfoFromBackend(t, dbPath, learnerID)
		require.True(t, mInfo.IsLearner, "%x is learner (%+v)", learnerID, mInfo)
	}

	t.Log("Upgrade to current release")
	for i := range epc.Procs {
		epc.Procs[i].Config().ExecPath = e2e.BinPath
	}
	require.NoError(t, epc.Restart(), "failed to restart etcd cluster")
	require.NoError(t, epc.Stop(), "failed to close etcd cluster")

	for _, proc := range epc.Procs {
		t.Logf("Checking %s v3-backend", proc.Config().Name)
		dbPath := datadir.ToBackendFileName(proc.Config().DataDirPath)
		mInfo := membershipInfoFromBackend(t, dbPath, learnerID)
		require.False(t, mInfo.IsLearner, "%x should not be learner (%+v)", learnerID, mInfo)
	}
}

func membershipInfoFromBackend(t *testing.T, dbPath string, memberID uint64) *membership.Member {
	db, err := bbolt.Open(dbPath, 0666, nil)
	require.NoError(t, err, "failed to open backend %s", dbPath)

	defer func() {
		require.NoError(t, db.Close())
	}()

	m := &membership.Member{}
	err = db.View(func(tx *bbolt.Tx) error {
		value := tx.Bucket(buckets.Members.Name()).
			Get([]byte(types.ID(memberID).String()))

		return json.Unmarshal(value, m)
	})
	require.NoError(t, err, "failed to read membership info for %x", memberID)
	return m
}
