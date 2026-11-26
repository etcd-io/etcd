// Copyright 2013 The etcd Authors
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

// Package transport provides network utility functions, complementing the more
// common ones in the net package.
package transport

import (
	"errors"
	"net"
	"sync"
	"time"
)

var ErrNotTCP = errors.New("only tcp connections have keepalive")

// LimitListener returns a Listener that accepts at most n simultaneous
// connections from the provided Listener.
func LimitListener(l net.Listener, n int) net.Listener {
	return &limitListener{Listener: l, sem: make(chan struct{}, n), done: make(chan struct{})}
}

type limitListener struct {
	net.Listener
	sem       chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func (l *limitListener) acquire() error {
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-l.done:
		return net.ErrClosed
	}
}
func (l *limitListener) release() {
	select {
	case <-l.sem:
	default:
	}
}
func (l *limitListener) Close() error {
	l.closeOnce.Do(func() { close(l.done) })
	return l.Listener.Close()
}

func (l *limitListener) Accept() (net.Conn, error) {
	if err := l.acquire(); err != nil {
		return nil, err
	}
	c, err := l.Listener.Accept()
	if err != nil {
		l.release()
		return nil, err
	}
	select {
	case <-l.done:
		l.release()
		c.Close()
		return nil, net.ErrClosed
	default:
	}
	return &limitListenerConn{Conn: c, release: l.release}, nil
}

type limitListenerConn struct {
	net.Conn
	releaseOnce sync.Once
	release     func()
}

func (l *limitListenerConn) Close() error {
	err := l.Conn.Close()
	l.releaseOnce.Do(l.release)
	return err
}

// SetKeepAlive sets keepalive
//
// Deprecated: use (*keepAliveConn) SetKeepAlive instead.
func (l *limitListenerConn) SetKeepAlive(doKeepAlive bool) error {
	tcpc, ok := l.Conn.(*net.TCPConn)
	if !ok {
		return ErrNotTCP
	}
	return tcpc.SetKeepAlive(doKeepAlive)
}

// SetKeepAlivePeriod sets keepalive period
//
// Deprecated: use (*keepAliveConn) SetKeepAlivePeriod instead.
func (l *limitListenerConn) SetKeepAlivePeriod(d time.Duration) error {
	tcpc, ok := l.Conn.(*net.TCPConn)
	if !ok {
		return ErrNotTCP
	}
	return tcpc.SetKeepAlivePeriod(d)
}
