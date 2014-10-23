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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

var (
	DefaultV2KeysPrefix = "/v2/keys"
	ErrTimeout          = context.DeadlineExceeded
)

// transport mimics http.Transport to provide an interface which can be
// substituted for testing (since the RoundTripper interface alone does not
// require the CancelRequest method)
type transport interface {
	http.RoundTripper
	CancelRequest(req *http.Request)
}

type httpClient struct {
	transport transport
	endpoint  url.URL
	timeout   time.Duration
}

func NewHTTPClient(tr *http.Transport, ep string, timeout time.Duration) (*httpClient, error) {
	u, err := url.Parse(ep)
	if err != nil {
		return nil, err
	}

	c := &httpClient{
		transport: tr,
		endpoint:  *u,
		timeout:   timeout,
	}

	return c, nil
}

func (c *httpClient) SetPrefix(p string) {
	DefaultV2KeysPrefix = p
}

func (c *httpClient) Create(key, val string, ttl time.Duration) (*Response, error) {
	create := &createAction{
		Key:   key,
		Value: val,
	}
	if ttl >= 0 {
		uttl := uint64(ttl.Seconds())
		create.TTL = &uttl
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	httpresp, body, err := c.do(ctx, create)
	cancel()

	if err != nil {
		return nil, err
	}

	return unmarshalHTTPResponse(httpresp.StatusCode, body)
}

func (c *httpClient) Get(key string) (*Response, error) {
	get := &getAction{
		Key:       key,
		Recursive: false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	httpresp, body, err := c.do(ctx, get)
	cancel()

	if err != nil {
		return nil, err
	}

	return unmarshalHTTPResponse(httpresp.StatusCode, body)
}

type roundTripResponse struct {
	resp *http.Response
	err  error
}

func (c *httpClient) do(ctx context.Context, act httpAction) (*http.Response, []byte, error) {
	req := act.httpRequest(c.endpoint)

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

func (c *httpClient) Watch(key string, idx uint64) Watcher {
	return &httpWatcher{
		httpClient: *c,
		nextWait: waitAction{
			Key:       key,
			WaitIndex: idx,
			Recursive: false,
		},
	}
}

func (c *httpClient) RecursiveWatch(key string, idx uint64) Watcher {
	return &httpWatcher{
		httpClient: *c,
		nextWait: waitAction{
			Key:       key,
			WaitIndex: idx,
			Recursive: true,
		},
	}
}

type httpWatcher struct {
	httpClient
	nextWait waitAction
}

func (hw *httpWatcher) Next() (*Response, error) {
	httpresp, body, err := hw.httpClient.do(context.Background(), &hw.nextWait)
	if err != nil {
		return nil, err
	}

	resp, err := unmarshalHTTPResponse(httpresp.StatusCode, body)
	if err != nil {
		return nil, err
	}

	hw.nextWait.WaitIndex = resp.Node.ModifiedIndex + 1
	return resp, nil
}

func v2URL(ep url.URL, key string) *url.URL {
	ep.Path = path.Join(ep.Path, DefaultV2KeysPrefix, key)
	return &ep
}

type httpAction interface {
	httpRequest(url.URL) *http.Request
}

type getAction struct {
	Key       string
	Recursive bool
}

func (g *getAction) httpRequest(ep url.URL) *http.Request {
	u := v2URL(ep, g.Key)

	params := u.Query()
	params.Set("recursive", strconv.FormatBool(g.Recursive))
	u.RawQuery = params.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

type waitAction struct {
	Key       string
	WaitIndex uint64
	Recursive bool
}

func (w *waitAction) httpRequest(ep url.URL) *http.Request {
	u := v2URL(ep, w.Key)

	params := u.Query()
	params.Set("wait", "true")
	params.Set("waitIndex", strconv.FormatUint(w.WaitIndex, 10))
	params.Set("recursive", strconv.FormatBool(w.Recursive))
	u.RawQuery = params.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

type createAction struct {
	Key   string
	Value string
	TTL   *uint64
}

func (c *createAction) httpRequest(ep url.URL) *http.Request {
	u := v2URL(ep, c.Key)

	params := u.Query()
	params.Set("prevExist", "false")
	u.RawQuery = params.Encode()

	form := url.Values{}
	form.Add("value", c.Value)
	if c.TTL != nil {
		form.Add("ttl", strconv.FormatUint(*c.TTL, 10))
	}
	body := strings.NewReader(form.Encode())

	req, _ := http.NewRequest("PUT", u.String(), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

func unmarshalHTTPResponse(code int, body []byte) (res *Response, err error) {
	switch code {
	case http.StatusOK, http.StatusCreated:
		res, err = unmarshalSuccessfulResponse(body)
	default:
		err = unmarshalErrorResponse(code)
	}

	return
}

func unmarshalSuccessfulResponse(body []byte) (*Response, error) {
	var res Response
	err := json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func unmarshalErrorResponse(code int) error {
	switch code {
	case http.StatusNotFound:
		return ErrKeyNoExist
	case http.StatusPreconditionFailed:
		return ErrKeyExists
	case http.StatusInternalServerError:
		// this isn't necessarily true
		return ErrNoLeader
	default:
	}

	return fmt.Errorf("unrecognized HTTP status code %d", code)
}
