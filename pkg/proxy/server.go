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
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/pkg/transport"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Server defines proxy server layer that simulates common network faults,
// such as latency spikes, packet drop/corruption, etc..
type Server interface {
	// From returns proxy source address in "scheme://host:port" format.
	From() string
	// To returns proxy destination address in "scheme://host:port" format.
	To() string

	// Ready returns when proxy is ready to serve.
	Ready() <-chan struct{}
	// Done returns when proxy has been closed.
	Done() <-chan struct{}
	// Error sends errors while serving proxy.
	Error() <-chan error
	// Close closes listener and transport.
	Close() error

	// DelayAccept adds latency ± random variable to accepting new incoming connections.
	DelayAccept(latency, rv time.Duration)
	// UndelayAccept removes sending latencies.
	UndelayAccept()
	// LatencyAccept returns current latency on accepting new incoming connections.
	LatencyAccept() time.Duration
	// DelayTx adds latency ± random variable to "sending" layer.
	DelayTx(latency, rv time.Duration)
	// UndelayTx removes sending latencies.
	UndelayTx()
	// LatencyTx returns current send latency.
	LatencyTx() time.Duration
	// DelayRx adds latency ± random variable to "receiving" layer.
	DelayRx(latency, rv time.Duration)
	// UndelayRx removes "receiving" latencies.
	UndelayRx()
	// LatencyRx returns current receive latency.
	LatencyRx() time.Duration

	// PauseAccept stops accepting new connections.
	PauseAccept()
	// UnpauseAccept removes pause operation on accepting new connections.
	UnpauseAccept()
	// PauseTx stops "forwarding" packets.
	PauseTx()
	// UnpauseTx removes "forwarding" pause operation.
	UnpauseTx()
	// PauseRx stops "receiving" packets to client.
	PauseRx()
	// UnpauseRx removes "receiving" pause operation.
	UnpauseRx()

	// BlackholeTx drops all incoming packets before "forwarding".
	BlackholeTx()
	// UnblackholeTx removes blackhole operation on "sending".
	UnblackholeTx()
	// BlackholeRx drops all incoming packets to client.
	BlackholeRx()
	// UnblackholeRx removes blackhole operation on "receiving".
	UnblackholeRx()

	// CorruptTx corrupts incoming packets from the listener.
	CorruptTx(f func(data []byte) []byte)
	// UncorruptTx removes corrupt operation on "forwarding".
	UncorruptTx()
	// CorruptRx corrupts incoming packets to client.
	CorruptRx(f func(data []byte) []byte)
	// UncorruptRx removes corrupt operation on "receiving".
	UncorruptRx()

	// ResetListener closes and restarts listener.
	ResetListener() error
}

type proxyServer struct {
	lg *zap.Logger

	from, to      url.URL
	tlsInfo       transport.TLSInfo
	dialTimeout   time.Duration
	bufferSize    int
	retryInterval time.Duration

	readyc chan struct{}
	donec  chan struct{}
	errc   chan error

	closeOnce sync.Once
	closeWg   sync.WaitGroup

	listenerMu sync.RWMutex
	listener   net.Listener

	latencyAcceptMu sync.RWMutex
	latencyAccept   time.Duration
	latencyTxMu     sync.RWMutex
	latencyTx       time.Duration
	latencyRxMu     sync.RWMutex
	latencyRx       time.Duration

	corruptTxMu sync.RWMutex
	corruptTx   func(data []byte) []byte
	corruptRxMu sync.RWMutex
	corruptRx   func(data []byte) []byte

	acceptMu     sync.Mutex
	pauseAcceptc chan struct{}
	txMu         sync.Mutex
	pauseTxc     chan struct{}
	blackholeTxc chan struct{}
	rxMu         sync.Mutex
	pauseRxc     chan struct{}
	blackholeRxc chan struct{}
}

// ServerConfig defines proxy server configuration.
type ServerConfig struct {
	Logger        *zap.Logger
	From          url.URL
	To            url.URL
	TLSInfo       transport.TLSInfo
	DialTimeout   time.Duration
	BufferSize    int
	RetryInterval time.Duration
}

var (
	defaultDialTimeout   = 3 * time.Second
	defaultBufferSize    = 48 * 1024
	defaultRetryInterval = 10 * time.Millisecond
	defaultLogger        *zap.Logger
)

func init() {
	var err error
	defaultLogger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

// NewServer returns a proxy implementation with no iptables/tc dependencies.
// The proxy layer overhead is <1ms.
func NewServer(cfg ServerConfig) Server {
	p := &proxyServer{
		lg: cfg.Logger,

		from:          cfg.From,
		to:            cfg.To,
		tlsInfo:       cfg.TLSInfo,
		dialTimeout:   cfg.DialTimeout,
		bufferSize:    cfg.BufferSize,
		retryInterval: cfg.RetryInterval,

		readyc: make(chan struct{}),
		donec:  make(chan struct{}),
		errc:   make(chan error, 16),

		pauseAcceptc: make(chan struct{}),
		pauseTxc:     make(chan struct{}),
		blackholeTxc: make(chan struct{}),
		pauseRxc:     make(chan struct{}),
		blackholeRxc: make(chan struct{}),
	}
	if p.dialTimeout == 0 {
		p.dialTimeout = defaultDialTimeout
	}
	if p.bufferSize == 0 {
		p.bufferSize = defaultBufferSize
	}
	if p.retryInterval == 0 {
		p.retryInterval = defaultRetryInterval
	}
	if p.lg == nil {
		p.lg = defaultLogger
	}
	close(p.pauseAcceptc)
	close(p.pauseTxc)
	close(p.pauseRxc)

	if strings.HasPrefix(p.from.Scheme, "http") {
		p.from.Scheme = "tcp"
	}
	if strings.HasPrefix(p.to.Scheme, "http") {
		p.to.Scheme = "tcp"
	}

	var ln net.Listener
	var err error
	if !p.tlsInfo.Empty() {
		ln, err = transport.NewListener(p.from.Host, p.from.Scheme, &p.tlsInfo)
	} else {
		ln, err = net.Listen(p.from.Scheme, p.from.Host)
	}
	if err != nil {
		p.errc <- err
		p.Close()
		return p
	}
	p.listener = ln

	p.closeWg.Add(1)
	go p.listenAndServe()

	p.lg.Info("started proxying", zap.String("from", p.From()), zap.String("to", p.To()))
	return p
}

func (p *proxyServer) From() string {
	return fmt.Sprintf("%s://%s", p.from.Scheme, p.from.Host)
}

func (p *proxyServer) To() string {
	return fmt.Sprintf("%s://%s", p.to.Scheme, p.to.Host)
}

// TODO: implement packet reordering from multiple TCP connections
// buffer packets per connection for awhile, reorder before transmit
// - https://github.com/coreos/etcd/issues/5614
// - https://github.com/coreos/etcd/pull/6918#issuecomment-264093034

func (p *proxyServer) listenAndServe() {
	defer p.closeWg.Done()

	p.lg.Info("proxy is listening on", zap.String("from", p.From()))
	close(p.readyc)

	for {
		p.acceptMu.Lock()
		pausec := p.pauseAcceptc
		p.acceptMu.Unlock()
		select {
		case <-pausec:
		case <-p.donec:
			return
		}

		p.latencyAcceptMu.RLock()
		lat := p.latencyAccept
		p.latencyAcceptMu.RUnlock()
		if lat > 0 {
			select {
			case <-time.After(lat):
			case <-p.donec:
				return
			}
		}

		p.listenerMu.RLock()
		ln := p.listener
		p.listenerMu.RUnlock()

		in, err := ln.Accept()
		if err != nil {
			select {
			case p.errc <- err:
				select {
				case <-p.donec:
					return
				default:
				}
			case <-p.donec:
				return
			}
			p.lg.Debug("listener accept error", zap.Error(err))

			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				select {
				case <-time.After(p.retryInterval):
				case <-p.donec:
					return
				}
				p.lg.Debug("listener is closed; retry listening on", zap.String("from", p.From()))

				if err = p.ResetListener(); err != nil {
					select {
					case p.errc <- err:
						select {
						case <-p.donec:
							return
						default:
						}
					case <-p.donec:
						return
					}
					p.lg.Warn("failed to reset listener", zap.Error(err))
				}
			}

			continue
		}

		var out net.Conn
		if !p.tlsInfo.Empty() {
			var tp *http.Transport
			tp, err = transport.NewTransport(p.tlsInfo, p.dialTimeout)
			if err != nil {
				select {
				case p.errc <- err:
					select {
					case <-p.donec:
						return
					default:
					}
				case <-p.donec:
					return
				}
				continue
			}
			out, err = tp.Dial(p.to.Scheme, p.to.Host)
		} else {
			out, err = net.Dial(p.to.Scheme, p.to.Host)
		}
		if err != nil {
			select {
			case p.errc <- err:
				select {
				case <-p.donec:
					return
				default:
				}
			case <-p.donec:
				return
			}
			p.lg.Debug("failed to dial", zap.Error(err))
			continue
		}

		go func() {
			// read incoming bytes from listener, dispatch to outgoing connection
			p.transmit(out, in)
			out.Close()
			in.Close()
		}()
		go func() {
			// read response from outgoing connection, write back to listener
			p.receive(in, out)
			in.Close()
			out.Close()
		}()
	}
}

func (p *proxyServer) transmit(dst io.Writer, src io.Reader) { p.ioCopy(dst, src, true) }
func (p *proxyServer) receive(dst io.Writer, src io.Reader)  { p.ioCopy(dst, src, false) }
func (p *proxyServer) ioCopy(dst io.Writer, src io.Reader, proxySend bool) {
	buf := make([]byte, p.bufferSize)
	for {
		nr, err := src.Read(buf)
		if err != nil {
			if err == io.EOF {
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
			case p.errc <- err:
				select {
				case <-p.donec:
					return
				default:
				}
			case <-p.donec:
				return
			}
			p.lg.Debug("failed to read", zap.Error(err))
			return
		}
		if nr == 0 {
			return
		}
		data := buf[:nr]

		var pausec chan struct{}
		var blackholec chan struct{}
		if proxySend {
			p.txMu.Lock()
			pausec = p.pauseTxc
			blackholec = p.blackholeTxc
			p.txMu.Unlock()
		} else {
			p.rxMu.Lock()
			pausec = p.pauseRxc
			blackholec = p.blackholeRxc
			p.rxMu.Unlock()
		}
		select {
		case <-pausec:
		case <-p.donec:
			return
		}
		blackholed := false
		select {
		case <-blackholec:
			blackholed = true
		case <-p.donec:
			return
		default:
		}
		if blackholed {
			if proxySend {
				p.lg.Debug(
					"dropped",
					zap.String("data-size", humanize.Bytes(uint64(nr))),
					zap.String("from", p.From()),
					zap.String("to", p.To()),
				)
			} else {
				p.lg.Debug(
					"dropped",
					zap.String("data-size", humanize.Bytes(uint64(nr))),
					zap.String("from", p.To()),
					zap.String("to", p.From()),
				)
			}
			continue
		}

		var lat time.Duration
		if proxySend {
			p.latencyTxMu.RLock()
			lat = p.latencyTx
			p.latencyTxMu.RUnlock()
		} else {
			p.latencyRxMu.RLock()
			lat = p.latencyRx
			p.latencyRxMu.RUnlock()
		}
		if lat > 0 {
			select {
			case <-time.After(lat):
			case <-p.donec:
				return
			}
		}

		if proxySend {
			p.corruptTxMu.RLock()
			if p.corruptTx != nil {
				data = p.corruptTx(data)
			}
			p.corruptTxMu.RUnlock()
		} else {
			p.corruptRxMu.RLock()
			if p.corruptRx != nil {
				data = p.corruptRx(data)
			}
			p.corruptRxMu.RUnlock()
		}

		var nw int
		nw, err = dst.Write(data)
		if err != nil {
			if err == io.EOF {
				return
			}
			select {
			case p.errc <- err:
				select {
				case <-p.donec:
					return
				default:
				}
			case <-p.donec:
				return
			}
			if proxySend {
				p.lg.Debug("failed to write while sending", zap.Error(err))
			} else {
				p.lg.Debug("failed to write while receiving", zap.Error(err))
			}
			return
		}

		if nr != nw {
			select {
			case p.errc <- io.ErrShortWrite:
				select {
				case <-p.donec:
					return
				default:
				}
			case <-p.donec:
				return
			}
			if proxySend {
				p.lg.Debug(
					"failed to write while sending; read/write bytes are different",
					zap.Int("read-bytes", nr),
					zap.Int("write-bytes", nw),
					zap.Error(io.ErrShortWrite),
				)
			} else {
				p.lg.Debug(
					"failed to write while receiving; read/write bytes are different",
					zap.Int("read-bytes", nr),
					zap.Int("write-bytes", nw),
					zap.Error(io.ErrShortWrite),
				)
			}
			return
		}

		if proxySend {
			p.lg.Debug(
				"transmitted",
				zap.String("data-size", humanize.Bytes(uint64(nr))),
				zap.String("from", p.From()),
				zap.String("to", p.To()),
			)
		} else {
			p.lg.Debug(
				"received",
				zap.String("data-size", humanize.Bytes(uint64(nr))),
				zap.String("from", p.To()),
				zap.String("to", p.From()),
			)
		}

	}
}

func (p *proxyServer) Ready() <-chan struct{} { return p.readyc }
func (p *proxyServer) Done() <-chan struct{}  { return p.donec }
func (p *proxyServer) Error() <-chan error    { return p.errc }
func (p *proxyServer) Close() (err error) {
	p.closeOnce.Do(func() {
		close(p.donec)
		p.listenerMu.Lock()
		if p.listener != nil {
			err = p.listener.Close()
			p.lg.Info(
				"closed proxy listener",
				zap.String("from", p.From()),
				zap.String("to", p.To()),
			)
		}
		p.lg.Sync()
		p.listenerMu.Unlock()
	})
	p.closeWg.Wait()
	return err
}

func (p *proxyServer) DelayAccept(latency, rv time.Duration) {
	if latency <= 0 {
		return
	}
	d := computeLatency(latency, rv)
	p.latencyAcceptMu.Lock()
	p.latencyAccept = d
	p.latencyAcceptMu.Unlock()

	p.lg.Info(
		"set accept latency",
		zap.Duration("latency", d),
		zap.Duration("given-latency", latency),
		zap.Duration("given-latency-random-variable", rv),
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) UndelayAccept() {
	p.latencyAcceptMu.Lock()
	d := p.latencyAccept
	p.latencyAccept = 0
	p.latencyAcceptMu.Unlock()

	p.lg.Info(
		"removed accept latency",
		zap.Duration("latency", d),
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) LatencyAccept() time.Duration {
	p.latencyAcceptMu.RLock()
	d := p.latencyAccept
	p.latencyAcceptMu.RUnlock()
	return d
}

func (p *proxyServer) DelayTx(latency, rv time.Duration) {
	if latency <= 0 {
		return
	}
	d := computeLatency(latency, rv)
	p.latencyTxMu.Lock()
	p.latencyTx = d
	p.latencyTxMu.Unlock()

	p.lg.Info(
		"set transmit latency",
		zap.Duration("latency", d),
		zap.Duration("given-latency", latency),
		zap.Duration("given-latency-random-variable", rv),
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) UndelayTx() {
	p.latencyTxMu.Lock()
	d := p.latencyTx
	p.latencyTx = 0
	p.latencyTxMu.Unlock()

	p.lg.Info(
		"removed transmit latency",
		zap.Duration("latency", d),
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) LatencyTx() time.Duration {
	p.latencyTxMu.RLock()
	d := p.latencyTx
	p.latencyTxMu.RUnlock()
	return d
}

func (p *proxyServer) DelayRx(latency, rv time.Duration) {
	if latency <= 0 {
		return
	}
	d := computeLatency(latency, rv)
	p.latencyRxMu.Lock()
	p.latencyRx = d
	p.latencyRxMu.Unlock()

	p.lg.Info(
		"set receive latency",
		zap.Duration("latency", d),
		zap.Duration("given-latency", latency),
		zap.Duration("given-latency-random-variable", rv),
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) UndelayRx() {
	p.latencyRxMu.Lock()
	d := p.latencyRx
	p.latencyRx = 0
	p.latencyRxMu.Unlock()

	p.lg.Info(
		"removed receive latency",
		zap.Duration("latency", d),
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) LatencyRx() time.Duration {
	p.latencyRxMu.RLock()
	d := p.latencyRx
	p.latencyRxMu.RUnlock()
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
	mrand.Seed(int64(now.Nanosecond()))
	sign := 1
	if now.Second()%2 == 0 {
		sign = -1
	}
	return lat + time.Duration(int64(sign)*mrand.Int63n(rv.Nanoseconds()))
}

func (p *proxyServer) PauseAccept() {
	p.acceptMu.Lock()
	p.pauseAcceptc = make(chan struct{})
	p.acceptMu.Unlock()

	p.lg.Info(
		"paused accepting new connections",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) UnpauseAccept() {
	p.acceptMu.Lock()
	select {
	case <-p.pauseAcceptc: // already unpaused
	case <-p.donec:
		p.acceptMu.Unlock()
		return
	default:
		close(p.pauseAcceptc)
	}
	p.acceptMu.Unlock()

	p.lg.Info(
		"unpaused accepting new connections",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) PauseTx() {
	p.txMu.Lock()
	p.pauseTxc = make(chan struct{})
	p.txMu.Unlock()

	p.lg.Info(
		"paused transmit listen",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) UnpauseTx() {
	p.txMu.Lock()
	select {
	case <-p.pauseTxc: // already unpaused
	case <-p.donec:
		p.txMu.Unlock()
		return
	default:
		close(p.pauseTxc)
	}
	p.txMu.Unlock()

	p.lg.Info(
		"unpaused transmit listen",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) PauseRx() {
	p.rxMu.Lock()
	p.pauseRxc = make(chan struct{})
	p.rxMu.Unlock()

	p.lg.Info(
		"paused receive listen",
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) UnpauseRx() {
	p.rxMu.Lock()
	select {
	case <-p.pauseRxc: // already unpaused
	case <-p.donec:
		p.rxMu.Unlock()
		return
	default:
		close(p.pauseRxc)
	}
	p.rxMu.Unlock()

	p.lg.Info(
		"unpaused receive listen",
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) BlackholeTx() {
	p.txMu.Lock()
	select {
	case <-p.blackholeTxc: // already blackholed
	case <-p.donec:
		p.txMu.Unlock()
		return
	default:
		close(p.blackholeTxc)
	}
	p.txMu.Unlock()

	p.lg.Info(
		"blackholed transmit",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) UnblackholeTx() {
	p.txMu.Lock()
	p.blackholeTxc = make(chan struct{})
	p.txMu.Unlock()

	p.lg.Info(
		"unblackholed transmit",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) BlackholeRx() {
	p.rxMu.Lock()
	select {
	case <-p.blackholeRxc: // already blackholed
	case <-p.donec:
		p.rxMu.Unlock()
		return
	default:
		close(p.blackholeRxc)
	}
	p.rxMu.Unlock()

	p.lg.Info(
		"blackholed receive",
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) UnblackholeRx() {
	p.rxMu.Lock()
	p.blackholeRxc = make(chan struct{})
	p.rxMu.Unlock()

	p.lg.Info(
		"unblackholed receive",
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) CorruptTx(f func([]byte) []byte) {
	p.corruptTxMu.Lock()
	p.corruptTx = f
	p.corruptTxMu.Unlock()

	p.lg.Info(
		"corrupting transmit",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) UncorruptTx() {
	p.corruptTxMu.Lock()
	p.corruptTx = nil
	p.corruptTxMu.Unlock()

	p.lg.Info(
		"stopped corrupting transmit",
		zap.String("from", p.From()),
		zap.String("to", p.To()),
	)
}

func (p *proxyServer) CorruptRx(f func([]byte) []byte) {
	p.corruptRxMu.Lock()
	p.corruptRx = f
	p.corruptRxMu.Unlock()
	p.lg.Info(
		"corrupting receive",
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) UncorruptRx() {
	p.corruptRxMu.Lock()
	p.corruptRx = nil
	p.corruptRxMu.Unlock()

	p.lg.Info(
		"stopped corrupting receive",
		zap.String("from", p.To()),
		zap.String("to", p.From()),
	)
}

func (p *proxyServer) ResetListener() error {
	p.listenerMu.Lock()
	defer p.listenerMu.Unlock()

	if err := p.listener.Close(); err != nil {
		// already closed
		if !strings.HasSuffix(err.Error(), "use of closed network connection") {
			return err
		}
	}

	var ln net.Listener
	var err error
	if !p.tlsInfo.Empty() {
		ln, err = transport.NewListener(p.from.Host, p.from.Scheme, &p.tlsInfo)
	} else {
		ln, err = net.Listen(p.from.Scheme, p.from.Host)
	}
	if err != nil {
		return err
	}
	p.listener = ln

	p.lg.Info(
		"reset listener on",
		zap.String("from", p.From()),
	)
	return nil
}
