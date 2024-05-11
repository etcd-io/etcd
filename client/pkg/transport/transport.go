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
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type unixTransport struct{ *http.Transport }

var httpTransportProxyParsingFunc = determineHTTPTransportProxyParsingFunc()

func determineHTTPTransportProxyParsingFunc() func(req *http.Request) (*url.URL, error) {
	// according to the comment of http.ProxyFromEnvironment: if the proxy URL is "localhost"
	// (with or without a port number), then a nil URL and nil error will be returned.
	// Thus, we workaround this limitation by manually setting an ENV named E2E_TEST_FORWARD_PROXY_IP
	// and parse the URL (which is a localhost in our case)
	if forwardProxy, exists := os.LookupEnv("E2E_TEST_FORWARD_PROXY_IP"); exists {
		return func(req *http.Request) (*url.URL, error) {
			return url.Parse(forwardProxy)
		}
	}
	return http.ProxyFromEnvironment
}

func NewTransport(info TLSInfo, dialtimeoutd time.Duration) (*http.Transport, error) {
	cfg, err := info.ClientConfig()
	if err != nil {
		return nil, err
	}

	t := &http.Transport{
		Proxy: httpTransportProxyParsingFunc,
		DialContext: (&net.Dialer{
			Timeout: dialtimeoutd,
			// value taken from http.DefaultTransport
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// value taken from http.DefaultTransport
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
	}

	dialer := &net.Dialer{
		Timeout:   dialtimeoutd,
		KeepAlive: 30 * time.Second,
	}

	dialContext := func(ctx context.Context, net, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", addr)
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
