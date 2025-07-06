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

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

func TestAuthority(t *testing.T) {
	tcs := []struct {
		name   string
		useTCP bool
		useTLS bool
		// Pattern used to generate endpoints for client. Fields filled
		// %d - will be filled with member grpc port
		// %s - will be filled with member name
		clientURLPattern string

		// Pattern used to validate authority received by server. Fields filled:
		// %d - will be filled with first member grpc port
		// %s - will be filled with first member name
		expectAuthorityPattern string
	}{
		{
			name:                   "unix:path",
			clientURLPattern:       "unix:localhost:%s",
			expectAuthorityPattern: "localhost:%s",
		},
		{
			name:                   "unix://absolute_path",
			clientURLPattern:       "unix://localhost:%s",
			expectAuthorityPattern: "localhost:%s",
		},
		// "unixs" is not standard schema supported by etcd
		{
			name:                   "unixs:absolute_path",
			useTLS:                 true,
			clientURLPattern:       "unixs:localhost:%s",
			expectAuthorityPattern: "localhost:%s",
		},
		{
			name:                   "unixs://absolute_path",
			useTLS:                 true,
			clientURLPattern:       "unixs://localhost:%s",
			expectAuthorityPattern: "localhost:%s",
		},
		{
			name:                   "http://domain[:port]",
			useTCP:                 true,
			clientURLPattern:       "http://localhost:%d",
			expectAuthorityPattern: "localhost:%d",
		},
		{
			name:                   "https://domain[:port]",
			useTLS:                 true,
			useTCP:                 true,
			clientURLPattern:       "https://localhost:%d",
			expectAuthorityPattern: "localhost:%d",
		},
		{
			name:                   "http://address[:port]",
			useTCP:                 true,
			clientURLPattern:       "http://127.0.0.1:%d",
			expectAuthorityPattern: "127.0.0.1:%d",
		},
		{
			name:                   "https://address[:port]",
			useTCP:                 true,
			useTLS:                 true,
			clientURLPattern:       "https://127.0.0.1:%d",
			expectAuthorityPattern: "127.0.0.1:%d",
		},
	}
	for _, tc := range tcs {
		for _, clusterSize := range []int{1, 3} {
			t.Run(fmt.Sprintf("Size: %d, Scenario: %q", clusterSize, tc.name), func(t *testing.T) {
				BeforeTest(t)
				cfg := ClusterConfig{
					Size:   clusterSize,
					UseTCP: tc.useTCP,
					UseIP:  tc.useTCP,
				}
				cfg, tlsConfig := setupTLS(t, tc.useTLS, cfg)
				clus := NewClusterV3(t, &cfg)
				defer clus.Terminate(t)

				kv := setupClient(t, tc.clientURLPattern, clus, tlsConfig)
				defer kv.Close()

				_, err := kv.Put(context.TODO(), "foo", "bar")
				if err != nil {
					t.Fatal(err)
				}

				assertAuthority(t, templateAuthority(t, tc.expectAuthorityPattern, clus.Members[0]), clus)
			})
		}
	}
}

func setupTLS(t *testing.T, useTLS bool, cfg ClusterConfig) (ClusterConfig, *tls.Config) {
	t.Helper()
	if useTLS {
		cfg.ClientTLS = &testTLSInfo
		tlsConfig, err := testTLSInfo.ClientConfig()
		if err != nil {
			t.Fatal(err)
		}
		return cfg, tlsConfig
	}
	return cfg, nil
}

func setupClient(t *testing.T, endpointPattern string, clus *ClusterV3, tlsConfig *tls.Config) *clientv3.Client {
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

func templateEndpoints(t *testing.T, pattern string, clus *ClusterV3) []string {
	t.Helper()
	endpoints := []string{}
	for _, m := range clus.Members {
		ent := pattern
		if strings.Contains(ent, "%d") {
			ent = fmt.Sprintf(ent, GrpcPortNumber(m.UniqNumber, m.MemberNumber))
		}
		if strings.Contains(ent, "%s") {
			ent = fmt.Sprintf(ent, m.Name)
		}
		if strings.Contains(ent, "%") {
			t.Fatalf("Failed to template pattern, %% symbol left %q", ent)
		}
		endpoints = append(endpoints, ent)
	}
	return endpoints
}

func templateAuthority(t *testing.T, pattern string, m *member) string {
	t.Helper()
	authority := pattern
	if strings.Contains(authority, "%d") {
		authority = fmt.Sprintf(authority, GrpcPortNumber(m.UniqNumber, m.MemberNumber))
	}
	if strings.Contains(authority, "%s") {
		authority = fmt.Sprintf(authority, m.Name)
	}
	if strings.Contains(authority, "%") {
		t.Fatalf("Failed to template pattern, %% symbol left %q", authority)
	}
	return authority
}

func assertAuthority(t *testing.T, expectedAuthority string, clus *ClusterV3) {
	t.Helper()
	requestsFound := 0
	for _, m := range clus.Members {
		for _, r := range m.RecordedRequests() {
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
