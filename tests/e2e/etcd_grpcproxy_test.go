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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestGrpcProxyAutoSync(t *testing.T) {
	e2e.SkipInShortMode(t)

	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize: 1,
	})
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, epc.Close())
	}()

	var (
		node1ClientURL = epc.Procs[0].Config().Acurl
		proxyClientURL = "127.0.0.1:32379"
	)

	// Run independent grpc-proxy instance
	proxyProc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "grpc-proxy", "start",
		"--advertise-client-url", proxyClientURL, "--listen-addr", proxyClientURL,
		"--endpoints", node1ClientURL,
		"--endpoints-auto-sync-interval", "1s",
	}, nil)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, proxyProc.Stop())
	}()

	err = e2e.SpawnWithExpect([]string{e2e.CtlBinPath, "--endpoints", proxyClientURL, "put", "k1", "v1"}, "OK")
	require.NoError(t, err)

	// Add and start second member
	err = epc.StartNewProc(t)
	require.NoError(t, err)

	// Wait for auto sync of endpoints
	err = waitForEndpointInLog(proxyProc, epc.Procs[1].Config().Acurl)
	require.NoError(t, err)

	err = epc.CloseProc(func(proc e2e.EtcdProcess) bool {
		return proc.Config().Acurl == node1ClientURL
	})
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

func waitForEndpointInLog(proxyProc *expect.ExpectProcess, endpoint string) error {
	endpoint = strings.Replace(endpoint, "http://", "", 1)

	_, err := proxyProc.ExpectFunc(func(s string) bool {
		if strings.Contains(s, endpoint) {
			return true
		}
		return false
	})

	return err
}
