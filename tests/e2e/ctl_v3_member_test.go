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
	"strings"
	"testing"

	"go.etcd.io/etcd/etcdserver/etcdserverpb"
)

func TestCtlV3MemberList(t *testing.T)          { testCtl(t, memberListTest) }
func TestCtlV3MemberListNoTLS(t *testing.T)     { testCtl(t, memberListTest, withCfg(configNoTLS)) }
func TestCtlV3MemberListClientTLS(t *testing.T) { testCtl(t, memberListTest, withCfg(configClientTLS)) }
func TestCtlV3MemberListClientAutoTLS(t *testing.T) {
	testCtl(t, memberListTest, withCfg(configClientAutoTLS))
}
func TestCtlV3MemberListPeerTLS(t *testing.T) { testCtl(t, memberListTest, withCfg(configPeerTLS)) }
func TestCtlV3MemberRemove(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig())
}
func TestCtlV3MemberRemoveNoTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(configNoTLS))
}
func TestCtlV3MemberRemoveClientTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(configClientTLS))
}
func TestCtlV3MemberRemoveClientAutoTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(
		// default clusterSize is 1
		etcdProcessClusterConfig{
			clusterSize:     3,
			isClientAutoTLS: true,
			clientTLS:       clientTLS,
			initialToken:    "new",
		}))
}
func TestCtlV3MemberRemovePeerTLS(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig(), withCfg(configPeerTLS))
}
func TestCtlV3MemberAdd(t *testing.T)          { testCtl(t, memberAddTest) }
func TestCtlV3MemberAddNoTLS(t *testing.T)     { testCtl(t, memberAddTest, withCfg(configNoTLS)) }
func TestCtlV3MemberAddClientTLS(t *testing.T) { testCtl(t, memberAddTest, withCfg(configClientTLS)) }
func TestCtlV3MemberAddClientAutoTLS(t *testing.T) {
	testCtl(t, memberAddTest, withCfg(configClientAutoTLS))
}
func TestCtlV3MemberAddPeerTLS(t *testing.T)    { testCtl(t, memberAddTest, withCfg(configPeerTLS)) }
func TestCtlV3MemberAddForLearner(t *testing.T) { testCtl(t, memberAddForLearnerTest) }
func TestCtlV3MemberUpdate(t *testing.T)        { testCtl(t, memberUpdateTest) }
func TestCtlV3MemberUpdateNoTLS(t *testing.T)   { testCtl(t, memberUpdateTest, withCfg(configNoTLS)) }
func TestCtlV3MemberUpdateClientTLS(t *testing.T) {
	testCtl(t, memberUpdateTest, withCfg(configClientTLS))
}
func TestCtlV3MemberUpdateClientAutoTLS(t *testing.T) {
	testCtl(t, memberUpdateTest, withCfg(configClientAutoTLS))
}
func TestCtlV3MemberUpdatePeerTLS(t *testing.T) { testCtl(t, memberUpdateTest, withCfg(configPeerTLS)) }

func memberListTest(cx ctlCtx) {
	if err := ctlV3MemberList(cx); err != nil {
		cx.t.Fatalf("memberListTest ctlV3MemberList error (%v)", err)
	}
}

func ctlV3MemberList(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "list")
	lines := make([]string, cx.cfg.clusterSize)
	for i := range lines {
		lines[i] = "started"
	}
	return spawnWithExpects(cmdArgs, lines...)
}

func getMemberList(cx ctlCtx) (etcdserverpb.MemberListResponse, error) {
	cmdArgs := append(cx.PrefixArgs(), "--write-out", "json", "member", "list")

	proc, err := spawnCmd(cmdArgs)
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

func memberRemoveTest(cx ctlCtx) {
	ep, memIDToRemove, clusterID := cx.memberToRemove()
	if err := ctlV3MemberRemove(cx, ep, memIDToRemove, clusterID); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3MemberRemove(cx ctlCtx, ep, memberID, clusterID string) error {
	cmdArgs := append(cx.prefixArgs([]string{ep}), "member", "remove", memberID)
	return spawnWithExpect(cmdArgs, fmt.Sprintf("%s removed from cluster %s", memberID, clusterID))
}

func memberAddTest(cx ctlCtx) {
	if err := ctlV3MemberAdd(cx, fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+11), false); err != nil {
		cx.t.Fatal(err)
	}
}

func memberAddForLearnerTest(cx ctlCtx) {
	if err := ctlV3MemberAdd(cx, fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+11), true); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3MemberAdd(cx ctlCtx, peerURL string, isLearner bool) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "add", "newmember", fmt.Sprintf("--peer-urls=%s", peerURL))
	if isLearner {
		cmdArgs = append(cmdArgs, "--learner")
	}
	return spawnWithExpect(cmdArgs, " added to cluster ")
}

func memberUpdateTest(cx ctlCtx) {
	mr, err := getMemberList(cx)
	if err != nil {
		cx.t.Fatal(err)
	}

	peerURL := fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+11)
	memberID := fmt.Sprintf("%x", mr.Members[0].ID)
	if err = ctlV3MemberUpdate(cx, memberID, peerURL); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3MemberUpdate(cx ctlCtx, memberID, peerURL string) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "update", memberID, fmt.Sprintf("--peer-urls=%s", peerURL))
	return spawnWithExpect(cmdArgs, " updated in cluster ")
}
