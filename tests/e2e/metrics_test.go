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
	"testing"

	"go.etcd.io/etcd/v3/version"
)

func TestV3MetricsSecure(t *testing.T) {
	cfg := configTLS
	cfg.clusterSize = 1
	cfg.metricsURLScheme = "https"
	testCtl(t, func(cx ctlCtx) {
		metricsTest(cx, false)
	})
}

func TestV3MetricsInsecure(t *testing.T) {
	cfg := configTLS
	cfg.clusterSize = 1
	cfg.metricsURLScheme = "http"
	testCtl(t, func(cx ctlCtx) {
		metricsTest(cx, false)
	})
}

func TestV3MetricsSecureTLSCertAuth(t *testing.T) {
	testCtl(t, func(cx ctlCtx) {
		metricsTest(cx, true)

		req := cURLReq{endpoint: "/metrics", metricsURLScheme: cx.cfg.metricsURLScheme}

		expectErr := []string{
			"curl: (60) SSL certificate problem: unable to get local issuer certificate",
			"More details here: https://curl.haxx.se/docs/sslcerts.html",
			"",
			"curl failed to verify the legitimacy of the server and therefore could not",
			"establish a secure connection to it. To learn more about this situation and",
			"how to fix it, please visit the web page mentioned above.",
		}

		if err := spawnWithExpects(cURLPrefixArgs(cx.epc, "GET", req), expectErr...); err != nil {
			cx.t.Fatalf("failed get with curl (%v)", err)
		}
	}, withCfg(configMetricsTLS))
}

func metricsTest(cx ctlCtx, useCertAuth bool) {
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

		if err := cURLGet(cx.epc, cURLReq{endpoint: test.endpoint, expected: test.expected, metricsURLScheme: cx.cfg.metricsURLScheme, useCertAuth: useCertAuth}); err != nil {
			cx.t.Fatalf("failed get with curl (%v)", err)
		}
	}
}
