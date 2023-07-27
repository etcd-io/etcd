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

package integration

import (
	"context"
	tls "crypto/tls"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestAuthority(t *testing.T) {
	tcs := []struct {
		name                   string
		useTCP                 bool
		useTLS                 bool
		clientURLPattern       string
		expectAuthorityPattern string
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
			name:                   "https://address[:port]",
			useTCP:                 true,
			useTLS:                 true,
			clientURLPattern:       "https://127.0.0.1:${MEMBER_PORT}",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
	}
	for _, tc := range tcs {
		for _, clusterSize := range []int{1, 3} {
			t.Run(fmt.Sprintf("Size: %d, Scenario: %q", clusterSize, tc.name), func(t *testing.T) {
				integration.BeforeTest(t)
				cfg := integration.ClusterConfig{
					Size:   clusterSize,
					UseTCP: tc.useTCP,
					UseIP:  tc.useTCP,
				}
				cfg, tlsConfig := setupTLS(t, tc.useTLS, cfg)
				clus := integration.NewCluster(t, &cfg)
				defer clus.Terminate(t)

				kv := setupClient(t, tc.clientURLPattern, clus, tlsConfig)
				defer kv.Close()

				putRequestMethod := "/etcdserverpb.KV/Put"
				for i := 0; i < 100; i++ {
					_, err := kv.Put(context.TODO(), "foo", "bar")
					if err != nil {
						t.Fatal(err)
					}
				}

				assertAuthority(t, tc.expectAuthorityPattern, clus, putRequestMethod)
			})
		}
	}
}

func setupTLS(t *testing.T, useTLS bool, cfg integration.ClusterConfig) (integration.ClusterConfig, *tls.Config) {
	t.Helper()
	if useTLS {
		cfg.ClientTLS = &integration.TestTLSInfo
		tlsConfig, err := integration.TestTLSInfo.ClientConfig()
		if err != nil {
			t.Fatal(err)
		}
		return cfg, tlsConfig
	}
	return cfg, nil
}

func setupClient(t *testing.T, endpointPattern string, clus *integration.Cluster, tlsConfig *tls.Config) *clientv3.Client {
	t.Helper()
	endpoints := templateEndpoints(t, endpointPattern, clus)
	kv, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         tlsConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	return kv
}

func templateEndpoints(t *testing.T, pattern string, clus *integration.Cluster) []string {
	t.Helper()
	var endpoints []string
	for _, m := range clus.Members {
		ent := pattern
		ent = strings.ReplaceAll(ent, "${MEMBER_PORT}", m.GrpcPortNumber())
		ent = strings.ReplaceAll(ent, "${MEMBER_NAME}", m.Name)
		endpoints = append(endpoints, ent)
	}
	return endpoints
}

func templateAuthority(t *testing.T, pattern string, m *integration.Member) string {
	t.Helper()
	authority := pattern
	authority = strings.ReplaceAll(authority, "${MEMBER_PORT}", m.GrpcPortNumber())
	authority = strings.ReplaceAll(authority, "${MEMBER_NAME}", m.Name)
	return authority
}

func assertAuthority(t *testing.T, expectedAuthorityPattern string, clus *integration.Cluster, filterMethod string) {
	t.Helper()
	for _, m := range clus.Members {
		requestsFound := 0
		expectedAuthority := templateAuthority(t, expectedAuthorityPattern, m)
		for _, r := range m.RecordedRequests() {
			if filterMethod != "" && r.FullMethod != filterMethod {
				continue
			}
			if r.Authority == expectedAuthority {
				requestsFound++
			} else {
				t.Errorf("Got unexpected authority header, member: %q, request: %q, got authority: %q, expected %q", m.Name, r.FullMethod, r.Authority, expectedAuthority)
			}
		}
		if requestsFound == 0 {
			t.Errorf("Expect at least one request with matched authority header value was recorded by the server intercepter on member %s but got 0", m.Name)
		}
	}
}
