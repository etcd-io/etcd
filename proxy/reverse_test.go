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

package proxy

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

type staticRoundTripper struct {
	res *http.Response
	err error
}

func (srt *staticRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return srt.res, srt.err
}

func TestReverseProxyServe(t *testing.T) {
	u := url.URL{Scheme: "http", Host: "192.0.2.3:4040"}

	tests := []struct {
		eps  []*endpoint
		rt   http.RoundTripper
		want int
	}{
		// no endpoints available so no requests are even made
		{
			eps: []*endpoint{},
			rt: &staticRoundTripper{
				res: &http.Response{
					StatusCode: http.StatusCreated,
					Body:       ioutil.NopCloser(&bytes.Reader{}),
				},
			},
			want: http.StatusServiceUnavailable,
		},

		// error is returned from one endpoint that should be available
		{
			eps:  []*endpoint{{URL: u, Available: true}},
			rt:   &staticRoundTripper{err: errors.New("what a bad trip")},
			want: http.StatusBadGateway,
		},

		// endpoint is available and returns success
		{
			eps: []*endpoint{{URL: u, Available: true}},
			rt: &staticRoundTripper{
				res: &http.Response{
					StatusCode: http.StatusCreated,
					Body:       ioutil.NopCloser(&bytes.Reader{}),
					Header:     map[string][]string{"Content-Type": {"application/json"}},
				},
			},
			want: http.StatusCreated,
		},
	}

	for i, tt := range tests {
		rp := reverseProxy{
			director:  &director{ep: tt.eps},
			transport: tt.rt,
		}

		req, _ := http.NewRequest("GET", "http://192.0.2.2:2379", nil)
		rr := httptest.NewRecorder()
		rp.ServeHTTP(rr, req)

		if rr.Code != tt.want {
			t.Errorf("#%d: unexpected HTTP status code: want = %d, got = %d", i, tt.want, rr.Code)
		}
		if gct := rr.Header().Get("Content-Type"); gct != "application/json" {
			t.Errorf("#%d: Content-Type = %s, want %s", i, gct, "application/json")
		}
	}
}

func TestRedirectRequest(t *testing.T) {
	loc := url.URL{
		Scheme: "http",
		Host:   "bar.example.com",
	}

	req := &http.Request{
		Method: "GET",
		Host:   "foo.example.com",
		URL: &url.URL{
			Host: "foo.example.com",
			Path: "/v2/keys/baz",
		},
	}

	redirectRequest(req, loc)

	want := &http.Request{
		Method: "GET",
		// this field must not change
		Host: "foo.example.com",
		URL: &url.URL{
			// the Scheme field is updated to that of the provided URL
			Scheme: "http",
			// the Host field is updated to that of the provided URL
			Host: "bar.example.com",
			Path: "/v2/keys/baz",
		},
	}

	if !reflect.DeepEqual(want, req) {
		t.Fatalf("HTTP request does not match expected criteria: want=%#v got=%#v", want, req)
	}
}

func TestMaybeSetForwardedFor(t *testing.T) {
	tests := []struct {
		raddr  string
		fwdFor string
		want   string
	}{
		{"192.0.2.3:8002", "", "192.0.2.3"},
		{"192.0.2.3:8002", "192.0.2.2", "192.0.2.2, 192.0.2.3"},
		{"192.0.2.3:8002", "192.0.2.1, 192.0.2.2", "192.0.2.1, 192.0.2.2, 192.0.2.3"},
		{"example.com:8002", "", "example.com"},

		// While these cases look valid, golang net/http will not let it happen
		// The RemoteAddr field will always be a valid host:port
		{":8002", "", ""},
		{"192.0.2.3", "", ""},

		// blatantly invalid host w/o a port
		{"12", "", ""},
		{"12", "192.0.2.3", "192.0.2.3"},
	}

	for i, tt := range tests {
		req := &http.Request{
			RemoteAddr: tt.raddr,
			Header:     make(http.Header),
		}

		if tt.fwdFor != "" {
			req.Header.Set("X-Forwarded-For", tt.fwdFor)
		}

		maybeSetForwardedFor(req)
		got := req.Header.Get("X-Forwarded-For")
		if tt.want != got {
			t.Errorf("#%d: incorrect header: want = %q, got = %q", i, tt.want, got)
		}
	}
}

func TestRemoveSingleHopHeaders(t *testing.T) {
	hdr := http.Header(map[string][]string{
		// single-hop headers that should be removed
		"Connection":          {"close"},
		"Keep-Alive":          {"foo"},
		"Proxy-Authenticate":  {"Basic realm=example.com"},
		"Proxy-Authorization": {"foo"},
		"Te":                {"deflate,gzip"},
		"Trailers":          {"ETag"},
		"Transfer-Encoding": {"chunked"},
		"Upgrade":           {"WebSocket"},

		// headers that should persist
		"Accept": {"application/json"},
		"X-Foo":  {"Bar"},
	})

	removeSingleHopHeaders(&hdr)

	want := http.Header(map[string][]string{
		"Accept": {"application/json"},
		"X-Foo":  {"Bar"},
	})

	if !reflect.DeepEqual(want, hdr) {
		t.Fatalf("unexpected result: want = %#v, got = %#v", want, hdr)
	}
}

func TestCopyHeader(t *testing.T) {
	tests := []struct {
		src  http.Header
		dst  http.Header
		want http.Header
	}{
		{
			src: http.Header(map[string][]string{
				"Foo": {"bar", "baz"},
			}),
			dst: http.Header(map[string][]string{}),
			want: http.Header(map[string][]string{
				"Foo": {"bar", "baz"},
			}),
		},
		{
			src: http.Header(map[string][]string{
				"Foo":  {"bar"},
				"Ping": {"pong"},
			}),
			dst: http.Header(map[string][]string{}),
			want: http.Header(map[string][]string{
				"Foo":  {"bar"},
				"Ping": {"pong"},
			}),
		},
		{
			src: http.Header(map[string][]string{
				"Foo": {"bar", "baz"},
			}),
			dst: http.Header(map[string][]string{
				"Foo": {"qux"},
			}),
			want: http.Header(map[string][]string{
				"Foo": {"qux", "bar", "baz"},
			}),
		},
	}

	for i, tt := range tests {
		copyHeader(tt.dst, tt.src)
		if !reflect.DeepEqual(tt.dst, tt.want) {
			t.Errorf("#%d: unexpected headers: want = %v, got = %v", i, tt.want, tt.dst)
		}
	}
}

func TestCheckProxyLoop(t *testing.T) {
	tests := []struct {
		proxyID   string
		endpoints []string
		rp        string
	}{
		{"testID", []string{"http://localhost:8000", "http://127.0.0.1:22379", "http://127.0.0.1:32379"}, "http://localhost:8000"},
		{"testID", []string{"http://localhost:8080", "http://127.0.0.1:22379", "http://127.0.0.1:32379"}, "http://localhost:8080"},
	}
	for i, tt := range tests {
		urlsFunc := func() []string {
			return tt.endpoints
		}
		p := &reverseProxy{
			director:  newDirector(urlsFunc, 30*time.Second, 30*time.Second),
			transport: http.DefaultTransport,
		}

		rq := &http.Request{}
		hm := make(map[string][]string)
		rq.Header = hm

		for _, ep := range p.director.endpoints() {
			if tt.rp != ep.URL.String() {
				continue
			}
			recordProxy(rq, tt.proxyID, ep.URL)
		}

		es, isLoop := checkProxyLoop(rq, tt.proxyID)
		if !isLoop {
			// es must be detected as a proxy cycle
			t.Errorf("#%d: got = %v, want true", i, isLoop)
		}

		if tt.rp != es {
			t.Errorf("#%d: %s should have been detected as a proxy loop but got %s", i, tt.rp, es)
		}
	}
}

func TestBanEndpoint(t *testing.T) {
	tests := []struct {
		endpoints []string
		rp        string
	}{
		{[]string{"http://localhost:8000", "http://127.0.0.1:22379", "http://127.0.0.1:32379"}, "http://localhost:8000"},
		{[]string{"http://localhost:8080", "http://127.0.0.1:22379", "http://127.0.0.1:32379"}, "http://localhost:8080"},
	}
	for i, tt := range tests {
		urlsFunc := func() []string {
			return tt.endpoints
		}
		d := newDirector(urlsFunc, 30*time.Second, 30*time.Second)
		d.banEndpoint(tt.rp)
		for j, ep := range d.endpoints() {
			if tt.rp == ep.URL.String() {
				t.Errorf("#%d-%d: %v should have been banned and removed from endpoints", i, j, ep.URL)
			}
		}
	}
}
