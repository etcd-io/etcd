// Copyright 2016 The etcd Authors
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
	"errors"
	"net"
	"time"
)

type listenerStoppable struct {
	net.Listener
	stopc <-chan struct{}
}

// NewListenerStoppable returns stoppable net.Listener.
func NewListenerStoppable(addr, scheme string, tlsConfig *tls.Config, stopc <-chan struct{}) (net.Listener, error) {
	ln, err := NewListener(addr, scheme, tlsConfig)
	if err != nil {
		return nil, err
	}
	ls := &listenerStoppable{
		Listener: ln,
		stopc:    stopc,
	}
	return ls, nil
}

// ErrListenerStopped is returned when the listener is stopped.
var ErrListenerStopped = errors.New("listener stopped")

func (ln *listenerStoppable) Accept() (net.Conn, error) {
	connc, errc := make(chan net.Conn, 1), make(chan error)
	go func() {
		conn, err := ln.Listener.Accept()
		if err != nil {
			errc <- err
			return
		}
		connc <- conn
	}()

	select {
	case <-ln.stopc:
		return nil, ErrListenerStopped
	case err := <-errc:
		return nil, err
	case conn := <-connc:
		tc, ok := conn.(*net.TCPConn)
		if ok {
			tc.SetKeepAlive(true)
			tc.SetKeepAlivePeriod(3 * time.Minute)
		}
		return conn, nil
	}
}
