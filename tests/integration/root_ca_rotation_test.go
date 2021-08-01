package integration

import (
	"bytes"
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
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
	integration.BeforeTest(t)

	tmpdir, err := ioutil.TempDir(os.TempDir(), "tlsdir-integration-reload")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	rootCAPath := path.Join(tmpdir, "ca-cert.pem")
	logger := zap.NewExample()
	rootCA, caBytes, privKey := createRootCertificateAuthority(rootCAPath, []byte{}, t)
	generateCerts(privKey, rootCA, tmpdir, "_itest_old", t)

	tlsInfo := &transport.TLSInfo{
		TrustedCAFile:      rootCAPath,
		CertFile:           path.Join(tmpdir, "cert_itest_old.pem"),
		KeyFile:            path.Join(tmpdir, "key_itest_old.pem"),
		ClientCertFile:     path.Join(tmpdir, "cert_itest_old.pem"),
		ClientKeyFile:      path.Join(tmpdir, "key_itest_old.pem"),
		Logger:             logger,
		RefreshDuration:    100 * time.Millisecond,
		EnableRootCAReload: true,
	}
	defer tlsInfo.Close()

	cluster := integration.NewCluster(
		t,
		&integration.ClusterConfig{
			Size:      1,
			ClientTLS: tlsInfo,
		},
	)
	defer cluster.Terminate(t)

	cc, err := tlsInfo.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}

	cli, cerr := integration.NewClient(t, clientv3.Config{
		Endpoints:   []string{cluster.Members[0].GRPCURL()},
		DialTimeout: time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         cc,
	})

	if cli != nil {
		cli.Close()
	}

	if cerr != nil {
		t.Fatalf("expected TLS handshake success, got %v", cerr)
	}

	// regenerate rootCA and sign new certs
	rootCA, _, privKey = createRootCertificateAuthority(rootCAPath, caBytes, t)
	generateCerts(privKey, rootCA, tmpdir, "_itest_new", t)

	// give server some time to reload new CA
	time.Sleep(time.Second)

	newClientTlsinfo := &transport.TLSInfo{
		TrustedCAFile:  rootCAPath,
		CertFile:       path.Join(tmpdir, "cert_itest_new.pem"),
		KeyFile:        path.Join(tmpdir, "key_itest_new.pem"),
		ClientCertFile: path.Join(tmpdir, "cert_itest_new.pem"),
		ClientKeyFile:  path.Join(tmpdir, "key_itest_new.pem"),
		Logger:         logger,
	}

	// old rootCA certs
	cli, cerr = integration.NewClient(t, clientv3.Config{
		Endpoints:   []string{cluster.Members[0].GRPCURL()},
		DialTimeout: time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         cc,
	})

	if cli != nil {
		cli.Close()
	}

	if cerr != nil {
		t.Fatalf("expected TLS handshake success, got %v", cerr)
	}

	// new rootCA certs
	cc, err = newClientTlsinfo.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}

	cli, cerr = integration.NewClient(t, clientv3.Config{
		Endpoints:   []string{cluster.Members[0].GRPCURL()},
		DialTimeout: time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         cc,
	})

	if cli != nil {
		cli.Close()
	}

	if cerr != nil {
		t.Fatalf("expected TLS handshake success, got %v", cerr)
	}
}
