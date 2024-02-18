// Copyright 2017 The etcd Authors
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
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestGrpcProxyAutoSync(t *testing.T) {
	e2e.SkipInShortMode(t)

	var (
		node1Name      = "node1"
		node1ClientURL = "http://localhost:12379"
		node1PeerURL   = "http://localhost:12380"

		node2Name      = "node2"
		node2ClientURL = "http://localhost:22379"
		node2PeerURL   = "http://localhost:22380"

		proxyClientURL = "127.0.0.1:32379"

		autoSyncInterval = 1 * time.Second
	)

	// Run cluster of one node
	proc1, err := runEtcdNode(
		node1Name, t.TempDir(),
		node1ClientURL, node1PeerURL,
		"new", fmt.Sprintf("%s=%s", node1Name, node1PeerURL),
	)
	require.NoError(t, err)

	// Run grpc-proxy instance
	proxyProc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "grpc-proxy", "start",
		"--advertise-client-url", proxyClientURL, "--listen-addr", proxyClientURL,
		"--endpoints", node1ClientURL,
		"--endpoints-auto-sync-interval", autoSyncInterval.String(),
	}, nil)
	require.NoError(t, err)

	err = e2e.SpawnWithExpect([]string{e2e.CtlBinPath, "--endpoints", proxyClientURL, "put", "k1", "v1"}, "OK")
	require.NoError(t, err)

	err = e2e.SpawnWithExpect([]string{e2e.CtlBinPath, "--endpoints", node1ClientURL, "member", "add", node2Name, "--peer-urls", node2PeerURL}, "added")
	require.NoError(t, err)

	// Run new member
	proc2, err := runEtcdNode(
		node2Name, t.TempDir(),
		node2ClientURL, node2PeerURL,
		"existing", fmt.Sprintf("%s=%s,%s=%s", node1Name, node1PeerURL, node2Name, node2PeerURL),
	)
	require.NoError(t, err)

	// Wait for auto sync of endpoints
	_, err = proxyProc.Expect(strings.Replace(node2ClientURL, "http://", "", 1))
	require.NoError(t, err)

	memberList, err := getMemberListFromEndpoint(node1ClientURL)
	require.NoError(t, err)

	node1MemberID, err := findMemberIDByEndpoint(memberList.Members, node1ClientURL)
	require.NoError(t, err)

	node2MemberID, err := findMemberIDByEndpoint(memberList.Members, node2ClientURL)
	require.NoError(t, err)

	// Remove node1 from member list and stop this node

	// Second node could be not ready yet
	for i := 0; i < 10; i++ {
		err = e2e.SpawnWithExpect([]string{e2e.CtlBinPath, "--endpoints", node2ClientURL, "member", "remove", fmt.Sprintf("%x", node1MemberID)}, "removed")
		if err != nil && strings.Contains(err.Error(), rpctypes.ErrGRPCUnhealthy.Error()) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	require.NoError(t, err)

	_, err = proc1.Expect("the member has been permanently removed from the cluster")
	require.NoError(t, err)
	require.NoError(t, proc1.Stop())

	_, err = proc2.Expect(fmt.Sprintf("%x became leader", node2MemberID))
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		err = e2e.SpawnWithExpect([]string{e2e.CtlBinPath, "--endpoints", proxyClientURL, "get", "k1"}, "v1")
		if err != nil && (strings.Contains(err.Error(), rpctypes.ErrGRPCLeaderChanged.Error()) ||
			strings.Contains(err.Error(), context.DeadlineExceeded.Error())) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	require.NoError(t, err)

	require.NoError(t, proc2.Stop())
	require.NoError(t, proxyProc.Stop())
}

func runEtcdNode(name, dataDir, clientURL, peerURL, clusterState, initialCluster string) (*expect.ExpectProcess, error) {
	proc, err := e2e.SpawnCmd([]string{e2e.BinPath,
		"--name", name,
		"--data-dir", dataDir,
		"--listen-client-urls", clientURL, "--advertise-client-urls", clientURL,
		"--listen-peer-urls", peerURL, "--initial-advertise-peer-urls", peerURL,
		"--initial-cluster-token", "etcd-cluster",
		"--initial-cluster-state", clusterState,
		"--initial-cluster", initialCluster,
	}, nil)
	if err != nil {
		return nil, err
	}

	_, err = proc.Expect("ready to serve client requests")

	return proc, err
}

func findMemberIDByEndpoint(members []*etcdserverpb.Member, endpoint string) (uint64, error) {
	for _, m := range members {
		if m.ClientURLs[0] == endpoint {
			return m.ID, nil
		}
	}

	return 0, fmt.Errorf("member not found")
}

func getMemberListFromEndpoint(endpoint string) (etcdserverpb.MemberListResponse, error) {
	proc, err := e2e.SpawnCmd([]string{e2e.CtlBinPath, "--endpoints", endpoint, "member", "list", "--write-out", "json"}, nil)
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
