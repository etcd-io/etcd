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

package client

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
)

type actionAssertingHTTPClient struct {
	t   *testing.T
	num int
	act httpAction

	resp http.Response
	body []byte
	err  error
}

func (a *actionAssertingHTTPClient) Do(_ context.Context, act httpAction) (*http.Response, []byte, error) {
	if !reflect.DeepEqual(a.act, act) {
		a.t.Errorf("#%d: unexpected httpAction: want=%#v got=%#v", a.num, a.act, act)
	}

	return &a.resp, a.body, a.err
}

type staticHTTPClient struct {
	resp http.Response
	body []byte
	err  error
}

func (s *staticHTTPClient) Do(context.Context, httpAction) (*http.Response, []byte, error) {
	return &s.resp, s.body, s.err
}

type staticHTTPAction struct {
	request http.Request
}

func (s *staticHTTPAction) HTTPRequest(url.URL) *http.Request {
	return &s.request
}

type staticHTTPResponse struct {
	resp http.Response
	body []byte
	err  error
}

type multiStaticHTTPClient struct {
	responses []staticHTTPResponse
	cur       int
}

func (s *multiStaticHTTPClient) Do(context.Context, httpAction) (*http.Response, []byte, error) {
	r := s.responses[s.cur]
	s.cur++
	return &r.resp, r.body, r.err
}

func newStaticHTTPClientFactory(responses []staticHTTPResponse) httpClientFactory {
	var cur int
	return func(url.URL) httpClient {
		r := responses[cur]
		cur++
		return &staticHTTPClient{resp: r.resp, body: r.body, err: r.err}
	}
}

type fakeTransport struct {
	respchan     chan *http.Response
	errchan      chan error
	startCancel  chan struct{}
	finishCancel chan struct{}
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		respchan:     make(chan *http.Response, 1),
		errchan:      make(chan error, 1),
		startCancel:  make(chan struct{}, 1),
		finishCancel: make(chan struct{}, 1),
	}
}

func (t *fakeTransport) CancelRequest(*http.Request) {
	t.startCancel <- struct{}{}
}

type fakeAction struct{}

func (a *fakeAction) HTTPRequest(url.URL) *http.Request {
	return &http.Request{}
}

func TestSimpleHTTPClientDoSuccess(t *testing.T) {
	tr := newFakeTransport()
	c := &simpleHTTPClient{transport: tr}

	tr.respchan <- &http.Response{
		StatusCode: http.StatusTeapot,
		Body:       io.NopCloser(strings.NewReader("foo")),
	}

	resp, body, err := c.Do(t.Context(), &fakeAction{})
	require.NoErrorf(t, err, "incorrect error value")
	wantCode := http.StatusTeapot
	require.Equalf(t, wantCode, resp.StatusCode, "invalid response code: want=%d got=%d", wantCode, resp.StatusCode)

	wantBody := []byte("foo")
	require.Truef(t, reflect.DeepEqual(wantBody, body), "invalid response body: want=%q got=%q", wantBody, body)
}

func TestSimpleHTTPClientDoError(t *testing.T) {
	tr := newFakeTransport()
	c := &simpleHTTPClient{transport: tr}

	tr.errchan <- errors.New("fixture")

	_, _, err := c.Do(t.Context(), &fakeAction{})
	assert.Errorf(t, err, "expected non-nil error, got nil")
}

type nilAction struct{}

func (a *nilAction) HTTPRequest(url.URL) *http.Request {
	return nil
}

func TestSimpleHTTPClientDoNilRequest(t *testing.T) {
	tr := newFakeTransport()
	c := &simpleHTTPClient{transport: tr}

	tr.errchan <- errors.New("fixture")

	_, _, err := c.Do(t.Context(), &nilAction{})
	require.ErrorIsf(t, err, ErrNoRequest, "expected non-nil error, got nil")
}

func TestSimpleHTTPClientDoCancelContext(t *testing.T) {
	tr := newFakeTransport()
	c := &simpleHTTPClient{transport: tr}

	tr.startCancel <- struct{}{}
	tr.finishCancel <- struct{}{}

	_, _, err := c.Do(t.Context(), &fakeAction{})
	assert.Errorf(t, err, "expected non-nil error, got nil")
}

type checkableReadCloser struct {
	io.ReadCloser
	closed bool
}

func (c *checkableReadCloser) Close() error {
	if !c.closed {
		c.closed = true
		return c.ReadCloser.Close()
	}
	return nil
}

func TestSimpleHTTPClientDoCancelContextResponseBodyClosed(t *testing.T) {
	tr := newFakeTransport()
	c := &simpleHTTPClient{transport: tr}

	// create an already-cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	body := &checkableReadCloser{ReadCloser: io.NopCloser(strings.NewReader("foo"))}
	go func() {
		// wait that simpleHTTPClient knows the context is already timed out,
		// and calls CancelRequest
		testutil.WaitSchedule()

		// response is returned before cancel effects
		tr.respchan <- &http.Response{Body: body}
	}()

	_, _, err := c.Do(ctx, &fakeAction{})
	require.Errorf(t, err, "expected non-nil error, got nil")

	require.Truef(t, body.closed, "expected closed body")
}

type blockingBody struct {
	c chan struct{}
}

func (bb *blockingBody) Read(p []byte) (n int, err error) {
	<-bb.c
	return 0, errors.New("closed")
}

func (bb *blockingBody) Close() error {
	close(bb.c)
	return nil
}

func TestSimpleHTTPClientDoCancelContextResponseBodyClosedWithBlockingBody(t *testing.T) {
	tr := newFakeTransport()
	c := &simpleHTTPClient{transport: tr}

	ctx, cancel := context.WithCancel(t.Context())
	body := &checkableReadCloser{ReadCloser: &blockingBody{c: make(chan struct{})}}
	go func() {
		tr.respchan <- &http.Response{Body: body}
		time.Sleep(2 * time.Millisecond)
		// cancel after the body is received
		cancel()
	}()

	_, _, err := c.Do(ctx, &fakeAction{})
	require.ErrorIsf(t, err, context.Canceled, "expected %+v, got %+v", context.Canceled, err)

	require.Truef(t, body.closed, "expected closed body")
}

func TestSimpleHTTPClientDoCancelContextWaitForRoundTrip(t *testing.T) {
	tr := newFakeTransport()
	c := &simpleHTTPClient{transport: tr}

	donechan := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		c.Do(ctx, &fakeAction{})
		close(donechan)
	}()

	// This should call CancelRequest and begin the cancellation process
	cancel()

	select {
	case <-donechan:
		t.Fatalf("simpleHTTPClient.Do should not have exited yet")
	default:
	}

	tr.finishCancel <- struct{}{}

	select {
	case <-donechan:
		// expected behavior
		return
	case <-time.After(time.Second):
		t.Fatalf("simpleHTTPClient.Do did not exit within 1s")
	}
}

func TestSimpleHTTPClientDoHeaderTimeout(t *testing.T) {
	tr := newFakeTransport()
	tr.finishCancel <- struct{}{}
	c := &simpleHTTPClient{transport: tr, headerTimeout: time.Millisecond}

	errc := make(chan error, 1)
	go func() {
		_, _, err := c.Do(t.Context(), &fakeAction{})
		errc <- err
	}()

	select {
	case err := <-errc:
		require.Errorf(t, err, "expected non-nil error, got nil")
	case <-time.After(time.Second):
		t.Fatalf("unexpected timeout when waiting for the test to finish")
	}
}

func TestHTTPClusterClientDo(t *testing.T) {
	fakeErr := errors.New("fake")
	fakeURL := url.URL{}
	tests := []struct {
		client *httpClusterClient
		ctx    context.Context

		wantCode   int
		wantErr    error
		wantPinned int
	}{
		// first good response short-circuits Do
		{
			client: &httpClusterClient{
				endpoints: []url.URL{fakeURL, fakeURL},
				clientFactory: newStaticHTTPClientFactory(
					[]staticHTTPResponse{
						{resp: http.Response{StatusCode: http.StatusTeapot}},
						{err: fakeErr},
					},
				),
				rand: rand.New(rand.NewSource(0)),
			},
			wantCode: http.StatusTeapot,
		},

		// fall through to good endpoint if err is arbitrary
		{
			client: &httpClusterClient{
				endpoints: []url.URL{fakeURL, fakeURL},
				clientFactory: newStaticHTTPClientFactory(
					[]staticHTTPResponse{
						{err: fakeErr},
						{resp: http.Response{StatusCode: http.StatusTeapot}},
					},
				),
				rand: rand.New(rand.NewSource(0)),
			},
			wantCode:   http.StatusTeapot,
			wantPinned: 1,
		},

		// context.Canceled short-circuits Do
		{
			client: &httpClusterClient{
				endpoints: []url.URL{fakeURL, fakeURL},
				clientFactory: newStaticHTTPClientFactory(
					[]staticHTTPResponse{
						{err: context.Canceled},
						{resp: http.Response{StatusCode: http.StatusTeapot}},
					},
				),
				rand: rand.New(rand.NewSource(0)),
			},
			wantErr: context.Canceled,
		},

		// return err if there are no endpoints
		{
			client: &httpClusterClient{
				endpoints:     []url.URL{},
				clientFactory: newHTTPClientFactory(nil, nil, 0),
				rand:          rand.New(rand.NewSource(0)),
			},
			wantErr: ErrNoEndpoints,
		},

		// return err if all endpoints return arbitrary errors
		{
			client: &httpClusterClient{
				endpoints: []url.URL{fakeURL, fakeURL},
				clientFactory: newStaticHTTPClientFactory(
					[]staticHTTPResponse{
						{err: fakeErr},
						{err: fakeErr},
					},
				),
				rand: rand.New(rand.NewSource(0)),
			},
			wantErr: &ClusterError{Errors: []error{fakeErr, fakeErr}},
		},

		// 500-level errors cause Do to fallthrough to next endpoint
		{
			client: &httpClusterClient{
				endpoints: []url.URL{fakeURL, fakeURL},
				clientFactory: newStaticHTTPClientFactory(
					[]staticHTTPResponse{
						{resp: http.Response{StatusCode: http.StatusBadGateway}},
						{resp: http.Response{StatusCode: http.StatusTeapot}},
					},
				),
				rand: rand.New(rand.NewSource(0)),
			},
			wantCode:   http.StatusTeapot,
			wantPinned: 1,
		},

		// 500-level errors cause one shot Do to fallthrough to next endpoint
		{
			client: &httpClusterClient{
				endpoints: []url.URL{fakeURL, fakeURL},
				clientFactory: newStaticHTTPClientFactory(
					[]staticHTTPResponse{
						{resp: http.Response{StatusCode: http.StatusBadGateway}},
						{resp: http.Response{StatusCode: http.StatusTeapot}},
					},
				),
				rand: rand.New(rand.NewSource(0)),
			},
			ctx:        context.WithValue(t.Context(), &oneShotCtxValue, &oneShotCtxValue),
			wantErr:    errors.New("client: etcd member  returns server error [Bad Gateway]"),
			wantPinned: 1,
		},
	}

	for i, tt := range tests {
		if tt.ctx == nil {
			tt.ctx = t.Context()
		}
		resp, _, err := tt.client.Do(tt.ctx, nil)
		if (tt.wantErr == nil && !errors.Is(err, tt.wantErr)) || (tt.wantErr != nil && tt.wantErr.Error() != err.Error()) {
			t.Errorf("#%d: got err=%v, want=%v", i, err, tt.wantErr)
			continue
		}

		if resp == nil {
			if tt.wantCode != 0 {
				t.Errorf("#%d: resp is nil, want=%d", i, tt.wantCode)
				continue
			}
		} else if resp.StatusCode != tt.wantCode {
			t.Errorf("#%d: resp code=%d, want=%d", i, resp.StatusCode, tt.wantCode)
			continue
		}

		if tt.client.pinned != tt.wantPinned {
			t.Errorf("#%d: pinned=%d, want=%d", i, tt.client.pinned, tt.wantPinned)
		}
	}
}

func TestHTTPClusterClientDoDeadlineExceedContext(t *testing.T) {
	fakeURL := url.URL{}
	tr := newFakeTransport()
	tr.finishCancel <- struct{}{}
	c := &httpClusterClient{
		clientFactory: newHTTPClientFactory(tr, DefaultCheckRedirect, 0),
		endpoints:     []url.URL{fakeURL},
	}

	errc := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
		defer cancel()
		_, _, err := c.Do(ctx, &fakeAction{})
		errc <- err
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %+v, want %+v", err, context.DeadlineExceeded)
		}
	case <-time.After(time.Second):
		t.Fatalf("unexpected timeout when waiting for request to deadline exceed")
	}
}

type fakeCancelContext struct{}

var errFakeCancelContext = errors.New("fake context canceled")

func (f fakeCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (f fakeCancelContext) Done() <-chan struct{} {
	d := make(chan struct{}, 1)
	d <- struct{}{}
	return d
}
func (f fakeCancelContext) Err() error        { return errFakeCancelContext }
func (f fakeCancelContext) Value(key any) any { return 1 }

func withTimeout(parent context.Context, _timeout time.Duration) (
	ctx context.Context,
	cancel context.CancelFunc,
) {
	ctx = parent
	cancel = func() {
		ctx = nil
	}
	return ctx, cancel
}

func TestHTTPClusterClientDoCanceledContext(t *testing.T) {
	fakeURL := url.URL{}
	tr := newFakeTransport()
	tr.finishCancel <- struct{}{}
	c := &httpClusterClient{
		clientFactory: newHTTPClientFactory(tr, DefaultCheckRedirect, 0),
		endpoints:     []url.URL{fakeURL},
	}

	errc := make(chan error, 1)
	go func() {
		ctx, cancel := withTimeout(fakeCancelContext{}, time.Millisecond)
		cancel()
		_, _, err := c.Do(ctx, &fakeAction{})
		errc <- err
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, errFakeCancelContext) {
			t.Errorf("err = %+v, want %+v", err, errFakeCancelContext)
		}
	case <-time.After(time.Second):
		t.Fatalf("unexpected timeout when waiting for request to fake context canceled")
	}
}

func TestRedirectedHTTPAction(t *testing.T) {
	act := &redirectedHTTPAction{
		action: &staticHTTPAction{
			request: http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Scheme: "https",
					Host:   "foo.example.com",
					Path:   "/ping",
				},
			},
		},
		location: url.URL{
			Scheme: "https",
			Host:   "bar.example.com",
			Path:   "/pong",
		},
	}

	want := &http.Request{
		Method: http.MethodDelete,
		URL: &url.URL{
			Scheme: "https",
			Host:   "bar.example.com",
			Path:   "/pong",
		},
	}
	got := act.HTTPRequest(url.URL{Scheme: "http", Host: "baz.example.com", Path: "/pang"})

	require.Truef(t, reflect.DeepEqual(want, got), "HTTPRequest is %#v, want %#v", want, got)
}

func TestRedirectFollowingHTTPClient(t *testing.T) {
	tests := []struct {
		checkRedirect CheckRedirectFunc
		client        httpClient
		wantCode      int
		wantErr       error
	}{
		// errors bubbled up
		{
			checkRedirect: func(int) error { return ErrTooManyRedirects },
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					{
						err: errors.New("fail"),
					},
				},
			},
			wantErr: errors.New("fail"),
		},

		// no need to follow redirect if none given
		{
			checkRedirect: func(int) error { return ErrTooManyRedirects },
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					{
						resp: http.Response{
							StatusCode: http.StatusTeapot,
						},
					},
				},
			},
			wantCode: http.StatusTeapot,
		},

		// redirects if less than max
		{
			checkRedirect: func(via int) error {
				if via >= 2 {
					return ErrTooManyRedirects
				}
				return nil
			},
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					{
						resp: http.Response{
							StatusCode: http.StatusTeapot,
						},
					},
				},
			},
			wantCode: http.StatusTeapot,
		},

		// succeed after reaching max redirects
		{
			checkRedirect: func(via int) error {
				if via >= 3 {
					return ErrTooManyRedirects
				}
				return nil
			},
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					{
						resp: http.Response{
							StatusCode: http.StatusTeapot,
						},
					},
				},
			},
			wantCode: http.StatusTeapot,
		},

		// fail if too many redirects
		{
			checkRedirect: func(via int) error {
				if via >= 2 {
					return ErrTooManyRedirects
				}
				return nil
			},
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					{
						resp: http.Response{
							StatusCode: http.StatusTeapot,
						},
					},
				},
			},
			wantErr: ErrTooManyRedirects,
		},

		// fail if Location header not set
		{
			checkRedirect: func(int) error { return ErrTooManyRedirects },
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
						},
					},
				},
			},
			wantErr: errors.New("location header not set"),
		},

		// fail if Location header is invalid
		{
			checkRedirect: func(int) error { return ErrTooManyRedirects },
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{":"}},
						},
					},
				},
			},
			//revive:disable:error-strings
			wantErr: errors.New("location header not valid URL: :"),
			//revive:enable:error-strings
		},

		// fail if redirects checked way too many times
		{
			checkRedirect: func(int) error { return nil },
			client: &staticHTTPClient{
				resp: http.Response{
					StatusCode: http.StatusTemporaryRedirect,
					Header:     http.Header{"Location": []string{"http://example.com"}},
				},
			},
			wantErr: errTooManyRedirectChecks,
		},
	}

	for i, tt := range tests {
		client := &redirectFollowingHTTPClient{client: tt.client, checkRedirect: tt.checkRedirect}
		resp, _, err := client.Do(t.Context(), nil)
		if (tt.wantErr == nil && !errors.Is(err, tt.wantErr)) || (tt.wantErr != nil && tt.wantErr.Error() != err.Error()) {
			t.Errorf("#%d: got err=%v, want=%v", i, err, tt.wantErr)
			continue
		}

		if resp == nil {
			if tt.wantCode != 0 {
				t.Errorf("#%d: resp is nil, want=%d", i, tt.wantCode)
			}
			continue
		}

		if resp.StatusCode != tt.wantCode {
			t.Errorf("#%d: resp code=%d, want=%d", i, resp.StatusCode, tt.wantCode)
			continue
		}
	}
}

func TestDefaultCheckRedirect(t *testing.T) {
	tests := []struct {
		num int
		err error
	}{
		{0, nil},
		{5, nil},
		{10, nil},
		{11, ErrTooManyRedirects},
		{29, ErrTooManyRedirects},
	}

	for i, tt := range tests {
		err := DefaultCheckRedirect(tt.num)
		if !reflect.DeepEqual(tt.err, err) {
			t.Errorf("#%d: want=%#v got=%#v", i, tt.err, err)
		}
	}
}

func TestHTTPClusterClientSync(t *testing.T) {
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
	}
	err := hc.SetEndpoints([]string{"http://127.0.0.1:2379"})
	require.NoErrorf(t, err, "unexpected error during setup")

	want := []string{"http://127.0.0.1:2379"}
	got := hc.Endpoints()
	require.Truef(t, reflect.DeepEqual(want, got), "incorrect endpoints: want=%#v got=%#v", want, got)

	err = hc.Sync(t.Context())
	require.NoErrorf(t, err, "unexpected error during Sync: %#v", err)

	want = []string{"http://127.0.0.1:2379", "http://127.0.0.1:4001", "http://127.0.0.1:4002", "http://127.0.0.1:4003"}
	got = hc.Endpoints()
	sort.Strings(got)
	require.Truef(t, reflect.DeepEqual(want, got), "incorrect endpoints post-Sync: want=%#v got=%#v", want, got)

	err = hc.SetEndpoints([]string{"http://127.0.0.1:4009"})
	require.NoErrorf(t, err, "unexpected error during reset: %#v", err)

	want = []string{"http://127.0.0.1:4009"}
	got = hc.Endpoints()
	require.Truef(t, reflect.DeepEqual(want, got), "incorrect endpoints post-reset: want=%#v got=%#v", want, got)
}

func TestHTTPClusterClientSyncFail(t *testing.T) {
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{err: errors.New("fail")},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
	}
	err := hc.SetEndpoints([]string{"http://127.0.0.1:2379"})
	require.NoErrorf(t, err, "unexpected error during setup")

	want := []string{"http://127.0.0.1:2379"}
	got := hc.Endpoints()
	require.Truef(t, reflect.DeepEqual(want, got), "incorrect endpoints: want=%#v got=%#v", want, got)

	err = hc.Sync(t.Context())
	require.Errorf(t, err, "got nil error during Sync")

	got = hc.Endpoints()
	require.Truef(t, reflect.DeepEqual(want, got), "incorrect endpoints after failed Sync: want=%#v got=%#v", want, got)
}

func TestHTTPClusterClientAutoSyncCancelContext(t *testing.T) {
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
	}
	err := hc.SetEndpoints([]string{"http://127.0.0.1:2379"})
	require.NoErrorf(t, err, "unexpected error during setup")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = hc.AutoSync(ctx, time.Hour)
	require.ErrorIsf(t, err, context.Canceled, "incorrect error value: want=%v got=%v", context.Canceled, err)
}

func TestHTTPClusterClientAutoSyncFail(t *testing.T) {
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{err: errors.New("fail")},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
	}
	err := hc.SetEndpoints([]string{"http://127.0.0.1:2379"})
	require.NoErrorf(t, err, "unexpected error during setup")

	err = hc.AutoSync(t.Context(), time.Hour)
	require.Truef(t, strings.HasPrefix(err.Error(), ErrClusterUnavailable.Error()), "incorrect error value: want=%v got=%v", ErrClusterUnavailable, err)
}

func TestHTTPClusterClientGetVersion(t *testing.T) {
	body := []byte(`{"etcdserver":"2.3.2","etcdcluster":"2.3.0"}`)
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Length": []string{"44"}}},
			body: body,
		},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
	}
	err := hc.SetEndpoints([]string{"http://127.0.0.1:4003", "http://127.0.0.1:2379", "http://127.0.0.1:4001", "http://127.0.0.1:4002"})
	require.NoErrorf(t, err, "unexpected error during setup")

	actual, err := hc.GetVersion(t.Context())
	if err != nil {
		t.Errorf("non-nil error: %#v", err)
	}
	expected := version.Versions{Server: "2.3.2", Cluster: "2.3.0"}
	if !reflect.DeepEqual(&expected, actual) {
		t.Errorf("incorrect Response: want=%#v got=%#v", expected, actual)
	}
}

// TestHTTPClusterClientSyncPinEndpoint tests that Sync() pins the endpoint when
// it gets the exactly same member list as before.
func TestHTTPClusterClientSyncPinEndpoint(t *testing.T) {
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
	}
	err := hc.SetEndpoints([]string{"http://127.0.0.1:4003", "http://127.0.0.1:2379", "http://127.0.0.1:4001", "http://127.0.0.1:4002"})
	require.NoErrorf(t, err, "unexpected error during setup")
	pinnedEndpoint := hc.endpoints[hc.pinned]

	for i := 0; i < 3; i++ {
		err = hc.Sync(t.Context())
		require.NoErrorf(t, err, "#%d: unexpected error during Sync", i)

		if g := hc.endpoints[hc.pinned]; g != pinnedEndpoint {
			t.Errorf("#%d: pinned endpoint = %v, want %v", i, g, pinnedEndpoint)
		}
	}
}

// TestHTTPClusterClientSyncUnpinEndpoint tests that Sync() unpins the endpoint when
// it gets a different member list than before.
func TestHTTPClusterClientSyncUnpinEndpoint(t *testing.T) {
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
	}
	err := hc.SetEndpoints([]string{"http://127.0.0.1:4003", "http://127.0.0.1:2379", "http://127.0.0.1:4001", "http://127.0.0.1:4002"})
	require.NoErrorf(t, err, "unexpected error during setup")
	wants := []string{"http://127.0.0.1:2379", "http://127.0.0.1:4001", "http://127.0.0.1:4002"}

	for i := 0; i < 3; i++ {
		err = hc.Sync(t.Context())
		require.NoErrorf(t, err, "#%d: unexpected error during Sync", i)

		if g := hc.endpoints[hc.pinned]; g.String() != wants[i] {
			t.Errorf("#%d: pinned endpoint = %v, want %v", i, g, wants[i])
		}
	}
}

// TestHTTPClusterClientSyncPinLeaderEndpoint tests that Sync() pins the leader
// when the selection mode is EndpointSelectionPrioritizeLeader
func TestHTTPClusterClientSyncPinLeaderEndpoint(t *testing.T) {
	cf := newStaticHTTPClientFactory([]staticHTTPResponse{
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]}`),
		},
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
		},
		{
			resp: http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}},
			body: []byte(`{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}`),
		},
	})

	hc := &httpClusterClient{
		clientFactory: cf,
		rand:          rand.New(rand.NewSource(0)),
		selectionMode: EndpointSelectionPrioritizeLeader,
		endpoints:     []url.URL{{}}, // Need somewhere to pretend to send to initially
	}

	wants := []string{"http://127.0.0.1:4003", "http://127.0.0.1:4002"}

	for i, want := range wants {
		err := hc.Sync(t.Context())
		require.NoErrorf(t, err, "#%d: unexpected error during Sync", i)

		pinned := hc.endpoints[hc.pinned].String()
		if pinned != want {
			t.Errorf("#%d: pinned endpoint = %v, want %v", i, pinned, want)
		}
	}
}

func TestHTTPClusterClientResetFail(t *testing.T) {
	tests := [][]string{
		// need at least one endpoint
		{},

		// urls must be valid
		{":"},
	}

	for i, tt := range tests {
		hc := &httpClusterClient{rand: rand.New(rand.NewSource(0))}
		err := hc.SetEndpoints(tt)
		if err == nil {
			t.Errorf("#%d: expected non-nil error", i)
		}
	}
}

func TestHTTPClusterClientResetPinRandom(t *testing.T) {
	round := 2000
	pinNum := 0
	for i := 0; i < round; i++ {
		hc := &httpClusterClient{rand: rand.New(rand.NewSource(int64(i)))}
		err := hc.SetEndpoints([]string{"http://127.0.0.1:4001", "http://127.0.0.1:4002", "http://127.0.0.1:4003"})
		require.NoErrorf(t, err, "#%d: reset error", i)
		if hc.endpoints[hc.pinned].String() == "http://127.0.0.1:4001" {
			pinNum++
		}
	}

	min := 1.0/3.0 - 0.05
	max := 1.0/3.0 + 0.05
	if ratio := float64(pinNum) / float64(round); ratio > max || ratio < min {
		t.Errorf("pinned ratio = %v, want [%v, %v]", ratio, min, max)
	}
}
