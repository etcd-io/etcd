// Copyright 2022 The etcd Authors
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

package common

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func newSerialNumber(t *testing.T) *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fail()
	}
	return serialNumber
}

func createRootCertificateAuthority(rootCaPath string, oldPem []byte, t *testing.T) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
	serialNumber := newSerialNumber(t)
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"etcd"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * (24 * time.Hour)),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	ca, err := x509.ParseCertificate(caBytes)
	if err != nil {
		t.Fatal(err)
	}
	caBlocks := [][]byte{caBytes}
	if len(oldPem) > 0 {
		caBlocks = append(caBlocks, oldPem)
	}
	marshalCerts(caBlocks, rootCaPath, t)
	return ca, caBytes, priv
}

func generateCerts(privKey *ecdsa.PrivateKey, rootCA *x509.Certificate, dir, suffix string, t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serialNumber := newSerialNumber(t)
	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"etcd"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * (24 * time.Hour)),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, rootCA, &priv.PublicKey, privKey)
	if err != nil {
		t.Fatal(err)
	}
	marshalCerts([][]byte{caBytes}, path.Join(dir, fmt.Sprintf("cert%s.pem", suffix)), t)
	marshalKeys(priv, path.Join(dir, fmt.Sprintf("key%s.pem", suffix)), t)
}

func marshalCerts(caBytes [][]byte, certPath string, t *testing.T) {
	var caPerm bytes.Buffer
	for _, caBlock := range caBytes {
		err := pem.Encode(&caPerm, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caBlock,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	ioutil.WriteFile(certPath, caPerm.Bytes(), 0644)
}

func marshalKeys(privKey *ecdsa.PrivateKey, keyPath string, t *testing.T) {
	privBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		t.Fatal(err)
	}

	var keyPerm bytes.Buffer
	err = pem.Encode(&keyPerm, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	ioutil.WriteFile(keyPath, keyPerm.Bytes(), 0644)
}

func TestRootCARotation(t *testing.T) {
	testRunner.BeforeTest(t)

	t.Run("server CA rotation", func(t *testing.T) {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "tlsdir-integration-reload")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpdir)
		rootCAPath := path.Join(tmpdir, "ca-cert.pem")
		rootCA, caBytes, privKey := createRootCertificateAuthority(rootCAPath, []byte{}, t)
		generateCerts(privKey, rootCA, tmpdir, "_itest_old", t)

		tlsInfo := &transport.TLSInfo{
			TrustedCAFile:      rootCAPath,
			CertFile:           path.Join(tmpdir, "cert_itest_old.pem"),
			KeyFile:            path.Join(tmpdir, "key_itest_old.pem"),
			ClientCertFile:     path.Join(tmpdir, "cert_itest_old.pem"),
			ClientKeyFile:      path.Join(tmpdir, "key_itest_old.pem"),
			EnableRootCAReload: true,
		}
		clusConfig := config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS, ClientTLSInfo: tlsInfo}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(clusConfig))
		defer clus.Close()

		cc, cerr := clus.Client(WithTLSInfo(tlsInfo))
		if cerr != nil {
			t.Fatalf("expected TLS handshake success, got %v", cerr)
		}
		testutils.ExecuteUntil(ctx, t, func() {
			key := "foo"
			_, err := cc.Get(ctx, key, config.GetOptions{})
			if err != nil {
				t.Fatalf("Unexpeted result, err: %s", err)
			}
		})

		// regenerate rootCA and sign new certs
		rootCA, _, privKey = createRootCertificateAuthority(rootCAPath, caBytes, t)
		generateCerts(privKey, rootCA, tmpdir, "_itest_new", t)

		// old rootCA certs
		cc, cerr = clus.Client(WithTLSInfo(tlsInfo))
		if cerr != nil {
			t.Fatalf("expected TLS handshake success, got %v", cerr)
		}
		testutils.ExecuteUntil(ctx, t, func() {
			key := "foo"
			_, err := cc.Get(ctx, key, config.GetOptions{})
			if err != nil {
				t.Fatalf("Unexpeted result, err: %s", err)
			}
		})

		// new rootCA certs
		newClientTlsinfo := &transport.TLSInfo{
			TrustedCAFile:  rootCAPath,
			CertFile:       path.Join(tmpdir, "cert_itest_new.pem"),
			KeyFile:        path.Join(tmpdir, "key_itest_new.pem"),
			ClientCertFile: path.Join(tmpdir, "cert_itest_new.pem"),
			ClientKeyFile:  path.Join(tmpdir, "key_itest_new.pem"),
		}

		cc, cerr = clus.Client(WithTLSInfo(newClientTlsinfo))
		if cerr != nil {
			t.Fatalf("expected TLS handshake success, got %v", cerr)
		}
		testutils.ExecuteUntil(ctx, t, func() {
			key := "foo"
			_, err := cc.Get(ctx, key, config.GetOptions{})
			if err != nil {
				t.Fatalf("Unexpeted result, err: %s", err)
			}
		})
	})

	// TODO(hongbin): added test for peer CA rotation
}
