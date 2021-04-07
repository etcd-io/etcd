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

package transport

import (
	"crypto/tls"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestNewTransportTLSInvalidCipherSuitesTLS12 expects a client with invalid
// cipher suites fail to handshake with the server.
func TestNewTransportTLSInvalidCipherSuitesTLS12(t *testing.T) {
	tlsInfo, del, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del()

	cipherSuites := []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}

	// make server and client have unmatched cipher suites
	srvTLS, cliTLS := *tlsInfo, *tlsInfo
	srvTLS.CipherSuites, cliTLS.CipherSuites = cipherSuites[:2], cipherSuites[2:]

	ln, err := NewListener("127.0.0.1:0", "https", &srvTLS)
	if err != nil {
		t.Fatalf("unexpected NewListener error: %v", err)
	}
	defer ln.Close()

	donec := make(chan struct{})
	go func() {
		ln.Accept()
		donec <- struct{}{}
	}()
	go func() {
		tr, err := NewTransport(cliTLS, 3*time.Second)
		tr.TLSClientConfig.MaxVersion = tls.VersionTLS12
		if err != nil {
			t.Errorf("unexpected NewTransport error: %v", err)
		}
		cli := &http.Client{Transport: tr}
		_, gerr := cli.Get("https://" + ln.Addr().String())
		if gerr == nil || !strings.Contains(gerr.Error(), "tls: handshake failure") {
			t.Error("expected client TLS handshake error")
		}
		ln.Close()
		donec <- struct{}{}
	}()
	<-donec
	<-donec
}
