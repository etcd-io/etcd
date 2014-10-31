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

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

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
