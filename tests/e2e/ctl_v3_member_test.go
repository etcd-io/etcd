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
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3MemberList(t *testing.T)        { testCtl(t, memberListTest) }
func TestCtlV3MemberListWithHex(t *testing.T) { testCtl(t, memberListWithHexTest) }
func TestCtlV3MemberUpdate(t *testing.T)      { testCtl(t, memberUpdateTest) }
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

func ctlV3MemberRemove(cx ctlCtx, ep, memberID, clusterID string) error {
	cmdArgs := append(cx.prefixArgs([]string{ep}), "member", "remove", memberID)
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, fmt.Sprintf("%s removed from cluster %s", memberID, clusterID))
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
