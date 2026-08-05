// Copyright 2026 The etcd Authors
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

//go:build !cluster_proxy

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestEtcdutlInit(t *testing.T) {
	tcs := []struct {
		name        string
		clusterSize int
	}{
		{name: "SingleMember", clusterSize: 1},
		{name: "MultiMember", clusterSize: 3},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)
			ctx := t.Context()

			epc, err := e2e.InitEtcdProcessCluster(t, e2e.NewConfig(
				e2e.WithClusterSize(tc.clusterSize),
				e2e.WithKeepDataDir(true),
			))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, epc.Close())
			}()

			t.Log("etcdutl init all members")
			for _, proc := range epc.Procs {
				require.NoError(t, e2e.SpawnWithExpect(
					initArgs(proc, proc.Config().InitialToken), expect.ExpectedResponse{Value: "added member"}))
			}

			t.Log("Dropping bootstrap flags, members start from the initialized data dirs alone")
			for _, proc := range epc.Procs {
				var minimal []string
				for _, arg := range proc.Config().Args {
					if strings.HasPrefix(arg, "--initial-cluster") ||
						strings.HasPrefix(arg, "--initial-advertise-peer-urls") {
						continue
					}
					minimal = append(minimal, arg)
				}
				proc.Config().Args = minimal
			}
			require.NoError(t, epc.Start(ctx))

			dialTimeout := 10 * time.Second
			prefixArgs := []string{
				e2e.BinPath.Etcdctl,
				"--endpoints", strings.Join(epc.EndpointsGRPC(), ","),
				"--dial-timeout", dialTimeout.String(),
			}

			t.Log("Writing and reading a key")
			require.NoError(t, e2e.SpawnWithExpect(
				append(prefixArgs, "put", "foo", "bar"), expect.ExpectedResponse{Value: "OK"}))
			require.NoError(t, e2e.SpawnWithExpect(
				append(prefixArgs, "get", "foo"), expect.ExpectedResponse{Value: "bar"}))

			t.Log("Checking all members are voting members")
			memberListArgs := append(prefixArgs, "member", "list")
			lines := make([]expect.ExpectedResponse, tc.clusterSize)
			for i := range lines {
				lines[i] = expect.ExpectedResponse{Value: "started"}
			}
			_, err = e2e.SpawnWithExpectLines(ctx, memberListArgs, nil, lines...)
			require.NoError(t, err)

			t.Log("Stopping the members")
			require.NoError(t, epc.Stop())

			t.Log("etcdutl init again is idempotent")
			for _, proc := range epc.Procs {
				require.NoError(t, e2e.SpawnWithExpect(
					initArgs(proc, proc.Config().InitialToken), expect.ExpectedResponse{Value: "already initialized"}))
			}

			t.Log("etcdutl init with a different cluster token fails")
			for _, proc := range epc.Procs {
				err := e2e.SpawnWithExpect(
					initArgs(proc, "different-token"), expect.ExpectedResponse{Value: "initialized with a different"})
				require.ErrorContains(t, err, "initialized with a different")
			}

			t.Log("Restarting the members, data must survive")
			require.NoError(t, epc.Restart(ctx))
			require.NoError(t, e2e.SpawnWithExpect(
				append(prefixArgs, "get", "foo"), expect.ExpectedResponse{Value: "bar"}))

			t.Log("Killing the members uncleanly")
			require.NoError(t, e2e.SpawnWithExpect(
				append(prefixArgs, "put", "foo2", "bar2"), expect.ExpectedResponse{Value: "OK"}))
			for _, proc := range epc.Procs {
				require.NoError(t, proc.Kill())
				require.NoError(t, proc.Wait(ctx))
			}

			t.Log("etcdutl init after unclean shutdown is idempotent")
			for _, proc := range epc.Procs {
				require.NoError(t, e2e.SpawnWithExpect(
					initArgs(proc, proc.Config().InitialToken), expect.ExpectedResponse{Value: "already initialized"}))
			}

			t.Log("Restarting the members after unclean shutdown, data must survive")
			require.NoError(t, epc.Restart(ctx))
			require.NoError(t, e2e.SpawnWithExpect(
				append(prefixArgs, "get", "foo2"), expect.ExpectedResponse{Value: "bar2"}))
		})
	}
}

// initArgs builds the etcdutl init invocation for a member from the
// configuration the e2e framework generated for its process.
func initArgs(proc e2e.EtcdProcess, clusterToken string) []string {
	cfg := proc.Config()
	return []string{
		e2e.BinPath.Etcdutl, "init",
		"--name", cfg.Name,
		"--data-dir", cfg.DataDirPath,
		"--initial-cluster", cfg.InitialCluster,
		"--initial-cluster-token", clusterToken,
		"--initial-advertise-peer-urls", cfg.PeerURL.String(),
	}
}
