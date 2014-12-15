/*
   Copyright 2014 CoreOS, Inc.

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

package client

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

var (
	ErrTimeout          = context.DeadlineExceeded
	ErrCanceled         = context.Canceled
	ErrNoEndpoints      = errors.New("no endpoints available")
	ErrTooManyRedirects = errors.New("too many redirects")

	DefaultRequestTimeout = 5 * time.Second
	DefaultMaxRedirects   = 10
)

type SyncableHTTPClient interface {
	HTTPClient
	Sync(context.Context) error
	Endpoints() []string
}

type HTTPClient interface {
	Do(context.Context, HTTPAction) (*http.Response, []byte, error)
}

type HTTPAction interface {
	HTTPRequest(url.URL) *http.Request
}

// CancelableTransport mimics http.Transport to provide an interface which can be
// substituted for testing (since the RoundTripper interface alone does not
// require the CancelRequest method)
type CancelableTransport interface {
	http.RoundTripper
	CancelRequest(req *http.Request)
}

func NewHTTPClient(tr CancelableTransport, eps []string) (SyncableHTTPClient, error) {
	return newHTTPClusterClient(tr, eps)
}

func newHTTPClusterClient(tr CancelableTransport, eps []string) (*httpClusterClient, error) {
	c := httpClusterClient{
		transport: tr,
		endpoints: eps,
		clients:   make([]HTTPClient, len(eps)),
	}

	for i, ep := range eps {
		u, err := url.Parse(ep)
		if err != nil {
			return nil, err
		}

		c.clients[i] = &redirectFollowingHTTPClient{
			max: DefaultMaxRedirects,
			client: &httpClient{
				transport: tr,
				endpoint:  *u,
			},
		}
	}

	return &c, nil
}

type httpClusterClient struct {
	transport CancelableTransport
	endpoints []string
	clients   []HTTPClient
}

func (c *httpClusterClient) Do(ctx context.Context, act HTTPAction) (resp *http.Response, body []byte, err error) {
	if len(c.clients) == 0 {
		return nil, nil, ErrNoEndpoints
	}
	for _, hc := range c.clients {
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
	return c.endpoints
}

func (c *httpClusterClient) Sync(ctx context.Context) error {
	mAPI := NewMembersAPI(c)
	ms, err := mAPI.List(ctx)
	if err != nil {
		return err
	}

	eps := make([]string, 0)
	for _, m := range ms {
		eps = append(eps, m.ClientURLs...)
	}
	if len(eps) == 0 {
		return ErrNoEndpoints
	}
	nc, err := newHTTPClusterClient(c.transport, eps)
	if err != nil {
		return err
	}

	*c = *nc
	return nil
}

type roundTripResponse struct {
	resp *http.Response
	err  error
}

type httpClient struct {
	transport CancelableTransport
	endpoint  url.URL
}

func (c *httpClient) Do(ctx context.Context, act HTTPAction) (*http.Response, []byte, error) {
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
	client HTTPClient
	max    int
}

func (r *redirectFollowingHTTPClient) Do(ctx context.Context, act HTTPAction) (*http.Response, []byte, error) {
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
	action   HTTPAction
	location url.URL
}

func (r *redirectedHTTPAction) HTTPRequest(ep url.URL) *http.Request {
	orig := r.action.HTTPRequest(ep)
	orig.URL = &r.location
	return orig
}
