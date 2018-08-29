// Copyright 2018 The etcd Authors
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

// +build !cov,!cluster_proxy

package e2e

import (
	"fmt"
	"testing"

	"go.etcd.io/etcd/version"
)

func TestV3CurlCipherSuitesValid(t *testing.T)    { testV3CurlCipherSuites(t, true) }
func TestV3CurlCipherSuitesMismatch(t *testing.T) { testV3CurlCipherSuites(t, false) }
func testV3CurlCipherSuites(t *testing.T, valid bool) {
	cc := configClientTLS
	cc.clusterSize = 1
	cc.cipherSuites = []string{
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
	}
	testFunc := cipherSuiteTestValid
	if !valid {
		testFunc = cipherSuiteTestMismatch
	}
	testCtl(t, testFunc, withCfg(cc))
}

func cipherSuiteTestValid(cx ctlCtx) {
	if err := cURLGet(cx.epc, cURLReq{
		endpoint:         "/metrics",
		expected:         fmt.Sprintf(`etcd_server_version{server_version="%s"} 1`, version.Version),
		metricsURLScheme: cx.cfg.metricsURLScheme,
		ciphers:          "ECDHE-RSA-AES128-GCM-SHA256", // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}

func cipherSuiteTestMismatch(cx ctlCtx) {
	var err error
	for _, exp := range []string{"alert handshake failure", "failed setting cipher list"} {
		err = cURLGet(cx.epc, cURLReq{
			endpoint:         "/metrics",
			expected:         exp,
			metricsURLScheme: cx.cfg.metricsURLScheme,
			ciphers:          "ECDHE-RSA-DES-CBC3-SHA", // TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}
