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

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestTLSClientCipherSuitesValid(t *testing.T)    { testTLSCipherSuites(t, true) }
func TestTLSClientCipherSuitesMismatch(t *testing.T) { testTLSCipherSuites(t, false) }

// testTLSCipherSuites ensures mismatching client-side cipher suite
// fail TLS handshake with the server.
func testTLSCipherSuites(t *testing.T, valid bool) {
	integration.BeforeTest(t)

	cipherSuites := []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}
	srvTLS, cliTLS := integration.TestTLSInfo, integration.TestTLSInfo
	if valid {
		srvTLS.CipherSuites, cliTLS.CipherSuites = cipherSuites, cipherSuites
	} else {
		srvTLS.CipherSuites, cliTLS.CipherSuites = cipherSuites[:2], cipherSuites[2:]
	}

	// go1.13 enables TLS 1.3 by default
	// and in TLS 1.3, cipher suites are not configurable,
	// so setting Max TLS version to TLS 1.2 to test cipher config.
	srvTLS.MaxVersion = tls.VersionTLS12
	cliTLS.MaxVersion = tls.VersionTLS12

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1, ClientTLS: &srvTLS})
	defer clus.Terminate(t)

	cc, err := cliTLS.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	cli, cerr := integration.NewClient(t, clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCURL()},
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

func TestTLSMinMaxVersion(t *testing.T) {
	integration.BeforeTest(t)

	tests := []struct {
		name        string
		minVersion  uint16
		maxVersion  uint16
		expectError bool
	}{
		{
			name:       "Connect with default TLS version should succeed",
			minVersion: 0,
			maxVersion: 0,
		},
		{
			name:        "Connect with TLS 1.2 only should fail",
			minVersion:  tls.VersionTLS12,
			maxVersion:  tls.VersionTLS12,
			expectError: true,
		},
		{
			name:       "Connect with TLS 1.2 and 1.3 should succeed",
			minVersion: tls.VersionTLS12,
			maxVersion: tls.VersionTLS13,
		},
		{
			name:       "Connect with TLS 1.3 only should succeed",
			minVersion: tls.VersionTLS13,
			maxVersion: tls.VersionTLS13,
		},
	}

	// Configure server to support TLS 1.3 only.
	srvTLS := integration.TestTLSInfo
	srvTLS.MinVersion = tls.VersionTLS13
	srvTLS.MaxVersion = tls.VersionTLS13
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1, ClientTLS: &srvTLS})
	defer clus.Terminate(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc, err := integration.TestTLSInfo.ClientConfig()
			assert.NoError(t, err)

			cc.MinVersion = tt.minVersion
			cc.MaxVersion = tt.maxVersion
			cli, cerr := integration.NewClient(t, clientv3.Config{
				Endpoints:   []string{clus.Members[0].GRPCURL()},
				DialTimeout: time.Second,
				DialOptions: []grpc.DialOption{grpc.WithBlock()},
				TLS:         cc,
			})
			if cerr != nil {
				assert.True(t, tt.expectError, "got TLS handshake error while expecting success: %v", cerr)
				assert.Equal(t, context.DeadlineExceeded, cerr, "expected %v with TLS handshake failure, got %v", context.DeadlineExceeded, cerr)
				return
			}

			cli.Close()
		})
	}
}
