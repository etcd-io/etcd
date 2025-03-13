// Copyright 2021 The etcd Authors
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
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestAuthority(t *testing.T) {
	tcs := []struct {
		name                   string
		useUnix                bool
		useTLS                 bool
		useInsecureTLS         bool
		clientURLPattern       string
		expectAuthorityPattern string
	}{
		{
			name:                   "unix:path",
			useUnix:                true,
			clientURLPattern:       "unix:localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "unix://absolute_path",
			useUnix:                true,
			clientURLPattern:       "unix://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		// "unixs" is not standard schema supported by etcd
		{
			name:                   "unixs:absolute_path",
			useUnix:                true,
			useTLS:                 true,
			clientURLPattern:       "unixs:localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "unixs://absolute_path",
			useUnix:                true,
			useTLS:                 true,
			clientURLPattern:       "unixs://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "http://domain[:port]",
			clientURLPattern:       "http://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "http://address[:port]",
			clientURLPattern:       "http://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
		{
			name:                   "https://domain[:port] insecure",
			useTLS:                 true,
			useInsecureTLS:         true,
			clientURLPattern:       "https://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "https://address[:port] insecure",
			useTLS:                 true,
			useInsecureTLS:         true,
			clientURLPattern:       "https://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
		{
			name:                   "https://domain[:port]",
			useTLS:                 true,
			clientURLPattern:       "https://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "https://address[:port]",
			useTLS:                 true,
			clientURLPattern:       "https://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
	}
	for _, tc := range tcs {
		for _, clusterSize := range []int{1, 3} {
			t.Run(fmt.Sprintf("Size: %d, Scenario: %q", clusterSize, tc.name), func(t *testing.T) {
				e2e.BeforeTest(t)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				cfg := e2e.NewConfigNoTLS()
				cfg.ClusterSize = clusterSize
				if tc.useTLS {
					cfg.Client.ConnectionType = e2e.ClientTLS
				}
				cfg.Client.AutoTLS = tc.useInsecureTLS
				// Enable debug mode to get logs with http2 headers (including authority)
				cfg.EnvVars = map[string]string{"GODEBUG": "http2debug=2"}
				if tc.useUnix {
					cfg.BaseClientScheme = "unix"
				}

				epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
				if err != nil {
					t.Fatalf("could not start etcd process cluster (%v)", err)
				}
				defer epc.Close()

				endpoints := templateEndpoints(t, tc.clientURLPattern, epc)
				client, err := e2e.NewEtcdctl(cfg.Client, endpoints)
				require.NoError(t, err)
				for i := 0; i < 100; i++ {
					require.NoError(t, client.Put(ctx, "foo", "bar", config.PutOptions{}))
				}

				testutils.ExecuteWithTimeout(t, 5*time.Second, func() {
					assertAuthority(t, tc.expectAuthorityPattern, epc)
				})
			})
		}
	}
}

func templateEndpoints(t *testing.T, pattern string, clus *e2e.EtcdProcessCluster) []string {
	t.Helper()
	var endpoints []string
	for i := 0; i < clus.Cfg.ClusterSize; i++ {
		ent := pattern
		ent = strings.ReplaceAll(ent, "${MEMBER_PORT}", fmt.Sprintf("%d", e2e.EtcdProcessBasePort+i*5))
		endpoints = append(endpoints, ent)
	}
	return endpoints
}

func assertAuthority(t *testing.T, expectAuthorityPattern string, clus *e2e.EtcdProcessCluster) {
	for i := range clus.Procs {
		line, _ := clus.Procs[i].Logs().ExpectWithContext(t.Context(), expect.ExpectedResponse{Value: `http2: decoded hpack field header field ":authority"`})
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		u, err := url.Parse(clus.Procs[i].EndpointsGRPC()[0])
		require.NoError(t, err)
		expectAuthority := strings.ReplaceAll(expectAuthorityPattern, "${MEMBER_PORT}", u.Port())
		expectLine := fmt.Sprintf(`http2: decoded hpack field header field ":authority" = %q`, expectAuthority)
		assert.Truef(t, strings.HasSuffix(line, expectLine), "Got %q expected suffix %q", line, expectLine)
	}
}
