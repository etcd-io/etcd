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
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3MoveLeaderScenarios(t *testing.T) {
	securityParent := map[string]struct {
		cfg e2e.EtcdProcessClusterConfig
	}{
		"Secure":   {cfg: *e2e.NewConfigTLS()},
		"Insecure": {cfg: *e2e.NewConfigNoTLS()},
	}

	tests := map[string]struct {
		env map[string]string
	}{
		"happy path": {env: map[string]string{}},
		"with env":   {env: map[string]string{"ETCDCTL_ENDPOINTS": "something-else-is-set"}},
	}

	for testName, tc := range securityParent {
		for subTestName, tx := range tests {
			t.Run(testName+" "+subTestName, func(t *testing.T) {
				testCtlV3MoveLeader(t, tc.cfg, tx.env)
			})
		}
	}
}

func testCtlV3MoveLeader(t *testing.T, cfg e2e.EtcdProcessClusterConfig, envVars map[string]string) {
	epc := setupEtcdctlTest(t, &cfg, true)
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	var tcfg *tls.Config
	if cfg.Client.ConnectionType == e2e.ClientTLS {
		tinfo := transport.TLSInfo{
			CertFile:      e2e.CertPath,
			KeyFile:       e2e.PrivateKeyPath,
			TrustedCAFile: e2e.CaPath,
		}
		var err error
		tcfg, err = tinfo.ClientConfig()
		require.NoError(t, err)
	}

	var leadIdx int
	var leaderID uint64
	var transferee uint64
	for i, ep := range epc.EndpointsGRPC() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{ep},
			DialTimeout: 3 * time.Second,
			TLS:         tcfg,
		})
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		resp, err := cli.Status(ctx, ep)
		if err != nil {
			t.Fatalf("failed to get status from endpoint %s: %v", ep, err)
		}
		cancel()
		cli.Close()

		if resp.Header.GetMemberId() == resp.Leader {
			leadIdx = i
			leaderID = resp.Leader
		} else {
			transferee = resp.Header.GetMemberId()
		}
	}

	cx := ctlCtx{
		t:           t,
		cfg:         *e2e.NewConfigNoTLS(),
		dialTimeout: 7 * time.Second,
		epc:         epc,
		envMap:      envVars,
	}

	tests := []struct {
		eps       []string
		expect    string
		expectErr bool
	}{
		{ // request to non-leader
			[]string{cx.epc.EndpointsGRPC()[(leadIdx+1)%3]},
			"no leader endpoint given at ",
			true,
		},
		{ // request to leader
			[]string{cx.epc.EndpointsGRPC()[leadIdx]},
			fmt.Sprintf("Leadership transferred from %s to %s", types.ID(leaderID), types.ID(transferee)),
			false,
		},
		{ // request to all endpoints
			cx.epc.EndpointsGRPC(),
			"Leadership transferred",
			false,
		},
	}
	for i, tc := range tests {
		prefix := cx.prefixArgs(tc.eps)
		cmdArgs := append(prefix, "move-leader", types.ID(transferee).String())
		err := e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: tc.expect})
		if tc.expectErr {
			require.ErrorContains(t, err, tc.expect)
		} else {
			require.NoErrorf(t, err, "#%d: %v", i, err)
		}
	}
}

func setupEtcdctlTest(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, quorum bool) *e2e.EtcdProcessCluster {
	if !quorum {
		cfg = e2e.ConfigStandalone(*cfg)
	}
	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}
