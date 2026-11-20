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
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/transport"
)

func TestServer_Unix_Insecure(t *testing.T)         { testServer(t, "unix", false, false) }
func TestServer_TCP_Insecure(t *testing.T)          { testServer(t, "tcp", false, false) }
func TestServer_Unix_Secure(t *testing.T)           { testServer(t, "unix", true, false) }
func TestServer_TCP_Secure(t *testing.T)            { testServer(t, "tcp", true, false) }
func TestServer_Unix_Insecure_DelayTx(t *testing.T) { testServer(t, "unix", false, true) }
func TestServer_TCP_Insecure_DelayTx(t *testing.T)  { testServer(t, "tcp", false, true) }
func TestServer_Unix_Secure_DelayTx(t *testing.T)   { testServer(t, "unix", true, true) }
func TestServer_TCP_Secure_DelayTx(t *testing.T)    { testServer(t, "tcp", true, true) }

func testServer(t *testing.T, scheme string, secure bool, delayTx bool) {
	lg := zaptest.NewLogger(t)
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
	tlsInfo := createTLSInfo(lg, secure)
	ln := listen(t, scheme, dstAddr, tlsInfo)
	defer ln.Close()

	cfg := ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	}
	if secure {
		cfg.TLSInfo = tlsInfo
	}
	p := NewServer(cfg)

	waitForServer(t, p)

	defer p.Close()

	data1 := []byte("Hello World!")
	donec, writec := make(chan struct{}), make(chan []byte)

	go func() {
		defer close(donec)
		for data := range writec {
			send(t, data, scheme, srcAddr, tlsInfo) //nolint:testifylint //FIXME
		}
	}()

	recvc := make(chan []byte, 1)
	go func() {
		for i := 0; i < 2; i++ {
			recvc <- receive(t, ln) //nolint:testifylint //FIXME
		}
	}()

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

	require.NoError(t, p.Close())

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

func createTLSInfo(lg *zap.Logger, secure bool) transport.TLSInfo {
	if secure {
		return transport.TLSInfo{
			KeyFile:        "../../tests/fixtures/server.key.insecure",
			CertFile:       "../../tests/fixtures/server.crt",
			TrustedCAFile:  "../../tests/fixtures/ca.crt",
			ClientCertAuth: true,
			Logger:         lg,
		}
	}
	return transport.TLSInfo{Logger: lg}
}

func TestServer_Unix_Insecure_DelayAccept(t *testing.T) { testServerDelayAccept(t, false) }
func TestServer_Unix_Secure_DelayAccept(t *testing.T)   { testServerDelayAccept(t, true) }
func testServerDelayAccept(t *testing.T, secure bool) {
	lg := zaptest.NewLogger(t)
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	tlsInfo := createTLSInfo(lg, secure)
	scheme := "unix"
	ln := listen(t, scheme, dstAddr, tlsInfo)
	defer ln.Close()

	cfg := ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	}
	if secure {
		cfg.TLSInfo = tlsInfo
	}
	p := NewServer(cfg)

	waitForServer(t, p)

	defer p.Close()

	data := []byte("Hello World!")

	now := time.Now()
	send(t, data, scheme, srcAddr, tlsInfo)
	d := receive(t, ln)
	require.Truef(t, bytes.Equal(data, d), "expected %q, got %q", string(data), string(d))
	took1 := time.Since(now)
	t.Logf("took %v with no latency", took1)

	lat, rv := 700*time.Millisecond, 10*time.Millisecond
	p.DelayAccept(lat, rv)
	defer p.UndelayAccept()
	require.NoError(t, p.ResetListener())
	time.Sleep(200 * time.Millisecond)

	now = time.Now()
	send(t, data, scheme, srcAddr, tlsInfo)
	d = receive(t, ln)
	require.Truef(t, bytes.Equal(data, d), "expected %q, got %q", string(data), string(d))
	took2 := time.Since(now)
	t.Logf("took %v with latency %v±%v", took2, lat, rv)

	require.Lessf(t, took1, took2, "expected took1 %v < took2 %v", took1, took2)
}

func TestServer_PauseTx(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})

	waitForServer(t, p)

	defer p.Close()

	p.PauseTx()

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})

	recvc := make(chan []byte, 1)
	go func() {
		recvc <- receive(t, ln) //nolint:testifylint //FIXME
	}()

	select {
	case d := <-recvc:
		t.Fatalf("received unexpected data %q during pause", string(d))
	case <-time.After(200 * time.Millisecond):
	}

	p.UnpauseTx()

	select {
	case d := <-recvc:
		require.Truef(t, bytes.Equal(data, d), "expected %q, got %q", string(data), string(d))
	case <-time.After(2 * time.Second):
		t.Fatal("took too long to receive after unpause")
	}
}

func TestServer_ModifyTx_corrupt(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})

	waitForServer(t, p)

	defer p.Close()

	p.ModifyTx(func(d []byte) []byte {
		d[len(d)/2]++
		return d
	})
	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	d := receive(t, ln)
	require.Falsef(t, bytes.Equal(d, data), "expected corrupted data, got %q", string(d))

	p.UnmodifyTx()
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	d = receive(t, ln)
	require.Truef(t, bytes.Equal(d, data), "expected uncorrupted data, got %q", string(d))
}

func TestServer_ModifyTx_packet_loss(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})

	waitForServer(t, p)

	defer p.Close()

	// 50% packet loss
	p.ModifyTx(func(d []byte) []byte {
		half := len(d) / 2
		return d[:half:half]
	})
	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	d := receive(t, ln)
	require.Falsef(t, bytes.Equal(d, data), "expected corrupted data, got %q", string(d))

	p.UnmodifyTx()
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	d = receive(t, ln)
	require.Truef(t, bytes.Equal(d, data), "expected uncorrupted data, got %q", string(d))
}

func TestServer_BlackholeTx(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})

	waitForServer(t, p)

	defer p.Close()

	p.BlackholeTx()

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})

	recvc := make(chan []byte, 1)
	go func() {
		recvc <- receive(t, ln) //nolint:testifylint //FIXME
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
		require.Truef(t, bytes.Equal(data, d), "expected %q, got %q", string(data), string(d))
	case <-time.After(2 * time.Second):
		t.Fatal("took too long to receive after unblackhole")
	}
}

func TestServer_BlackholeEstablishedConnDrops(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	waitForServer(t, p)
	defer p.Close()

	client, err := net.Dial(scheme, srcAddr)
	require.NoError(t, err)
	defer client.Close()
	target, err := ln.Accept()
	require.NoError(t, err)
	defer target.Close()

	// Verify both directions forward before blackholing.
	_, err = client.Write([]byte("pre-tx"))
	require.NoError(t, err)
	buf := make([]byte, 64)
	require.NoError(t, target.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, err := io.ReadFull(target, buf[:6])
	require.NoError(t, err)
	require.Equal(t, "pre-tx", string(buf[:n]))

	// BlackholeTx drops client->target bytes on the open connection while
	// leaving target->client open, and UnmodifyTx does not clear BlackholeTx.
	p.BlackholeTx()
	_, err = client.Write([]byte("dropped-tx"))
	require.NoError(t, err)
	require.NoError(t, target.SetReadDeadline(time.Now().Add(150*time.Millisecond)))
	_, err = target.Read(buf)
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)

	p.UnmodifyTx()
	_, err = client.Write([]byte("still-dropped-tx"))
	require.NoError(t, err)
	require.NoError(t, target.SetReadDeadline(time.Now().Add(150*time.Millisecond)))
	_, err = target.Read(buf)
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)

	_, err = target.Write([]byte("live-rx"))
	require.NoError(t, err)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, err = io.ReadFull(client, buf[:7])
	require.NoError(t, err)
	require.Equal(t, "live-rx", string(buf[:n]))

	// BlackholeRx also drops target->client bytes on the open connection.
	p.BlackholeRx()
	_, err = target.Write([]byte("dropped-rx"))
	require.NoError(t, err)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(150*time.Millisecond)))
	_, err = client.Read(buf)
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)

	// Partial unblackholing keeps tainted Tx dropped until all blackholes lift.
	p.UnblackholeTx()
	_, err = client.Write([]byte("tainted-tx"))
	require.NoError(t, err)
	require.NoError(t, target.SetReadDeadline(time.Now().Add(150*time.Millisecond)))
	_, err = target.Read(buf)
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)

	// Lifting the remaining blackhole closes both sides of the tainted connection.
	p.UnblackholeRx()
	requireConnClosed(t, client)
	requireConnClosed(t, target)

	select {
	case proxyErr := <-p.Error():
		t.Fatalf("unexpected proxy error: %v", proxyErr)
	default:
	}

	// Fresh connections after unblackholing forward cleanly again.
	send(t, []byte("post-unblackhole"), scheme, srcAddr, transport.TLSInfo{})
	got := receive(t, ln)
	require.Equal(t, "post-unblackhole", string(got))

	// Unidirectional BlackholeTx + UnblackholeTx while an Rx write is in flight
	// closes the connection without emitting net.ErrClosed on Server.Error().
	client2, err := net.Dial(scheme, srcAddr)
	require.NoError(t, err)
	defer client2.Close()
	target2, err := ln.Accept()
	require.NoError(t, err)
	defer target2.Close()

	var rxRead2Once sync.Once
	rxRead2 := make(chan struct{})
	p.ModifyRx(func(d []byte) []byte {
		rxRead2Once.Do(func() { close(rxRead2) })
		return d
	})
	p.DelayRx(60*time.Millisecond, 0)
	_, err = target2.Write([]byte("inflight-rx"))
	require.NoError(t, err)
	select {
	case <-rxRead2:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for proxy to read inflight-rx")
	}
	p.UnmodifyRx()
	p.BlackholeTx()
	p.UnblackholeTx()
	time.Sleep(100 * time.Millisecond)
	p.UndelayRx()

	select {
	case proxyErr := <-p.Error():
		t.Fatalf("unexpected proxy error after unblackhole with in-flight rx: %v", proxyErr)
	default:
	}

	// Unidirectional BlackholeTx hiding client.Close() while Rx writes to the
	// dead client socket does not emit EPIPE/ECONNRESET on Server.Error().
	client3, err := net.Dial(scheme, srcAddr)
	require.NoError(t, err)
	defer client3.Close()
	target3, err := ln.Accept()
	require.NoError(t, err)
	defer target3.Close()

	var rxRead3Once sync.Once
	rxRead3 := make(chan struct{})
	p.ModifyRx(func(d []byte) []byte {
		rxRead3Once.Do(func() { close(rxRead3) })
		return d
	})
	p.BlackholeTx()
	require.NoError(t, client3.Close())
	_, err = target3.Write([]byte("rx-after-hidden-client-close"))
	require.NoError(t, err)
	select {
	case <-rxRead3:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for proxy to read rx-after-hidden-client-close")
	}
	p.UnmodifyRx()
	time.Sleep(30 * time.Millisecond)

	select {
	case proxyErr := <-p.Error():
		t.Fatalf("unexpected proxy error while blackholed after rx write to closed client: %v", proxyErr)
	default:
	}

	p.UnblackholeTx()
	requireConnClosed(t, target3)

	select {
	case proxyErr := <-p.Error():
		t.Fatalf("unexpected proxy error after unblackhole following rx write to closed client: %v", proxyErr)
	default:
	}
}

func TestServer_BlackholeHidesClose(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	waitForServer(t, p)
	defer p.Close()

	dialPair := func() (net.Conn, net.Conn) {
		c, err := net.Dial(scheme, srcAddr)
		require.NoError(t, err)
		sConn, err := ln.Accept()
		require.NoError(t, err)
		return c, sConn
	}

	client1, target1 := dialPair()
	defer client1.Close()
	defer target1.Close()

	client2, target2 := dialPair()
	defer client2.Close()
	defer target2.Close()

	client3, target3 := dialPair()
	defer client3.Close()
	defer target3.Close()

	watchClosed := func(c net.Conn) <-chan struct{} {
		closed := make(chan struct{})
		go func() {
			io.Copy(io.Discard, c)
			close(closed)
		}()
		return closed
	}
	target1Closed := watchClosed(target1)
	client2Closed := watchClosed(client2)
	client3Closed := watchClosed(client3)
	target3Closed := watchClosed(target3)

	p.BlackholeTx()
	p.BlackholeRx()

	// A close by the client must not reach the target while blackholed.
	require.NoError(t, client1.Close())
	select {
	case <-target1Closed:
		t.Fatal("client close propagated to target during blackhole")
	case <-time.After(200 * time.Millisecond):
	}

	// A close by the target must not reach the client while blackholed.
	require.NoError(t, target2.Close())
	select {
	case <-client2Closed:
		t.Fatal("target close propagated to client during blackhole")
	case <-time.After(200 * time.Millisecond):
	}

	// New connections must be drained without dialing the target while blackholed.
	accepted := make(chan net.Conn, 1)
	go func() {
		if c, aerr := ln.Accept(); aerr == nil {
			accepted <- c
		}
	}()
	client4, err := net.Dial(scheme, srcAddr)
	require.NoError(t, err)
	defer client4.Close()
	_, err = client4.Write([]byte("discarded while blackholed"))
	require.NoError(t, err)
	select {
	case c := <-accepted:
		c.Close()
		t.Fatal("connection forwarded to target during blackhole")
	case <-time.After(200 * time.Millisecond):
	}

	// Removing all blackholes closes every connection that was open during them,
	// including connections where neither peer closed during the blackhole.
	p.UnblackholeTx()
	p.UnblackholeRx()
	for name, ch := range map[string]<-chan struct{}{
		"target1": target1Closed,
		"client2": client2Closed,
		"client3": client3Closed,
		"target3": target3Closed,
	} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s connection not closed after unblackhole", name)
		}
	}
	requireConnClosed(t, client4)

	select {
	case proxyErr := <-p.Error():
		t.Fatalf("unexpected proxy error: %v", proxyErr)
	default:
	}
}

func TestServer_CloseWhileBlackholed(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	waitForServer(t, p)

	client1, err := net.Dial(scheme, srcAddr)
	require.NoError(t, err)
	defer client1.Close()
	target1, err := ln.Accept()
	require.NoError(t, err)
	defer target1.Close()

	// Park s.receive (normally in its DelayRx sleep) so Close() must wake it
	// before closeWg.Wait() can return.
	var rxReadOnce sync.Once
	rxRead := make(chan struct{})
	p.ModifyRx(func(d []byte) []byte {
		rxReadOnce.Do(func() { close(rxRead) })
		return d
	})
	p.DelayRx(time.Hour, 0)
	_, err = target1.Write([]byte("in-flight"))
	require.NoError(t, err)
	select {
	case <-rxRead:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for proxy to read in-flight packet")
	}

	p.BlackholeTx()
	p.BlackholeRx()

	client2, err := net.Dial(scheme, srcAddr)
	require.NoError(t, err)
	defer client2.Close()
	_, err = client2.Write([]byte("blackholed"))
	require.NoError(t, err)

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- p.Close()
	}()
	select {
	case closeErr := <-closeDone:
		require.NoError(t, closeErr)
	case <-time.After(3 * time.Second):
		t.Fatal("p.Close() hung while blackholed")
	}

	requireConnClosed(t, client1)
	requireConnClosed(t, target1)
	requireConnClosed(t, client2)

	for {
		select {
		case proxyErr := <-p.Error():
			if !strings.HasPrefix(proxyErr.Error(), "accept ") {
				t.Fatalf("unexpected non-accept error on Close(): %v", proxyErr)
			}
		default:
			return
		}
	}
}

// requireConnClosed asserts that the proxy closed its end of c. Depending on
// whether unread bytes from c were still buffered in the proxy socket when the
// proxy closed it, the kernel reports either EOF or ECONNRESET.
func requireConnClosed(t *testing.T, c net.Conn) {
	t.Helper()
	var buf [16]byte
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err := c.Read(buf[:])
	require.Error(t, err)
	require.NotErrorIs(t, err, os.ErrDeadlineExceeded)
	if !errors.Is(err, io.EOF) &&
		!strings.Contains(err.Error(), "connection reset by peer") &&
		!strings.Contains(err.Error(), "forcibly closed") {
		t.Fatalf("expected EOF or connection reset on closed peer, got: %v", err)
	}
}

func TestServer_BlackholeConcurrentAcceptAndUnblackhole(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				io.Copy(conn, conn)
			}(c)
		}
	}()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})
	waitForServer(t, p)
	defer p.Close()

	for i := 0; i < 200; i++ {
		p.BlackholeTx()
		p.BlackholeRx()

		var wg sync.WaitGroup
		var conn net.Conn
		var dialErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			p.UnblackholeTx()
			p.UnblackholeRx()
		}()
		go func() {
			defer wg.Done()
			conn, dialErr = net.Dial(scheme, srcAddr)
		}()
		wg.Wait()
		require.NoErrorf(t, dialErr, "iteration %d: dial failed", i)

		// Once UnblackholeTx/Rx have returned, conn must either already be
		// closed (accepted during the blackhole) or connected to the echo
		// target; it must never stay open discarding bytes into io.Discard.
		_, writeErr := conn.Write([]byte("ping"))
		if writeErr == nil {
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
			buf := make([]byte, 4)
			_, readErr := io.ReadFull(conn, buf)
			if readErr != nil {
				require.NotErrorIsf(t, readErr, os.ErrDeadlineExceeded, "iteration %d: connection stuck in blackhole discard after unblackhole", i)
			} else {
				require.Equalf(t, "ping", string(buf), "iteration %d: unexpected echo payload", i)
			}
		}
		_ = conn.Close()
	}

	select {
	case proxyErr := <-p.Error():
		t.Fatalf("unexpected proxy error: %v", proxyErr)
	default:
	}
}

func TestServer_Shutdown(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()
	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})

	waitForServer(t, p)

	defer p.Close()

	s, _ := p.(*server)
	s.listener.Close()
	time.Sleep(200 * time.Millisecond)

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	d := receive(t, ln)
	require.Truef(t, bytes.Equal(d, data), "expected %q, got %q", string(data), string(d))
}

func TestServer_ShutdownListener(t *testing.T) {
	lg := zaptest.NewLogger(t)
	scheme := "unix"
	srcAddr, dstAddr := newUnixAddr(), newUnixAddr()
	defer func() {
		os.RemoveAll(srcAddr)
		os.RemoveAll(dstAddr)
	}()

	ln := listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	p := NewServer(ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	})

	waitForServer(t, p)

	defer p.Close()

	// shut down destination
	ln.Close()
	time.Sleep(200 * time.Millisecond)

	ln = listen(t, scheme, dstAddr, transport.TLSInfo{})
	defer ln.Close()

	data := []byte("Hello World!")
	send(t, data, scheme, srcAddr, transport.TLSInfo{})
	d := receive(t, ln)
	require.Truef(t, bytes.Equal(d, data), "expected %q, got %q", string(data), string(d))
}

func TestServerHTTP_Insecure_DelayTx(t *testing.T) { testServerHTTP(t, false, true) }
func TestServerHTTP_Secure_DelayTx(t *testing.T)   { testServerHTTP(t, true, true) }
func TestServerHTTP_Insecure_DelayRx(t *testing.T) { testServerHTTP(t, false, false) }
func TestServerHTTP_Secure_DelayRx(t *testing.T)   { testServerHTTP(t, true, false) }
func testServerHTTP(t *testing.T, secure, delayTx bool) {
	lg := zaptest.NewLogger(t)
	scheme := "tcp"
	ln1, ln2 := listen(t, scheme, "localhost:0", transport.TLSInfo{}), listen(t, scheme, "localhost:0", transport.TLSInfo{})
	srcAddr, dstAddr := ln1.Addr().String(), ln2.Addr().String()
	ln1.Close()
	ln2.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) {
		d, err := io.ReadAll(req.Body)
		req.Body.Close()
		require.NoError(t, err) //nolint:testifylint //FIXME
		_, err = w.Write([]byte(fmt.Sprintf("%q(confirmed)", string(d))))
		require.NoError(t, err) //nolint:testifylint //FIXME
	})
	tlsInfo := createTLSInfo(lg, secure)
	var tlsConfig *tls.Config
	if secure {
		_, err := tlsInfo.ServerConfig()
		require.NoError(t, err)
	}
	srv := &http.Server{
		Addr:      dstAddr,
		Handler:   mux,
		TLSConfig: tlsConfig,
		ErrorLog:  log.New(io.Discard, "net/http", 0),
	}

	donec := make(chan struct{})
	defer func() {
		srv.Close()
		<-donec
	}()
	go func() {
		if !secure {
			srv.ListenAndServe()
		} else {
			srv.ListenAndServeTLS(tlsInfo.CertFile, tlsInfo.KeyFile)
		}
		defer close(donec)
	}()
	time.Sleep(200 * time.Millisecond)

	cfg := ServerConfig{
		Logger: lg,
		From:   url.URL{Scheme: scheme, Host: srcAddr},
		To:     url.URL{Scheme: scheme, Host: dstAddr},
	}
	if secure {
		cfg.TLSInfo = tlsInfo
	}
	p := NewServer(cfg)

	waitForServer(t, p)

	defer func() {
		lg.Info("closing Proxy server...")
		p.Close()
		lg.Info("closed Proxy server.")
	}()

	data := "Hello World!"

	var resp *http.Response
	var err error
	now := time.Now()
	if secure {
		tp, terr := transport.NewTransport(tlsInfo, 3*time.Second)
		require.NoError(t, terr)
		cli := &http.Client{Transport: tp}
		resp, err = cli.Post("https://"+srcAddr+"/hello", "", strings.NewReader(data))
		defer cli.CloseIdleConnections()
		defer tp.CloseIdleConnections()
	} else {
		resp, err = http.Post("http://"+srcAddr+"/hello", "", strings.NewReader(data))
		defer http.DefaultClient.CloseIdleConnections()
	}
	require.NoError(t, err)
	d, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	took1 := time.Since(now)
	t.Logf("took %v with no latency", took1)

	rs1 := string(d)
	exp := fmt.Sprintf("%q(confirmed)", data)
	require.Equalf(t, exp, rs1, "got %q, expected %q", rs1, exp)

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
		tp, terr := transport.NewTransport(tlsInfo, 3*time.Second)
		require.NoError(t, terr)
		cli := &http.Client{Transport: tp}
		resp, err = cli.Post("https://"+srcAddr+"/hello", "", strings.NewReader(data))
		defer cli.CloseIdleConnections()
		defer tp.CloseIdleConnections()
	} else {
		resp, err = http.Post("http://"+srcAddr+"/hello", "", strings.NewReader(data))
		defer http.DefaultClient.CloseIdleConnections()
	}
	require.NoError(t, err)
	d, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	took2 := time.Since(now)
	t.Logf("took %v with latency %v±%v", took2, lat, rv)

	rs2 := string(d)
	require.Equalf(t, exp, rs2, "got %q, expected %q", rs2, exp)
	require.LessOrEqualf(t, took1, took2, "expected took1 %v < took2 %v", took1, took2)
}

func newUnixAddr() string {
	now := time.Now().UnixNano()
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
	require.NoError(t, err)
	return ln
}

func send(t *testing.T, data []byte, scheme, addr string, tlsInfo transport.TLSInfo) {
	var out net.Conn
	var err error
	if !tlsInfo.Empty() {
		tp, terr := transport.NewTransport(tlsInfo, 3*time.Second)
		require.NoError(t, terr)
		out, err = tp.DialContext(t.Context(), scheme, addr)
	} else {
		out, err = net.Dial(scheme, addr)
	}
	require.NoError(t, err)
	_, err = out.Write(data)
	require.NoError(t, err)
	require.NoError(t, out.Close())
}

func receive(t *testing.T, ln net.Listener) (data []byte) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	for {
		in, err := ln.Accept()
		require.NoError(t, err)
		var n int64
		n, err = buf.ReadFrom(in)
		require.NoError(t, err)
		if n > 0 {
			break
		}
	}
	return buf.Bytes()
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
