// Copyright 2026 The etcd Authors
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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	integration "go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestCARotation tests live CA rotation without cluster restart.
// It starts a cluster with CA1, adds CA2 to the trust bundle, then
// removes CA1, verifying connections work throughout.
func TestCARotation(t *testing.T) {
	integration.BeforeTest(t)

	certDir := t.TempDir()

	// Phase 1: Generate initial CA and certs signed by it
	ca1Key, ca1Cert := generateTestCA(t, "CA1")
	serverKey, serverCert := generateTestCert(t, ca1Key, ca1Cert, "server", true)
	clientKey, clientCert := generateTestCert(t, ca1Key, ca1Cert, "client", false)

	// Write CA1 trust bundle
	writePEMCert(t, filepath.Join(certDir, "ca.pem"), ca1Cert)
	// Write server cert and key
	writePEMCert(t, filepath.Join(certDir, "server.crt"), serverCert)
	writePEMKey(t, filepath.Join(certDir, "server.key"), serverKey)
	// Write client cert and key
	writePEMCert(t, filepath.Join(certDir, "client.crt"), clientCert)
	writePEMKey(t, filepath.Join(certDir, "client.key"), clientKey)

	tlsInfo := transport.TLSInfo{
		CertFile:         filepath.Join(certDir, "server.crt"),
		KeyFile:          filepath.Join(certDir, "server.key"),
		TrustedCAFile:    filepath.Join(certDir, "ca.pem"),
		ClientCertFile:   filepath.Join(certDir, "client.crt"),
		ClientKeyFile:    filepath.Join(certDir, "client.key"),
		ClientCertAuth:   true,
		ReloadTrustedCA:  true,
		CAReloadInterval: 100 * time.Millisecond, // Fast reload for testing
		Logger:           zaptest.NewLogger(t),
	}
	// Enable tracking so we can clean up CAReloaders later
	tlsInfo.EnableCAReloaderTracking()
	t.Cleanup(func() {
		tlsInfo.Close()
	})

	// Start single-node cluster with CA reload enabled
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:      1,
		PeerTLS:   &tlsInfo,
		ClientTLS: &tlsInfo,
		UseTCP:    true, // Use TCP for TLS testing
		UseIP:     true,
	})
	t.Cleanup(func() {
		clus.Terminate(t)
	})

	// Verify cluster works with CA1
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := clus.Client(0).Put(ctx, "phase1", "ca1-only")
	cancel()
	require.NoError(t, err, "put with CA1 should succeed")
	t.Log("Phase 1: Cluster working with CA1")

	// Phase 2: Generate CA2 and add to trust bundle
	ca2Key, ca2Cert := generateTestCA(t, "CA2")

	// Write combined trust bundle [CA1 + CA2]
	writePEMCerts(t, filepath.Join(certDir, "ca.pem"), ca1Cert, ca2Cert)
	t.Log("Phase 2: Trust bundle updated to [CA1 + CA2]")

	// Wait for CA reload
	time.Sleep(300 * time.Millisecond)

	// Verify cluster still works
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	_, err = clus.Client(0).Put(ctx, "phase2", "ca1-and-ca2")
	cancel()
	require.NoError(t, err, "put with [CA1+CA2] trust bundle should succeed")
	t.Log("Phase 2: Cluster working with [CA1 + CA2] trust bundle")

	// Phase 3: Remove CA1 from trust bundle (keep only CA2)
	// Note: Since server/client certs are still signed by CA1, we need to
	// generate new certs signed by CA2 first.

	// Save CA1-signed client certs for Phase 4 rejection test
	writePEMCert(t, filepath.Join(certDir, "client-ca1.crt"), clientCert)
	writePEMKey(t, filepath.Join(certDir, "client-ca1.key"), clientKey)

	// Generate new server/client certs signed by CA2
	serverKey2, serverCert2 := generateTestCert(t, ca2Key, ca2Cert, "server", true)
	clientKey2, clientCert2 := generateTestCert(t, ca2Key, ca2Cert, "client", false)

	// Write new certs
	writePEMCert(t, filepath.Join(certDir, "server.crt"), serverCert2)
	writePEMKey(t, filepath.Join(certDir, "server.key"), serverKey2)
	writePEMCert(t, filepath.Join(certDir, "client.crt"), clientCert2)
	writePEMKey(t, filepath.Join(certDir, "client.key"), clientKey2)
	t.Log("Phase 3: Server and client certs rotated to CA2-signed certs")

	// Wait for cert reload
	time.Sleep(300 * time.Millisecond)

	// Now update trust bundle to CA2 only
	writePEMCert(t, filepath.Join(certDir, "ca.pem"), ca2Cert)
	t.Log("Phase 3: Trust bundle updated to [CA2] only")

	// Wait for CA reload
	time.Sleep(300 * time.Millisecond)

	// Create a new client with the new CA2-signed client certs
	// (The cluster's internal client may still use old connection)
	newTLSInfo := transport.TLSInfo{
		CertFile:       filepath.Join(certDir, "client.crt"),
		KeyFile:        filepath.Join(certDir, "client.key"),
		TrustedCAFile:  filepath.Join(certDir, "ca.pem"),
		ClientCertAuth: true,
	}
	cc, err := newTLSInfo.ClientConfig()
	require.NoError(t, err, "client TLS config should succeed")

	newClient, err := integration.NewClient(t, clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCURL},
		DialTimeout: 5 * time.Second,
		TLS:         cc,
	})
	require.NoError(t, err, "new client connection should succeed with CA2")
	t.Cleanup(func() {
		newClient.Close()
	})

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	_, err = newClient.Put(ctx, "phase3", "ca2-only")
	cancel()
	require.NoError(t, err, "put with CA2-only trust bundle should succeed")
	t.Log("Phase 3: Cluster working with CA2 only")

	// Phase 4: Verify CA1-signed clients are now rejected
	// This confirms the old CA was actually removed from the trust bundle
	ca1ClientTLSInfo := transport.TLSInfo{
		CertFile:       filepath.Join(certDir, "client-ca1.crt"),
		KeyFile:        filepath.Join(certDir, "client-ca1.key"),
		TrustedCAFile:  filepath.Join(certDir, "ca.pem"), // Now contains only CA2
		ClientCertAuth: true,
	}
	ca1ClientConfig, err := ca1ClientTLSInfo.ClientConfig()
	require.NoError(t, err, "CA1 client TLS config should succeed")

	// Attempt connection with CA1-signed cert - should fail
	// Note: gRPC uses lazy connection, so we need to make an actual RPC call
	// to trigger the TLS handshake and get the rejection error
	oldClient, err := integration.NewClient(t, clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCURL},
		DialTimeout: 2 * time.Second,
		TLS:         ca1ClientConfig,
	})
	if err == nil {
		t.Cleanup(func() {
			oldClient.Close()
		})
		// Client created, but connection should fail on actual RPC
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		_, err = oldClient.Put(ctx, "phase4", "should-fail")
		cancel()
		require.Error(t, err, "RPC with CA1-signed cert should be rejected")
	}
	// Either NewClient failed or the RPC failed - both are acceptable
	t.Log("Phase 4: CA1-signed client correctly rejected - CA rotation verified")
}

// generateTestCA creates a new self-signed CA certificate using ECC P-256.
func generateTestCA(t *testing.T, name string) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"etcd"},
			CommonName:   name,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return key, cert
}

// generateTestCert creates a certificate signed by the given CA.
// If isServer is true, creates a server certificate; otherwise, a client certificate.
func generateTestCert(t *testing.T, caKey *ecdsa.PrivateKey, caCert *x509.Certificate, name string, isServer bool) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"etcd"},
			CommonName:   name,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	if isServer {
		template.KeyUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	} else {
		template.KeyUsage = x509.KeyUsageDigitalSignature
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return key, cert
}

// writePEMCert writes a single certificate to a PEM file.
func writePEMCert(t *testing.T, path string, cert *x509.Certificate) {
	t.Helper()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	require.NoError(t, os.WriteFile(path, certPEM, 0644))
}

// writePEMCerts writes multiple certificates to a PEM file (trust bundle).
func writePEMCerts(t *testing.T, path string, certs ...*x509.Certificate) {
	t.Helper()
	var bundle []byte
	for _, cert := range certs {
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		bundle = append(bundle, certPEM...)
	}
	require.NoError(t, os.WriteFile(path, bundle, 0644))
}

// writePEMKey writes an ECDSA private key to a PEM file.
func writePEMKey(t *testing.T, path string, key *ecdsa.PrivateKey) {
	t.Helper()
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(path, keyPEM, 0600))
}
