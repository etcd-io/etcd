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

package e2e

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3ClusterOperations(t *testing.T) {
	testCtl(t, testCurlV3ClusterOperations, withCfg(*e2e.NewConfig(e2e.WithClusterSize(1))))
}

func testCurlV3ClusterOperations(cx ctlCtx) {
	var (
		peerURL        = "http://127.0.0.1:22380"
		updatedPeerURL = "http://127.0.0.1:32380"
	)

	// add member
	cx.t.Logf("Adding member %q", peerURL)
	addMemberReq, err := json.Marshal(&pb.MemberAddRequest{PeerURLs: []string{peerURL}, IsLearner: true})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/cluster/member/add",
		Value:    string(addMemberReq),
		Expected: expect.ExpectedResponse{Value: peerURL},
	}), "testCurlV3ClusterOperations failed to add member")

	// list members and get the new member's ID
	cx.t.Log("Listing members after adding a member")
	members := mustListMembers(cx)
	require.Len(cx.t, members, 2)
	cx.t.Logf("members: %+v", members)

	var newMemberIDStr string
	for _, m := range members {
		mObj := m.(map[string]any)
		pURL := mObj["peerURLs"].([]any)[0].(string)
		if pURL == peerURL {
			newMemberIDStr = mObj["ID"].(string)
			break
		}
	}
	require.Positive(cx.t, newMemberIDStr)

	// update member
	cx.t.Logf("Update peerURL from %q to %q for member %q", peerURL, updatedPeerURL, newMemberIDStr)
	newMemberID, err := strconv.ParseUint(newMemberIDStr, 10, 64)
	require.NoError(cx.t, err)

	updateMemberReq, err := json.Marshal(&pb.MemberUpdateRequest{ID: newMemberID, PeerURLs: []string{updatedPeerURL}})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/cluster/member/update",
		Value:    string(updateMemberReq),
		Expected: expect.ExpectedResponse{Value: updatedPeerURL},
	}), "testCurlV3ClusterOperations failed to update member")

	// promote member
	cx.t.Logf("Promoting the member %d", newMemberID)
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/cluster/member/promote",
		Value:    fmt.Sprintf(`{"ID": %d}`, newMemberID),
		Expected: expect.ExpectedResponse{Value: "etcdserver: can only promote a learner member which is in sync with leader"},
	}), "testCurlV3ClusterOperations failed to promote member")

	// remove member
	cx.t.Logf("Removing the member %d", newMemberID)
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/cluster/member/remove",
		Value:    fmt.Sprintf(`{"ID": %d}`, newMemberID),
		Expected: expect.ExpectedResponse{Value: "members"},
	}), "testCurlV3ClusterOperations failed to remove member")

	// list members again after deleting a member
	cx.t.Log("Listing members again after deleting a member")
	members = mustListMembers(cx)
	require.Len(cx.t, members, 1)
}

func mustListMembers(cx ctlCtx) []any {
	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[0], "POST", e2e.CURLReq{
		Endpoint: "/v3/cluster/member/list",
		Value:    "{}",
	})
	resp, err := runCommandAndReadJSONOutput(args)
	require.NoError(cx.t, err)

	members, ok := resp["members"]
	require.True(cx.t, ok)
	return members.([]any)
}
