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

//go:build !cov && !cluster_proxy
// +build !cov,!cluster_proxy

package e2e

import (
	"fmt"
	"testing"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestV3CurlCipherSuitesValid(t *testing.T)    { testV3CurlCipherSuites(t, true) }
func TestV3CurlCipherSuitesMismatch(t *testing.T) { testV3CurlCipherSuites(t, false) }
func testV3CurlCipherSuites(t *testing.T, valid bool) {
	cc := e2e.NewConfigClientTLS()
	cc.ClusterSize = 1
	cc.CipherSuites = []string{
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
	testCtl(t, testFunc, withCfg(*cc))
}

func cipherSuiteTestValid(cx ctlCtx) {
	if err := e2e.CURLGet(cx.epc, e2e.CURLReq{
		Endpoint:         "/metrics",
		Expected:         fmt.Sprintf(`etcd_server_version{server_version="%s"} 1`, version.Version),
		MetricsURLScheme: cx.cfg.MetricsURLScheme,
		Ciphers:          "ECDHE-RSA-AES128-GCM-SHA256", // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}

func cipherSuiteTestMismatch(cx ctlCtx) {
	var err error
	for _, exp := range []string{"alert handshake failure", "failed setting cipher list"} {
		err = e2e.CURLGet(cx.epc, e2e.CURLReq{
			Endpoint:         "/metrics",
			Expected:         exp,
			MetricsURLScheme: cx.cfg.MetricsURLScheme,
			Ciphers:          "ECDHE-RSA-DES-CBC3-SHA", // TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}
