// Copyright 2015 The etcd Authors
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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/tlsutil"
)

func createSelfCert(t *testing.T) (*TLSInfo, error) {
	t.Helper()
	return createSelfCertEx(t, "127.0.0.1")
}

func createSelfCertEx(t *testing.T, host string, additionalUsages ...x509.ExtKeyUsage) (*TLSInfo, error) {
	t.Helper()
	d := t.TempDir()
	info, err := SelfCert(zaptest.NewLogger(t), d, []string{host + ":0"}, 1, additionalUsages...)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func fakeCertificateParserFunc(err error) func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
	return func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
		return tls.Certificate{}, err
	}
}

// TestNewListenerTLSInfo tests that NewListener with valid TLSInfo returns
// a TLS listener that accepts TLS connections.
func TestNewListenerTLSInfo(t *testing.T) {
	tlsInfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")
	testNewListenerTLSInfoAccept(t, *tlsInfo)
}

func TestNewListenerWithOpts(t *testing.T) {
	tlsInfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")

	tests := map[string]struct {
		opts        []ListenerOption
		scheme      string
		expectedErr bool
	}{
		"https scheme no TLSInfo": {
			opts:        []ListenerOption{},
			expectedErr: true,
			scheme:      "https",
		},
		"https scheme no TLSInfo with skip check": {
			opts:        []ListenerOption{WithSkipTLSInfoCheck(true)},
			expectedErr: false,
			scheme:      "https",
		},
		"https scheme empty TLSInfo with skip check": {
			opts: []ListenerOption{
				WithSkipTLSInfoCheck(true),
				WithTLSInfo(&TLSInfo{}),
			},
			expectedErr: false,
			scheme:      "https",
		},
		"https scheme empty TLSInfo no skip check": {
			opts: []ListenerOption{
				WithTLSInfo(&TLSInfo{}),
			},
			expectedErr: true,
			scheme:      "https",
		},
		"https scheme with TLSInfo and skip check": {
			opts: []ListenerOption{
				WithSkipTLSInfoCheck(true),
				WithTLSInfo(tlsInfo),
			},
			expectedErr: false,
			scheme:      "https",
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ln, err := NewListenerWithOpts("127.0.0.1:0", test.scheme, test.opts...)
			if ln != nil {
				defer ln.Close()
			}
			require.Falsef(t, test.expectedErr && err == nil, "expected error")
			if !test.expectedErr {
				require.NoErrorf(t, err, "unexpected error: %v", err)
			}
		})
	}
}

func TestNewListenerWithSocketOpts(t *testing.T) {
	tlsInfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")

	tests := map[string]struct {
		opts        []ListenerOption
		scheme      string
		expectedErr bool
	}{
		"nil socketopts": {
			opts:        []ListenerOption{WithSocketOpts(nil)},
			expectedErr: true,
			scheme:      "http",
		},
		"empty socketopts": {
			opts:        []ListenerOption{WithSocketOpts(&SocketOpts{})},
			expectedErr: true,
			scheme:      "http",
		},

		"reuse address": {
			opts:        []ListenerOption{WithSocketOpts(&SocketOpts{ReuseAddress: true})},
			scheme:      "http",
			expectedErr: true,
		},
		"reuse address with TLS": {
			opts: []ListenerOption{
				WithSocketOpts(&SocketOpts{ReuseAddress: true}),
				WithTLSInfo(tlsInfo),
			},
			scheme:      "https",
			expectedErr: true,
		},
		"reuse address and port": {
			opts:        []ListenerOption{WithSocketOpts(&SocketOpts{ReuseAddress: true, ReusePort: true})},
			scheme:      "http",
			expectedErr: false,
		},
		"reuse address and port with TLS": {
			opts: []ListenerOption{
				WithSocketOpts(&SocketOpts{ReuseAddress: true, ReusePort: true}),
				WithTLSInfo(tlsInfo),
			},
			scheme:      "https",
			expectedErr: false,
		},
		"reuse port with TLS and timeout": {
			opts: []ListenerOption{
				WithSocketOpts(&SocketOpts{ReusePort: true}),
				WithTLSInfo(tlsInfo),
				WithTimeout(5*time.Second, 5*time.Second),
			},
			scheme:      "https",
			expectedErr: false,
		},
		"reuse port with https scheme and no TLSInfo skip check": {
			opts: []ListenerOption{
				WithSocketOpts(&SocketOpts{ReusePort: true}),
				WithSkipTLSInfoCheck(true),
			},
			scheme:      "https",
			expectedErr: false,
		},
		"reuse port": {
			opts:        []ListenerOption{WithSocketOpts(&SocketOpts{ReusePort: true})},
			scheme:      "http",
			expectedErr: false,
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ln, err := NewListenerWithOpts("127.0.0.1:0", test.scheme, test.opts...)
			require.NoErrorf(t, err, "unexpected NewListenerWithSocketOpts error")
			defer ln.Close()
			ln2, err := NewListenerWithOpts(ln.Addr().String(), test.scheme, test.opts...)
			if ln2 != nil {
				ln2.Close()
			}
			if test.expectedErr {
				require.Errorf(t, err, "expected error")
			}
			if !test.expectedErr {
				require.NoErrorf(t, err, "unexpected error: %v", err)
			}

			if test.scheme == "http" {
				lnOpts := newListenOpts(test.opts...)
				if !lnOpts.IsSocketOpts() && !lnOpts.IsTimeout() {
					_, ok := ln.(*keepaliveListener)
					require.Truef(t, ok, "ln: unexpected listener type: %T, wanted *keepaliveListener", ln)
				}
			}
		})
	}
}

func testNewListenerTLSInfoAccept(t *testing.T, tlsInfo TLSInfo) {
	t.Helper()
	ln, err := NewListener("127.0.0.1:0", "https", &tlsInfo)
	require.NoErrorf(t, err, "unexpected NewListener error")
	defer ln.Close()

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	cli := &http.Client{Transport: tr}
	go cli.Get("https://" + ln.Addr().String())

	conn, err := ln.Accept()
	require.NoErrorf(t, err, "unexpected Accept error")
	defer conn.Close()
	if _, ok := conn.(*tls.Conn); !ok {
		t.Error("failed to accept *tls.Conn")
	}
}

// TestNewListenerTLSInfoSkipClientSANVerify tests that if client IP address mismatches
// with specified address in its certificate the connection is still accepted
// if the flag SkipClientSANVerify is set (i.e. checkSAN() is disabled for the client side)
func TestNewListenerTLSInfoSkipClientSANVerify(t *testing.T) {
	tests := []struct {
		skipClientSANVerify bool
		goodClientHost      bool
		acceptExpected      bool
	}{
		{false, true, true},
		{false, false, false},
		{true, true, true},
		{true, false, true},
	}
	for _, test := range tests {
		testNewListenerTLSInfoClientCheck(t, test.skipClientSANVerify, test.goodClientHost, test.acceptExpected)
	}
}

func testNewListenerTLSInfoClientCheck(t *testing.T, skipClientSANVerify, goodClientHost, acceptExpected bool) {
	t.Helper()
	tlsInfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")

	host := "127.0.0.222"
	if goodClientHost {
		host = "127.0.0.1"
	}
	clientTLSInfo, err := createSelfCertEx(t, host, x509.ExtKeyUsageClientAuth)
	require.NoErrorf(t, err, "unable to create cert")

	tlsInfo.SkipClientSANVerify = skipClientSANVerify
	tlsInfo.TrustedCAFile = clientTLSInfo.CertFile

	rootCAs := x509.NewCertPool()
	loaded, err := os.ReadFile(tlsInfo.CertFile)
	require.NoErrorf(t, err, "unexpected missing certfile")
	rootCAs.AppendCertsFromPEM(loaded)

	clientCert, err := tls.LoadX509KeyPair(clientTLSInfo.CertFile, clientTLSInfo.KeyFile)
	require.NoErrorf(t, err, "unable to create peer cert")

	tlsConfig := &tls.Config{}
	tlsConfig.InsecureSkipVerify = false
	tlsConfig.Certificates = []tls.Certificate{clientCert}
	tlsConfig.RootCAs = rootCAs

	ln, err := NewListener("127.0.0.1:0", "https", tlsInfo)
	require.NoErrorf(t, err, "unexpected NewListener error")
	defer ln.Close()

	tr := &http.Transport{TLSClientConfig: tlsConfig}
	cli := &http.Client{Transport: tr}
	chClientErr := make(chan error, 1)
	go func() {
		_, err := cli.Get("https://" + ln.Addr().String())
		chClientErr <- err
	}()

	chAcceptErr := make(chan error, 1)
	chAcceptConn := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			chAcceptErr <- err
		} else {
			chAcceptConn <- conn
		}
	}()

	select {
	case <-chClientErr:
		if acceptExpected {
			t.Errorf("accepted for good client address: skipClientSANVerify=%t, goodClientHost=%t", skipClientSANVerify, goodClientHost)
		}
	case acceptErr := <-chAcceptErr:
		t.Fatalf("unexpected Accept error: %v", acceptErr)
	case conn := <-chAcceptConn:
		defer conn.Close()
		if _, ok := conn.(*tls.Conn); !ok {
			t.Errorf("failed to accept *tls.Conn")
		}
		if !acceptExpected {
			t.Errorf("accepted for bad client address: skipClientSANVerify=%t, goodClientHost=%t", skipClientSANVerify, goodClientHost)
		}
	}
}

func TestNewListenerTLSEmptyInfo(t *testing.T) {
	_, err := NewListener("127.0.0.1:0", "https", nil)
	if err == nil {
		t.Errorf("err = nil, want not presented error")
	}
}

func TestNewTransportTLSInfo(t *testing.T) {
	tlsinfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")

	tests := []TLSInfo{
		{},
		{
			CertFile: tlsinfo.CertFile,
			KeyFile:  tlsinfo.KeyFile,
		},
		{
			CertFile:      tlsinfo.CertFile,
			KeyFile:       tlsinfo.KeyFile,
			TrustedCAFile: tlsinfo.TrustedCAFile,
		},
		{
			TrustedCAFile: tlsinfo.TrustedCAFile,
		},
	}

	for i, tt := range tests {
		tt.parseFunc = fakeCertificateParserFunc(nil)
		trans, err := NewTransport(tt, time.Second)
		require.NoErrorf(t, err, "Received unexpected error from NewTransport")
		require.NotNilf(t, trans.TLSClientConfig, "#%d: want non-nil TLSClientConfig", i)
	}
}

func TestTLSInfoNonexist(t *testing.T) {
	tlsInfo := TLSInfo{CertFile: "@badname", KeyFile: "@badname"}
	_, err := tlsInfo.ServerConfig()
	werr := &os.PathError{
		Op:   "open",
		Path: "@badname",
		Err:  errors.New("no such file or directory"),
	}
	if err.Error() != werr.Error() {
		t.Errorf("err = %v, want %v", err, werr)
	}
}

func TestTLSInfoEmpty(t *testing.T) {
	tests := []struct {
		info TLSInfo
		want bool
	}{
		{TLSInfo{}, true},
		{TLSInfo{TrustedCAFile: "baz"}, true},
		{TLSInfo{CertFile: "foo"}, false},
		{TLSInfo{KeyFile: "bar"}, false},
		{TLSInfo{CertFile: "foo", KeyFile: "bar"}, false},
		{TLSInfo{CertFile: "foo", TrustedCAFile: "baz"}, false},
		{TLSInfo{KeyFile: "bar", TrustedCAFile: "baz"}, false},
		{TLSInfo{CertFile: "foo", KeyFile: "bar", TrustedCAFile: "baz"}, false},
	}

	for i, tt := range tests {
		got := tt.info.Empty()
		if tt.want != got {
			t.Errorf("#%d: result of Empty() incorrect: want=%t got=%t", i, tt.want, got)
		}
	}
}

func TestTLSInfoMissingFields(t *testing.T) {
	tlsinfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")

	tests := []TLSInfo{
		{CertFile: tlsinfo.CertFile},
		{KeyFile: tlsinfo.KeyFile},
		{CertFile: tlsinfo.CertFile, TrustedCAFile: tlsinfo.TrustedCAFile},
		{KeyFile: tlsinfo.KeyFile, TrustedCAFile: tlsinfo.TrustedCAFile},
	}

	for i, info := range tests {
		if _, err = info.ServerConfig(); err == nil {
			t.Errorf("#%d: expected non-nil error from ServerConfig()", i)
		}

		if _, err = info.ClientConfig(); err == nil {
			t.Errorf("#%d: expected non-nil error from ClientConfig()", i)
		}
	}
}

func TestTLSInfoParseFuncError(t *testing.T) {
	tlsinfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")

	tests := []struct {
		info TLSInfo
	}{
		{
			info: *tlsinfo,
		},

		{
			info: TLSInfo{CertFile: "", KeyFile: "", TrustedCAFile: tlsinfo.CertFile, EmptyCN: true},
		},
	}

	for i, tt := range tests {
		tt.info.parseFunc = fakeCertificateParserFunc(errors.New("fake"))

		if _, err = tt.info.ServerConfig(); err == nil {
			t.Errorf("#%d: expected non-nil error from ServerConfig()", i)
		}

		if _, err = tt.info.ClientConfig(); err == nil {
			t.Errorf("#%d: expected non-nil error from ClientConfig()", i)
		}
	}
}

func TestTLSInfoConfigFuncs(t *testing.T) {
	ln := zaptest.NewLogger(t)
	tlsinfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create cert")

	tests := []struct {
		info       TLSInfo
		clientAuth tls.ClientAuthType
		wantCAs    bool
	}{
		{
			info:       TLSInfo{CertFile: tlsinfo.CertFile, KeyFile: tlsinfo.KeyFile, Logger: ln},
			clientAuth: tls.NoClientCert,
			wantCAs:    false,
		},

		{
			info:       TLSInfo{CertFile: tlsinfo.CertFile, KeyFile: tlsinfo.KeyFile, TrustedCAFile: tlsinfo.CertFile, Logger: ln},
			clientAuth: tls.RequireAndVerifyClientCert,
			wantCAs:    true,
		},
	}

	for i, tt := range tests {
		tt.info.parseFunc = fakeCertificateParserFunc(nil)

		sCfg, err := tt.info.ServerConfig()
		if err != nil {
			t.Errorf("#%d: expected nil error from ServerConfig(), got non-nil: %v", i, err)
		}

		if tt.wantCAs != (sCfg.ClientCAs != nil) {
			t.Errorf("#%d: wantCAs=%t but ClientCAs=%v", i, tt.wantCAs, sCfg.ClientCAs)
		}

		cCfg, err := tt.info.ClientConfig()
		if err != nil {
			t.Errorf("#%d: expected nil error from ClientConfig(), got non-nil: %v", i, err)
		}

		if tt.wantCAs != (cCfg.RootCAs != nil) {
			t.Errorf("#%d: wantCAs=%t but RootCAs=%v", i, tt.wantCAs, sCfg.RootCAs)
		}
	}
}

func TestNewListenerUnixSocket(t *testing.T) {
	l, err := NewListener("testsocket", "unix", nil)
	if err != nil {
		t.Errorf("error listening on unix socket (%v)", err)
	}
	l.Close()
}

// TestNewListenerTLSInfoSelfCert tests that a new certificate accepts connections.
func TestNewListenerTLSInfoSelfCert(t *testing.T) {
	tmpdir := t.TempDir()

	tlsinfo, err := SelfCert(zaptest.NewLogger(t), tmpdir, []string{"127.0.0.1"}, 1)
	require.NoError(t, err)
	require.Falsef(t, tlsinfo.Empty(), "tlsinfo should have certs (%+v)", tlsinfo)
	testNewListenerTLSInfoAccept(t, tlsinfo)

	assert.Panicsf(t, func() {
		SelfCert(nil, tmpdir, []string{"127.0.0.1"}, 1)
	}, "expected panic with nil log")
}

func TestIsClosedConnError(t *testing.T) {
	l, err := NewListener("testsocket", "unix", nil)
	if err != nil {
		t.Errorf("error listening on unix socket (%v)", err)
	}
	l.Close()
	_, err = l.Accept()
	require.Truef(t, IsClosedConnError(err), "expect true, got false (%v)", err)
}

func TestSocktOptsEmpty(t *testing.T) {
	tests := []struct {
		sopts SocketOpts
		want  bool
	}{
		{SocketOpts{}, true},
		{SocketOpts{ReuseAddress: true, ReusePort: false}, false},
		{SocketOpts{ReusePort: true}, false},
	}

	for i, tt := range tests {
		got := tt.sopts.Empty()
		if tt.want != got {
			t.Errorf("#%d: result of Empty() incorrect: want=%t got=%t", i, tt.want, got)
		}
	}
}

// TestNewListenerWithACRLFile tests when a revocation list is present.
func TestNewListenerWithACRLFile(t *testing.T) {
	clientTLSInfo, err := createSelfCertEx(t, "127.0.0.1", x509.ExtKeyUsageClientAuth)
	require.NoErrorf(t, err, "unable to create client cert")

	loadFileAsPEM := func(fileName string) []byte {
		loaded, readErr := os.ReadFile(fileName)
		require.NoErrorf(t, readErr, "unable to read file %q", fileName)
		block, _ := pem.Decode(loaded)
		return block.Bytes
	}

	clientCert, err := x509.ParseCertificate(loadFileAsPEM(clientTLSInfo.CertFile))
	require.NoErrorf(t, err, "unable to parse client cert")

	tests := map[string]struct {
		expectHandshakeError      bool
		revokedCertificateEntries []x509.RevocationListEntry
		revocationListContents    []byte
	}{
		"empty revocation list": {
			expectHandshakeError: false,
		},
		"client cert is revoked": {
			expectHandshakeError: true,
			revokedCertificateEntries: []x509.RevocationListEntry{
				{
					SerialNumber:   clientCert.SerialNumber,
					RevocationTime: time.Now(),
				},
			},
		},
		"invalid CRL file content": {
			expectHandshakeError:   true,
			revocationListContents: []byte("@invalidcontent"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			tmpdir := t.TempDir()
			tlsInfo, err := createSelfCert(t)
			require.NoErrorf(t, err, "unable to create server cert")
			tlsInfo.TrustedCAFile = clientTLSInfo.CertFile
			tlsInfo.CRLFile = filepath.Join(tmpdir, "revoked.r0")

			cert, err := x509.ParseCertificate(loadFileAsPEM(tlsInfo.CertFile))
			require.NoErrorf(t, err, "unable to decode server cert")

			key, err := x509.ParseECPrivateKey(loadFileAsPEM(tlsInfo.KeyFile))
			require.NoErrorf(t, err, "unable to parse server key")

			revocationListContents := test.revocationListContents
			if len(revocationListContents) == 0 {
				tmpl := &x509.RevocationList{
					RevokedCertificateEntries: test.revokedCertificateEntries,
					ThisUpdate:                time.Now(),
					NextUpdate:                time.Now().Add(time.Hour),
					Number:                    big.NewInt(1),
				}
				revocationListContents, err = x509.CreateRevocationList(rand.Reader, tmpl, cert, key)
				require.NoErrorf(t, err, "unable to create revocation list")
			}

			err = os.WriteFile(tlsInfo.CRLFile, revocationListContents, 0o600)
			require.NoErrorf(t, err, "unable to write revocation list")

			chHandshakeFailure := make(chan error, 1)
			tlsInfo.HandshakeFailure = func(_ *tls.Conn, err error) {
				if err != nil {
					chHandshakeFailure <- err
				}
			}

			rootCAs := x509.NewCertPool()
			rootCAs.AddCert(cert)

			clientCert, err := tls.LoadX509KeyPair(clientTLSInfo.CertFile, clientTLSInfo.KeyFile)
			require.NoErrorf(t, err, "unable to create peer cert")

			ln, err := NewListener("127.0.0.1:0", "https", tlsInfo)
			require.NoErrorf(t, err, "unable to start listener")

			tlsConfig := &tls.Config{}
			tlsConfig.InsecureSkipVerify = false
			tlsConfig.Certificates = []tls.Certificate{clientCert}
			tlsConfig.RootCAs = rootCAs

			tr := &http.Transport{TLSClientConfig: tlsConfig}
			cli := &http.Client{Transport: tr, Timeout: 5 * time.Second}
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				if _, gerr := cli.Get("https://" + ln.Addr().String()); gerr != nil {
					t.Logf("http GET failed: %v", gerr)
				}
			}()

			chAcceptConn := make(chan net.Conn, 1)
			go func() {
				defer wg.Done()
				conn, err := ln.Accept()
				if err == nil {
					chAcceptConn <- conn
				}
			}()

			timer := time.NewTimer(5 * time.Second)
			defer func() {
				if !timer.Stop() {
					<-timer.C
				}
			}()

			select {
			case err := <-chHandshakeFailure:
				if !test.expectHandshakeError {
					t.Errorf("expecting no handshake error, got: %v", err)
				}
			case conn := <-chAcceptConn:
				if test.expectHandshakeError {
					t.Errorf("expecting handshake error, got nothing")
				}
				conn.Close()
			case <-timer.C:
				t.Error("timed out waiting for closed connection or handshake error")
			}

			ln.Close()
			wg.Wait()
		})
	}
}

// TestServerConfig_ReloadTrustedCA_WithAllowedCN tests that when both ReloadTrustedCA
// and AllowedCN are configured, client connections are properly verified.
// This is a regression test for the bug where VerifyPeerCertificate from baseConfig()
// would fail because verifiedChains is nil with RequireAnyClientCert.
func TestServerConfig_ReloadTrustedCA_WithAllowedCN(t *testing.T) {
	// Generate a test CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{Organization: []string{"Test CA"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(caCertDER)
	require.NoError(t, err)

	// Helper to generate a certificate with a specific CN
	generateCertWithCN := func(cn string, isServer bool) (certPEM, keyPEM []byte) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		require.NoError(t, err)

		template := &x509.Certificate{
			SerialNumber:          serial,
			Subject:               pkix.Name{CommonName: cn},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(time.Hour),
			IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
			DNSNames:              []string{"localhost"},
			BasicConstraintsValid: true,
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

		certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

		return certPEM, keyPEM
	}

	// Write CA to temp file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	require.NoError(t, os.WriteFile(caFile, caPEM, 0600))

	// Generate server cert
	serverCertPEM, serverKeyPEM := generateCertWithCN("server", true)
	serverCertFile := filepath.Join(tmpDir, "server.crt")
	serverKeyFile := filepath.Join(tmpDir, "server.key")
	require.NoError(t, os.WriteFile(serverCertFile, serverCertPEM, 0600))
	require.NoError(t, os.WriteFile(serverKeyFile, serverKeyPEM, 0600))

	// Generate client cert with valid CN
	validClientCertPEM, validClientKeyPEM := generateCertWithCN("valid-client", false)
	validClientCertFile := filepath.Join(tmpDir, "valid-client.crt")
	validClientKeyFile := filepath.Join(tmpDir, "valid-client.key")
	require.NoError(t, os.WriteFile(validClientCertFile, validClientCertPEM, 0600))
	require.NoError(t, os.WriteFile(validClientKeyFile, validClientKeyPEM, 0600))

	// Generate client cert with wrong CN
	wrongClientCertPEM, wrongClientKeyPEM := generateCertWithCN("wrong-client", false)
	wrongClientCertFile := filepath.Join(tmpDir, "wrong-client.crt")
	wrongClientKeyFile := filepath.Join(tmpDir, "wrong-client.key")
	require.NoError(t, os.WriteFile(wrongClientCertFile, wrongClientCertPEM, 0600))
	require.NoError(t, os.WriteFile(wrongClientKeyFile, wrongClientKeyPEM, 0600))

	tests := []struct {
		name           string
		clientCertFile string
		clientKeyFile  string
		expectAccept   bool
	}{
		{
			name:           "valid CN should be accepted",
			clientCertFile: validClientCertFile,
			clientKeyFile:  validClientKeyFile,
			expectAccept:   true,
		},
		{
			name:           "wrong CN should be rejected",
			clientCertFile: wrongClientCertFile,
			clientKeyFile:  wrongClientKeyFile,
			expectAccept:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create CAReloader
			caReloader, err := tlsutil.NewCAReloader([]string{caFile}, zaptest.NewLogger(t))
			require.NoError(t, err)
			caReloader.WithInterval(100 * time.Millisecond)
			caReloader.Start()
			t.Cleanup(func() { caReloader.Stop() })

			// Create server TLSInfo with CAReloader and AllowedCN
			tlsInfo := TLSInfo{
				CertFile:       serverCertFile,
				KeyFile:        serverKeyFile,
				TrustedCAFile:  caFile,
				ClientCertAuth: true,
				CAReloader:     caReloader,
				AllowedCN:      "valid-client",
				Logger:         zaptest.NewLogger(t),
			}

			ln, err := NewListener("127.0.0.1:0", "https", &tlsInfo)
			require.NoError(t, err)
			t.Cleanup(func() { ln.Close() })

			// Setup client TLS config
			clientCert, err := tls.LoadX509KeyPair(tt.clientCertFile, tt.clientKeyFile)
			require.NoError(t, err)

			rootCAs := x509.NewCertPool()
			rootCAs.AddCert(caCert)

			clientTLSConfig := &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      rootCAs,
				ServerName:   "127.0.0.1",
			}

			// Try to connect
			chConnErr := make(chan error, 1)
			go func() {
				conn, err := tls.Dial("tcp", ln.Addr().String(), clientTLSConfig)
				if err != nil {
					chConnErr <- err
					return
				}
				// Try handshake explicitly
				err = conn.Handshake()
				conn.Close()
				chConnErr <- err
			}()

			chAcceptErr := make(chan error, 1)
			chAcceptConn := make(chan net.Conn, 1)
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					chAcceptErr <- err
					return
				}
				// Force handshake on server side
				tlsConn := conn.(*tls.Conn)
				if err := tlsConn.Handshake(); err != nil {
					conn.Close()
					chAcceptErr <- err
					return
				}
				chAcceptConn <- conn
			}()

			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()

			select {
			case conn := <-chAcceptConn:
				conn.Close()
				if !tt.expectAccept {
					t.Error("expected connection to be rejected, but it was accepted")
				}
			case err := <-chAcceptErr:
				if tt.expectAccept {
					t.Errorf("expected connection to be accepted, but got error: %v", err)
				}
			case err := <-chConnErr:
				if tt.expectAccept && err != nil {
					t.Errorf("expected connection to succeed, but client got error: %v", err)
				}
				// If we don't expect accept and client got error, that's fine
			case <-timer.C:
				t.Error("timed out waiting for connection result")
			}
		})
	}
}

// TestServerConfig_ReloadTrustedCA_WithAllowedCN_DynamicReload tests that dynamic
// CA reloading works correctly when AllowedCN is also configured. This verifies
// that after the CA file is updated, new connections are verified against the
// updated CA pool while still enforcing the AllowedCN constraint.
func TestServerConfig_ReloadTrustedCA_WithAllowedCN_DynamicReload(t *testing.T) {
	tmpDir := t.TempDir()

	// Helper to generate a CA
	generateCA := func(name string) (*ecdsa.PrivateKey, *x509.Certificate, []byte) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		require.NoError(t, err)

		template := &x509.Certificate{
			SerialNumber:          serial,
			Subject:               pkix.Name{CommonName: name},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			BasicConstraintsValid: true,
		}

		certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		require.NoError(t, err)

		cert, err := x509.ParseCertificate(certDER)
		require.NoError(t, err)

		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		return key, cert, certPEM
	}

	// Helper to generate a client cert signed by a CA with a specific CN
	generateClientCert := func(caKey *ecdsa.PrivateKey, caCert *x509.Certificate, cn string) (certFile, keyFile string) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		require.NoError(t, err)

		template := &x509.Certificate{
			SerialNumber:          serial,
			Subject:               pkix.Name{CommonName: cn},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
			BasicConstraintsValid: true,
		}

		certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		require.NoError(t, err)

		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

		certPath := filepath.Join(tmpDir, cn+"-"+caCert.Subject.CommonName+".crt")
		keyPath := filepath.Join(tmpDir, cn+"-"+caCert.Subject.CommonName+".key")
		require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
		require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))

		return certPath, keyPath
	}

	// Helper to attempt a TLS connection - returns error if either client or server handshake fails
	tryConnect := func(ln net.Listener, clientCertFile, clientKeyFile string, rootCAs *x509.CertPool) error {
		clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		require.NoError(t, err)

		clientTLSConfig := &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      rootCAs,
			ServerName:   "127.0.0.1",
		}

		chClientErr := make(chan error, 1)
		chServerErr := make(chan error, 1)

		// Client goroutine
		go func() {
			conn, err := tls.Dial("tcp", ln.Addr().String(), clientTLSConfig)
			if err != nil {
				chClientErr <- err
				return
			}
			conn.Close()
			chClientErr <- nil
		}()

		// Server goroutine - capture handshake error
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				chServerErr <- err
				return
			}
			tlsConn := conn.(*tls.Conn)
			err = tlsConn.Handshake()
			conn.Close()
			chServerErr <- err
		}()

		// Wait for both client and server to complete
		var clientErr, serverErr error
		for i := 0; i < 2; i++ {
			select {
			case clientErr = <-chClientErr:
			case serverErr = <-chServerErr:
			case <-time.After(5 * time.Second):
				return errors.New("connection timed out")
			}
		}

		// Return either error (server rejection should trigger error on both sides)
		if serverErr != nil {
			return serverErr
		}
		return clientErr
	}

	// Generate CA1 and CA2
	ca1Key, ca1Cert, ca1PEM := generateCA("CA1")
	ca2Key, ca2Cert, ca2PEM := generateCA("CA2")

	// Generate server cert (signed by CA1, will be used throughout)
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serverSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	serverTemplate := &x509.Certificate{
		SerialNumber:          serverSerial,
		Subject:               pkix.Name{CommonName: "server"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, ca1Cert, &serverKey.PublicKey, ca1Key)
	require.NoError(t, err)
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	serverKeyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	require.NoError(t, err)
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: serverKeyDER})

	serverCertFile := filepath.Join(tmpDir, "server.crt")
	serverKeyFile := filepath.Join(tmpDir, "server.key")
	require.NoError(t, os.WriteFile(serverCertFile, serverCertPEM, 0600))
	require.NoError(t, os.WriteFile(serverKeyFile, serverKeyPEM, 0600))

	// Generate client certs
	ca1ValidCertFile, ca1ValidKeyFile := generateClientCert(ca1Key, ca1Cert, "valid-client")
	ca2ValidCertFile, ca2ValidKeyFile := generateClientCert(ca2Key, ca2Cert, "valid-client")

	// Write initial CA file (CA1 only)
	caFile := filepath.Join(tmpDir, "ca.crt")
	require.NoError(t, os.WriteFile(caFile, ca1PEM, 0600))

	// Create CAReloader
	caReloader, err := tlsutil.NewCAReloader([]string{caFile}, zaptest.NewLogger(t))
	require.NoError(t, err)
	caReloader.WithInterval(50 * time.Millisecond) // Fast reload for testing
	caReloader.Start()
	t.Cleanup(func() { caReloader.Stop() })

	// Create server TLSInfo with CAReloader and AllowedCN
	tlsInfo := TLSInfo{
		CertFile:       serverCertFile,
		KeyFile:        serverKeyFile,
		TrustedCAFile:  caFile,
		ClientCertAuth: true,
		CAReloader:     caReloader,
		AllowedCN:      "valid-client",
		Logger:         zaptest.NewLogger(t),
	}

	ln, err := NewListener("127.0.0.1:0", "https", &tlsInfo)
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })

	// Client needs to trust CA1 initially (for server cert verification)
	clientRootCAs := x509.NewCertPool()
	clientRootCAs.AddCert(ca1Cert)

	// Phase 1: Connect with CA1-signed client cert - should succeed
	err = tryConnect(ln, ca1ValidCertFile, ca1ValidKeyFile, clientRootCAs)
	require.NoError(t, err, "Phase 1: CA1-signed client with valid CN should be accepted")
	t.Log("Phase 1: CA1-signed client accepted")

	// Phase 2: Update CA file to include both CA1 and CA2
	combinedCAPEM := append(ca1PEM, ca2PEM...)
	require.NoError(t, os.WriteFile(caFile, combinedCAPEM, 0600))

	// Wait for CA reload (need to wait longer than reload interval to ensure reload happens)
	time.Sleep(150 * time.Millisecond)

	// Phase 2: Connect with CA2-signed client cert - should now succeed
	err = tryConnect(ln, ca2ValidCertFile, ca2ValidKeyFile, clientRootCAs)
	require.NoError(t, err, "Phase 2: CA2-signed client with valid CN should be accepted after reload")
	t.Log("Phase 2: CA2-signed client accepted after CA file updated to [CA1+CA2]")

	// Phase 3: Update CA file to CA2 only (remove CA1)
	require.NoError(t, os.WriteFile(caFile, ca2PEM, 0600))
	t.Log("Phase 3: CA file updated, waiting for reload...")

	// Wait for CA reload - use 3x the reload interval to be safe
	time.Sleep(200 * time.Millisecond)

	// Phase 3: CA1-signed client should now be rejected
	// This is the critical test for the fix - verifying that dynamic CA reload
	// works correctly with AllowedCN configured
	err = tryConnect(ln, ca1ValidCertFile, ca1ValidKeyFile, clientRootCAs)
	require.Error(t, err, "Phase 3: CA1-signed client should be rejected after CA1 removed from trust bundle")
	t.Log("Phase 3: CA1-signed client correctly rejected - dynamic CA reload with AllowedCN works!")
}
