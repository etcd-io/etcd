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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/pkg/v3/transport"

	"go.uber.org/zap"
)

// enable DebugLevel
var testLogger = zap.NewExample()

var testTLSInfo = transport.TLSInfo{
	KeyFile:        "../../tests/fixtures/server.key.insecure",
	CertFile:       "../../tests/fixtures/server.crt",
	TrustedCAFile:  "../../tests/fixtures/ca.crt",
	ClientCertAuth: true,
}

func TestServer_Unix_Insecure(t *testing.T)         { testServer(t, "unix", false, false) }
func TestServer_TCP_Insecure(t *testing.T)          { testServer(t, "tcp", false, false) }
func TestServer_Unix_Secure(t *testing.T)           { testServer(t, "unix", true, false) }
func TestServer_TCP_Secure(t *testing.T)            { testServer(t, "tcp", true, false) }
func TestServer_Unix_Insecure_DelayTx(t *testing.T) { testServer(t, "unix", false, true) }
func TestServer_TCP_Insecure_DelayTx(t *testing.T)  { testServer(t, "tcp", false, true) }
func TestServer_Unix_Secure_DelayTx(t *testing.T)   { testServer(t, "unix", true, true) }
func TestServer_TCP_Secure_DelayTx(t *testing.T)    { testServer(t, "tcp", true, true) }

func testServer(t *testing.T, scheme string, secure bool, delayTx bool) {
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	if scheme == "tcp" {
		ln1, ln2 := listen(t, "tcp", "localhost:0", transport.TLSInfo{}), listen(t, "tcp", "localhost:0", transport.TLSInfo{})
		srcAddr, dstAddr = ln1.Addr().String(), ln2.Addr().String()
		ln1.Close()
		ln2.Close()
	} else {
		defer func() {
			os.RemoveAll(srcAddr)
			os.RemoveAll(dstAddr)
		}()
	}
	tlsInfo := testTLSInfo
	if !secure {
		tlsInfo = transport.TLSInfo{}
	}
	ln := listen(t, scheme, dstAddr, tlsInfo)
	defer ln.Close()

	cfg := ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	}
	if secure {
		cfg.TLSInfo = testTLSInfo
	}
	p := NewServer(cfg)
	<-p.Ready()
	defer p.Close()

	data1 := []byte("Hello World!")
	donec, writec := make(chan struct{}), make(chan []byte)

	go func() {
		defer close(donec)
		for data := range writec {
			send(t, data, scheme, srcAddr, tlsInfo)
		}
	}()

	recvc := make(chan []byte, 1)
	go func() {
		for i := 0; i < 2; i++ {
			recvc <- receive(t, ln)
		}
	}()

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
			t.Fatalf("expected took2 %v (with latency) > delay: %v", took2, lat-rv)
		}
	}

	close(writec)
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

func TestServer_Unix_Insecure_DelayAccept(t *testing.T) { testServerDelayAccept(t, false) }
func TestServer_Unix_Secure_DelayAccept(t *testing.T)   { testServerDelayAccept(t, true) }
func testServerDelayAccept(t *testing.T, secure bool) {
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	tlsInfo := testTLSInfo
	if !secure {
		tlsInfo = transport.TLSInfo{}
	}
	scheme := "unix"
	ln := listen(t, scheme, dstAddr, tlsInfo)
	defer ln.Close()

	cfg := ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	}
	if secure {
		cfg.TLSInfo = testTLSInfo
	}
	p := NewServer(cfg)
	<-p.Ready()
	defer p.Close()

	data := []byte("Hello World!")

	now := time.Now()
	send(t, data, scheme, srcAddr, tlsInfo)
	if d := receive(t, ln); !bytes.Equal(data, d) {
		t.Fatalf("expected %q, got %q", string(data), string(d))
	}
	took1 := time.Since(now)
	t.Logf("took %v with no latency", took1)

	lat, rv := 700*time.Millisecond, 10*time.Millisecond
	p.DelayAccept(lat, rv)
	defer p.UndelayAccept()
	if err := p.ResetListener(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	now = time.Now()
	send(t, data, scheme, srcAddr, tlsInfo)
	if d := receive(t, ln); !bytes.Equal(data, d) {
		t.Fatalf("expected %q, got %q", string(data), string(d))
	}
	took2 := time.Since(now)
	t.Logf("took %v with latency %v±%v", took2, lat, rv)

	if took1 >= took2 {
		t.Fatalf("expected took1 %v < took2 %v", took1, took2)
	}
}

func TestServer_PauseTx(t *testing.T) {
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	<-p.Ready()
	defer p.Close()

	p.PauseTx()

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})

	recvc := make(chan []byte, 1)
	go func() {
		recvc <- receive(t, ln)
	}()

	select {
	case d := <-recvc:
		t.Fatalf("received unexpected data %q during pause", string(d))
	case <-time.After(200 * time.Millisecond):
	}

	p.UnpauseTx()

	select {
	case d := <-recvc:
		if !bytes.Equal(data, d) {
			t.Fatalf("expected %q, got %q", string(data), string(d))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("took too long to receive after unpause")
	}
}

func TestServer_ModifyTx_corrupt(t *testing.T) {
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	<-p.Ready()
	defer p.Close()

	p.ModifyTx(func(d []byte) []byte {
		d[len(d)/2]++
		return d
	})
	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	if d := receive(t, ln); bytes.Equal(d, data) {
		t.Fatalf("expected corrupted data, got %q", string(d))
	}

	p.UnmodifyTx()
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	if d := receive(t, ln); !bytes.Equal(d, data) {
		t.Fatalf("expected uncorrupted data, got %q", string(d))
	}
}

func TestServer_ModifyTx_packet_loss(t *testing.T) {
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	<-p.Ready()
	defer p.Close()

	// 50% packet loss
	p.ModifyTx(func(d []byte) []byte {
		half := len(d) / 2
		return d[:half:half]
	})
	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	if d := receive(t, ln); bytes.Equal(d, data) {
		t.Fatalf("expected corrupted data, got %q", string(d))
	}

	p.UnmodifyTx()
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	if d := receive(t, ln); !bytes.Equal(d, data) {
		t.Fatalf("expected uncorrupted data, got %q", string(d))
	}
}

func TestServer_BlackholeTx(t *testing.T) {
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	<-p.Ready()
	defer p.Close()

	p.BlackholeTx()

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})

	recvc := make(chan []byte, 1)
	go func() {
		recvc <- receive(t, ln)
	}()

	select {
	case d := <-recvc:
		t.Fatalf("unexpected data receive %q during blackhole", string(d))
	case <-time.After(200 * time.Millisecond):
	}

	p.UnblackholeTx()

	// expect different data, old data dropped
	data[0]++
	send(t, data, scheme, srcAddr, transport.TLSInfo{})

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
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	<-p.Ready()
	defer p.Close()

	s, _ := p.(*server)
	s.listener.Close()
	time.Sleep(200 * time.Millisecond)

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	if d := receive(t, ln); !bytes.Equal(d, data) {
		t.Fatalf("expected %q, got %q", string(data), string(d))
	}
}

func TestServer_ShutdownListener(t *testing.T) {
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()

	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	<-p.Ready()
	defer p.Close()

	// shut down destination
	ln.Close()
	time.Sleep(200 * time.Millisecond)

	ln = listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	if d := receive(t, ln); !bytes.Equal(d, data) {
		t.Fatalf("expected %q, got %q", string(data), string(d))
	}
}

func TestServerHTTP_Insecure_DelayTx(t *testing.T) { testServerHTTP(t, false, true) }
func TestServerHTTP_Secure_DelayTx(t *testing.T)   { testServerHTTP(t, true, true) }
func TestServerHTTP_Insecure_DelayRx(t *testing.T) { testServerHTTP(t, false, false) }
func TestServerHTTP_Secure_DelayRx(t *testing.T)   { testServerHTTP(t, true, false) }
func testServerHTTP(t *testing.T, secure, delayTx bool) {
	scheme := "tcp"
	ln1, ln2 := listen(t, scheme, "localhost:0", transport.TLSInfo{}), listen(t, scheme, "localhost:0", transport.TLSInfo{})
	srcAddr, dstAddr := ln1.Addr().String(), ln2.Addr().String()
	ln1.Close()
	ln2.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) {
		d, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(fmt.Sprintf("%q(confirmed)", string(d)))); err != nil {
			t.Fatal(err)
		}
	})
	var tlsConfig *tls.Config
	var err error
	if secure {
		tlsConfig, err = testTLSInfo.ServerConfig()
		if err != nil {
			t.Fatal(err)
		}
	}
	srv := &http.Server{
		Addr:      dstAddr,
		Handler:   mux,
		TLSConfig: tlsConfig,
		ErrorLog:  log.New(ioutil.Discard, "net/http", 0),
	}

	donec := make(chan struct{})
	defer func() {
		srv.Close()
		<-donec
	}()
	go func() {
		defer close(donec)
		if !secure {
			srv.ListenAndServe()
		} else {
			srv.ListenAndServeTLS(testTLSInfo.CertFile, testTLSInfo.KeyFile)
		}
	}()
	time.Sleep(200 * time.Millisecond)

	cfg := ServerConfig{
		Logger: testLogger,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	}
	if secure {
		cfg.TLSInfo = testTLSInfo
	}
	p := NewServer(cfg)
	<-p.Ready()
	defer p.Close()

	data := "Hello World!"

	now := time.Now()
	var resp *http.Response
	if secure {
		tp, terr := transport.NewTransport(testTLSInfo, 3*time.Second)
		if terr != nil {
			t.Fatal(terr)
		}
		cli := &http.Client{Transport: tp}
		resp, err = cli.Post("https://"+srcAddr+"/hello", "", strings.NewReader(data))
	} else {
		resp, err = http.Post("http://"+srcAddr+"/hello", "", strings.NewReader(data))
	}
	if err != nil {
		t.Fatal(err)
	}
	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	took1 := time.Since(now)
	t.Logf("took %v with no latency", took1)

	rs1 := string(d)
	exp := fmt.Sprintf("%q(confirmed)", data)
	if rs1 != exp {
		t.Fatalf("got %q, expected %q", rs1, exp)
	}

	lat, rv := 100*time.Millisecond, 10*time.Millisecond
	if delayTx {
		p.DelayTx(lat, rv)
		defer p.UndelayTx()
	} else {
		p.DelayRx(lat, rv)
		defer p.UndelayRx()
	}

	now = time.Now()
	if secure {
		tp, terr := transport.NewTransport(testTLSInfo, 3*time.Second)
		if terr != nil {
			t.Fatal(terr)
		}
		cli := &http.Client{Transport: tp}
		resp, err = cli.Post("https://"+srcAddr+"/hello", "", strings.NewReader(data))
	} else {
		resp, err = http.Post("http://"+srcAddr+"/hello", "", strings.NewReader(data))
	}
	if err != nil {
		t.Fatal(err)
	}
	d, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	took2 := time.Since(now)
	t.Logf("took %v with latency %v±%v", took2, lat, rv)

	rs2 := string(d)
	if rs2 != exp {
		t.Fatalf("got %q, expected %q", rs2, exp)
	}
	if took1 > took2 {
		t.Fatalf("expected took1 %v < took2 %v", took1, took2)
	}
}

func newUnixAddr() string {
	now := time.Now().UnixNano()
	rand.Seed(now)
	addr := fmt.Sprintf("%X%X.unix-conn", now, rand.Intn(35000))
	os.RemoveAll(addr)
	return addr
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

func send(t *testing.T, data []byte, scheme, addr string, tlsInfo transport.TLSInfo) {
	var out net.Conn
	var err error
	if !tlsInfo.Empty() {
		tp, terr := transport.NewTransport(tlsInfo, 3*time.Second)
		if terr != nil {
			t.Fatal(terr)
		}
		out, err = tp.Dial(scheme, addr)
	} else {
		out, err = net.Dial(scheme, addr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err = out.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = out.Close(); err != nil {
		t.Fatal(err)
	}
}

func receive(t *testing.T, ln net.Listener) (data []byte) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	for {
		in, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		var n int64
		n, err = buf.ReadFrom(in)
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			break
		}
	}
	return buf.Bytes()
}
