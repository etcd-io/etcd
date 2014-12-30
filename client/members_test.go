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
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/coreos/etcd/pkg/types"
)

func TestMembersAPIActionList(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com"}
	act := &membersAPIActionList{}

	wantURL := &url.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/v2/members",
	}

	got := *act.HTTPRequest(ep)
	err := assertResponse(got, wantURL, http.Header{}, nil)
	if err != nil {
		t.Error(err.Error())
	}
}

func TestMembersAPIActionAdd(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com"}
	act := &membersAPIActionAdd{
		peerURLs: types.URLs([]url.URL{
			url.URL{Scheme: "https", Host: "127.0.0.1:8081"},
			url.URL{Scheme: "http", Host: "127.0.0.1:8080"},
		}),
	}

	wantURL := &url.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/v2/members",
	}
	wantHeader := http.Header{
		"Content-Type": []string{"application/json"},
	}
	wantBody := []byte(`{"peerURLs":["https://127.0.0.1:8081","http://127.0.0.1:8080"]}`)

	got := *act.HTTPRequest(ep)
	err := assertResponse(got, wantURL, wantHeader, wantBody)
	if err != nil {
		t.Error(err.Error())
	}
}

func TestMembersAPIActionRemove(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com"}
	act := &membersAPIActionRemove{memberID: "XXX"}

	wantURL := &url.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/v2/members/XXX",
	}

	got := *act.HTTPRequest(ep)
	err := assertResponse(got, wantURL, http.Header{}, nil)
	if err != nil {
		t.Error(err.Error())
	}
}

func TestAssertStatusCode(t *testing.T) {
	if err := assertStatusCode(404, 400); err == nil {
		t.Errorf("assertStatusCode failed to detect conflict in 400 vs 404")
	}

	if err := assertStatusCode(404, 400, 404); err != nil {
		t.Errorf("assertStatusCode found conflict in (404,400) vs 400: %v", err)
	}
}

func TestV2MembersURL(t *testing.T) {
	got := v2MembersURL(url.URL{
		Scheme: "http",
		Host:   "foo.example.com:4002",
		Path:   "/pants",
	})
	want := &url.URL{
		Scheme: "http",
		Host:   "foo.example.com:4002",
		Path:   "/pants/v2/members",
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("v2MembersURL got %#v, want %#v", got, want)
	}
}
