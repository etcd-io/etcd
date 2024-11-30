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
	"crypto/tls"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewKeepAliveListener tests NewKeepAliveListener returns a listener
// that accepts connections.
// TODO: verify the keepalive option is set correctly
func TestNewKeepAliveListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoErrorf(t, err, "unexpected listen error")

	ln, err = NewKeepAliveListener(ln, "http", nil)
	require.NoErrorf(t, err, "unexpected NewKeepAliveListener error")

	go http.Get("http://" + ln.Addr().String())
	conn, err := ln.Accept()
	require.NoErrorf(t, err, "unexpected Accept error")
	_, ok := conn.(*keepAliveConn)
	require.Truef(t, ok, "Unexpected conn type: %T, wanted *keepAliveConn", conn)
	conn.Close()
	ln.Close()

	ln, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoErrorf(t, err, "unexpected Listen error")

	// tls
	tlsinfo, err := createSelfCert(t)
	require.NoErrorf(t, err, "unable to create tmpfile")
	tlsInfo := TLSInfo{CertFile: tlsinfo.CertFile, KeyFile: tlsinfo.KeyFile}
	tlsInfo.parseFunc = fakeCertificateParserFunc(nil)
	tlscfg, err := tlsInfo.ServerConfig()
	require.NoErrorf(t, err, "unexpected serverConfig error")
	tlsln, err := NewKeepAliveListener(ln, "https", tlscfg)
	require.NoErrorf(t, err, "unexpected NewKeepAliveListener error")

	go http.Get("https://" + tlsln.Addr().String())
	conn, err = tlsln.Accept()
	require.NoErrorf(t, err, "unexpected Accept error")
	if _, ok := conn.(*tls.Conn); !ok {
		t.Errorf("failed to accept *tls.Conn")
	}
	conn.Close()
	tlsln.Close()
}

func TestNewKeepAliveListenerTLSEmptyConfig(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoErrorf(t, err, "unexpected listen error")

	_, err = NewKeepAliveListener(ln, "https", nil)
	if err == nil {
		t.Errorf("err = nil, want not presented error")
	}
}
