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
	"strings"
	"time"
)

// CancelableTransport provides an http.Transport interface that binds
// dials and RoundTrips to a context belonging to the transport which
// can be canceled when the transport is torn down.
type CancelableTransport struct {
	*http.Transport
	ctx    context.Context
	cancel context.CancelFunc
}

// Ctx is the context that is bound to RoundTrip requests and dials. It is
// canceled when the transport is canceled.
func (c *CancelableTransport) Ctx() context.Context { return c.ctx }

// Cancel cancels all current and future dials and requests on the transport.
func (c *CancelableTransport) Cancel() { c.cancel() }

// RoundTrip is a RoundTrip wrapper that overrides http requests using the
// default context to use the transport's cancelable context.
func (c *CancelableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Context() != context.Background() {
		// request defaults to context.Background; override
		return c.Transport.RoundTrip(req)
	}
	return c.Transport.RoundTrip(req.WithContext(c.ctx))
}

type unixTransport struct{ *http.Transport }

func NewTransport(info TLSInfo, dialtimeoutd time.Duration) (*CancelableTransport, error) {
	cfg, err := info.ClientConfig()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.TODO())

	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		// value taken from http.DefaultTransport
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
	}
	ct := &CancelableTransport{
		Transport: t,
		ctx:       ctx,
		cancel:    cancel,
	}
	tdialer := &net.Dialer{
		Timeout: dialtimeoutd,
		// value taken from http.DefaultTransport
		KeepAlive: 30 * time.Second,
	}
	tdial := func(net, addr string) (net.Conn, error) {
		return tdialer.DialContext(ctx, net, addr)
	}
	t.Dial = tdial

	dialer := (&net.Dialer{
		Timeout:   dialtimeoutd,
		KeepAlive: 30 * time.Second,
	})
	udial := func(net, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", addr)
	}
	tu := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                udial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
	}
	ut := &unixTransport{tu}

	t.RegisterProtocol("unix", ut)
	t.RegisterProtocol("unixs", ut)

	return ct, nil
}

func (urt *unixTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := *req.URL
	req.URL = &url
	req.URL.Scheme = strings.Replace(req.URL.Scheme, "unix", "http", 1)
	return urt.Transport.RoundTrip(req)
}
