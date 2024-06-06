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

func (sh *dummyServerHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	log.Println("ServeHTTP")

	defer req.Body.Close()
	resp.WriteHeader(200)

	if data, err := io.ReadAll(req.Body); err != nil {
		sh.t.Fatal(err)
	} else {
		sh.output <- data
	}
}

func prepare(t *testing.T) (chan []byte, chan struct{}, chan []byte, Server, *http.Server) {
	lg := zaptest.NewLogger(t)
	scheme := "tcp"
	L7Scheme := "http"

	// we always send the traffic to destination with HTTPS, allowing us to just
	// handle byte streams.
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
	log.Println("Proxy URL", proxyURL.String())

	donec, writec := make(chan struct{}), make(chan []byte)

	go func() {
		defer close(donec)
		for data := range writec {
			t.Logf("send %s", data)
			send(t, data, scheme, dstAddr, tlsInfo)
		}
	}()

	return recvc, donec, writec, p, httpServer
}

func destroy(t *testing.T, writec chan []byte, donec chan struct{}, p Server, httpServer *http.Server) {
	close(writec)
	if err := httpServer.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	select {
	case <-donec:
	case <-time.After(3 * time.Second):
		t.Fatal("took too long to write")
	}

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

func send(t *testing.T, data []byte, scheme, addr string, tlsInfo transport.TLSInfo) {
	var err error
	var tp *http.Transport
	if !tlsInfo.Empty() {
		tp, err = transport.NewTransport(tlsInfo, 3*time.Second)
	} else {
		tp, err = transport.NewTransport(tlsInfo, 3*time.Second)
	}
	if err != nil {
		t.Fatal(err)
	}
	tp.IdleConnTimeout = 1 * time.Second // without this, the test will hang for a while, because RoundTripper is designed to reuse the underlying connection!

	// If you do Dial(), you will get a Conn that you can write the byte stream directly
	// If you do RoundTrip(), you will get a connection managed for you, but you need to send valid HTTP request
	dataReader := bytes.NewReader(data)
	protocolScheme := scheme
	if scheme == "tcp" {
		protocolScheme = "http"
		if !tlsInfo.Empty() {
			protocolScheme = "https"
		}
	} else if scheme == "unix" {
		protocolScheme = scheme
	} else {
		panic("Scheme not supported")
	}
	rawURL := url.URL{
		Scheme: protocolScheme,
		Host:   addr,
	}

	log.Println("Before POST", rawURL.String())
	req, err := http.NewRequest("POST", rawURL.String(), dataReader)
	if err != nil {
		t.Fatal(err)
	}
	res, err := tp.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	log.Println("After POST")

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

func TestServer_TCP_Secure(t *testing.T)         { testServer(t, false) }
func TestServer_TCP_Secure_DelayTx(t *testing.T) { testServer(t, true) }

func testServer(t *testing.T, delayTx bool) {
	recvc, donec, writec, p, httpServer := prepare(t)
	defer destroy(t, writec, donec, p, httpServer)

	data1 := []byte("Hello World!")
	writec <- data1
	now := time.Now()
	if d := <-recvc; !bytes.Equal(data1, d) {
		close(writec)
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
		close(writec)
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

// func TestServer_Unix_Insecure_DelayAccept(t *testing.T) { testServerDelayAccept(t, false) }
// func TestServer_Unix_Secure_DelayAccept(t *testing.T)   { testServerDelayAccept(t, true) }
// func testServerDelayAccept(t *testing.T, secure bool) {
// 	recvc, donec, writec, p, httpServer := prepare(t)
// 	defer (p).Close()
// 	defer httpServer.Shutdown(context.Background())

// 	data := []byte("Hello World!")
// 	now := time.Now()
// 	writec <- data
// 	if d := <-recvc; !bytes.Equal(data, d) {
// 		close(writec)
// 		t.Fatalf("expected %q, got %q", string(data), string(d))
// 	}
// 	took1 := time.Since(now)
// 	t.Logf("took %v with no latency", took1)

// 	lat, rv := 700*time.Millisecond, 10*time.Millisecond
// 	p.DelayAccept(lat, rv)
// 	defer p.UndelayAccept()

// 	now = time.Now()
// 	writec <- data
// 	if d := <-recvc; !bytes.Equal(data, d) {
// 		close(writec)
// 		t.Fatalf("expected %q, got %q", string(data), string(d))
// 	}
// 	took2 := time.Since(now)
// 	t.Logf("took %v with latency %v±%v", took2, lat, rv)

// 	if took1 >= took2 {
// 		t.Fatalf("expected took1 %v < took2 %v", took1, took2)
// 	}

// 	close(writec)
// }

// func TestServer_PauseTx(t *testing.T) {
// 	lg := zaptest.NewLogger(t)
// 	scheme := "unix"
// 	forwardProxyAddr, dstAddr := newUnixAddr(), newUnixAddr()
// 	defer func() {
// 		os.RemoveAll(forwardProxyAddr)
// 		os.RemoveAll(dstAddr)
// 	}()
// 	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
// 	defer ln.Close()

// 	// setup forward proxy
// 	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", forwardProxyAddr)

// 	p := NewServer(ServerConfig{
// 		Logger: lg,
// 		Listen: url.URL{Scheme: scheme, Host: forwardProxyAddr},
// 	})

// 	waitForServer(t, p)

// 	defer p.Close()

// 	p.PauseTx()

// 	data := []byte("Hello World!")
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})

// 	recvc := make(chan []byte, 1)
// 	go func() {
// 		recvc <- receive(t, ln)
// 	}()

// 	select {
// 	case d := <-recvc:
// 		t.Fatalf("received unexpected data %q during pause", string(d))
// 	case <-time.After(200 * time.Millisecond):
// 	}

// 	p.UnpauseTx()

// 	select {
// 	case d := <-recvc:
// 		if !bytes.Equal(data, d) {
// 			t.Fatalf("expected %q, got %q", string(data), string(d))
// 		}
// 	case <-time.After(2 * time.Second):
// 		t.Fatal("took too long to receive after unpause")
// 	}
// }

// func TestServer_ModifyTx_corrupt(t *testing.T) {
// 	lg := zaptest.NewLogger(t)
// 	scheme := "unix"
// 	forwardProxyAddr, dstAddr := newUnixAddr(), newUnixAddr()
// 	defer func() {
// 		os.RemoveAll(forwardProxyAddr)
// 		os.RemoveAll(dstAddr)
// 	}()
// 	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
// 	defer ln.Close()

// 	// setup forward proxy
// 	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", forwardProxyAddr)

// 	p := NewServer(ServerConfig{
// 		Logger: lg,
// 		Listen: url.URL{Scheme: scheme, Host: forwardProxyAddr},
// 	})

// 	waitForServer(t, p)

// 	defer p.Close()

// 	p.ModifyTx(func(d []byte) []byte {
// 		d[len(d)/2]++
// 		return d
// 	})
// 	data := []byte("Hello World!")
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})
// 	if d := receive(t, ln); bytes.Equal(d, data) {
// 		t.Fatalf("expected corrupted data, got %q", string(d))
// 	}

// 	p.UnmodifyTx()
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})
// 	if d := receive(t, ln); !bytes.Equal(d, data) {
// 		t.Fatalf("expected uncorrupted data, got %q", string(d))
// 	}
// }

// func TestServer_ModifyTx_packet_loss(t *testing.T) {
// 	lg := zaptest.NewLogger(t)
// 	scheme := "unix"
// 	forwardProxyAddr, dstAddr := newUnixAddr(), newUnixAddr()
// 	defer func() {
// 		os.RemoveAll(forwardProxyAddr)
// 		os.RemoveAll(dstAddr)
// 	}()
// 	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
// 	defer ln.Close()

// 	// setup forward proxy
// 	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", forwardProxyAddr)

// 	p := NewServer(ServerConfig{
// 		Logger: lg,
// 		Listen: url.URL{Scheme: scheme, Host: forwardProxyAddr},
// 	})

// 	waitForServer(t, p)

// 	defer p.Close()

// 	// 50% packet loss
// 	p.ModifyTx(func(d []byte) []byte {
// 		half := len(d) / 2
// 		return d[:half:half]
// 	})
// 	data := []byte("Hello World!")
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})
// 	if d := receive(t, ln); bytes.Equal(d, data) {
// 		t.Fatalf("expected corrupted data, got %q", string(d))
// 	}

// 	p.UnmodifyTx()
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})
// 	if d := receive(t, ln); !bytes.Equal(d, data) {
// 		t.Fatalf("expected uncorrupted data, got %q", string(d))
// 	}
// }

// func TestServer_BlackholeTx(t *testing.T) {
// 	lg := zaptest.NewLogger(t)
// 	scheme := "unix"
// 	forwardProxyAddr, dstAddr := newUnixAddr(), newUnixAddr()
// 	defer func() {
// 		os.RemoveAll(forwardProxyAddr)
// 		os.RemoveAll(dstAddr)
// 	}()
// 	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
// 	defer ln.Close()

// 	// setup forward proxy
// 	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", forwardProxyAddr)

// 	p := NewServer(ServerConfig{
// 		Logger: lg,
// 		Listen: url.URL{Scheme: scheme, Host: forwardProxyAddr},
// 	})

// 	waitForServer(t, p)

// 	defer p.Close()

// 	p.BlackholeTx()

// 	data := []byte("Hello World!")
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})

// 	recvc := make(chan []byte, 1)
// 	go func() {
// 		recvc <- receive(t, ln)
// 	}()

// 	select {
// 	case d := <-recvc:
// 		t.Fatalf("unexpected data receive %q during blackhole", string(d))
// 	case <-time.After(200 * time.Millisecond):
// 	}

// 	p.UnblackholeTx()

// 	// expect different data, old data dropped
// 	data[0]++
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})

// 	select {
// 	case d := <-recvc:
// 		if !bytes.Equal(data, d) {
// 			t.Fatalf("expected %q, got %q", string(data), string(d))
// 		}
// 	case <-time.After(2 * time.Second):
// 		t.Fatal("took too long to receive after unblackhole")
// 	}
// }

// func TestServer_Shutdown(t *testing.T) {
// 	lg := zaptest.NewLogger(t)
// 	scheme := "unix"
// 	forwardProxyAddr, dstAddr := newUnixAddr(), newUnixAddr()
// 	defer func() {
// 		os.RemoveAll(forwardProxyAddr)
// 		os.RemoveAll(dstAddr)
// 	}()
// 	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
// 	defer ln.Close()

// 	// setup forward proxy
// 	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", forwardProxyAddr)

// 	p := NewServer(ServerConfig{
// 		Logger: lg,
// 		Listen: url.URL{Scheme: scheme, Host: forwardProxyAddr},
// 	})

// 	waitForServer(t, p)

// 	defer p.Close()

// 	s, _ := p.(*server)
// 	s.listener.Close()
// 	time.Sleep(200 * time.Millisecond)

// 	data := []byte("Hello World!")
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})
// 	if d := receive(t, ln); !bytes.Equal(d, data) {
// 		t.Fatalf("expected %q, got %q", string(data), string(d))
// 	}
// }

// func TestServer_ShutdownListener(t *testing.T) {
// 	lg := zaptest.NewLogger(t)
// 	scheme := "unix"
// 	forwardProxyAddr, dstAddr := newUnixAddr(), newUnixAddr()
// 	defer func() {
// 		os.RemoveAll(forwardProxyAddr)
// 		os.RemoveAll(dstAddr)
// 	}()

// 	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
// 	defer ln.Close()

// 	// setup forward proxy
// 	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", forwardProxyAddr)

// 	p := NewServer(ServerConfig{
// 		Logger: lg,
// 		Listen: url.URL{Scheme: scheme, Host: forwardProxyAddr},
// 	})

// 	waitForServer(t, p)

// 	defer p.Close()

// 	// shut down destination
// 	ln.Close()
// 	time.Sleep(200 * time.Millisecond)

// 	ln = listen(t, scheme, dstAddr, transport.TLSInfo{})
// 	defer ln.Close()

// 	data := []byte("Hello World!")
// 	send(t, data, scheme, dstAddr, transport.TLSInfo{})
// 	if d := receive(t, ln); !bytes.Equal(d, data) {
// 		t.Fatalf("expected %q, got %q", string(data), string(d))
// 	}
// }

// func TestServerHTTP_Insecure_DelayTx(t *testing.T) { testServerHTTP(t, false, true) }
// func TestServerHTTP_Secure_DelayTx(t *testing.T)   { testServerHTTP(t, true, true) }
// func TestServerHTTP_Insecure_DelayRx(t *testing.T) { testServerHTTP(t, false, false) }
// func TestServerHTTP_Secure_DelayRx(t *testing.T)   { testServerHTTP(t, true, false) }
// func testServerHTTP(t *testing.T, secure, delayTx bool) {
// 	lg := zaptest.NewLogger(t)
// 	scheme := "tcp"
// 	ln1, ln2 := listen(t, scheme, "localhost:0", transport.TLSInfo{}), listen(t, scheme, "localhost:0", transport.TLSInfo{})
// 	forwardProxyAddr, dstAddr := ln1.Addr().String(), ln2.Addr().String()
// 	ln1.Close()
// 	ln2.Close()

// 	mux := http.NewServeMux()
// 	mux.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) {
// 		d, err := io.ReadAll(req.Body)
// 		req.Body.Close()
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		if _, err = w.Write([]byte(fmt.Sprintf("%q(confirmed)", string(d)))); err != nil {
// 			t.Fatal(err)
// 		}
// 	})
// 	tlsInfo := createTLSInfo(lg, secure)
// 	var tlsConfig *tls.Config
// 	if secure {
// 		_, err := tlsInfo.ServerConfig()
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 	}
// 	srv := &http.Server{
// 		Addr:      dstAddr,
// 		Handler:   mux,
// 		TLSConfig: tlsConfig,
// 		ErrorLog:  log.New(io.Discard, "net/http", 0),
// 	}

// 	donec := make(chan struct{})
// 	defer func() {
// 		srv.Close()
// 		<-donec
// 	}()
// 	go func() {
// 		if !secure {
// 			srv.ListenAndServe()
// 		} else {
// 			srv.ListenAndServeTLS(tlsInfo.CertFile, tlsInfo.KeyFile)
// 		}
// 		defer close(donec)
// 	}()
// 	time.Sleep(200 * time.Millisecond)

// 	// setup forward proxy
// 	t.Setenv("E2E_TEST_FORWARD_PROXY_IP", forwardProxyAddr)

// 	cfg := ServerConfig{
// 		Logger: lg,
// 		Listen: url.URL{Scheme: scheme, Host: forwardProxyAddr},
// 	}
// 	if secure {
// 		cfg.TLSInfo = tlsInfo
// 	}
// 	p := NewServer(cfg)

// 	waitForServer(t, p)

// 	defer func() {
// 		lg.Info("closing Proxy server...")
// 		p.Close()
// 		lg.Info("closed Proxy server.")
// 	}()

// 	data := "Hello World!"

// 	var resp *http.Response
// 	var err error
// 	now := time.Now()
// 	if secure {
// 		tp, terr := transport.NewTransport(tlsInfo, 3*time.Second)
// 		assert.NoError(t, terr)
// 		cli := &http.Client{Transport: tp}
// 		resp, err = cli.Post("https://"+dstAddr+"/hello", "", strings.NewReader(data))
// 		defer cli.CloseIdleConnections()
// 		defer tp.CloseIdleConnections()
// 	} else {
// 		resp, err = http.Post("http://"+dstAddr+"/hello", "", strings.NewReader(data))
// 		defer http.DefaultClient.CloseIdleConnections()
// 	}
// 	assert.NoError(t, err)
// 	d, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	resp.Body.Close()
// 	took1 := time.Since(now)
// 	t.Logf("took %v with no latency", took1)

// 	rs1 := string(d)
// 	exp := fmt.Sprintf("%q(confirmed)", data)
// 	if rs1 != exp {
// 		t.Fatalf("got %q, expected %q", rs1, exp)
// 	}

// 	lat, rv := 100*time.Millisecond, 10*time.Millisecond
// 	if delayTx {
// 		p.DelayTx(lat, rv)
// 		defer p.UndelayTx()
// 	} else {
// 		p.DelayRx(lat, rv)
// 		defer p.UndelayRx()
// 	}

// 	now = time.Now()
// 	if secure {
// 		tp, terr := transport.NewTransport(tlsInfo, 3*time.Second)
// 		if terr != nil {
// 			t.Fatal(terr)
// 		}
// 		cli := &http.Client{Transport: tp}
// 		resp, err = cli.Post("https://"+dstAddr+"/hello", "", strings.NewReader(data))
// 		defer cli.CloseIdleConnections()
// 		defer tp.CloseIdleConnections()
// 	} else {
// 		resp, err = http.Post("http://"+dstAddr+"/hello", "", strings.NewReader(data))
// 		defer http.DefaultClient.CloseIdleConnections()
// 	}
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	d, err = io.ReadAll(resp.Body)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	resp.Body.Close()
// 	took2 := time.Since(now)
// 	t.Logf("took %v with latency %v±%v", took2, lat, rv)

// 	rs2 := string(d)
// 	if rs2 != exp {
// 		t.Fatalf("got %q, expected %q", rs2, exp)
// 	}
// 	if took1 > took2 {
// 		t.Fatalf("expected took1 %v < took2 %v", took1, took2)
// 	}
// }
