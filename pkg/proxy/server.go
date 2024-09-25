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
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/bits"
	mrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/transport"
)

var (
	defaultDialTimeout   = 3 * time.Second
	defaultBufferSize    = 48 * 1024
	defaultRetryInterval = 10 * time.Millisecond
)

// Server defines proxy server layer that simulates common network faults:
// latency spikes and packet drop or corruption. The proxy overhead is very
// small overhead (<500μs per request). Please run tests to compute actual
// overhead.
//
// Note that the current implementation is a forward proxy, thus, unix socket
// is not supported, due to the forwarding is done in L7, which requires
// properly constructed HTTP header and body
//
// Also, because we are forced to use TLS to communicate with the proxy server
// and using well-formed header to talk to the destination server,
// so in the L7 forward proxy design we drop features such as random packet
// modification, etc.
type Server interface {
	// Listen returns proxy listen address in "scheme://host:port" format.
	Listen() string

	// Ready returns when proxy is ready to serve.
	Ready() <-chan struct{}
	// Done returns when proxy has been closed.
	Done() <-chan struct{}
	// Error sends errors while serving proxy.
	Error() <-chan error
	// Close closes listener and transport.
	Close() error

	// DelayTx adds latency ± random variable for "outgoing" traffic
	// in "sending" layer.
	DelayTx(latency, rv time.Duration)
	// UndelayTx removes sending latencies.
	UndelayTx()
	// LatencyTx returns current send latency.
	LatencyTx() time.Duration

	// DelayRx adds latency ± random variable for "incoming" traffic
	// in "receiving" layer.
	DelayRx(latency, rv time.Duration)
	// UndelayRx removes "receiving" latencies.
	UndelayRx()
	// LatencyRx returns current receive latency.
	LatencyRx() time.Duration

	// ModifyTx alters/corrupts/drops "outgoing" packets from the listener
	// with the given edit function.
	ModifyTx(f func(data []byte) []byte)
	// UnmodifyTx removes modify operation on "forwarding".
	UnmodifyTx()

	// ModifyRx alters/corrupts/drops "incoming" packets to client
	// with the given edit function.
	ModifyRx(f func(data []byte) []byte)
	// UnmodifyRx removes modify operation on "receiving".
	UnmodifyRx()

	// BlackholeTx drops all "outgoing" packets before "forwarding".
	// "BlackholeTx" operation is a wrapper around "ModifyTx" with
	// a function that returns empty bytes.
	BlackholeTx()
	// UnblackholeTx removes blackhole operation on "sending".
	UnblackholeTx()

	// BlackholeRx drops all "incoming" packets to client.
	// "BlackholeRx" operation is a wrapper around "ModifyRx" with
	// a function that returns empty bytes.
	BlackholeRx()
	// UnblackholeRx removes blackhole operation on "receiving".
	UnblackholeRx()

	// BlackholePeerTx drops all outgoing traffic of a peer.
	BlackholePeerTx(peer url.URL)
	// UnblackholePeerTx removes blackhole operation on "sending".
	UnblackholePeerTx(peer url.URL)

	// BlackholePeerTx drops all incoming traffic of a peer.
	BlackholePeerRx(peer url.URL)
	// UnblackholePeerRx removes blackhole operation on "receiving".
	UnblackholePeerRx(peer url.URL)
}

// ServerConfig defines proxy server configuration.
type ServerConfig struct {
	Logger        *zap.Logger
	Listen        url.URL
	TLSInfo       transport.TLSInfo
	DialTimeout   time.Duration
	BufferSize    int
	RetryInterval time.Duration
}

const (
	blackholePeerTypeNone uint8 = iota
	blackholePeerTypeTx
	blackholePeerTypeRx
)

type server struct {
	lg *zap.Logger

	listen     url.URL
	listenPort int

	tlsInfo     transport.TLSInfo
	dialTimeout time.Duration

	bufferSize    int
	retryInterval time.Duration

	readyc chan struct{}
	donec  chan struct{}
	errc   chan error

	closeOnce         sync.Once
	closeWg           sync.WaitGroup
	closeHijackedConn sync.WaitGroup

	listenerMu sync.RWMutex
	listener   *net.Listener

	modifyTxMu sync.RWMutex
	modifyTx   func(data []byte) []byte

	modifyRxMu sync.RWMutex
	modifyRx   func(data []byte) []byte

	latencyTxMu sync.RWMutex
	latencyTx   time.Duration

	latencyRxMu sync.RWMutex
	latencyRx   time.Duration

	blackholePeerMap   map[int]uint8 // port number, blackhole type
	blackholePeerMapMu sync.RWMutex

	httpServer *http.Server
}

// NewServer returns a proxy implementation with no iptables/tc dependencies.
// The proxy layer overhead is <1ms.
func NewServer(cfg ServerConfig) Server {
	s := &server{
		lg: cfg.Logger,

		listen: cfg.Listen,

		tlsInfo:     cfg.TLSInfo,
		dialTimeout: cfg.DialTimeout,

		bufferSize:    cfg.BufferSize,
		retryInterval: cfg.RetryInterval,

		readyc: make(chan struct{}),
		donec:  make(chan struct{}),
		errc:   make(chan error, 16),

		blackholePeerMap: make(map[int]uint8),
	}

	var err error
	var fromPort string

	if s.dialTimeout == 0 {
		s.dialTimeout = defaultDialTimeout
	}
	if s.bufferSize == 0 {
		s.bufferSize = defaultBufferSize
	}
	if s.retryInterval == 0 {
		s.retryInterval = defaultRetryInterval
	}

	// L7 is http (scheme), L4 is tcp (network listener)
	addr := ""
	if strings.HasPrefix(s.listen.Scheme, "http") {
		s.listen.Scheme = "tcp"

		if _, fromPort, err = net.SplitHostPort(cfg.Listen.Host); err != nil {
			s.errc <- err
			s.Close()
			return nil
		}
		if s.listenPort, err = strconv.Atoi(fromPort); err != nil {
			s.errc <- err
			s.Close()
			return nil
		}

		addr = fmt.Sprintf(":%d", s.listenPort)
	} else {
		panic(fmt.Sprintf("%s is not supported", s.listen.Scheme))
	}

	s.closeWg.Add(1)
	var ln net.Listener
	if !s.tlsInfo.Empty() {
		ln, err = transport.NewListener(addr, s.listen.Scheme, &s.tlsInfo)
	} else {
		ln, err = net.Listen(s.listen.Scheme, addr)
	}
	if err != nil {
		s.errc <- err
		s.Close()
		return nil
	}

	s.listener = &ln

	go func() {
		defer s.closeWg.Done()

		s.httpServer = &http.Server{
			Handler: &serverHandler{s: s},
		}

		s.lg.Info("proxy is listening on", zap.String("listen on", s.Listen()))
		close(s.readyc)
		if err := s.httpServer.Serve(*s.listener); err != http.ErrServerClosed {
			// always returns error. ErrServerClosed on graceful close
			panic(fmt.Sprintf("startHTTPServer Serve(): %v", err))
		}
	}()

	s.lg.Info("started proxying", zap.String("listen on", s.Listen()))
	return s
}

type serverHandler struct {
	s *server
}

func (sh *serverHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	hijacker, _ := resp.(http.Hijacker)
	in, _, err := hijacker.Hijack()
	if err != nil {
		select {
		case sh.s.errc <- err:
			select {
			case <-sh.s.donec:
				return
			default:
			}
		case <-sh.s.donec:
			return
		}
		sh.s.lg.Debug("ServeHTTP hijack error", zap.Error(err))
		panic(err)
	}

	targetScheme := "tcp"
	targetHost := req.URL.Host
	ctx := context.Background()

	/*
		If the traffic to the destination is HTTPS, a CONNECT request will be sent
		first (containing the intended destination HOST).

		If the traffic to the destination is HTTP, no CONNECT request will be sent
		first. Only normal HTTP request is sent, with the HOST set to the final destination.
		This will be troublesome since we need to manually forward the request to the
		destination, and we can't do bte stream manipulation.

		Thus, we need to send the traffic to destination with HTTPS, allowing us to
		handle byte streams.
	*/
	if req.Method == "CONNECT" {
		// for CONNECT, we need to send 200 response back first
		in.Write([]byte("HTTP/1.0 200 Connection established\r\n\r\n"))
	}

	var out net.Conn
	if !sh.s.tlsInfo.Empty() {
		var tp *http.Transport
		tp, err = transport.NewTransport(sh.s.tlsInfo, sh.s.dialTimeout)
		if err != nil {
			select {
			case sh.s.errc <- err:
				select {
				case <-sh.s.donec:
					return
				default:
				}
			case <-sh.s.donec:
				return
			}
			sh.s.lg.Debug("failed to get new Transport", zap.Error(err))
			return
		}
		out, err = tp.DialContext(ctx, targetScheme, targetHost)
	} else {
		out, err = net.Dial(targetScheme, targetHost)
	}
	if err != nil {
		select {
		case sh.s.errc <- err:
			select {
			case <-sh.s.donec:
				return
			default:
			}
		case <-sh.s.donec:
			return
		}
		sh.s.lg.Debug("failed to dial", zap.Error(err))
		return
	}

	var dstPort int
	dstPort, err = getPort(out.RemoteAddr())
	if err != nil {
		select {
		case sh.s.errc <- err:
			select {
			case <-sh.s.donec:
				return
			default:
			}
		case <-sh.s.donec:
			return
		}
		sh.s.lg.Debug("failed to parse port in transmit", zap.Error(err))
		return
	}

	sh.s.closeHijackedConn.Add(2)
	go func() {
		defer sh.s.closeHijackedConn.Done()
		// read incoming bytes from listener, dispatch to outgoing connection
		sh.s.transmit(out, in, dstPort)
		out.Close()
		in.Close()
	}()
	go func() {
		defer sh.s.closeHijackedConn.Done()
		// read response from outgoing connection, write back to listener
		sh.s.receive(in, out, dstPort)
		in.Close()
		out.Close()
	}()
}

func (s *server) Listen() string {
	return fmt.Sprintf("%s://%s", s.listen.Scheme, s.listen.Host)
}

func getPort(addr net.Addr) (int, error) {
	switch addr := addr.(type) {
	case *net.TCPAddr:
		return addr.Port, nil
	case *net.UDPAddr:
		return addr.Port, nil
	default:
		return 0, fmt.Errorf("unsupported address type: %T", addr)
	}
}

func (s *server) transmit(dst, src net.Conn, port int) {
	s.ioCopy(dst, src, proxyTx, port)
}

func (s *server) receive(dst, src net.Conn, port int) {
	s.ioCopy(dst, src, proxyRx, port)
}

type proxyType uint8

const (
	proxyTx proxyType = iota
	proxyRx
)

func (s *server) ioCopy(dst, src net.Conn, ptype proxyType, peerPort int) {
	buf := make([]byte, s.bufferSize)
	for {
		nr1, err := src.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			// connection already closed
			if strings.HasSuffix(err.Error(), "read: connection reset by peer") {
				return
			}
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				return
			}
			select {
			case s.errc <- err:
				select {
				case <-s.donec:
					return
				default:
				}
			case <-s.donec:
				return
			}
			s.lg.Debug("failed to read", zap.Error(err))
			return
		}
		if nr1 == 0 {
			return
		}
		data := buf[:nr1]

		// alters/corrupts/drops data
		switch ptype {
		case proxyTx:
			s.modifyTxMu.RLock()
			if s.modifyTx != nil {
				data = s.modifyTx(data)
			}
			s.modifyTxMu.RUnlock()

			s.blackholePeerMapMu.RLock()
			// Tx from other peers is Rx for the target peer
			if val, exist := s.blackholePeerMap[peerPort]; exist {
				if (val & blackholePeerTypeRx) > 0 {
					data = nil
				}
			}
			s.blackholePeerMapMu.RUnlock()
		case proxyRx:
			s.modifyRxMu.RLock()
			if s.modifyRx != nil {
				data = s.modifyRx(data)
			}
			s.modifyRxMu.RUnlock()

			s.blackholePeerMapMu.RLock()
			// Rx from other peers is Tx for the target peer
			if val, exist := s.blackholePeerMap[peerPort]; exist {
				if (val & blackholePeerTypeTx) > 0 {
					data = nil
				}
			}
			s.blackholePeerMapMu.RUnlock()
		default:
			panic("unknown proxy type")
		}
		nr2 := len(data)
		switch ptype {
		case proxyTx:
			s.lg.Debug(
				"proxyTx",
				zap.String("data-received", humanize.Bytes(uint64(nr1))),
				zap.String("data-modified", humanize.Bytes(uint64(nr2))),
				zap.String("proxy listening on", s.Listen()),
				zap.Int("to peer port", peerPort),
			)
		case proxyRx:
			s.lg.Debug(
				"proxyRx",
				zap.String("data-received", humanize.Bytes(uint64(nr1))),
				zap.String("data-modified", humanize.Bytes(uint64(nr2))),
				zap.String("proxy listening on", s.Listen()),
				zap.Int("to peer port", peerPort),
			)
		default:
			panic("unknown proxy type")
		}

		if nr2 == 0 {
			continue
		}

		// block before forwarding
		var lat time.Duration
		switch ptype {
		case proxyTx:
			s.latencyTxMu.RLock()
			lat = s.latencyTx
			s.latencyTxMu.RUnlock()
		case proxyRx:
			s.latencyRxMu.RLock()
			lat = s.latencyRx
			s.latencyRxMu.RUnlock()
		default:
			panic("unknown proxy type")
		}
		if lat > 0 {
			s.lg.Debug(
				"before delay TX/RX",
				zap.String("data-received", humanize.Bytes(uint64(nr1))),
				zap.String("data-modified", humanize.Bytes(uint64(nr2))),
				zap.String("proxy listening on", s.Listen()),
				zap.Int("to peer port", peerPort),
				zap.Duration("latency", lat),
			)
			select {
			case <-time.After(lat):
			case <-s.donec:
				return
			}
			s.lg.Debug(
				"after delay TX/RX",
				zap.String("data-received", humanize.Bytes(uint64(nr1))),
				zap.String("data-modified", humanize.Bytes(uint64(nr2))),
				zap.String("proxy listening on", s.Listen()),
				zap.Int("to peer port", peerPort),
				zap.Duration("latency", lat),
			)
		}

		// now forward packets to target
		var nw int
		nw, err = dst.Write(data)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			select {
			case s.errc <- err:
				select {
				case <-s.donec:
					return
				default:
				}
			case <-s.donec:
				return
			}
			switch ptype {
			case proxyTx:
				s.lg.Debug("write fail on tx", zap.Error(err))
			case proxyRx:
				s.lg.Debug("write fail on rx", zap.Error(err))
			default:
				panic("unknown proxy type")
			}
			return
		}

		if nr2 != nw {
			select {
			case s.errc <- io.ErrShortWrite:
				select {
				case <-s.donec:
					return
				default:
				}
			case <-s.donec:
				return
			}
			switch ptype {
			case proxyTx:
				s.lg.Debug(
					"write fail on tx; read/write bytes are different",
					zap.Int("read-bytes", nr1),
					zap.Int("write-bytes", nw),
					zap.Error(io.ErrShortWrite),
				)
			case proxyRx:
				s.lg.Debug(
					"write fail on rx; read/write bytes are different",
					zap.Int("read-bytes", nr1),
					zap.Int("write-bytes", nw),
					zap.Error(io.ErrShortWrite),
				)
			default:
				panic("unknown proxy type")
			}
			return
		}

		switch ptype {
		case proxyTx:
			s.lg.Debug(
				"transmitted",
				zap.String("data-size", humanize.Bytes(uint64(nr1))),
				zap.String("proxy listening on", s.Listen()),
				zap.Int("to peer port", peerPort),
			)
		case proxyRx:
			s.lg.Debug(
				"received",
				zap.String("data-size", humanize.Bytes(uint64(nr1))),
				zap.String("proxy listening on", s.Listen()),
				zap.Int("to peer port", peerPort),
			)
		default:
			panic("unknown proxy type")
		}
	}
}

func (s *server) Ready() <-chan struct{} { return s.readyc }
func (s *server) Done() <-chan struct{}  { return s.donec }
func (s *server) Error() <-chan error    { return s.errc }
func (s *server) Close() (err error) {
	s.closeOnce.Do(func() {
		close(s.donec)

		// we shutdown the server
		log.Println("we shutdown the server")
		if err = s.httpServer.Shutdown(context.TODO()); err != nil {
			return
		}
		s.httpServer = nil

		log.Println("waiting for listenerMu")
		// listener was closed by the Shutdown() call
		s.listenerMu.Lock()
		s.listener = nil
		s.lg.Sync()
		s.listenerMu.Unlock()

		// the hijacked connections aren't tracked by the server so we need to wait for them
		log.Println("waiting for closeHijackedConn")
		s.closeHijackedConn.Wait()
	})
	s.closeWg.Wait()

	return err
}

func (s *server) DelayTx(latency, rv time.Duration) {
	if latency <= 0 {
		return
	}
	d := computeLatency(latency, rv)
	s.latencyTxMu.Lock()
	s.latencyTx = d
	s.latencyTxMu.Unlock()

	s.lg.Info(
		"set transmit latency",
		zap.Duration("latency", d),
		zap.Duration("given-latency", latency),
		zap.Duration("given-latency-random-variable", rv),
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) UndelayTx() {
	s.latencyTxMu.Lock()
	d := s.latencyTx
	s.latencyTx = 0
	s.latencyTxMu.Unlock()

	s.lg.Info(
		"removed transmit latency",
		zap.Duration("latency", d),
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) LatencyTx() time.Duration {
	s.latencyTxMu.RLock()
	d := s.latencyTx
	s.latencyTxMu.RUnlock()
	return d
}

func (s *server) DelayRx(latency, rv time.Duration) {
	if latency <= 0 {
		return
	}
	d := computeLatency(latency, rv)
	s.latencyRxMu.Lock()
	s.latencyRx = d
	s.latencyRxMu.Unlock()

	s.lg.Info(
		"set receive latency",
		zap.Duration("latency", d),
		zap.Duration("given-latency", latency),
		zap.Duration("given-latency-random-variable", rv),
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) UndelayRx() {
	s.latencyRxMu.Lock()
	d := s.latencyRx
	s.latencyRx = 0
	s.latencyRxMu.Unlock()

	s.lg.Info(
		"removed receive latency",
		zap.Duration("latency", d),
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) LatencyRx() time.Duration {
	s.latencyRxMu.RLock()
	d := s.latencyRx
	s.latencyRxMu.RUnlock()
	return d
}

func computeLatency(lat, rv time.Duration) time.Duration {
	if rv == 0 {
		return lat
	}
	if rv < 0 {
		rv *= -1
	}
	if rv > lat {
		rv = lat / 10
	}
	now := time.Now()
	sign := 1
	if now.Second()%2 == 0 {
		sign = -1
	}
	return lat + time.Duration(int64(sign)*mrand.Int63n(rv.Nanoseconds()))
}

func (s *server) ModifyTx(f func([]byte) []byte) {
	s.modifyTxMu.Lock()
	s.modifyTx = f
	s.modifyTxMu.Unlock()

	s.lg.Info(
		"modifying tx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) UnmodifyTx() {
	s.modifyTxMu.Lock()
	s.modifyTx = nil
	s.modifyTxMu.Unlock()

	s.lg.Info(
		"unmodifyed tx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) ModifyRx(f func([]byte) []byte) {
	s.modifyRxMu.Lock()
	s.modifyRx = f
	s.modifyRxMu.Unlock()
	s.lg.Info(
		"modifying rx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) UnmodifyRx() {
	s.modifyRxMu.Lock()
	s.modifyRx = nil
	s.modifyRxMu.Unlock()

	s.lg.Info(
		"unmodifyed rx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) BlackholeTx() {
	s.ModifyTx(func([]byte) []byte { return nil })
	s.lg.Info(
		"blackholed tx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) UnblackholeTx() {
	s.UnmodifyTx()
	s.lg.Info(
		"unblackholed tx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) BlackholeRx() {
	s.ModifyRx(func([]byte) []byte { return nil })
	s.lg.Info(
		"blackholed rx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) UnblackholeRx() {
	s.UnmodifyRx()
	s.lg.Info(
		"unblackholed rx",
		zap.String("proxy listening on", s.Listen()),
	)
}

func (s *server) BlackholePeerTx(peer url.URL) {
	s.blackholePeerMapMu.Lock()
	defer s.blackholePeerMapMu.Unlock()

	port, err := strconv.Atoi(peer.Port())
	if err != nil {
		panic("port parsing failed")
	}
	if val, exist := s.blackholePeerMap[port]; exist {
		val |= blackholePeerTypeTx
		s.blackholePeerMap[port] = val
	} else {
		s.blackholePeerMap[port] = blackholePeerTypeTx
	}
}

func (s *server) UnblackholePeerTx(peer url.URL) {
	s.blackholePeerMapMu.Lock()
	defer s.blackholePeerMapMu.Unlock()

	port, err := strconv.Atoi(peer.Port())
	if err != nil {
		panic("port parsing failed")
	}
	if val, exist := s.blackholePeerMap[port]; exist {
		val &= bits.Reverse8(blackholePeerTypeTx)
		s.blackholePeerMap[port] = val
	}
}

func (s *server) BlackholePeerRx(peer url.URL) {
	s.blackholePeerMapMu.Lock()
	defer s.blackholePeerMapMu.Unlock()

	port, err := strconv.Atoi(peer.Port())
	if err != nil {
		panic("port parsing failed")
	}
	if val, exist := s.blackholePeerMap[port]; exist {
		val |= blackholePeerTypeRx
		s.blackholePeerMap[port] = val
	} else {
		s.blackholePeerMap[port] = blackholePeerTypeTx
	}
}

func (s *server) UnblackholePeerRx(peer url.URL) {
	s.blackholePeerMapMu.Lock()
	defer s.blackholePeerMapMu.Unlock()

	port, err := strconv.Atoi(peer.Port())
	if err != nil {
		panic("port parsing failed")
	}
	if val, exist := s.blackholePeerMap[port]; exist {
		val &= bits.Reverse8(blackholePeerTypeRx)
		s.blackholePeerMap[port] = val
	}
}
