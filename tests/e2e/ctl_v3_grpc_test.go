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
// +build !cluster_proxy

package e2e

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestAuthority(t *testing.T) {
	tcs := []struct {
		name           string
		useTLS         bool
		useInsecureTLS bool
		// Pattern used to generate endpoints for client. Fields filled
		// %d - will be filled with member grpc port
		clientURLPattern string

		// Pattern used to validate authority received by server. Fields filled:
		// ${MEMBER_PORT} - will be filled with member grpc port
		expectAuthorityPattern string
	}{
		{
			name:                   "http://domain[:port]",
			clientURLPattern:       "http://localhost:%d",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "http://address[:port]",
			clientURLPattern:       "http://127.0.0.1:%d",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
		{
			name:                   "https://domain[:port] insecure",
			useTLS:                 true,
			useInsecureTLS:         true,
			clientURLPattern:       "https://localhost:%d",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "https://address[:port] insecure",
			useTLS:                 true,
			useInsecureTLS:         true,
			clientURLPattern:       "https://127.0.0.1:%d",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
		{
			name:                   "https://domain[:port]",
			useTLS:                 true,
			clientURLPattern:       "https://localhost:%d",
			expectAuthorityPattern: "localhost:${MEMBER_PORT}",
		},
		{
			name:                   "https://address[:port]",
			useTLS:                 true,
			clientURLPattern:       "https://127.0.0.1:%d",
			expectAuthorityPattern: "127.0.0.1:${MEMBER_PORT}",
		},
	}
	for _, tc := range tcs {
		for _, clusterSize := range []int{1, 3} {
			t.Run(fmt.Sprintf("Size: %d, Scenario: %q", clusterSize, tc.name), func(t *testing.T) {
				e2e.BeforeTest(t)

				cfg := e2e.NewConfigNoTLS()
				cfg.ClusterSize = clusterSize
				if tc.useTLS {
					cfg.ClientTLS = e2e.ClientTLS
				}
				cfg.IsClientAutoTLS = tc.useInsecureTLS
				// Enable debug mode to get logs with http2 headers (including authority)
				cfg.EnvVars = map[string]string{"GODEBUG": "http2debug=2"}

				epc, err := e2e.NewEtcdProcessCluster(t, cfg)
				if err != nil {
					t.Fatalf("could not start etcd process cluster (%v)", err)
				}
				defer epc.Close()
				endpoints := templateEndpoints(t, tc.clientURLPattern, epc)

				client := clusterEtcdctlV3(cfg, endpoints)
				for i := 0; i < 100; i++ {
					err = client.Put("foo", "bar")
					if err != nil {
						t.Fatal(err)
					}
				}

				executeWithTimeout(t, 5*time.Second, func() {
					assertAuthority(t, tc.expectAuthorityPattern, epc)
				})
			})
		}
	}
}

func templateEndpoints(t *testing.T, pattern string, clus *e2e.EtcdProcessCluster) []string {
	t.Helper()
	endpoints := []string{}
	for i := 0; i < clus.Cfg.ClusterSize; i++ {
		ent := pattern
		if strings.Contains(ent, "%d") {
			ent = fmt.Sprintf(ent, e2e.EtcdProcessBasePort+i*5)
		}
		if strings.Contains(ent, "%") {
			t.Fatalf("Failed to template pattern, %% symbol left %q", ent)
		}
		endpoints = append(endpoints, ent)
	}
	return endpoints
}

func assertAuthority(t *testing.T, expectAuthorityPattern string, clus *e2e.EtcdProcessCluster) {
	for i := range clus.Procs {
		line, _ := clus.Procs[i].Logs().Expect(`http2: decoded hpack field header field ":authority"`)
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
		u, err := url.Parse(clus.Procs[i].EndpointsGRPC()[0])
		if err != nil {
			t.Fatal(err)
		}
		expectAuthority := strings.ReplaceAll(expectAuthorityPattern, "${MEMBER_PORT}", u.Port())
		expectLine := fmt.Sprintf(`http2: decoded hpack field header field ":authority" = %q`, expectAuthority)
		assert.True(t, strings.HasSuffix(line, expectLine), fmt.Sprintf("Got %q expected suffix %q", line, expectLine))
	}
}

func executeWithTimeout(t *testing.T, timeout time.Duration, f func()) {
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		f()
	}()

	select {
	case <-time.After(timeout):
		testutil.FatalStack(t, fmt.Sprintf("test timed out after %v", timeout))
	case <-donec:
	}
}

type etcdctlV3 struct {
	cfg       *e2e.EtcdProcessClusterConfig
	endpoints []string
}

func clusterEtcdctlV3(cfg *e2e.EtcdProcessClusterConfig, endpoints []string) *etcdctlV3 {
	return &etcdctlV3{
		cfg:       cfg,
		endpoints: endpoints,
	}
}

func (ctl *etcdctlV3) Put(key, value string) error {
	return ctl.runCmd("put", key, value)
}

func (ctl *etcdctlV3) runCmd(args ...string) error {
	cmdArgs := []string{e2e.CtlBinPath + "3"}
	for k, v := range ctl.flags() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	cmdArgs = append(cmdArgs, args...)
	return e2e.SpawnWithExpect(cmdArgs, "OK")
}

func (ctl *etcdctlV3) flags() map[string]string {
	fmap := make(map[string]string)
	if ctl.cfg.ClientTLS == e2e.ClientTLS {
		if ctl.cfg.IsClientAutoTLS {
			fmap["insecure-transport"] = "false"
			fmap["insecure-skip-tls-verify"] = "true"
		} else if ctl.cfg.IsClientCRL {
			fmap["cacert"] = e2e.CaPath
			fmap["cert"] = e2e.RevokedCertPath
			fmap["key"] = e2e.RevokedPrivateKeyPath
		} else {
			fmap["cacert"] = e2e.CaPath
			fmap["cert"] = e2e.CertPath
			fmap["key"] = e2e.PrivateKeyPath
		}
	}
	fmap["endpoints"] = strings.Join(ctl.endpoints, ",")
	return fmap
}
