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

package connmux

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const (
	// gracefullShutdownDuration is the time to wait before closing the listeners
	gracefullShutdownDuration = 200 * time.Millisecond
	// readDeadlineTimeout is the maximum time to wait for a read operation
	readDeadlineTimeout = 1 * time.Second
)

// ConnMux can multiple multiple HTTP and GRPC connections in the same address and port.
// It is able to multiplex connections with and without TLS.
// ConnMux can only forward connections to one HTTP or one GRPC server.
// If only one of those is available it will forward all connections directly.
type ConnMux struct {
	lg    *zap.Logger
	root  net.Listener
	donec chan struct{}

	secure    bool // serve TLS
	insecure  bool // serve insecure
	tlsConfig *tls.Config

	mu   sync.Mutex
	http *muxListener
	grpc *muxListener
}

type Config struct {
	Logger    *zap.Logger
	Listener  net.Listener
	Secure    bool
	Insecure  bool
	TLSConfig *tls.Config
}

// New creates a new ConnMux.
func New(cfg Config) *ConnMux {
	// defaulting
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	return &ConnMux{
		lg:        cfg.Logger,
		root:      cfg.Listener,
		insecure:  cfg.Insecure,
		secure:    cfg.Secure,
		tlsConfig: cfg.TLSConfig,
		donec:     make(chan struct{}),
	}
}

// Serve starts serving connections
func (c *ConnMux) Serve() error {
	for {
		conn, err := c.root.Accept()
		if err != nil {
			c.lg.Error("connection error", zap.Error(err))
			return c.Close()
		}
		go c.serve(conn)
	}
}

// peekHTTP2PrefaceBytes define the bytes we need to peek to be able to differentiate between HTTP and GRPC.
// Since GRPC uses HTTP2 we need to match the connection preface
// https://httpwg.org/specs/rfc9113.html#preface (24 octects)
// 3.5. HTTP/2 Connection Preface
// The client connection preface starts with a sequence of 24 octets, which in hex notation is:
// 0x505249202a20485454502f322e300d0a0d0a534d0d0a0d0a
// That is, the connection preface starts with the string PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n).
// This sequence MUST be followed by a SETTINGS frame (Section 6.5), which MAY be empty.
// 4.1. Frame Format
// All frames begin with a fixed 9-octet header followed by a variable-length payload.
// min peek size is 24 + 9 = 33
const peekHTTP2PrefaceBytes = 33

func (c *ConnMux) serve(conn net.Conn) {
	// avoid to get blocked in any read operation
	conn.SetDeadline(time.Now().Add(readDeadlineTimeout))
	defer conn.SetReadDeadline(time.Time{})

	var proxiedConn bufferedConn

	buffConn := newBufferedConn(conn)
	// Check if is TLS or plain TCP
	b, err := buffConn.Peek(peekHTTP2PrefaceBytes)
	if err != nil {
		// exit since it will panic trying to detect if is TLS
		c.lg.Error("error reading from the connection", zap.Error(err))
		conn.Close()
		return
	}
	// is TLS
	isTLS := b[0] == 0x16
	if isTLS {
		if !c.secure {
			c.lg.Error("secure connections not enabled")
			conn.Close()
			return
		}
		// Establish the TLS connection
		tlsConn := tls.Server(buffConn, c.tlsConfig)
		err = tlsConn.Handshake()
		if err != nil {
			c.lg.Error("error establishing the TLS connection", zap.Error(err))
			conn.Close()
			return
		}

		proxiedConn = newBufferedConn(tlsConn)
		// It is a "new" connection obtained after the handshake so we have to do another read
		// to get the new data, but be careful to not get blocked in the read if there are no enough data.
		_, err = proxiedConn.Peek(peekHTTP2PrefaceBytes)
		if err != nil {
			// try to see how far we go, it will be discarded by the server if we route it to the wrong one
			c.lg.Error("error reading from the TLS connection", zap.Error(err))
		}
	} else {
		if !c.insecure {
			c.lg.Error("insecure connections not enabled")
			conn.Close()
			return
		}
		proxiedConn = buffConn
	}

	// read the whole buffer
	b, err = proxiedConn.Peek(proxiedConn.r.Buffered())
	if err != nil && err != io.EOF {
		c.lg.Error("error reading", zap.Error(err))
		conn.Close()
		return
	}
	c.lg.Debug("connection received", zap.String("remote address", conn.RemoteAddr().String()), zap.Int("buffer size", len(b)), zap.String("buffer content", string(b)))
	reader := bytes.NewReader(b)
	isHTTP2 := isHTTP2Connection(reader)
	// if is not http2 it is not grpc
	if !isHTTP2 {
		c.forward(false, proxiedConn)
	} else {
		c.forward(isGRPCConnection(c.lg, reader), proxiedConn)
	}
}

func (c *ConnMux) forward(isGRPC bool, conn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !isGRPC && c.http != nil {
		c.lg.Debug("forwarding connection to the HTTP backend", zap.String("remote address", conn.RemoteAddr().String()))
		select {
		case c.http.connc <- conn:
		case <-c.donec:
		}
		return
	}

	if c.grpc != nil {
		c.lg.Debug("forwarding connection to the GRPC backend", zap.String("remote address", conn.RemoteAddr().String()))
		select {
		case c.grpc.connc <- conn:
		case <-c.donec:
		}
		return
	}

	log.Println("unknown connection")
	conn.Close()
}

// Close closes the listeners
func (c *ConnMux) Close() error {
	time.Sleep(gracefullShutdownDuration)
	c.closeDoneChans()
	return c.root.Close()
}

func (c *ConnMux) closeDoneChans() {
	select {
	case <-c.donec:
	default:
		close(c.donec)
	}
}

// muxListener is the listener exposed to the HTTP and GRPC servers
// The root listener Accept() method is overriden
// The multiplexed servers have access to the Close() and Address()
// methods on the root listener.
type muxListener struct {
	net.Listener
	connc chan net.Conn
	donec chan struct{}
}

var _ net.Listener = (*muxListener)(nil)

func (l muxListener) Accept() (net.Conn, error) {
	select {
	case c, ok := <-l.connc:
		if !ok {
			return nil, net.ErrClosed
		}
		return c, nil
	case <-l.donec:
		return nil, net.ErrClosed
	}
}

// HTTPListener returns a net.Listener that will receive http requests
func (c *ConnMux) HTTPListener() net.Listener {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.http == nil {
		c.http = &muxListener{c.root, make(chan net.Conn), c.donec}
	}
	return c.http
}

// GRPCListener returns a net.Listener that will receive grpc requests
func (c *ConnMux) GRPCListener() net.Listener {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.grpc == nil {
		c.grpc = &muxListener{c.root, make(chan net.Conn), c.donec}
	}
	return c.grpc
}

// bufferedConn allows to peek in the buffer of the connection
// without advancing the reader.
type bufferedConn struct {
	r *bufio.Reader
	net.Conn
}

func newBufferedConn(c net.Conn) bufferedConn {
	return bufferedConn{bufio.NewReader(c), c}
}

func (b bufferedConn) Peek(n int) ([]byte, error) {
	return b.r.Peek(n)
}

func (b bufferedConn) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

func isHTTP2Connection(r io.Reader) bool {
	// check the http2 client preface
	buf := make([]byte, len(http2.ClientPreface))
	n, err := r.Read(buf)
	if err != nil || n != len(http2.ClientPreface) {
		return false
	}
	return bytes.Equal(buf, []byte(http2.ClientPreface))
}

func isGRPCConnection(lg *zap.Logger, r io.Reader) bool {
	done := false
	isGRPC := false
	// check if len of the settings frame is 0 or the headers with the "content-type"
	// indicates is an grpc connection.
	framer := http2.NewFramer(io.Discard, r)
	// use the default value for the maxDynamixTable size
	// https://pkg.go.dev/golang.org/x/net/http2
	// "If zero, the default value of 4096 is used."
	hdec := hpack.NewDecoder(4096, func(hf hpack.HeaderField) {
		// match headers names or values based on https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md
		if strings.Contains(hf.Name, "grpc") {
			isGRPC = true
			done = true
			lg.Debug("found grpc header", zap.String("header", hf.Name))
			return
		}
		if hf.Name == "content-type" {
			isGRPC = strings.Contains(hf.Value, "application/grpc")
			lg.Debug("found content-type header", zap.String("content-type", hf.Value))
		}
		done = true
	})
	for {
		f, err := framer.ReadFrame()
		if err != nil {
			break
		}
		switch f := f.(type) {
		case *http2.SettingsFrame:
			// Observed behavior is that etcd GRPC clients sends an empty setting frame
			// and block waiting for an answer.
			if f.Length == 0 {
				isGRPC = true
				done = true
				lg.Debug("found setting frame with zero length")
			}
		case *http2.HeadersFrame:
			hdec.Write(f.HeaderBlockFragment())
		}
		if done {
			break
		}
	}
	return isGRPC
}
