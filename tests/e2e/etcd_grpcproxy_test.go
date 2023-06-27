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
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestGrpcProxyAutoSync(t *testing.T) {
	e2e.SkipInShortMode(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithClusterSize(1))
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, epc.Close())
	}()

	var (
		node1ClientURL = epc.Procs[0].Config().ClientURL
		proxyClientURL = "127.0.0.1:32379"
	)

	// Run independent grpc-proxy instance
	proxyProc, err := e2e.SpawnCmd([]string{e2e.BinPath.Etcd, "grpc-proxy", "start",
		"--advertise-client-url", proxyClientURL, "--listen-addr", proxyClientURL,
		"--endpoints", node1ClientURL,
		"--endpoints-auto-sync-interval", "1s",
	}, nil)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, proxyProc.Stop())
	}()

	proxyCtl, err := e2e.NewEtcdctl(e2e.ClientConfig{}, []string{proxyClientURL})
	require.NoError(t, err)
	err = proxyCtl.Put(ctx, "k1", "v1", config.PutOptions{})
	require.NoError(t, err)

	// Add and start second member
	_, err = epc.StartNewProc(ctx, nil, t, false /* addAsLearner */)
	require.NoError(t, err)

	// Wait for auto sync of endpoints
	err = waitForEndpointInLog(ctx, proxyProc, epc.Procs[1].Config().ClientURL)
	require.NoError(t, err)

	err = epc.CloseProc(ctx, func(proc e2e.EtcdProcess) bool {
		return proc.Config().ClientURL == node1ClientURL
	})
	require.NoError(t, err)

	var resp *clientv3.GetResponse
	for i := 0; i < 10; i++ {
		resp, err = proxyCtl.Get(ctx, "k1", config.GetOptions{})
		if err != nil && strings.Contains(err.Error(), rpctypes.ErrGRPCLeaderChanged.Error()) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}
	require.NoError(t, err)

	kvs := testutils.KeyValuesFromGetResponse(resp)
	assert.Equal(t, []testutils.KV{{Key: "k1", Val: "v1"}}, kvs)
}

func waitForEndpointInLog(ctx context.Context, proxyProc *expect.ExpectProcess, endpoint string) error {
	endpoint = strings.Replace(endpoint, "http://", "", 1)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := proxyProc.ExpectFunc(ctx, func(s string) bool {
		if strings.Contains(s, endpoint) {
			return true
		}
		return false
	})

	return err
}
