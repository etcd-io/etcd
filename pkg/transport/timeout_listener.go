// Copyright 2015 The etcd Authors
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
	"context"
	"fmt"
	"net"
	"time"
)

// NewTimeoutListener returns a listener that listens on the given address.
// If read/write on the accepted connection blocks longer than its time limit,
// it will return timeout error.
func NewTimeoutListener(addr string, scheme string, tlsinfo *TLSInfo, rdtimeoutd, wtimeoutd time.Duration) (net.Listener, error) {
	ln, err := newListener(addr, scheme)
	if err != nil {
		return nil, err
	}
	return newTimeoutListener(ln, addr, scheme, rdtimeoutd, wtimeoutd, tlsinfo)
}

func NewTimeoutListerWithSocketOpts(addr string, scheme string, tlsinfo *TLSInfo, rdtimeoutd, wtimeoutd time.Duration, sopts SocketOpts ) (net.Listener, error) {
	config, err := newListenConfig(addr, scheme, sopts)
	if err != nil {
		return nil, err
	}
	ln, err := config.Listen(context.TODO(), scheme, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %v: %v", addr, err)
	}
	return newTimeoutListener(ln, addr, scheme, rdtimeoutd, wtimeoutd, tlsinfo)
}

func newTimeoutListener(ln net.Listener, addr string, scheme string, rdtimeoutd, wtimeoutd time.Duration, tlsinfo *TLSInfo,) (net.Listener, error) {
	timeoutListener := &rwTimeoutListener{
		Listener:   ln,
		rdtimeoutd: rdtimeoutd,
		wtimeoutd:  wtimeoutd,
	}
	return wrapTLS(scheme, tlsinfo, timeoutListener)
}

type rwTimeoutListener struct {
	net.Listener
	wtimeoutd  time.Duration
	rdtimeoutd time.Duration
}

func (rwln *rwTimeoutListener) Accept() (net.Conn, error) {
	c, err := rwln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return timeoutConn{
		Conn:       c,
		wtimeoutd:  rwln.wtimeoutd,
		rdtimeoutd: rwln.rdtimeoutd,
	}, nil
}
