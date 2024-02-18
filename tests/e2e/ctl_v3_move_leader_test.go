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
	"os"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/client/v3"
)

func TestCtlV3MoveLeaderScenarios(t *testing.T) {
	securityParent := map[string]struct {
		cfg etcdProcessClusterConfig
	}{
		"Secure":   {cfg: *newConfigTLS()},
		"Insecure": {cfg: *newConfigNoTLS()},
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

func testCtlV3MoveLeader(t *testing.T, cfg etcdProcessClusterConfig, envVars map[string]string) {
	BeforeTest(t)

	epc := setupEtcdctlTest(t, &cfg, true)
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	var tcfg *tls.Config
	if cfg.clientTLS == clientTLS {
		tinfo := transport.TLSInfo{
			CertFile:      certPath,
			KeyFile:       privateKeyPath,
			TrustedCAFile: caPath,
		}
		var err error
		tcfg, err = tinfo.ClientConfig()
		if err != nil {
			t.Fatal(err)
		}
	}

	var leadIdx int
	var leaderID uint64
	var transferee uint64
	for i, ep := range epc.EndpointsV3() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{ep},
			DialTimeout: 3 * time.Second,
			TLS:         tcfg,
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         *newConfigNoTLS(),
		dialTimeout: 7 * time.Second,
		epc:         epc,
		envMap:      envVars,
	}

	tests := []struct {
		eps    []string
		expect string
	}{
		{ // request to non-leader
			[]string{cx.epc.EndpointsV3()[(leadIdx+1)%3]},
			"no leader endpoint given at ",
		},
		{ // request to leader
			[]string{cx.epc.EndpointsV3()[leadIdx]},
			fmt.Sprintf("Leadership transferred from %s to %s", types.ID(leaderID), types.ID(transferee)),
		},
		{ // request to all endpoints
			cx.epc.EndpointsV3(),
			fmt.Sprintf("Leadership transferred"),
		},
	}
	for i, tc := range tests {
		prefix := cx.prefixArgs(tc.eps)
		cmdArgs := append(prefix, "move-leader", types.ID(transferee).String())
		if err := spawnWithExpectWithEnv(cmdArgs, cx.envMap, tc.expect); err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
	}
}
