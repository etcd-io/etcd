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

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/transport"
)

/* Helper functions */
type dummyServerHandler struct {
	t      *testing.T
	output chan<- []byte
}

// reads the request body and write back to the response object
func (sh *dummyServerHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	resp.WriteHeader(200)

	if data, err := io.ReadAll(req.Body); err != nil {
		sh.t.Fatal(err)
	} else {
		sh.output <- data
	}
}

func prepare(t *testing.T, serverIsClosed bool) (chan []byte, chan struct{}, chan []byte, Server, *http.Server, func(data []byte)) {
	lg := zaptest.NewLogger(t)
	scheme := "tcp"
	L7Scheme := "http"

	// we always send the traffic to destination with HTTPS
	// this will force the CONNECT header to be sent first
	tlsInfo := createTLSInfo(lg)

	ln1, ln2 := listen(t, "tcp", "localhost:0", transport.TLSInfo{}), listen(t, "tcp", "localhost:0", transport.TLSInfo{})
	forwardProxyAddr, dstAddr := ln1.Addr().String(), ln2.Addr().String()
	ln1.Close()
	ln2.Close()

	recvc := make(chan []byte, 1)
	httpServer := &http.Server{
		Handler: &dummyServerHandler{
			t:      t,
			output: recvc,
		},
	}
	go startHTTPServer(scheme, dstAddr, tlsInfo, httpServer)

	// we connect to the proxy without TLS
	proxyURL := url.URL{Scheme: L7Scheme, Host: forwardProxyAddr}
	cfg := ServerConfig{
		Logger: lg,
		Listen: proxyURL,
	}
	p := NewServer(cfg)
	waitForServer(t, p)

	// setup forward proxy
	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", proxyURL.String())
	t.Logf("Proxy URL %s", proxyURL.String())

	donec, writec := make(chan struct{}), make(chan []byte)

	var tp *http.Transport
	var err error
	if !tlsInfo.Empty() {
		tp, err = transport.NewTransport(tlsInfo, 1*time.Second)
	} else {
		tp, err = transport.NewTransport(tlsInfo, 1*time.Second)
	}
	if err != nil {
		t.Fatal(err)
	}
	tp.IdleConnTimeout = 100 * time.Microsecond

	sendData := func(data []byte) {
		send(tp, t, data, scheme, dstAddr, tlsInfo, serverIsClosed)
	}

	return recvc, donec, writec, p, httpServer, sendData
}

func destroy(t *testing.T, writec chan []byte, donec chan struct{}, p Server, serverIsClosed bool, httpServer *http.Server) {
	close(writec)
	if err := httpServer.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	select {
	case <-donec:
	case <-time.After(3 * time.Second):
		t.Fatal("took too long to write")
	}

	if !serverIsClosed {
		select {
		case <-p.Done():
			t.Fatal("unexpected done")
		case err := <-p.Error():
			t.Fatal(err)
		default:
		}

		if err := p.Close(); err != nil {
			t.Fatal(err)
		}

		select {
		case <-p.Done():
		case err := <-p.Error():
			if !strings.HasPrefix(err.Error(), "accept ") &&
				!strings.HasSuffix(err.Error(), "use of closed network connection") {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("took too long to close")
		}
	}
}

func createTLSInfo(lg *zap.Logger) transport.TLSInfo {
	return transport.TLSInfo{
		KeyFile:        "../../tests/fixtures/server.key.insecure",
		CertFile:       "../../tests/fixtures/server.crt",
		TrustedCAFile:  "../../tests/fixtures/ca.crt",
		ClientCertAuth: true,
		Logger:         lg,
	}
}

func listen(t *testing.T, scheme, addr string, tlsInfo transport.TLSInfo) (ln net.Listener) {
	var err error
	if !tlsInfo.Empty() {
		ln, err = transport.NewListener(addr, scheme, &tlsInfo)
	} else {
		ln, err = net.Listen(scheme, addr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func startHTTPServer(scheme, addr string, tlsInfo transport.TLSInfo, httpServer *http.Server) {
	var err error
	var ln net.Listener

	ln, err = net.Listen(scheme, addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("HTTP Server started on", addr)
	if err := httpServer.ServeTLS(ln, tlsInfo.CertFile, tlsInfo.KeyFile); err != http.ErrServerClosed {
		// always returns error. ErrServerClosed on graceful close
		log.Fatalf(fmt.Sprintf("startHTTPServer ServeTLS(): %v", err))
	}
}

func send(tp *http.Transport, t *testing.T, data []byte, scheme, addr string, tlsInfo transport.TLSInfo, serverIsClosed bool) {
	defer func() {
		tp.CloseIdleConnections()
	}()

	// If you call Dial(), you will get a Conn that you can write the byte stream directly
	// If you call RoundTrip(), you will get a connection managed for you, but you need to send valid HTTP request
	dataReader := bytes.NewReader(data)
	protocolScheme := scheme
	if scheme == "tcp" {
		if !tlsInfo.Empty() {
			protocolScheme = "https"
		} else {
			panic("only https is supported")
		}
	} else {
		panic("scheme not supported")
	}
	rawURL := url.URL{
		Scheme: protocolScheme,
		Host:   addr,
	}

	req, err := http.NewRequest("POST", rawURL.String(), dataReader)
	if err != nil {
		t.Fatal(err)
	}
	res, err := tp.RoundTrip(req)
	if err != nil {
		if strings.Contains(err.Error(), "TLS handshake timeout") {
			t.Logf("TLS handshake timeout")
			return
		}
		if serverIsClosed {
			// when the proxy server is closed before sending, we will get this error message
			if strings.Contains(err.Error(), "connect: connection refused") {
				t.Logf("connect: connection refused")
				return
			}
		}
		panic(err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	if res.StatusCode != 200 {
		t.Fatalf("status code not 200")
	}
}

// Waits until a proxy is ready to serve.
// Aborts test on proxy start-up error.
func waitForServer(t *testing.T, s Server) {
	select {
	case <-s.Ready():
	case err := <-s.Error():
		t.Fatal(err)
	}
}

/* Unit tests */
func TestServer_TCP(t *testing.T)         { testServer(t, false) }
func TestServer_TCP_DelayTx(t *testing.T) { testServer(t, true) }
func testServer(t *testing.T, delayTx bool) {
	recvc, donec, writec, p, httpServer, sendData := prepare(t, false)
	defer destroy(t, writec, donec, p, false, httpServer)
	go func() {
		defer close(donec)
		for data := range writec {
			sendData(data)
		}
	}()

	data1 := []byte("Hello World!")
	writec <- data1
	now := time.Now()
	if d := <-recvc; !bytes.Equal(data1, d) {
		t.Fatalf("expected %q, got %q", string(data1), string(d))
	}
	took1 := time.Since(now)
	t.Logf("took %v with no latency", took1)

	lat, rv := 50*time.Millisecond, 5*time.Millisecond
	if delayTx {
		p.DelayTx(lat, rv)
	}

	data2 := []byte("new data")
	writec <- data2
	now = time.Now()
	if d := <-recvc; !bytes.Equal(data2, d) {
		t.Fatalf("expected %q, got %q", string(data2), string(d))
	}
	took2 := time.Since(now)
	if delayTx {
		t.Logf("took %v with latency %v+-%v", took2, lat, rv)
	} else {
		t.Logf("took %v with no latency", took2)
	}

	if delayTx {
		p.UndelayTx()
		if took2 < lat-rv {
			close(writec)
			t.Fatalf("expected took2 %v (with latency) > delay: %v", took2, lat-rv)
		}
	}
}

func TestServer_DelayAccept(t *testing.T) {
	recvc, donec, writec, p, httpServer, sendData := prepare(t, false)
	defer destroy(t, writec, donec, p, false, httpServer)
	go func() {
		defer close(donec)
		for data := range writec {
			sendData(data)
		}
	}()

	data := []byte("Hello World!")
	now := time.Now()
	writec <- data
	if d := <-recvc; !bytes.Equal(data, d) {
		t.Fatalf("expected %q, got %q", string(data), string(d))
	}
	took1 := time.Since(now)
	t.Logf("took %v with no latency", took1)
	time.Sleep(1 * time.Second) // wait for the idle connection to timeout

	lat, rv := 700*time.Millisecond, 10*time.Millisecond
	p.DelayAccept(lat, rv)
	defer p.UndelayAccept()

	now = time.Now()
	writec <- data
	if d := <-recvc; !bytes.Equal(data, d) {
		t.Fatalf("expected %q, got %q", string(data), string(d))
	}
	took2 := time.Since(now)
	t.Logf("took %v with latency %v±%v", took2, lat, rv)

	if took1 >= took2 {
		t.Fatalf("expected took1 %v < took2 %v", took1, took2)
	}
}

func TestServer_BlackholeTx(t *testing.T) {
	recvc, donec, writec, p, httpServer, sendData := prepare(t, false)
	defer destroy(t, writec, donec, p, false, httpServer)
	// the sendData function must be in a goroutine
	// otherwise, the pauseTx will cause the sendData to block
	go func() {
		defer close(donec)
		for data := range writec {
			sendData(data)
		}
	}()

	// before enabling blacklhole
	data := []byte("Hello World!")
	writec <- data
	if d := <-recvc; !bytes.Equal(data, d) {
		t.Fatalf("expected %q, got %q", string(data), string(d))
	}

	// enable blackhole
	// note that the transport is set to use 10s for TLSHandshakeTimeout, so
	// this test will require at least 10s to execute, since send() is a
	// blocking call thus we need to wait for ssl handshake to timeout
	p.BlackholeTx()

	writec <- data
	select {
	case d := <-recvc:
		t.Fatalf("unexpected data receive %q during blackhole", string(d))
	case <-time.After(200 * time.Millisecond):
	}

	p.UnblackholeTx()

	// disable blackhole
	// TODO: figure out why HTTPS won't attempt to reconnect when the blackhole is disabled

	// expect different data, old data dropped
	data[0]++
	writec <- data
	select {
	case d := <-recvc:
		if !bytes.Equal(data, d) {
			t.Fatalf("expected %q, got %q", string(data), string(d))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("took too long to receive after unblackhole")
	}
}

func TestServer_Shutdown(t *testing.T) {
	recvc, donec, writec, p, httpServer, sendData := prepare(t, true)
	defer destroy(t, writec, donec, p, true, httpServer)
	go func() {
		defer close(donec)
		for data := range writec {
			sendData(data)
		}
	}()

	s, _ := p.(*server)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	p = nil
	time.Sleep(200 * time.Millisecond)

	data := []byte("Hello World!")
	sendData(data)

	select {
	case d := <-recvc:
		if bytes.Equal(data, d) {
			t.Fatalf("expected nothing, got %q", string(d))
		}
	case <-time.After(2 * time.Second):
		t.Log("nothing was received, proxy server seems to be closed so no traffic is forwarded")
	}
}
