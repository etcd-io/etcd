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

package integration

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

func TestTLSClientCipherSuitesValid(t *testing.T)    { testTLSCipherSuites(t, true) }
func TestTLSClientCipherSuitesMismatch(t *testing.T) { testTLSCipherSuites(t, false) }

// testTLSCipherSuites ensures mismatching client-side cipher suite
// fail TLS handshake with the server.
func testTLSCipherSuites(t *testing.T, valid bool) {
	BeforeTest(t)

	cipherSuites := []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}
	srvTLS, cliTLS := testTLSInfo, testTLSInfo
	if valid {
		srvTLS.CipherSuites, cliTLS.CipherSuites = cipherSuites, cipherSuites
	} else {
		srvTLS.CipherSuites, cliTLS.CipherSuites = cipherSuites[:2], cipherSuites[2:]
	}

	clus := NewClusterV3(t, &ClusterConfig{Size: 1, ClientTLS: &srvTLS})
	defer clus.Terminate(t)

	cc, err := cliTLS.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	cli, cerr := clientv3.New(clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCAddr()},
		DialTimeout: time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         cc,
	})
	if cli != nil {
		cli.Close()
	}
	if !valid && cerr != context.DeadlineExceeded {
		t.Fatalf("expected %v with TLS handshake failure, got %v", context.DeadlineExceeded, cerr)
	}
	if valid && cerr != nil {
		t.Fatalf("expected TLS handshake success, got %v", cerr)
	}
}
