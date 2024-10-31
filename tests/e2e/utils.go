// Copyright 2023 The etcd Authors
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

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/stringutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func newClient(t *testing.T, entpoints []string, cfg e2e.ClientConfig) *clientv3.Client {
	tlscfg, err := tlsInfo(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ccfg := clientv3.Config{
		Endpoints:   entpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	if tlscfg != nil {
		ccfg.TLS, err = tlscfg.ClientConfig()
		if err != nil {
			t.Fatal(err)
		}
	}
	c, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Close()
	})
	return c
}

// tlsInfo follows the Client-to-server communication in https://etcd.io/docs/v3.6/op-guide/security/#basic-setup
func tlsInfo(t testing.TB, cfg e2e.ClientConfig) (*transport.TLSInfo, error) {
	switch cfg.ConnectionType {
	case e2e.ClientNonTLS, e2e.ClientTLSAndNonTLS:
		return nil, nil
	case e2e.ClientTLS:
		if cfg.AutoTLS {
			tls, err := transport.SelfCert(zap.NewNop(), t.TempDir(), []string{"localhost"}, 1)
			if err != nil {
				return nil, fmt.Errorf("failed to generate cert: %w", err)
			}
			return &tls, nil
		}
		return &integration.TestTLSInfo, nil
	default:
		return nil, fmt.Errorf("config %v not supported", cfg)
	}
}

func fillEtcdWithData(ctx context.Context, c *clientv3.Client, dbSize int) error {
	g := errgroup.Group{}
	concurrency := 10
	keyCount := 100
	keysPerRoutine := keyCount / concurrency
	valueSize := dbSize / keyCount
	for i := 0; i < concurrency; i++ {
		i := i
		g.Go(func() error {
			for j := 0; j < keysPerRoutine; j++ {
				_, err := c.Put(ctx, fmt.Sprintf("%d", i*keysPerRoutine+j), stringutil.RandString(uint(valueSize)))
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	return g.Wait()
}

func curl(endpoint string, method string, curlReq e2e.CURLReq, connType e2e.ClientConnType) (string, error) {
	args := e2e.CURLPrefixArgs(endpoint, e2e.ClientConfig{ConnectionType: connType}, false, method, curlReq)
	lines, err := e2e.RunUtilCompletion(args, nil)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func runCommandAndReadJSONOutput(args []string) (map[string]any, error) {
	lines, err := e2e.RunUtilCompletion(args, nil)
	if err != nil {
		return nil, err
	}

	var resp map[string]any
	err = json.Unmarshal([]byte(strings.Join(lines, "\n")), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func getMemberIDByName(ctx context.Context, c *e2e.EtcdctlV3, name string) (id uint64, found bool, err error) {
	resp, err := c.MemberList(ctx, false)
	if err != nil {
		return 0, false, err
	}
	for _, member := range resp.Members {
		if name == member.Name {
			return member.ID, true, nil
		}
	}
	return 0, false, nil
}

func patchArgs(args []string, flag, newValue string) error {
	for i, arg := range args {
		if strings.Contains(arg, flag) {
			args[i] = fmt.Sprintf("--%s=%s", flag, newValue)
			return nil
		}
	}
	return fmt.Errorf("--%s flag not found", flag)
}

func generateCertsForIPs(tempDir string, ips []net.IP) (caFile string, certFiles []string, keyFiles []string, err error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1001),
		Subject: pkix.Name{
			Organization:       []string{"etcd"},
			OrganizationalUnit: []string{"etcd Security"},
			Locality:           []string{"San Francisco"},
			Province:           []string{"California"},
			Country:            []string{"USA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", nil, nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return "", nil, nil, err
	}

	caFile, _, err = saveCertToFile(tempDir, caBytes, nil)
	if err != nil {
		return "", nil, nil, err
	}

	for i, ip := range ips {
		cert := &x509.Certificate{
			SerialNumber: big.NewInt(1001 + int64(i)),
			Subject: pkix.Name{
				Organization:       []string{"etcd"},
				OrganizationalUnit: []string{"etcd Security"},
				Locality:           []string{"San Francisco"},
				Province:           []string{"California"},
				Country:            []string{"USA"},
			},
			IPAddresses:  []net.IP{ip},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().AddDate(0, 0, 1),
			SubjectKeyId: []byte{1, 2, 3, 4, 5},
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
		certKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return "", nil, nil, err
		}
		certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certKey.PublicKey, caKey)
		if err != nil {
			return "", nil, nil, err
		}
		certFile, keyFile, err := saveCertToFile(tempDir, certBytes, certKey)
		if err != nil {
			return "", nil, nil, err
		}
		certFiles = append(certFiles, certFile)
		keyFiles = append(keyFiles, keyFile)
	}

	return caFile, certFiles, keyFiles, nil
}

func saveCertToFile(tempDir string, certBytes []byte, key *rsa.PrivateKey) (certFile string, keyFile string, err error) {
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	cf, err := os.CreateTemp(tempDir, "*.crt")
	if err != nil {
		return "", "", err
	}
	defer cf.Close()
	if _, err := cf.Write(certPEM.Bytes()); err != nil {
		return "", "", err
	}

	if key != nil {
		certKeyPEM := new(bytes.Buffer)
		pem.Encode(certKeyPEM, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})

		kf, err := os.CreateTemp(tempDir, "*.key.insecure")
		if err != nil {
			return "", "", err
		}
		defer kf.Close()
		if _, err := kf.Write(certKeyPEM.Bytes()); err != nil {
			return "", "", err
		}

		return cf.Name(), kf.Name(), nil
	}

	return cf.Name(), "", nil
}

func getLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddress := conn.LocalAddr().(*net.UDPAddr)

	return localAddress.IP.String(), nil
}
