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
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

type staticHTTPClient struct {
	resp http.Response
	err  error
}

func (s *staticHTTPClient) Do(context.Context, HTTPAction) (*http.Response, []byte, error) {
	return &s.resp, nil, s.err
}

type staticHTTPAction struct {
	request http.Request
}

type staticHTTPResponse struct {
	resp http.Response
	err  error
}

func (s *staticHTTPAction) HTTPRequest(url.URL) *http.Request {
	return &s.request
}

type multiStaticHTTPClient struct {
	responses []staticHTTPResponse
	cur       int
}

func (s *multiStaticHTTPClient) Do(context.Context, HTTPAction) (*http.Response, []byte, error) {
	r := s.responses[s.cur]
	s.cur++
	return &r.resp, nil, r.err
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

func (t *fakeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	select {
	case resp := <-t.respchan:
		return resp, nil
	case err := <-t.errchan:
		return nil, err
	case <-t.startCancel:
		// wait on finishCancel to simulate taking some amount of
		// time while calling CancelRequest
		<-t.finishCancel
		return nil, errors.New("cancelled")
	}
}

func (t *fakeTransport) CancelRequest(*http.Request) {
	t.startCancel <- struct{}{}
}

type fakeAction struct{}

func (a *fakeAction) HTTPRequest(url.URL) *http.Request {
	return &http.Request{}
}

func TestHTTPClientDoSuccess(t *testing.T) {
	tr := newFakeTransport()
	c := &httpClient{transport: tr}

	tr.respchan <- &http.Response{
		StatusCode: http.StatusTeapot,
		Body:       ioutil.NopCloser(strings.NewReader("foo")),
	}

	resp, body, err := c.Do(context.Background(), &fakeAction{})
	if err != nil {
		t.Fatalf("incorrect error value: want=nil got=%v", err)
	}

	wantCode := http.StatusTeapot
	if wantCode != resp.StatusCode {
		t.Fatalf("invalid response code: want=%d got=%d", wantCode, resp.StatusCode)
	}

	wantBody := []byte("foo")
	if !reflect.DeepEqual(wantBody, body) {
		t.Fatalf("invalid response body: want=%q got=%q", wantBody, body)
	}
}

func TestHTTPClientDoError(t *testing.T) {
	tr := newFakeTransport()
	c := &httpClient{transport: tr}

	tr.errchan <- errors.New("fixture")

	_, _, err := c.Do(context.Background(), &fakeAction{})
	if err == nil {
		t.Fatalf("expected non-nil error, got nil")
	}
}

func TestHTTPClientDoCancelContext(t *testing.T) {
	tr := newFakeTransport()
	c := &httpClient{transport: tr}

	tr.startCancel <- struct{}{}
	tr.finishCancel <- struct{}{}

	_, _, err := c.Do(context.Background(), &fakeAction{})
	if err == nil {
		t.Fatalf("expected non-nil error, got nil")
	}
}

func TestHTTPClientDoCancelContextWaitForRoundTrip(t *testing.T) {
	tr := newFakeTransport()
	c := &httpClient{transport: tr}

	donechan := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.Do(ctx, &fakeAction{})
		close(donechan)
	}()

	// This should call CancelRequest and begin the cancellation process
	cancel()

	select {
	case <-donechan:
		t.Fatalf("httpClient.do should not have exited yet")
	default:
	}

	tr.finishCancel <- struct{}{}

	select {
	case <-donechan:
		//expected behavior
		return
	case <-time.After(time.Second):
		t.Fatalf("httpClient.do did not exit within 1s")
	}
}

func TestHTTPClusterClientDo(t *testing.T) {
	fakeErr := errors.New("fake!")
	tests := []struct {
		client   *httpClusterClient
		wantCode int
		wantErr  error
	}{
		// first good response short-circuits Do
		{
			client: &httpClusterClient{
				clients: []HTTPClient{
					&staticHTTPClient{resp: http.Response{StatusCode: http.StatusTeapot}},
					&staticHTTPClient{err: fakeErr},
				},
			},
			wantCode: http.StatusTeapot,
		},

		// fall through to good endpoint if err is arbitrary
		{
			client: &httpClusterClient{
				clients: []HTTPClient{
					&staticHTTPClient{err: fakeErr},
					&staticHTTPClient{resp: http.Response{StatusCode: http.StatusTeapot}},
				},
			},
			wantCode: http.StatusTeapot,
		},

		// ErrTimeout short-circuits Do
		{
			client: &httpClusterClient{
				clients: []HTTPClient{
					&staticHTTPClient{err: ErrTimeout},
					&staticHTTPClient{resp: http.Response{StatusCode: http.StatusTeapot}},
				},
			},
			wantErr: ErrTimeout,
		},

		// ErrCanceled short-circuits Do
		{
			client: &httpClusterClient{
				clients: []HTTPClient{
					&staticHTTPClient{err: ErrCanceled},
					&staticHTTPClient{resp: http.Response{StatusCode: http.StatusTeapot}},
				},
			},
			wantErr: ErrCanceled,
		},

		// return err if there are no endpoints
		{
			client: &httpClusterClient{
				clients: []HTTPClient{},
			},
			wantErr: ErrNoEndpoints,
		},

		// return err if all endpoints return arbitrary errors
		{
			client: &httpClusterClient{
				clients: []HTTPClient{
					&staticHTTPClient{err: fakeErr},
					&staticHTTPClient{err: fakeErr},
				},
			},
			wantErr: fakeErr,
		},

		// 500-level errors cause Do to fallthrough to next endpoint
		{
			client: &httpClusterClient{
				clients: []HTTPClient{
					&staticHTTPClient{resp: http.Response{StatusCode: http.StatusBadGateway}},
					&staticHTTPClient{resp: http.Response{StatusCode: http.StatusTeapot}},
				},
			},
			wantCode: http.StatusTeapot,
		},
	}

	for i, tt := range tests {
		resp, _, err := tt.client.Do(context.Background(), nil)
		if !reflect.DeepEqual(tt.wantErr, err) {
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

func TestRedirectedHTTPAction(t *testing.T) {
	act := &redirectedHTTPAction{
		action: &staticHTTPAction{
			request: http.Request{
				Method: "DELETE",
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
		Method: "DELETE",
		URL: &url.URL{
			Scheme: "https",
			Host:   "bar.example.com",
			Path:   "/pong",
		},
	}
	got := act.HTTPRequest(url.URL{Scheme: "http", Host: "baz.example.com", Path: "/pang"})

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("HTTPRequest is %#v, want %#v", want, got)
	}
}

func TestRedirectFollowingHTTPClient(t *testing.T) {
	tests := []struct {
		max      int
		client   HTTPClient
		wantCode int
		wantErr  error
	}{
		// errors bubbled up
		{
			max: 2,
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					staticHTTPResponse{
						err: errors.New("fail!"),
					},
				},
			},
			wantErr: errors.New("fail!"),
		},

		// no need to follow redirect if none given
		{
			max: 2,
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					staticHTTPResponse{
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
			max: 2,
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					staticHTTPResponse{
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
			max: 2,
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTeapot,
						},
					},
				},
			},
			wantCode: http.StatusTeapot,
		},

		// fail at max+1 redirects
		{
			max: 1,
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{"http://example.com"}},
						},
					},
					staticHTTPResponse{
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
			max: 1,
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
						},
					},
				},
			},
			wantErr: errors.New("Location header not set"),
		},

		// fail if Location header is invalid
		{
			max: 1,
			client: &multiStaticHTTPClient{
				responses: []staticHTTPResponse{
					staticHTTPResponse{
						resp: http.Response{
							StatusCode: http.StatusTemporaryRedirect,
							Header:     http.Header{"Location": []string{":"}},
						},
					},
				},
			},
			wantErr: errors.New("Location header not valid URL: :"),
		},
	}

	for i, tt := range tests {
		client := &redirectFollowingHTTPClient{client: tt.client, max: tt.max}
		resp, _, err := client.Do(context.Background(), nil)
		if !reflect.DeepEqual(tt.wantErr, err) {
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
