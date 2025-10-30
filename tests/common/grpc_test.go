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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

type clientSocket int

const (
	clientSocketTCP clientSocket = iota
	clientSocketUnix
)

func TestAuthority(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name                   string
		clientSocket           clientSocket
		useTLS                 bool
		useInsecureTLS         bool
		clientURLPattern       string
		expectAuthorityPattern string
	}{
		{
			name:                   "unix:path",
			clientSocket:           clientSocketUnix,
			clientURLPattern:       "unix:localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "unix://absolute_path",
			clientSocket:           clientSocketUnix,
			clientURLPattern:       "unix://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		// "unixs" is not standard schema supported by etcd
		{
			name:                   "unixs:absolute_path",
			clientSocket:           clientSocketUnix,
			useTLS:                 true,
			clientURLPattern:       "unixs:localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "unixs://absolute_path",
			clientSocket:           clientSocketUnix,
			useTLS:                 true,
			clientURLPattern:       "unixs://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "http://domain[:port]",
			clientSocket:           clientSocketTCP,
			clientURLPattern:       "http://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "http://address[:port]",
			clientSocket:           clientSocketTCP,
			clientURLPattern:       "http://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
		{
			name:                   "https://domain[:port] insecure",
			clientSocket:           clientSocketTCP,
			useTLS:                 true,
			useInsecureTLS:         true,
			clientURLPattern:       "https://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "https://address[:port] insecure",
			clientSocket:           clientSocketTCP,
			useTLS:                 true,
			useInsecureTLS:         true,
			clientURLPattern:       "https://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
		{
			name:                   "https://domain[:port]",
			clientSocket:           clientSocketTCP,
			useTLS:                 true,
			clientURLPattern:       "https://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "https://address[:port]",
			clientSocket:           clientSocketTCP,
			useTLS:                 true,
			clientURLPattern:       "https://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
	}
	for _, tc := range tcs {
		for _, clusterSize := range []int{1, 3} {
			t.Run(fmt.Sprintf("Size: %d, Scenario: %q", clusterSize, tc.name), func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				cfg := config.NewClusterConfig()
				cfg.ClusterSize = clusterSize

				switch {
				case tc.useInsecureTLS:
					cfg.ClientTLS = config.AutoTLS
				case tc.useTLS:
					cfg.ClientTLS = config.ManualTLS
				default:
					cfg.ClientTLS = config.NoTLS
				}

				opts := []config.ClusterOption{
					config.WithClusterConfig(cfg),
					WithHTTP2Debug(), // enable http2 header logs only for e2e tests
				}

				switch tc.clientSocket {
				case clientSocketTCP:
					opts = append(opts, WithTCPClient())
				case clientSocketUnix:
					opts = append(opts, WithUnixClient())
				}

				clus := testRunner.NewCluster(ctx, t, opts...)
				defer clus.Close()

				tmpEndpoints, ok := clus.(intf.TemplateEndpoints)
				require.Truef(t, ok, "cluster does not implement TemplateEndpoints")

				endpoints := tmpEndpoints.TemplateEndpoints(t, tc.clientURLPattern)

				cc := testutils.MustClient(clus.Client(WithEndpoints(endpoints)))

				for i := 0; i < 100; i++ {
					require.NoError(t, cc.Put(ctx, "foo", "bar", config.PutOptions{}))
				}

				testutils.ExecuteWithTimeout(t, 5*time.Second, func() {
					asserter, ok := clus.(intf.AssertAuthority)
					require.Truef(t, ok, "cluster does not implement AssertAuthority")
					asserter.AssertAuthority(t, tc.expectAuthorityPattern)
				})
			})
		}
	}
}
