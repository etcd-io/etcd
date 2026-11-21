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

package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A PEM-encoded CRL (the default output of `openssl ca -gencrl`) must be
// accepted the same way as a DER-encoded one; rejecting the PEM spelling made
// every TLS client connection fail closed. See #22877.
func TestCheckCRLAcceptsPEMAndDER(t *testing.T) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	revokedSerial := big.NewInt(1000)
	crlDER, err := caCert.CreateCRL(rand.Reader, caKey, []pkix.RevokedCertificate{
		{SerialNumber: revokedSerial, RevocationTime: time.Now()},
	}, time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	pemPath := filepath.Join(dir, "crl.pem")
	if err := os.WriteFile(pemPath, pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: crlDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	derPath := filepath.Join(dir, "crl.der")
	if err := os.WriteFile(derPath, crlDER, 0o600); err != nil {
		t.Fatal(err)
	}

	// Both spellings must parse; neither may reject the connection of a
	// non-revoked certificate (serial 1000 is the only revoked one).
	if err := checkCRL(pemPath, []*x509.Certificate{{SerialNumber: big.NewInt(2)}}); err != nil {
		t.Errorf("PEM CRL rejected a valid certificate: %v", err)
	}
	if err := checkCRL(derPath, []*x509.Certificate{{SerialNumber: big.NewInt(2)}}); err != nil {
		t.Errorf("DER CRL rejected a valid certificate: %v", err)
	}

	// The revocation check itself must still work on both spellings.
	revoked := &x509.Certificate{SerialNumber: revokedSerial}
	if err := checkCRL(pemPath, []*x509.Certificate{revoked}); err == nil {
		t.Error("PEM CRL did not reject the revoked certificate")
	}
	if err := checkCRL(derPath, []*x509.Certificate{revoked}); err == nil {
		t.Error("DER CRL did not reject the revoked certificate")
	}
}
