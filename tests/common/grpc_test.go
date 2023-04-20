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
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func Test_Authority(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name                   string
		useTLS                 bool
		useInsecureTLS         bool
		clientURLPattern       string
		expectAuthorityPattern string
		clientOptions          []config.ClientOption
	}{
		{
			name:                   "unix:path",
			clientURLPattern:       "unix:localhost:${MEMBER_NAME}",
			expectAuthorityPattern: "localhost:${MEMBER_NAME}",
		},
		{
			name:                   "unix://absolute_path",
			clientURLPattern:       "unix://localhost:${MEMBER_NAME}",
			expectAuthorityPattern: "localhost:${MEMBER_NAME}",
		},
		// "unixs" is not standard schema supported by etcd
		{
			name:                   "unixs:absolute_path",
			useTLS:                 true,
			clientURLPattern:       "unixs:localhost:${MEMBER_NAME}",
			expectAuthorityPattern: "localhost:${MEMBER_NAME}",
		},
		{
			name:                   "unixs://absolute_path",
			useTLS:                 true,
			clientURLPattern:       "unixs://localhost:${MEMBER_NAME}",
			expectAuthorityPattern: "localhost:${MEMBER_NAME}",
		},
		{
			name:                   "http://domain[:port]",
			useTCP:                 true,
			clientURLPattern:       "http://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "https://domain[:port]",
			useTLS:                 true,
			useTCP:                 true,
			clientURLPattern:       "https://localhost:${MEMBER_PORT}",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "http://address[:port]",
			useTCP:                 true,
			clientURLPattern:       "http://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
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
			name:                   "https://address[:port]",
			useTCP:                 true,
			useTLS:                 true,
			clientURLPattern:       "https://127.0.0.1:${MEMBER_PORT}",
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
	for _, tc := range clusterTestCases() {
		for _, nc := range tcs {
			t.Run(tc.name+"/"+nc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
				defer clus.Close()
				cc := testutils.MustClient(clus.Client(tc.ClientOptions))

				testutils.ExecuteUntil(ctx, t, func() {
					putRequestMethod := "/etcdserverpb.KV/Put"
					_, err := kv.Put(context.TODO(), "foo", "bar")
					if err != nil {
						t.Fatal(err)
					}

					assertAuthority(t, templateAuthority(t, tc.expectAuthorityPattern, clus.Members[0]), clus, putRequestMethod)
				})
			})
		}
	}
}

func templateAuthority(t *testing.T, pattern string, m *integration.Member) string {
	t.Helper()
	authority := pattern
	authority = strings.ReplaceAll(authority, "${MEMBER_PORT}", m.GrpcPortNumber())
	authority = strings.ReplaceAll(authority, "${MEMBER_NAME}", m.Name)
	return authority
}

func assertAuthority(t *testing.T, expectedAuthority string, clus *integration.Cluster, filterMethod string) {
	t.Helper()
	requestsFound := 0
	for _, m := range clus.Members {
		for _, r := range m.RecordedRequests() {
			if filterMethod != "" && r.FullMethod != filterMethod {
				continue
			}
			requestsFound++
			if r.Authority != expectedAuthority {
				t.Errorf("Got unexpected authority header, member: %q, request: %q, got authority: %q, expected %q", m.Name, r.FullMethod, r.Authority, expectedAuthority)
			}
		}
	}
	if requestsFound == 0 {
		t.Errorf("Expected at least one request")
	}
}

/*
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
*/
