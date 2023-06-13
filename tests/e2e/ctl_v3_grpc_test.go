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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
				ctx, cancel := context.WithCancel(context.Background())
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

				epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t, e2e.WithConfig(cfg))
				if err != nil {
					t.Fatalf("could not start etcd process cluster (%v)", err)
				}
				defer epc.Close()
				endpoints := templateEndpoints(t, tc.clientURLPattern, epc)

				client, err := e2e.NewEtcdctl(cfg.Client, endpoints)
				assert.NoError(t, err)
				err = client.Put(ctx, "foo", "bar", config.PutOptions{})
				if err != nil {
					t.Fatal(err)
				}

				testutils.ExecuteWithTimeout(t, 5*time.Second, func() {
					assertAuthority(t, strings.ReplaceAll(tc.expectAuthorityPattern, "${MEMBER_PORT}", "20000"), epc)
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

func assertAuthority(t *testing.T, expectAurhority string, clus *e2e.EtcdProcessCluster) {
	var logs []e2e.LogsExpect
	for _, proc := range clus.Procs {
		logs = append(logs, proc.Logs())
	}
	line := firstMatch(t, `http2: decoded hpack field header field ":authority"`, logs...)
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	expectLine := fmt.Sprintf(`http2: decoded hpack field header field ":authority" = %q`, expectAurhority)
	assert.True(t, strings.HasSuffix(line, expectLine), fmt.Sprintf("Got %q expected suffix %q", line, expectLine))
}

func firstMatch(t *testing.T, expectLine string, logs ...e2e.LogsExpect) string {
	t.Helper()
	match := make(chan string, len(logs))
	for i := range logs {
		go func(l e2e.LogsExpect) {
			line, _ := l.ExpectWithContext(context.TODO(), expectLine)
			match <- line
		}(logs[i])
	}
	return <-match
}
