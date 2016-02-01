// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

package ctxhttp

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

const (
	requestDuration = 100 * time.Millisecond
	requestBody     = "ok"
)

func TestNoTimeout(t *testing.T) {
	ctx := context.Background()
	resp, err := doRequest(ctx)

	if resp == nil || err != nil {
		t.Fatalf("error received from client: %v %v", err, resp)
	}
}

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(requestDuration / 2)
		cancel()
	}()

	resp, err := doRequest(ctx)

	if resp != nil || err == nil {
		t.Fatalf("expected error, didn't get one. resp: %v", resp)
	}
	if err != ctx.Err() {
		t.Fatalf("expected error from context but got: %v", err)
	}
}

func TestCancelAfterRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	resp, err := doRequest(ctx)

	// Cancel before reading the body.
	// Request.Body should still be readable after the context is canceled.
	cancel()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(b) != requestBody {
		t.Fatalf("could not read body: %q %v", b, err)
	}
}

func TestCancelAfterHangingRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-w.(http.CloseNotifier).CloseNotify()
	})

	serv := httptest.NewServer(handler)
	defer serv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	resp, err := Get(ctx, nil, serv.URL)
	if err != nil {
		t.Fatalf("unexpected error in Get: %v", err)
	}

	// Cancel befer reading the body.
	// Reading Request.Body should fail, since the request was
	// canceled before anything was written.
	cancel()

	done := make(chan struct{})

	go func() {
		b, err := ioutil.ReadAll(resp.Body)
		if len(b) != 0 || err == nil {
			t.Errorf(`Read got (%q, %v); want ("", error)`, b, err)
		}
		close(done)
	}()

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Test timed out")
	case <-done:
	}
}

func doRequest(ctx context.Context) (*http.Response, error) {
	var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(requestDuration)
		w.Write([]byte(requestBody))
	})

	serv := httptest.NewServer(okHandler)
	defer serv.Close()

	return Get(ctx, nil, serv.URL)
}

// golang.org/issue/14065
func TestClosesResponseBodyOnCancel(t *testing.T) {
	defer func() { testHookContextDoneBeforeHeaders = nop }()
	defer func() { testHookDoReturned = nop }()
	defer func() { testHookDidBodyClose = nop }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// closed when Do enters select case <-ctx.Done()
	enteredDonePath := make(chan struct{})

	testHookContextDoneBeforeHeaders = func() {
		close(enteredDonePath)
	}

	testHookDoReturned = func() {
		// We now have the result (the Flush'd headers) at least,
		// so we can cancel the request.
		cancel()

		// But block the client.Do goroutine from sending
		// until Do enters into the <-ctx.Done() path, since
		// otherwise if both channels are readable, select
		// picks a random one.
		<-enteredDonePath
	}

	sawBodyClose := make(chan struct{})
	testHookDidBodyClose = func() { close(sawBodyClose) }

	tr := &http.Transport{}
	defer tr.CloseIdleConnections()
	c := &http.Client{Transport: tr}
	req, _ := http.NewRequest("GET", ts.URL, nil)
	_, doErr := Do(ctx, c, req)

	select {
	case <-sawBodyClose:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for body to close")
	}

	if doErr != ctx.Err() {
		t.Errorf("Do error = %v; want %v", doErr, ctx.Err())
	}
}

type noteCloseConn struct {
	net.Conn
	onceClose sync.Once
	closefn   func()
}

func (c *noteCloseConn) Close() error {
	c.onceClose.Do(c.closefn)
	return c.Conn.Close()
}
