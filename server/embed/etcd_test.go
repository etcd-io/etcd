// Copyright 2024 The etcd Authors
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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/transport"
)

func TestEmptyClientTLSInfo_createMetricsListener(t *testing.T) {
	e := &Etcd{
		cfg: Config{
			ClientTLSInfo: transport.TLSInfo{},
		},
	}

	murl := url.URL{
		Scheme: "https",
		Host:   "localhost:8080",
	}
	_, err := e.createMetricsListener(murl)
	require.ErrorIsf(t, err, ErrMissingClientTLSInfoForMetricsURL, "expected error %v, got %v", ErrMissingClientTLSInfoForMetricsURL, err)
}

func TestCustomClientTLSConfig_startEtcd(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dataDir := fmt.Sprintf("%s/data/", tmpDir)

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()
	cfg.Dir = dataDir
	cfg.CustomClientTLSConfig = &tls.Config{
		ClientAuth: tls.RequestClientCert,
	}

	e, err := StartEtcd(cfg)
	require.NoError(t, err)
	assert.Equal(t, tls.RequestClientCert, e.cfg.CustomClientTLSConfig.ClientAuth)
}

func TestEmptyPeerTLSInfo_configurePeerListeners(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()
	cfg.PeerTLSInfo = transport.TLSInfo{}

	u, err := url.Parse("http://localhost:0")
	require.NoError(t, err)
	cfg.ListenPeerUrls = []url.URL{*u}

	peers, err := configurePeerListeners(cfg)
	require.NoError(t, err)
	actPeerHost, _, err := net.SplitHostPort(peers[0].Listener.Addr().String())
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", actPeerHost)
}

func TestPeerSelfCert_configurePeerListeners(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_EXIT") == "1" {
		cfg := NewConfig()
		cfg.Logger = "zap"
		cfg.LogOutputs = []string{"/dev/null"}
		cfg.setupLogging()
		cfg.PeerAutoTLS = true
		cfg.SelfSignedCertValidity = 0

		u, err := url.Parse("http://localhost:0")
		require.NoError(t, err)
		cfg.ListenPeerUrls = []url.URL{*u}

		configurePeerListeners(cfg)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestPeerSelfCert_configurePeerListeners")
	cmd.Env = append(os.Environ(), "TEST_EXIT=1")
	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && !exitErr.Success() {
		t.Logf("os.Exit was called with code: %d", exitErr.ExitCode())
	} else {
		t.Fatalf("expected os.Exit to be called, but it wasn't")
	}
}

func TestCustomPeerTLSConfig_MinMaxVersion_configurePeerListeners(t *testing.T) {
	t.Parallel()

	caCert, caPrivateKey := generateCACert(t, &defaultCACertificateSubject)
	serverCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, &defaultServerCertificateSubject)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()
	cfg.TlsMinVersion = "TLS1.2"
	cfg.TlsMaxVersion = "TLS1.3"
	cfg.CustomPeerTLSConfig = tlsConfig

	u, err := url.Parse("http://localhost:0")
	require.NoError(t, err)
	cfg.ListenPeerUrls = []url.URL{*u}

	_, err = configurePeerListeners(cfg)
	require.NoError(t, err)
	expMinVersion := uint16(tls.VersionTLS12)
	expMaxVersion := uint16(tls.VersionTLS13)
	assert.Equal(t, expMinVersion, cfg.CustomPeerTLSConfig.MinVersion)
	assert.Equal(t, expMaxVersion, cfg.CustomPeerTLSConfig.MaxVersion)
}

func TestPeerTLSInfo_HTTP_configurePeerListeners(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	caCert, caPrivateKey := generateCACert(t, &defaultCACertificateSubject)
	serverCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, &defaultServerCertificateSubject)

	keyFilePath := fmt.Sprintf("%s/server-key.pem", tmpDir)
	certFilePath := fmt.Sprintf("%s/server-cert.pem", tmpDir)

	saveKey(serverCert, keyFilePath)
	saveCert(serverCert, certFilePath)

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()

	cfg.PeerTLSInfo = transport.TLSInfo{
		CertFile:       certFilePath,
		KeyFile:        keyFilePath,
		ClientCertAuth: true,
	}

	urlString := "http://localhost:0"
	u, err := url.Parse(urlString)
	require.NoError(t, err)
	cfg.ListenPeerUrls = []url.URL{*u}

	peers, err := configurePeerListeners(cfg)
	require.NoError(t, err)
	assert.Len(t, peers, 1)
	actPeerHost, _, err := net.SplitHostPort(peers[0].Listener.Addr().String())
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", actPeerHost)
}

func TestPeerTLSInfo_listenerError_configurePeerListeners(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	caCert, caPrivateKey := generateCACert(t, &defaultCACertificateSubject)
	serverCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, &defaultServerCertificateSubject)

	keyFilePath := fmt.Sprintf("%s/server-key.pem", tmpDir)
	certFilePath := fmt.Sprintf("%s/server-cert.pem", tmpDir)

	saveKey(serverCert, keyFilePath)
	saveCert(serverCert, certFilePath)

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()

	cfg.PeerTLSInfo = transport.TLSInfo{
		CertFile:       certFilePath,
		KeyFile:        keyFilePath,
		ClientCertAuth: true,
	}

	u := &url.URL{
		Scheme: "http",
		Host:   "malformed:0",
	}
	cfg.ListenPeerUrls = []url.URL{*u}

	_, err := configurePeerListeners(cfg)

	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected *net.OpError, got %T", err)
	}

	if opErr.Op != "listen" {
		t.Errorf("unexpected Op: got %s, want listen", opErr.Op)
	}
}

func TestCustomPeerTLSConfig_listenerError_configurePeerListeners(t *testing.T) {
	t.Parallel()

	caCert, caPrivateKey := generateCACert(t, &defaultCACertificateSubject)
	serverCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, &defaultServerCertificateSubject)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()
	cfg.CustomPeerTLSConfig = tlsConfig

	u := &url.URL{
		Scheme: "http",
		Host:   "malformed:0",
	}
	cfg.ListenPeerUrls = []url.URL{*u}

	_, err := configurePeerListeners(cfg)

	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected *net.OpError, got %T", err)
	}

	if opErr.Op != "listen" {
		t.Errorf("unexpected Op: got %s, want listen", opErr.Op)
	}
}

func TestClientSelfCert_fatal_configureClientListeners(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_EXIT") == "1" {
		cfg := NewConfig()
		cfg.Logger = "zap"
		cfg.LogOutputs = []string{"/dev/null"}
		cfg.setupLogging()
		cfg.ClientAutoTLS = true
		cfg.SelfSignedCertValidity = 0

		u, err := url.Parse("http://localhost:0")
		require.NoError(t, err)
		cfg.ListenPeerUrls = []url.URL{*u}

		configureClientListeners(cfg)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestClientSelfCert_fatal_configureClientListeners")
	cmd.Env = append(os.Environ(), "TEST_EXIT=1")
	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && !exitErr.Success() {
		t.Logf("os.Exit was called with code: %d", exitErr.ExitCode())
	} else {
		t.Fatalf("expected os.Exit to be called, but it wasn't")
	}
}

func TestClientTLSInfo_listener_configureClientListeners(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	caCert, caPrivateKey := generateCACert(t, &defaultCACertificateSubject)
	clientCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, &defaultClientCertificateSubject)

	keyFilePath := fmt.Sprintf("%s/client-key.pem", tmpDir)
	certFilePath := fmt.Sprintf("%s/client-cert.pem", tmpDir)

	saveKey(clientCert, keyFilePath)
	saveCert(clientCert, certFilePath)

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()

	cfg.ClientTLSInfo = transport.TLSInfo{
		CertFile:       certFilePath,
		KeyFile:        keyFilePath,
		ClientCertAuth: true,
	}

	address := "localhost:0"
	urlString := fmt.Sprintf("https://%s", address)
	u, err := url.Parse(urlString)
	require.NoError(t, err)
	cfg.ListenClientUrls = []url.URL{*u}

	sctxs, err := configureClientListeners(cfg)
	require.NoError(t, err)
	sctx := sctxs[address]
	require.NotNil(t, sctx)
	assert.Equal(t, "tcp", sctx.network)
	assert.Equal(t, "localhost:0", sctx.addr)
	assert.False(t, sctx.insecure)
}

func TestCustomClientTLSConfig_listener_configureClientListeners(t *testing.T) {
	t.Parallel()

	caCert, caPrivateKey := generateCACert(t, &defaultCACertificateSubject)
	clientCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, &defaultClientCertificateSubject)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
	}

	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.setupLogging()
	cfg.CustomClientTLSConfig = tlsConfig

	address := "localhost:0"
	urlString := fmt.Sprintf("https://%s", address)
	u, err := url.Parse(urlString)
	require.NoError(t, err)
	cfg.ListenClientUrls = []url.URL{*u}

	sctxs, err := configureClientListeners(cfg)
	require.NoError(t, err)
	sctx := sctxs[address]
	require.NotNil(t, sctx)
	assert.Equal(t, "tcp", sctx.network)
	assert.Equal(t, "localhost:0", sctx.addr)
	assert.False(t, sctx.insecure)
}
