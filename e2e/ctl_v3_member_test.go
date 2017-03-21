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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestCtlV3MemberList(t *testing.T) { testCtl(t, memberListTest) }
func TestCtlV3MemberRemove(t *testing.T) {
	testCtl(t, memberRemoveTest, withQuorum(), withNoStrictReconfig())
}
func TestCtlV3MemberAdd(t *testing.T)    { testCtl(t, memberAddTest) }
func TestCtlV3MemberUpdate(t *testing.T) { testCtl(t, memberUpdateTest) }

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
	if err := ctlV3MemberAdd(cx, "newmember", fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+11)); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3MemberAdd(cx ctlCtx, name, peerURL string) error {
	cmdArgs := append(cx.PrefixArgs(), "member", "add", name, fmt.Sprintf("--peer-urls=%s", peerURL))
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

// TestCtlV3MemberAddRemove tests cluster member add,remove
// and makes sure removed member cannot join the cluster and exits "in time".
// (Related https://github.com/coreos/etcd/issues/7512)
func TestCtlV3MemberAddRemove(t *testing.T) {
	defer testutil.AfterTest(t)

	// 1. start single-member cluster
	readyc1, stopc1, closedc1 := make(chan ctlCtx), make(chan struct{}), make(chan struct{})
	go func() {
		testCtl(t, func(ret ctlCtx) {},
			withCfg(configNoTLS), withNoStrictReconfig(), withTestTimeout(time.Minute),
			withKeepDataDirStop(), withReadyc(readyc1), withStopc(stopc1), withClosedc(closedc1))
	}()
	c1 := <-readyc1
	c1s := c1.epc.procs[0].cfg.initialCluster
	dataDir1 := c1.epc.procs[0].cfg.dataDirPath
	defer os.RemoveAll(dataDir1)

	// 2. add a new member
	nameprefix := "newmember"
	basePort := etcdProcessBasePort + 10000
	addc := make(chan struct{})
	go func() {
		defer close(addc)
		if err := ctlV3MemberAdd(c1, nameprefix+"0", fmt.Sprintf("http://localhost:%d", basePort+1)); err != nil {
			t.Fatal(err)
		}
	}()
	select {
	case <-time.After(c1.dialTimeout + 3*time.Second):
		t.Fatalf("c1 member add timed out after %v", c1.dialTimeout+3*time.Second)
	case <-addc:
	}

	// 3. start new member with 'existing' flag
	cc2 := configNoTLS
	cc2.basePort = basePort
	readyc2, stopc2, closedc2 := make(chan ctlCtx), make(chan struct{}), make(chan struct{})
	go func() {
		testCtl(t, func(ret ctlCtx) {},
			withCfg(cc2), withNoStrictReconfig(), withNamePrefix(nameprefix),
			withExistingInitialCluster(c1s), withExistingCluster(), withTestTimeout(time.Minute),
			withReadyc(readyc2), withStopc(stopc2), withClosedc(closedc2))
	}()
	c2 := <-readyc2
	defer func() {
		// shut down second member
		stopc2 <- struct{}{}
		<-closedc2
	}()

	ls, err := getMemberList(c1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls.Members) != 2 {
		t.Fatalf("len(ls.Members) expected 2, got %d", len(ls.Members))
	}
	cid := ls.Header.ClusterId
	var id1, id2 uint64
	for _, m := range ls.Members {
		if m.Name == c1.epc.procs[0].cfg.name {
			id1 = m.ID
		} else {
			id2 = m.ID
		}
	}

	// 4. remove first member
	removec := make(chan struct{})
	go func() {
		defer close(removec)
		if err = ctlV3MemberRemove(c2, c1.epc.grpcEndpoints()[0], fmt.Sprintf("%16x", id1), fmt.Sprintf("%16x", cid)); err != nil {
			t.Fatal(err)
		}
	}()
	select {
	case <-time.After(c2.dialTimeout + 3*time.Second):
		t.Fatalf("c2 member remove timed out after %v", c2.dialTimeout+3*time.Second)
	case <-removec:
	}

	// 5. shut down first member, without deleting data directory
	stopc1 <- struct{}{}
	<-closedc1

	// 6. wait for new leader
	if err = waitExpect(c2.epc.procs[0].proc, []string{fmt.Sprintf("raft: %x became leader", id2)}); err != nil {
		t.Fatal(err)
	}

	// 7. restart first member with 'initial-cluster-state=new'; wrong config, expects exit within ReqTimeout
	exitc := make(chan struct{})
	go func() {
		defer close(exitc)
		c1.epc.procs[0].cfg.keepDataDirStart = true
		if err := c1.epc.procs[0].Restart(); err == nil {
			t.Fatal("first member Restart expected error, got nil")
		}
	}()
	select {
	case <-time.After(time.Minute):
		t.Fatalf("test timed out waiting %v for process exit", time.Minute)
	case <-exitc:
	}
}
