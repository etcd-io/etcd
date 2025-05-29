// Copyright 2025 The etcd Authors
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

package embed

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type certificateSubject struct {
	CommonName    string
	SerialNumber  string
	Organization  []string
	Country       []string
	Province      []string
	Locality      []string
	StreetAddress []string
	PostalCode    []string
}

var (
	defaultCACertificateSubject = certificateSubject{
		CommonName:    "CA",
		SerialNumber:  "1",
		Organization:  []string{"Test"},
		Country:       []string{"Test"},
		Province:      []string{"Test Province"},
		Locality:      []string{"Testing Department"},
		StreetAddress: []string{"Testing Street"},
		PostalCode:    []string{"01234"},
	}
	defaultServerCertificateSubject = certificateSubject{
		CommonName:    "Server",
		SerialNumber:  "2",
		Organization:  []string{"Test"},
		Country:       []string{"Test"},
		Province:      []string{"Test Province"},
		Locality:      []string{"Testing Department"},
		StreetAddress: []string{"Testing Street"},
		PostalCode:    []string{"01234"},
	}
	defaultClientCertificateSubject = certificateSubject{
		CommonName:    "Client",
		SerialNumber:  "3",
		Organization:  []string{"Test"},
		Country:       []string{"Test"},
		Province:      []string{"Test Province"},
		Locality:      []string{"Testing Department"},
		StreetAddress: []string{"Testing Street"},
		PostalCode:    []string{"01234"},
	}
)

func generateCACert(t *testing.T, subject *certificateSubject) (*x509.Certificate, *rsa.PrivateKey) {
	serialNumberInt64, _ := strconv.ParseInt(subject.SerialNumber, 10, 64)
	caCert := &x509.Certificate{
		IsCA:         true,
		SerialNumber: big.NewInt(serialNumberInt64),
		Subject: pkix.Name{
			CommonName:    subject.CommonName,
			SerialNumber:  subject.SerialNumber,
			Organization:  subject.Organization,
			Country:       subject.Country,
			Province:      subject.Province,
			Locality:      subject.Locality,
			StreetAddress: subject.StreetAddress,
			PostalCode:    subject.PostalCode,
		},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		BasicConstraintsValid: true,
	}

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)

	caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caPrivateKey.PublicKey, caPrivateKey)
	require.NoError(t, err)

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivateKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	})

	return caCert, caPrivateKey
}

func generateHostCertificateFromCA(t *testing.T, caCert *x509.Certificate, caPrivateKey *rsa.PrivateKey, subject *certificateSubject) tls.Certificate {
	serialNumberInt64, _ := strconv.ParseInt(subject.SerialNumber, 10, 64)
	tlsCertSpecification := &x509.Certificate{
		IsCA:         false,
		SerialNumber: big.NewInt(serialNumberInt64),
		Subject: pkix.Name{
			CommonName:    subject.CommonName,
			SerialNumber:  subject.SerialNumber,
			Organization:  subject.Organization,
			Country:       subject.Country,
			Province:      subject.Province,
			Locality:      subject.Locality,
			StreetAddress: subject.StreetAddress,
			PostalCode:    subject.PostalCode,
		},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)

	certBytes, err := x509.CreateCertificate(rand.Reader, tlsCertSpecification, caCert, &certPrivKey.PublicKey, caPrivateKey)
	require.NoError(t, err)

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	certificate, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	require.NoError(t, err)

	return certificate
}

func saveKey(cert tls.Certificate, keyPath string) error {
	keyOut, err := os.OpenFile(
		keyPath,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0o600,
	)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	var keyBytes []byte

	switch key := cert.PrivateKey.(type) {
	case *rsa.PrivateKey:
		keyBytes = x509.MarshalPKCS1PrivateKey(key)
		return pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	case *ecdsa.PrivateKey:
		keyBytes, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return err
		}
		return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	default:
		return fmt.Errorf("unsupported private key type: %T", cert.PrivateKey)
	}
}

func saveCert(cert tls.Certificate, certPath string) error {
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()

	for _, derBytes := range cert.Certificate {
		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		}
		if err := pem.Encode(certOut, block); err != nil {
			return err
		}
	}
	return nil
}
