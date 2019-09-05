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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

func createSelfCert(hosts ...string) (*TLSInfo, func(), error) {
	return createSelfCertEx("127.0.0.1")
}

func createSelfCertEx(host string, additionalUsages ...x509.ExtKeyUsage) (*TLSInfo, func(), error) {
	d, terr := ioutil.TempDir("", "etcd-test-tls-")
	if terr != nil {
		return nil, nil, terr
	}
	info, err := SelfCert(zap.NewExample(), d, []string{host + ":0"}, additionalUsages...)
	if err != nil {
		return nil, nil, err
	}
	return &info, func() { os.RemoveAll(d) }, nil
}

func fakeCertificateParserFunc(cert tls.Certificate, err error) func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
	return func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
		return cert, err
	}
}

// TestNewListenerTLSInfo tests that NewListener with valid TLSInfo returns
// a TLS listener that accepts TLS connections.
func TestNewListenerTLSInfo(t *testing.T) {
	tlsInfo, del, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del()
	testNewListenerTLSInfoAccept(t, *tlsInfo)
}

func testNewListenerTLSInfoAccept(t *testing.T, tlsInfo TLSInfo) {
	ln, err := NewListener("127.0.0.1:0", "https", &tlsInfo)
	if err != nil {
		t.Fatalf("unexpected NewListener error: %v", err)
	}
	defer ln.Close()

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	cli := &http.Client{Transport: tr}
	go cli.Get("https://" + ln.Addr().String())

	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("unexpected Accept error: %v", err)
	}
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
	tlsInfo, del, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del()

	host := "127.0.0.222"
	if goodClientHost {
		host = "127.0.0.1"
	}
	clientTLSInfo, del2, err := createSelfCertEx(host, x509.ExtKeyUsageClientAuth)
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del2()

	tlsInfo.SkipClientSANVerify = skipClientSANVerify
	tlsInfo.TrustedCAFile = clientTLSInfo.CertFile

	rootCAs := x509.NewCertPool()
	loaded, err := ioutil.ReadFile(tlsInfo.CertFile)
	if err != nil {
		t.Fatalf("unexpected missing certfile: %v", err)
	}
	rootCAs.AppendCertsFromPEM(loaded)

	clientCert, err := tls.LoadX509KeyPair(clientTLSInfo.CertFile, clientTLSInfo.KeyFile)
	if err != nil {
		t.Fatalf("unable to create peer cert: %v", err)
	}

	tlsConfig := &tls.Config{}
	tlsConfig.InsecureSkipVerify = false
	tlsConfig.Certificates = []tls.Certificate{clientCert}
	tlsConfig.RootCAs = rootCAs

	ln, err := NewListener("127.0.0.1:0", "https", tlsInfo)
	if err != nil {
		t.Fatalf("unexpected NewListener error: %v", err)
	}
	defer ln.Close()

	tr := &http.Transport{TLSClientConfig: tlsConfig}
	cli := &http.Client{Transport: tr}
	chClientErr := make(chan error)
	go func() {
		_, err := cli.Get("https://" + ln.Addr().String())
		chClientErr <- err
	}()

	chAcceptErr := make(chan error)
	chAcceptConn := make(chan net.Conn)
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
	tlsinfo, del, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del()

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
		tt.parseFunc = fakeCertificateParserFunc(tls.Certificate{}, nil)
		trans, err := NewTransport(tt, time.Second)
		if err != nil {
			t.Fatalf("Received unexpected error from NewTransport: %v", err)
		}

		if trans.TLSClientConfig == nil {
			t.Fatalf("#%d: want non-nil TLSClientConfig", i)
		}
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
	tlsinfo, del, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del()

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
	tlsinfo, del, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del()

	tlsinfo.parseFunc = fakeCertificateParserFunc(tls.Certificate{}, errors.New("fake"))

	if _, err = tlsinfo.ServerConfig(); err == nil {
		t.Errorf("expected non-nil error from ServerConfig()")
	}

	if _, err = tlsinfo.ClientConfig(); err == nil {
		t.Errorf("expected non-nil error from ClientConfig()")
	}
}

func TestTLSInfoConfigFuncs(t *testing.T) {
	tlsinfo, del, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer del()

	tests := []struct {
		info       TLSInfo
		clientAuth tls.ClientAuthType
		wantCAs    bool
	}{
		{
			info:       TLSInfo{CertFile: tlsinfo.CertFile, KeyFile: tlsinfo.KeyFile},
			clientAuth: tls.NoClientCert,
			wantCAs:    false,
		},

		{
			info:       TLSInfo{CertFile: tlsinfo.CertFile, KeyFile: tlsinfo.KeyFile, TrustedCAFile: tlsinfo.CertFile},
			clientAuth: tls.RequireAndVerifyClientCert,
			wantCAs:    true,
		},
	}

	for i, tt := range tests {
		tt.info.parseFunc = fakeCertificateParserFunc(tls.Certificate{}, nil)

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
	tmpdir, err := ioutil.TempDir(os.TempDir(), "tlsdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	tlsinfo, err := SelfCert(zap.NewExample(), tmpdir, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if tlsinfo.Empty() {
		t.Fatalf("tlsinfo should have certs (%+v)", tlsinfo)
	}
	testNewListenerTLSInfoAccept(t, tlsinfo)
}

func TestIsClosedConnError(t *testing.T) {
	l, err := NewListener("testsocket", "unix", nil)
	if err != nil {
		t.Errorf("error listening on unix socket (%v)", err)
	}
	l.Close()
	_, err = l.Accept()
	if !IsClosedConnError(err) {
		t.Fatalf("expect true, got false (%v)", err)
	}
}
