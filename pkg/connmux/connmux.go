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
	// maxConnRead read ahead up to 512 bytes to identify if the connection is GRPC
	maxConnRead = 512
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
			c.lg.Error("connection accept error", zap.Error(err))
			return c.Close()
		}
		go c.serve(conn)
	}
}

func (c *ConnMux) serve(conn net.Conn) {
	// avoid to get blocked in any read operation
	conn.SetDeadline(time.Now().Add(readDeadlineTimeout))
	defer conn.SetReadDeadline(time.Time{})

	buf := make([]byte, maxConnRead)
	buffConn := newBufferedConn(conn)
	// Check if is TLS or plain TCP
	n, err := buffConn.sniffReader().Read(buf)
	if err != nil && err != io.EOF {
		// exit since it will panic trying to detect if is TLS
		c.lg.Error("error reading from the connection", zap.Error(err))
		conn.Close()
		return
	}
	r := bytes.NewReader(buf[:n])
	c.lg.Debug("connection received", zap.String("remote address", conn.RemoteAddr().String()), zap.Int("read size", n), zap.String("buffer content", string(buf[:n])))
	// Check if is TLS or plain TCP
	isTLS := buf[0] == 0x16
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
		// It is a "new" connection obtained after the handshake so we have to do another read
		proxiedConn := newBufferedConn(tlsConn)
		isGRPC := isGRPCConnection(c.lg, proxiedConn, proxiedConn.sniffReader())
		c.forward(isGRPC, proxiedConn)
	} else {
		if !c.insecure {
			c.lg.Error("insecure connections not enabled")
			conn.Close()
			return
		}
		isGRPC := isGRPCConnection(c.lg, buffConn, io.MultiReader(r, buffConn.sniffReader()))
		c.forward(isGRPC, buffConn)
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

	if isGRPC && c.grpc != nil {
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
	net.Conn
	buf *bytes.Buffer
}

var _ net.Conn = &bufferedConn{}

func newBufferedConn(c net.Conn) *bufferedConn {
	return &bufferedConn{
		Conn: c,
		buf:  new(bytes.Buffer),
	}
}

func (b *bufferedConn) Read(p []byte) (int, error) {
	if b.buf == nil {
		return b.Conn.Read(p)
	}
	n := b.buf.Len()
	if n == 0 {
		b.buf = nil
		return b.Conn.Read(p)
	}
	if n < len(p) {
		p = p[:n]
	}
	return b.buf.Read(p)
}

func (b *bufferedConn) Close() error {
	if b.buf != nil {
		b.buf = nil
	}
	return b.Conn.Close()
}

func (b *bufferedConn) sniffReader() io.Reader {
	return io.TeeReader(b.Conn, b.buf)
}

func isGRPCConnection(lg *zap.Logger, w io.Writer, r io.Reader) bool {
	// check the http2 client preface
	buf := make([]byte, len(http2.ClientPreface))
	n, err := r.Read(buf)
	if err != nil || n < len(http2.ClientPreface) {
		return false
	}

	if !bytes.Equal(buf, []byte(http2.ClientPreface)) {
		lg.Debug("not found http2 client preface", zap.String("preface", string(buf)))
		return false
	}
	lg.Debug("found http2 client preface", zap.Int("bytes read", n))

	framer := http2.NewFramer(w, r)
	// identify GRPC connections matching match headers names or values defined
	// on https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md
	done := false
	isGRPC := false
	// use the default value for the maxDynamixTable size
	// https://pkg.go.dev/golang.org/x/net/http2
	// "If zero, the default value of 4096 is used."
	hdec := hpack.NewDecoder(4096, func(hf hpack.HeaderField) {
		// If Content-Type does not begin with "application/grpc", gRPC servers SHOULD respond with HTTP status of 415 (Unsupported Media Type).
		// This will prevent other HTTP/2 clients from interpreting a gRPC error response, which uses status 200 (OK), as successful.
		lg.Debug("found header", zap.String("name", hf.Name), zap.String("value", hf.Value))
		if hf.Name == "content-type" {
			isGRPC = strings.HasPrefix(hf.Value, "application/grpc")
			done = true
		}
		if hf.Name == ":method" && hf.Value != "POST" {
			isGRPC = false
			done = true
		}
	})

	lg.Debug("reading frames")
	for !done {
		f, err := framer.ReadFrame()
		if err != nil {
			lg.Debug("error reading frame", zap.Error(err))
			return false
		}
		switch f := f.(type) {
		//  The SETTINGS frames received from a peer as part of the connection
		// preface MUST be acknowledged (see Section 6.5.3) after sending the
		// connection preface.
		case *http2.SettingsFrame:
			if f.IsAck() {
				lg.Debug("found setting ACK frame")
				continue
			}
			// only write the settings frame if length is zero
			// otherwise when passed to an http2 server with prior knowledge
			// it will send a PROTOCOL_ERROR
			if f.Length == 0 {
				lg.Debug("write http2 settings")
				err = framer.WriteSettings()
				if err != nil {
					lg.Debug("error sending setting frame", zap.Error(err))
					return false
				}
			}
		case *http2.ContinuationFrame:
			if _, err := hdec.Write(f.HeaderBlockFragment()); err != nil {
				return false
			}
		case *http2.HeadersFrame:
			if _, err := hdec.Write(f.HeaderBlockFragment()); err != nil {
				return false
			}
		}
	}
	return isGRPC
}
