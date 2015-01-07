/*
   Copyright 2015 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package transport

import (
	"net"
	"time"
)

// NewKeepAliveListener returns a listener that listens on the given address.
// http://tldp.org/HOWTO/TCP-Keepalive-HOWTO/overview.html
func NewKeepAliveListener(addr string, scheme string, info TLSInfo) (net.Listener, error) {
	ln, err := NewListener(addr, scheme, info)
	if err != nil {
		return nil, err
	}
	return &keepaliveListener{
		Listener: ln,
	}, nil
}

type keepaliveListener struct {
	net.Listener
}

func (kln *keepaliveListener) Accept() (net.Conn, error) {
	c, err := kln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tcpc := c.(*net.TCPConn)
	// detection time: tcp_keepalive_time + tcp_keepalive_probes + tcp_keepalive_intvl
	// default on linux:  30 + 8 * 30
	// default on osx:    30 + 8 * 75
	tcpc.SetKeepAlive(true)
	tcpc.SetKeepAlivePeriod(30 * time.Second)
	return tcpc, nil
}
