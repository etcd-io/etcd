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
	"fmt"
	"net/http"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/version"
)

func TestV3MetricsSecure(t *testing.T) {
	cfg := configTLS
	cfg.clusterSize = 1
	cfg.metricsURLScheme = "https"
	testCtl(t, metricsTest)
}

func TestV3MetricsInsecure(t *testing.T) {
	cfg := configTLS
	cfg.clusterSize = 1
	cfg.metricsURLScheme = "http"
	testCtl(t, metricsTest)
}

func metricsTest(cx ctlCtx) {
	if err := ctlV3Put(cx, "k", "v", ""); err != nil {
		cx.t.Fatal(err)
	}

	i := 0
	for _, test := range []struct {
		endpoint, expected string
	}{
		{"/metrics", fmt.Sprintf("etcd_mvcc_put_total 2")},
		{"/metrics", fmt.Sprintf("etcd_debugging_mvcc_keys_total 1")},
		{"/metrics", fmt.Sprintf("etcd_mvcc_delete_total 3")},
		{"/metrics", fmt.Sprintf(`etcd_server_version{server_version="%s"} 1`, version.Version)},
		{"/metrics", fmt.Sprintf(`etcd_cluster_version{cluster_version="%s"} 1`, version.Cluster(version.Version))},
		{"/health", `{"health":"true","reason":""}`},
	} {
		i++
		if err := ctlV3Put(cx, fmt.Sprintf("%d", i), "v", ""); err != nil {
			cx.t.Fatal(err)
		}
		if err := ctlV3Del(cx, []string{fmt.Sprintf("%d", i)}, 1); err != nil {
			cx.t.Fatal(err)
		}

		if err := cURLGet(cx.epc, cURLReq{endpoint: test.endpoint, expected: test.expected, metricsURLScheme: cx.cfg.metricsURLScheme}); err != nil {
			cx.t.Fatalf("failed get with curl (%v)", err)
		}
	}
}

func TestInsecureHealthEndpointEnabled(t *testing.T) {
	cfg := etcdProcessClusterConfig{
		clusterSize:                   1,
		initialToken:                  "new",
		enableV2:                      false,
		insecureHealthEndpointEnabled: true,
	}
	testCtl(t, func(cx ctlCtx) {
		if err := ctlV3Put(cx, "k", "v", ""); err != nil {
			cx.t.Fatal(err)
		}
		addr := cx.epc.procs[0].Config().insecureHealthEndpoint
		if len(addr) == 0 {
			t.Fatal("expected endpoint")
		}
		endpoint := fmt.Sprintf("http://%s/health", addr)
		client := http.Client{
			Timeout: 5 * time.Second,
		}
		resp, err := client.Get(endpoint)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if e, a := http.StatusOK, resp.StatusCode; e != a {
			t.Fatalf("expected %d response from health endpoint %q, got %d", e, endpoint, a)
		}
	}, withCfg(cfg))
}
