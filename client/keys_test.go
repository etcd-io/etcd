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
	"testing"
)

func TestV2KeysURLHelper(t *testing.T) {
	tests := []struct {
		endpoint url.URL
		prefix   string
		key      string
		want     url.URL
	}{
		// key is empty, no problem
		{
			endpoint: url.URL{Scheme: "http", Host: "example.com", Path: "/v2/keys"},
			prefix:   "",
			key:      "",
			want:     url.URL{Scheme: "http", Host: "example.com", Path: "/v2/keys"},
		},

		// key is joined to path
		{
			endpoint: url.URL{Scheme: "http", Host: "example.com", Path: "/v2/keys"},
			prefix:   "",
			key:      "/foo/bar",
			want:     url.URL{Scheme: "http", Host: "example.com", Path: "/v2/keys/foo/bar"},
		},

		// key is joined to path when path is empty
		{
			endpoint: url.URL{Scheme: "http", Host: "example.com", Path: ""},
			prefix:   "",
			key:      "/foo/bar",
			want:     url.URL{Scheme: "http", Host: "example.com", Path: "/foo/bar"},
		},

		// Host field carries through with port
		{
			endpoint: url.URL{Scheme: "http", Host: "example.com:8080", Path: "/v2/keys"},
			prefix:   "",
			key:      "",
			want:     url.URL{Scheme: "http", Host: "example.com:8080", Path: "/v2/keys"},
		},

		// Scheme carries through
		{
			endpoint: url.URL{Scheme: "https", Host: "example.com", Path: "/v2/keys"},
			prefix:   "",
			key:      "",
			want:     url.URL{Scheme: "https", Host: "example.com", Path: "/v2/keys"},
		},
		// Prefix is applied
		{
			endpoint: url.URL{Scheme: "https", Host: "example.com", Path: "/foo"},
			prefix:   "/bar",
			key:      "/baz",
			want:     url.URL{Scheme: "https", Host: "example.com", Path: "/foo/bar/baz"},
		},
	}

	for i, tt := range tests {
		got := v2KeysURL(tt.endpoint, tt.prefix, tt.key)
		if tt.want != *got {
			t.Errorf("#%d: want=%#v, got=%#v", i, tt.want, *got)
		}
	}
}

func TestGetAction(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com/v2/keys"}
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
		got := *f.HTTPRequest(ep)

		wantURL := wantURL
		wantURL.RawQuery = tt.wantQuery

		err := assertResponse(got, wantURL, wantHeader, nil)
		if err != nil {
			t.Errorf("#%d: %v", i, err)
		}
	}
}

func TestWaitAction(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com/v2/keys"}
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
		got := *f.HTTPRequest(ep)

		wantURL := wantURL
		wantURL.RawQuery = tt.wantQuery

		err := assertResponse(got, wantURL, wantHeader, nil)
		if err != nil {
			t.Errorf("#%d: %v", i, err)
		}
	}
}

func TestCreateAction(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com/v2/keys"}
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
		got := *f.HTTPRequest(ep)

		err := assertResponse(got, wantURL, wantHeader, []byte(tt.wantBody))
		if err != nil {
			t.Errorf("#%d: %v", i, err)
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
			return fmt.Errorf("want.Body=%v got.Body=%s", wantBody, got.Body)
		} else {
			gotBytes, err := ioutil.ReadAll(got.Body)
			if err != nil {
				return err
			}

			if !reflect.DeepEqual(wantBody, gotBytes) {
				return fmt.Errorf("want.Body=%s got.Body=%s", wantBody, gotBytes)
			}
		}
	}

	return nil
}

func TestUnmarshalSuccessfulResponse(t *testing.T) {
	tests := []struct {
		indexHeader string
		body        string
		res         *Response
		expectError bool
	}{
		// Neither PrevNode or Node
		{
			"1",
			`{"action":"delete"}`,
			&Response{Action: "delete", Index: 1},
			false,
		},

		// PrevNode
		{
			"15",
			`{"action":"delete", "prevNode": {"key": "/foo", "value": "bar", "modifiedIndex": 12, "createdIndex": 10}}`,
			&Response{Action: "delete", Index: 15, PrevNode: &Node{Key: "/foo", Value: "bar", ModifiedIndex: 12, CreatedIndex: 10}},
			false,
		},

		// Node
		{
			"15",
			`{"action":"get", "node": {"key": "/foo", "value": "bar", "modifiedIndex": 12, "createdIndex": 10}}`,
			&Response{Action: "get", Index: 15, Node: &Node{Key: "/foo", Value: "bar", ModifiedIndex: 12, CreatedIndex: 10}},
			false,
		},

		// PrevNode and Node
		{
			"15",
			`{"action":"update", "prevNode": {"key": "/foo", "value": "baz", "modifiedIndex": 10, "createdIndex": 10}, "node": {"key": "/foo", "value": "bar", "modifiedIndex": 12, "createdIndex": 10}}`,
			&Response{Action: "update", Index: 15, PrevNode: &Node{Key: "/foo", Value: "baz", ModifiedIndex: 10, CreatedIndex: 10}, Node: &Node{Key: "/foo", Value: "bar", ModifiedIndex: 12, CreatedIndex: 10}},
			false,
		},

		// Garbage in body
		{
			"",
			`garbage`,
			nil,
			true,
		},
	}

	for i, tt := range tests {
		h := make(http.Header)
		h.Add("X-Etcd-Index", tt.indexHeader)
		res, err := unmarshalSuccessfulResponse(h, []byte(tt.body))
		if tt.expectError != (err != nil) {
			t.Errorf("#%d: expectError=%t, err=%v", i, tt.expectError, err)
		}

		if (res == nil) != (tt.res == nil) {
			t.Errorf("#%d: received res==%v, but expected res==%v", i, res, tt.res)
			continue
		} else if tt.res == nil {
			// expected and successfully got nil response
			continue
		}

		if res.Action != tt.res.Action {
			t.Errorf("#%d: Action=%s, expected %s", i, res.Action, tt.res.Action)
		}
		if res.Index != tt.res.Index {
			t.Errorf("#%d: Index=%d, expected %d", i, res.Index, tt.res.Index)
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
		{http.StatusGatewayTimeout, ErrTimeout},
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
