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
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
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

	ctx := t.Context()

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
	if len(resp.Members) == 0 {
		cx.t.Fatal("member number is 0")
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

	decoder := json.NewDecoder(strings.NewReader(txt))
	hexResponse := struct {
		Header  map[string]any   `json:"header"`
		Members []map[string]any `json:"members"`
	}{}
	if decodeErr := decoder.Decode(&hexResponse); errors.Is(decodeErr, io.EOF) {
		cx.t.Fatalf("memberListWithHexTest error (%v)", decodeErr)
	}

	if len(resp.Members) != len(hexResponse.Members) {
		cx.t.Fatalf("member number, expected %d,got %d", len(resp.Members), len(hexResponse.Members))
	}

	if clusterID, _ := strconv.ParseUint(hexResponse.Header["cluster_id"].(string), 16, 64); resp.Header.ClusterId != clusterID {
		cx.t.Fatalf("Unexpected Cluster ID in response header: expected %x, got %x", resp.Header.ClusterId, clusterID)
	}
	if memberID, _ := strconv.ParseUint(hexResponse.Header["member_id"].(string), 16, 64); resp.Header.MemberId != memberID {
		cx.t.Fatalf("Unexpected Member ID in response header: expected %x, got %x", resp.Header.MemberId, memberID)
	}
	if raftTerm := uint64(hexResponse.Header["raft_term"].(float64)); resp.Header.RaftTerm != raftTerm {
		cx.t.Fatalf("Unexpected Raft Term in response header: expected %d, got %d", resp.Header.RaftTerm, raftTerm)
	}

	for i := 0; i < len(resp.Members); i++ {
		if id, _ := strconv.ParseUint(hexResponse.Members[i]["ID"].(string), 16, 64); resp.Members[i].ID != id {
			cx.t.Fatalf("Unexpected Member ID: expected %x, got %x", resp.Members[i].ID, id)
		}
		if name := hexResponse.Members[i]["name"]; resp.Members[i].Name != name {
			cx.t.Fatalf("Unexpected Member Name: expected %s, got %s", resp.Members[i].Name, name)
		}

		urls := hexResponse.Members[i]["peerURLs"].([]any)
		peerURLs := make([]string, len(urls))
		for j, url := range urls {
			peerURLs[j] = url.(string)
		}
		if !reflect.DeepEqual(resp.Members[i].PeerURLs, peerURLs) {
			cx.t.Fatalf("Unexpected Member peerURLs: expected %v, got %v", resp.Members[i].ClientURLs, peerURLs)
		}

		urls = hexResponse.Members[i]["clientURLs"].([]any)
		clientURLs := make([]string, len(urls))
		for j, url := range urls {
			clientURLs[j] = url.(string)
		}
		if !reflect.DeepEqual(resp.Members[i].ClientURLs, clientURLs) {
			cx.t.Fatalf("Unexpected Member clientURLs: expected %v, got %v", resp.Members[i].ClientURLs, clientURLs)
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
	ctx := t.Context()

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
