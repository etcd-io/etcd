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

	"github.com/coreos/etcd/internal/version"
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
	if err := cURLGet(cx.epc, cURLReq{endpoint: "/metrics", expected: `etcd_debugging_mvcc_keys_total 1`, metricsURLScheme: cx.cfg.metricsURLScheme}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
	if err := cURLGet(cx.epc, cURLReq{endpoint: "/metrics", expected: fmt.Sprintf(`etcd_server_version{server_version="%s"} 1`, version.Version), metricsURLScheme: cx.cfg.metricsURLScheme}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
	if err := cURLGet(cx.epc, cURLReq{endpoint: "/health", expected: `{"health":"true"}`, metricsURLScheme: cx.cfg.metricsURLScheme}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}
