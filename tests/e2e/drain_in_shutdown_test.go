// Copyright 2024 The etcd Authors
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
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestShouldDrainRequestDuringShutdown(t *testing.T) {
	e2e.BeforeTest(t)

	// defaultBuildSnapshotConn is to setup a database with 10 MiB and a
	// inflight snapshot streaming RPC.
	defaultBuildSnapshotConn := func(ctx context.Context, t *testing.T, cli *clientv3.Client) io.ReadCloser {
		t.Helper()

		require.NoError(t, fillEtcdWithData(ctx, cli, 10*1024*1024))

		rc, err := cli.Snapshot(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { rc.Close() })

		// make sure that streaming RPC is in progress
		buf := make([]byte, 1)
		n, err := rc.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, 1, n)

		return rc
	}

	// defaultVerifySnapshotConn is to make sure that connection is still
	// working even if the server is in shutdown state.
	defaultVerifySnapshotConn := func(t *testing.T, rc io.ReadCloser) {
		t.Helper()

		_, err := io.Copy(io.Discard, rc)
		require.NoError(t, err)
	}

	tcs := []struct {
		name    string
		options []e2e.EPClusterOption
		cliOpt  e2e.ClientConfig

		buildSnapshotConn  func(ctx context.Context, t *testing.T, cli *clientv3.Client) io.ReadCloser
		verifySnapshotConn func(t *testing.T, rc io.ReadCloser)
	}{
		{
			name: "no-tls",
			options: []e2e.EPClusterOption{
				e2e.WithClusterSize(1),
				e2e.WithClientAutoTLS(false),
			},
			cliOpt: e2e.ClientConfig{ConnectionType: e2e.ClientNonTLS},

			buildSnapshotConn:  defaultBuildSnapshotConn,
			verifySnapshotConn: defaultVerifySnapshotConn,
		},
		{
			name: "auto-tls_http_separated",
			options: []e2e.EPClusterOption{
				e2e.WithClusterSize(1),
				e2e.WithClientAutoTLS(true),
				e2e.WithClientConnType(e2e.ClientTLS),
				e2e.WithClientHTTPSeparate(true),
			},
			cliOpt: e2e.ClientConfig{
				ConnectionType: e2e.ClientTLS,
				AutoTLS:        true,
			},
			buildSnapshotConn:  defaultBuildSnapshotConn,
			verifySnapshotConn: defaultVerifySnapshotConn,
		},
		{
			name: "auto-tls_cmux",
			options: []e2e.EPClusterOption{
				e2e.WithClusterSize(1),
				e2e.WithClientAutoTLS(true),
				e2e.WithClientConnType(e2e.ClientTLS),
				e2e.WithClientHTTPSeparate(false),
				e2e.WithGoFailEnabled(true),
				// NOTE: Using failpoint is to make sure that
				// the RPC handler won't exit because of closed
				// connection.
				e2e.WithEnvVars(map[string]string{
					"GOFAIL_FAILPOINTS": `v3rpcBeforeSnapshot=sleep("8s")`,
				}),
			},
			cliOpt: e2e.ClientConfig{
				ConnectionType: e2e.ClientTLS,
				AutoTLS:        true,
			},
			buildSnapshotConn: func(ctx context.Context, t *testing.T, cli *clientv3.Client) io.ReadCloser {
				t.Helper()

				rc, err := cli.Snapshot(ctx)
				require.NoError(t, err)
				t.Cleanup(func() { rc.Close() })

				// make sure server receives the RPC.
				time.Sleep(2 * time.Second)
				return rc
			},
			verifySnapshotConn: func(t *testing.T, rc io.ReadCloser) {
				t.Helper()

				_, err := io.Copy(io.Discard, rc)
				require.Error(t, err) // connection will be closed forcely
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			epc, err := e2e.NewEtcdProcessCluster(ctx, t, tc.options...)
			require.NoError(t, err)
			t.Cleanup(func() { epc.Close() })

			grpcEndpoint := epc.Procs[0].EndpointsGRPC()[0]
			if tc.cliOpt.ConnectionType == e2e.ClientTLS {
				grpcEndpoint = e2e.ToTLS(grpcEndpoint)
			}

			cli := newClient(t, []string{grpcEndpoint}, tc.cliOpt)

			rc := tc.buildSnapshotConn(ctx, t, cli)

			errCh := make(chan error, 1)
			go func() {
				defer close(errCh)
				errCh <- epc.Stop()
			}()

			select {
			case <-time.After(4 * time.Second):
			case err := <-errCh:
				t.Fatalf("should drain request but got error from cluster stop: %v", err)
			}

			tc.verifySnapshotConn(t, rc)

			require.NoError(t, <-errCh)
		})
	}
}
