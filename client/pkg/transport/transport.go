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
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/tlsutil"
)

type unixTransport struct{ *http.Transport }

func NewTransport(info TLSInfo, dialtimeoutd time.Duration) (*http.Transport, error) {
	cfg, err := info.ClientConfig()
	if err != nil {
		return nil, err
	}

	var ipAddr net.Addr
	if info.LocalAddr != "" {
		ipAddr, err = net.ResolveTCPAddr("tcp", info.LocalAddr+":0")
		if err != nil {
			return nil, err
		}
	}

	dialer := &net.Dialer{
		Timeout:   dialtimeoutd,
		LocalAddr: ipAddr,
		// value taken from http.DefaultTransport
		KeepAlive: 30 * time.Second,
	}

	t := &http.Transport{
		Proxy:       http.ProxyFromEnvironment,
		DialContext: dialer.DialContext,
		// value taken from http.DefaultTransport
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
	}

	// When ReloadTrustedCA is enabled, use DialTLSContext to create
	// a fresh tls.Config with updated RootCAs per connection.
	// This avoids needing InsecureSkipVerify.
	cs := info.cafiles()
	if info.ReloadTrustedCA && len(cs) > 0 {
		caReloader, err := tlsutil.NewCAReloader(cs, info.Logger)
		if err != nil {
			return nil, err
		}
		if info.CAReloadInterval > 0 {
			caReloader.WithInterval(info.CAReloadInterval)
		}
		caReloader.Start()

		// Track CAReloader for cleanup
		info.trackCAReloader(caReloader)

		t.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Clone the base config and update RootCAs with fresh pool
			tlsCfg := cfg.Clone()
			tlsCfg.RootCAs = caReloader.GetCertPool()

			// Set ServerName if not already set
			if tlsCfg.ServerName == "" {
				host, _, err := net.SplitHostPort(addr)
				if err != nil {
					host = addr
				}
				tlsCfg.ServerName = host
			}

			// Dial the connection
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}

			// Wrap with TLS
			tlsConn := tls.Client(conn, tlsCfg)

			// Perform handshake with context
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, err
			}

			return tlsConn, nil
		}
	}

	unixDialer := &net.Dialer{
		Timeout:   dialtimeoutd,
		KeepAlive: 30 * time.Second,
	}

	dialContext := func(ctx context.Context, net, addr string) (net.Conn, error) {
		return unixDialer.DialContext(ctx, "unix", addr)
	}
	tu := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         dialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
		// Cost of reopening connection on sockets is low, and they are mostly used in testing.
		// Long living unix-transport connections were leading to 'leak' test flakes.
		// Alternatively the returned Transport (t) should override CloseIdleConnections to
		// forward it to 'tu' as well.
		IdleConnTimeout: time.Microsecond,
	}
	ut := &unixTransport{tu}

	t.RegisterProtocol("unix", ut)
	t.RegisterProtocol("unixs", ut)

	return t, nil
}

func (urt *unixTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := *req.URL
	req.URL = &url
	req.URL.Scheme = strings.Replace(req.URL.Scheme, "unix", "http", 1)
	return urt.Transport.RoundTrip(req)
}
