// Copyright 2015 CoreOS, Inc.
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

package client

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

var (
	ErrTimeout          = context.DeadlineExceeded
	ErrCanceled         = context.Canceled
	ErrUnavailable      = errors.New("client: no available etcd endpoints")
	ErrNoLeader         = errors.New("client: no leader")
	ErrNoEndpoints      = errors.New("no endpoints available")
	ErrTooManyRedirects = errors.New("too many redirects")

	ErrKeyNoExist = errors.New("client: key does not exist")
	ErrKeyExists  = errors.New("client: key already exists")

	DefaultRequestTimeout = 5 * time.Second
	DefaultMaxRedirects   = 10
)

type Config struct {
	// Endpoints defines a set of URLs (schemes, hosts and ports only)
	// that can be used to communicate with a logical etcd cluster. For
	// example, a three-node cluster could be provided like so:
	//
	// 	Endpoints: []string{
	//		"http://node1.example.com:4001",
	//		"http://node2.example.com:2379",
	//		"http://node3.example.com:4001",
	//	}
	//
	// If multiple endpoints are provided, the Client will attempt to
	// use them all in the event that one or more of them are unusable.
	//
	// If Client.Sync is ever called, the Client may cache an alternate
	// set of endpoints to continue operation.
	Endpoints []string

	// Transport is used by the Client to drive HTTP requests. If not
	// provided, net/http.DefaultTransport will be used.
	Transport CancelableTransport
}

// CancelableTransport mimics net/http.Transport, but requires that
// the object also support request cancellation.
type CancelableTransport interface {
	http.RoundTripper
	CancelRequest(req *http.Request)
}

type Client interface {
	// Sync updates the internal cache of the etcd cluster's membership.
	Sync(context.Context) error

	// Endpoints returns a copy of the current set of API endpoints used
	// by Client to resolve HTTP requests. If Sync has ever been called,
	// this may differ from the initial Endpoints provided in the Config.
	Endpoints() []string

	httpClient
}

func New(cfg Config) (Client, error) {
	c := &httpClusterClient{clientFactory: newHTTPClientFactory(cfg.Transport)}
	if err := c.reset(cfg.Endpoints); err != nil {
		return nil, err
	}
	return c, nil
}

type httpClient interface {
	Do(context.Context, httpAction) (*http.Response, []byte, error)
}

func newHTTPClientFactory(tr CancelableTransport) httpClientFactory {
	return func(ep url.URL) httpClient {
		return &redirectFollowingHTTPClient{
			max: DefaultMaxRedirects,
			client: &simpleHTTPClient{
				transport: tr,
				endpoint:  ep,
			},
		}
	}
}

type httpClientFactory func(url.URL) httpClient

type httpAction interface {
	HTTPRequest(url.URL) *http.Request
}

type httpClusterClient struct {
	clientFactory httpClientFactory
	endpoints     []url.URL
	sync.RWMutex
}

func (c *httpClusterClient) reset(eps []string) error {
	if len(eps) == 0 {
		return ErrNoEndpoints
	}

	neps := make([]url.URL, len(eps))
	for i, ep := range eps {
		u, err := url.Parse(ep)
		if err != nil {
			return err
		}
		neps[i] = *u
	}

	c.endpoints = neps

	return nil
}

func (c *httpClusterClient) Do(ctx context.Context, act httpAction) (resp *http.Response, body []byte, err error) {
	c.RLock()
	leps := len(c.endpoints)
	eps := make([]url.URL, leps)
	n := copy(eps, c.endpoints)
	c.RUnlock()

	if leps == 0 {
		err = ErrNoEndpoints
		return
	}

	if leps != n {
		err = errors.New("unable to pick endpoint: copy failed")
		return
	}

	for _, ep := range eps {
		hc := c.clientFactory(ep)
		resp, body, err = hc.Do(ctx, act)
		if err != nil {
			if err == ErrTimeout || err == ErrCanceled {
				return nil, nil, err
			}
			continue
		}
		if resp.StatusCode/100 == 5 {
			continue
		}
		break
	}

	return
}

func (c *httpClusterClient) Endpoints() []string {
	c.RLock()
	defer c.RUnlock()

	eps := make([]string, len(c.endpoints))
	for i, ep := range c.endpoints {
		eps[i] = ep.String()
	}

	return eps
}

func (c *httpClusterClient) Sync(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	mAPI := NewMembersAPI(c)
	ms, err := mAPI.List(ctx)
	if err != nil {
		return err
	}

	eps := make([]string, 0)
	for _, m := range ms {
		eps = append(eps, m.ClientURLs...)
	}

	return c.reset(eps)
}

type roundTripResponse struct {
	resp *http.Response
	err  error
}

type simpleHTTPClient struct {
	transport CancelableTransport
	endpoint  url.URL
}

func (c *simpleHTTPClient) Do(ctx context.Context, act httpAction) (*http.Response, []byte, error) {
	req := act.HTTPRequest(c.endpoint)

	rtchan := make(chan roundTripResponse, 1)
	go func() {
		resp, err := c.transport.RoundTrip(req)
		rtchan <- roundTripResponse{resp: resp, err: err}
		close(rtchan)
	}()

	var resp *http.Response
	var err error

	select {
	case rtresp := <-rtchan:
		resp, err = rtresp.resp, rtresp.err
	case <-ctx.Done():
		c.transport.CancelRequest(req)
		// wait for request to actually exit before continuing
		<-rtchan
		err = ctx.Err()
	}

	// always check for resp nil-ness to deal with possible
	// race conditions between channels above
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	if err != nil {
		return nil, nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	return resp, body, err
}

type redirectFollowingHTTPClient struct {
	client httpClient
	max    int
}

func (r *redirectFollowingHTTPClient) Do(ctx context.Context, act httpAction) (*http.Response, []byte, error) {
	for i := 0; i <= r.max; i++ {
		resp, body, err := r.client.Do(ctx, act)
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode/100 == 3 {
			hdr := resp.Header.Get("Location")
			if hdr == "" {
				return nil, nil, fmt.Errorf("Location header not set")
			}
			loc, err := url.Parse(hdr)
			if err != nil {
				return nil, nil, fmt.Errorf("Location header not valid URL: %s", hdr)
			}
			act = &redirectedHTTPAction{
				action:   act,
				location: *loc,
			}
			continue
		}
		return resp, body, nil
	}
	return nil, nil, ErrTooManyRedirects
}

type redirectedHTTPAction struct {
	action   httpAction
	location url.URL
}

func (r *redirectedHTTPAction) HTTPRequest(ep url.URL) *http.Request {
	orig := r.action.HTTPRequest(ep)
	orig.URL = &r.location
	return orig
}
