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

package tlsutil

import (
	"crypto/tls"
	"testing"
)

func TestGetCipherSuite_not_existing(t *testing.T) {
	_, ok := GetCipherSuite("not_existing")
	if ok {
		t.Fatal("Expected not ok")
	}
}

func CipherSuiteExpectedToExist(tb testing.TB, cipher string, expectedId uint16) {
	vid, ok := GetCipherSuite(cipher)
	if !ok {
		tb.Errorf("Expected %v cipher to exist", cipher)
	}
	if vid != expectedId {
		tb.Errorf("For %v expected=%v found=%v", cipher, expectedId, vid)
	}
}

func TestGetCipherSuite_success(t *testing.T) {
	CipherSuiteExpectedToExist(t, "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA)
	CipherSuiteExpectedToExist(t, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)

	// Explicit test for legacy names
	CipherSuiteExpectedToExist(t, "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256)
	CipherSuiteExpectedToExist(t, "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256)
}

func TestGetCipherSuite_insecure(t *testing.T) {
	CipherSuiteExpectedToExist(t, "TLS_ECDHE_RSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA)
}
