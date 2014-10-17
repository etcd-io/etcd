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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestV2URLHelper(t *testing.T) {
	tests := []struct {
		endpoint url.URL
		key      string
		want     url.URL
	}{
		// key is empty, no problem
		{
			endpoint: url.URL{Scheme: "http", Host: "example.com", Path: ""},
			key:      "",
			want:     url.URL{Scheme: "http", Host: "example.com", Path: "/v2/keys"},
		},

		// key is joined to path
		{
			endpoint: url.URL{Scheme: "http", Host: "example.com", Path: ""},
			key:      "/foo/bar",
			want:     url.URL{Scheme: "http", Host: "example.com", Path: "/v2/keys/foo/bar"},
		},

		// Host field carries through with port
		{
			endpoint: url.URL{Scheme: "http", Host: "example.com:8080", Path: ""},
			key:      "",
			want:     url.URL{Scheme: "http", Host: "example.com:8080", Path: "/v2/keys"},
		},

		// Scheme carries through
		{
			endpoint: url.URL{Scheme: "https", Host: "example.com", Path: ""},
			key:      "",
			want:     url.URL{Scheme: "https", Host: "example.com", Path: "/v2/keys"},
		},

		// Path on endpoint is not ignored
		{
			endpoint: url.URL{Scheme: "https", Host: "example.com", Path: "/prefix"},
			key:      "/foo",
			want:     url.URL{Scheme: "https", Host: "example.com", Path: "/prefix/v2/keys/foo"},
		},
	}

	for i, tt := range tests {
		got := v2URL(tt.endpoint, tt.key)
		if tt.want != *got {
			t.Errorf("#%d: want=%#v, got=%#v", i, tt.want, *got)
		}
	}
}

func TestGetAction(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com"}
	wantURL := &url.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/v2/keys/foo/bar",
	}
	wantHeader := http.Header{}

	tests := []struct {
		recursive bool
		wantQuery string
	}{
		{
			recursive: false,
			wantQuery: "recursive=false",
		},
		{
			recursive: true,
			wantQuery: "recursive=true",
		},
	}

	for i, tt := range tests {
		f := getAction{
			Key:       "/foo/bar",
			Recursive: tt.recursive,
		}
		got := *f.httpRequest(ep)

		wantURL := wantURL
		wantURL.RawQuery = tt.wantQuery

		err := assertResponse(got, wantURL, wantHeader, nil)
		if err != nil {
			t.Errorf("%#d: %v", i, err)
		}
	}
}

func TestWaitAction(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com"}
	wantURL := &url.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/v2/keys/foo/bar",
	}
	wantHeader := http.Header{}

	tests := []struct {
		waitIndex uint64
		recursive bool
		wantQuery string
	}{
		{
			recursive: false,
			waitIndex: uint64(0),
			wantQuery: "recursive=false&wait=true&waitIndex=0",
		},
		{
			recursive: false,
			waitIndex: uint64(12),
			wantQuery: "recursive=false&wait=true&waitIndex=12",
		},
		{
			recursive: true,
			waitIndex: uint64(12),
			wantQuery: "recursive=true&wait=true&waitIndex=12",
		},
	}

	for i, tt := range tests {
		f := waitAction{
			Key:       "/foo/bar",
			WaitIndex: tt.waitIndex,
			Recursive: tt.recursive,
		}
		got := *f.httpRequest(ep)

		wantURL := wantURL
		wantURL.RawQuery = tt.wantQuery

		err := assertResponse(got, wantURL, wantHeader, nil)
		if err != nil {
			t.Errorf("%#d: %v", i, err)
		}
	}
}

func TestCreateAction(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com"}
	wantURL := &url.URL{
		Scheme:   "http",
		Host:     "example.com",
		Path:     "/v2/keys/foo/bar",
		RawQuery: "prevExist=false",
	}
	wantHeader := http.Header(map[string][]string{
		"Content-Type": []string{"application/x-www-form-urlencoded"},
	})

	ttl12 := uint64(12)
	tests := []struct {
		value    string
		ttl      *uint64
		wantBody string
	}{
		{
			value:    "baz",
			wantBody: "value=baz",
		},
		{
			value:    "baz",
			ttl:      &ttl12,
			wantBody: "ttl=12&value=baz",
		},
	}

	for i, tt := range tests {
		f := createAction{
			Key:   "/foo/bar",
			Value: tt.value,
			TTL:   tt.ttl,
		}
		got := *f.httpRequest(ep)

		err := assertResponse(got, wantURL, wantHeader, []byte(tt.wantBody))
		if err != nil {
			t.Errorf("%#d: %v", i, err)
		}
	}
}

func assertResponse(got http.Request, wantURL *url.URL, wantHeader http.Header, wantBody []byte) error {
	if !reflect.DeepEqual(wantURL, got.URL) {
		return fmt.Errorf("want.URL=%#v got.URL=%#v", wantURL, got.URL)
	}

	if !reflect.DeepEqual(wantHeader, got.Header) {
		return fmt.Errorf("want.Header=%#v got.Header=%#v", wantHeader, got.Header)
	}

	if got.Body == nil {
		if wantBody != nil {
			return fmt.Errorf("want.Body=%v got.Body=%v", wantBody, got.Body)
		}
	} else {
		if wantBody == nil {
			return fmt.Errorf("want.Body=%v got.Body=%v", wantBody, got.Body)
		} else {
			gotBytes, err := ioutil.ReadAll(got.Body)
			if err != nil {
				return err
			}

			if !reflect.DeepEqual(wantBody, gotBytes) {
				return fmt.Errorf("want.Body=%v got.Body=%v", wantBody, gotBytes)
			}
		}
	}

	return nil
}

func TestUnmarshalSuccessfulResponse(t *testing.T) {
	tests := []struct {
		body        string
		res         *Response
		expectError bool
	}{
		// Neither PrevNode or Node
		{
			`{"action":"delete"}`,
			&Response{Action: "delete"},
			false,
		},

		// PrevNode
		{
			`{"action":"delete", "prevNode": {"key": "/foo", "value": "bar", "modifiedIndex": 12, "createdIndex": 10}}`,
			&Response{Action: "delete", PrevNode: &Node{Key: "/foo", Value: "bar", ModifiedIndex: 12, CreatedIndex: 10}},
			false,
		},

		// Node
		{
			`{"action":"get", "node": {"key": "/foo", "value": "bar", "modifiedIndex": 12, "createdIndex": 10}}`,
			&Response{Action: "get", Node: &Node{Key: "/foo", Value: "bar", ModifiedIndex: 12, CreatedIndex: 10}},
			false,
		},

		// PrevNode and Node
		{
			`{"action":"update", "prevNode": {"key": "/foo", "value": "baz", "modifiedIndex": 10, "createdIndex": 10}, "node": {"key": "/foo", "value": "bar", "modifiedIndex": 12, "createdIndex": 10}}`,
			&Response{Action: "update", PrevNode: &Node{Key: "/foo", Value: "baz", ModifiedIndex: 10, CreatedIndex: 10}, Node: &Node{Key: "/foo", Value: "bar", ModifiedIndex: 12, CreatedIndex: 10}},
			false,
		},

		// Garbage in body
		{
			`garbage`,
			nil,
			true,
		},
	}

	for i, tt := range tests {
		res, err := unmarshalSuccessfulResponse([]byte(tt.body))
		if tt.expectError != (err != nil) {
			t.Errorf("#%d: expectError=%t, err=%v", i, tt.expectError, err)
		}

		if (res == nil) != (tt.res == nil) {
			t.Errorf("#%d: received res==%v, but expected res==%v", i, res, tt.res)
			continue
		} else if tt.res == nil {
			// expected and succesfully got nil response
			continue
		}

		if res.Action != tt.res.Action {
			t.Errorf("#%d: Action=%s, expected %s", i, res.Action, tt.res.Action)
		}

		if !reflect.DeepEqual(res.Node, tt.res.Node) {
			t.Errorf("#%d: Node=%v, expected %v", i, res.Node, tt.res.Node)
		}
	}
}

func TestUnmarshalErrorResponse(t *testing.T) {
	unrecognized := errors.New("test fixture")

	tests := []struct {
		code int
		want error
	}{
		{http.StatusBadRequest, unrecognized},
		{http.StatusUnauthorized, unrecognized},
		{http.StatusPaymentRequired, unrecognized},
		{http.StatusForbidden, unrecognized},
		{http.StatusNotFound, ErrKeyNoExist},
		{http.StatusMethodNotAllowed, unrecognized},
		{http.StatusNotAcceptable, unrecognized},
		{http.StatusProxyAuthRequired, unrecognized},
		{http.StatusRequestTimeout, unrecognized},
		{http.StatusConflict, unrecognized},
		{http.StatusGone, unrecognized},
		{http.StatusLengthRequired, unrecognized},
		{http.StatusPreconditionFailed, ErrKeyExists},
		{http.StatusRequestEntityTooLarge, unrecognized},
		{http.StatusRequestURITooLong, unrecognized},
		{http.StatusUnsupportedMediaType, unrecognized},
		{http.StatusRequestedRangeNotSatisfiable, unrecognized},
		{http.StatusExpectationFailed, unrecognized},
		{http.StatusTeapot, unrecognized},

		{http.StatusInternalServerError, ErrNoLeader},
		{http.StatusNotImplemented, unrecognized},
		{http.StatusBadGateway, unrecognized},
		{http.StatusServiceUnavailable, unrecognized},
		{http.StatusGatewayTimeout, unrecognized},
		{http.StatusHTTPVersionNotSupported, unrecognized},
	}

	for i, tt := range tests {
		want := tt.want
		if reflect.DeepEqual(unrecognized, want) {
			want = fmt.Errorf("unrecognized HTTP status code %d", tt.code)
		}

		got := unmarshalErrorResponse(tt.code)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("#%d: want=%v, got=%v", i, want, got)
		}
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

func (a *fakeAction) httpRequest(url.URL) *http.Request {
	return &http.Request{}
}

func TestHTTPClientDoSuccess(t *testing.T) {
	tr := newFakeTransport()
	c := &httpClient{transport: tr}

	tr.respchan <- &http.Response{
		StatusCode: http.StatusTeapot,
		Body:       ioutil.NopCloser(strings.NewReader("foo")),
	}

	resp, body, err := c.do(context.Background(), &fakeAction{})
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

	_, _, err := c.do(context.Background(), &fakeAction{})
	if err == nil {
		t.Fatalf("expected non-nil error, got nil")
	}
}

func TestHTTPClientDoCancelContext(t *testing.T) {
	tr := newFakeTransport()
	c := &httpClient{transport: tr}

	tr.startCancel <- struct{}{}
	tr.finishCancel <- struct{}{}

	_, _, err := c.do(context.Background(), &fakeAction{})
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
		c.do(ctx, &fakeAction{})
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
