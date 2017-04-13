// Copyright 2017 The etcd Authors
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
	"fmt"
	"net"
	"sync"
)

// tlsListener overrides a TLS listener so it will reject client
// certificates with insufficient SAN credentials.
type tlsListener struct {
	net.Listener
	connc            chan net.Conn
	donec            chan struct{}
	err              error
	handshakeFailure func(*tls.Conn, error)
}

func newTLSListener(l net.Listener, tlsinfo *TLSInfo) (net.Listener, error) {
	if tlsinfo == nil || tlsinfo.Empty() {
		l.Close()
		return nil, fmt.Errorf("cannot listen on TLS for %s: KeyFile and CertFile are not presented", l.Addr().String())
	}
	tlscfg, err := tlsinfo.ServerConfig()
	if err != nil {
		return nil, err
	}
	tlsl := &tlsListener{
		Listener:         tls.NewListener(l, tlscfg),
		connc:            make(chan net.Conn),
		donec:            make(chan struct{}),
		handshakeFailure: tlsinfo.HandshakeFailure,
	}
	go tlsl.acceptLoop()
	return tlsl, nil
}

func (l *tlsListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connc:
		return conn, nil
	case <-l.donec:
		return nil, l.err
	}
}

// acceptLoop launches each TLS handshake in a separate goroutine
// to prevent a hanging TLS connection from blocking other connections.
func (l *tlsListener) acceptLoop() {
	var wg sync.WaitGroup
	var pendingMu sync.Mutex

	pending := make(map[net.Conn]struct{})
	stopc := make(chan struct{})
	defer func() {
		close(stopc)
		pendingMu.Lock()
		for c := range pending {
			c.Close()
		}
		pendingMu.Unlock()
		wg.Wait()
		close(l.donec)
	}()

	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			l.err = err
			return
		}

		pendingMu.Lock()
		pending[conn] = struct{}{}
		pendingMu.Unlock()

		wg.Add(1)
		go func() {
			defer func() {
				if conn != nil {
					conn.Close()
				}
				wg.Done()
			}()

			tlsConn := conn.(*tls.Conn)
			herr := tlsConn.Handshake()
			pendingMu.Lock()
			delete(pending, conn)
			pendingMu.Unlock()
			if herr != nil {
				if l.handshakeFailure != nil {
					l.handshakeFailure(tlsConn, herr)
				}
				return
			}

			st := tlsConn.ConnectionState()
			if len(st.PeerCertificates) > 0 {
				cert := st.PeerCertificates[0]
				if len(cert.IPAddresses) > 0 || len(cert.DNSNames) > 0 {
					addr := tlsConn.RemoteAddr().String()
					h, _, herr := net.SplitHostPort(addr)
					if herr != nil || cert.VerifyHostname(h) != nil {
						return
					}
				}
			}
			select {
			case l.connc <- tlsConn:
				conn = nil
			case <-stopc:
			}
		}()
	}
}

func (l *tlsListener) Close() error {
	err := l.Listener.Close()
	<-l.donec
	return err
}
