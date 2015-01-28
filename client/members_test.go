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

package client

import (
	"encoding/json"
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
	err := assertRequest(got, "GET", wantURL, http.Header{}, nil)
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
	err := assertRequest(got, "POST", wantURL, wantHeader, wantBody)
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
	err := assertRequest(got, "DELETE", wantURL, http.Header{}, nil)
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

func TestMemberUnmarshal(t *testing.T) {
	tests := []struct {
		body       []byte
		wantMember Member
		wantError  bool
	}{
		// no URLs, just check ID & Name
		{
			body:       []byte(`{"id": "c", "name": "dungarees"}`),
			wantMember: Member{ID: "c", Name: "dungarees", PeerURLs: nil, ClientURLs: nil},
		},

		// both client and peer URLs
		{
			body: []byte(`{"peerURLs": ["http://127.0.0.1:4001"], "clientURLs": ["http://127.0.0.1:4001"]}`),
			wantMember: Member{
				PeerURLs: []string{
					"http://127.0.0.1:4001",
				},
				ClientURLs: []string{
					"http://127.0.0.1:4001",
				},
			},
		},

		// multiple peer URLs
		{
			body: []byte(`{"peerURLs": ["http://127.0.0.1:4001", "https://example.com"]}`),
			wantMember: Member{
				PeerURLs: []string{
					"http://127.0.0.1:4001",
					"https://example.com",
				},
				ClientURLs: nil,
			},
		},

		// multiple client URLs
		{
			body: []byte(`{"clientURLs": ["http://127.0.0.1:4001", "https://example.com"]}`),
			wantMember: Member{
				PeerURLs: nil,
				ClientURLs: []string{
					"http://127.0.0.1:4001",
					"https://example.com",
				},
			},
		},

		// invalid JSON
		{
			body:      []byte(`{"peerU`),
			wantError: true,
		},
	}

	for i, tt := range tests {
		got := Member{}
		err := json.Unmarshal(tt.body, &got)
		if tt.wantError != (err != nil) {
			t.Errorf("#%d: want error %t, got %v", i, tt.wantError, err)
			continue
		}

		if !reflect.DeepEqual(tt.wantMember, got) {
			t.Errorf("#%d: incorrect output: want=%#v, got=%#v", i, tt.wantMember, got)
		}
	}
}

func TestMemberCollectionUnmarshal(t *testing.T) {
	tests := []struct {
		body []byte
		want memberCollection
	}{
		{
			body: []byte(`{"members":[]}`),
			want: memberCollection([]Member{}),
		},
		{
			body: []byte(`{"members":[{"id":"2745e2525fce8fe","peerURLs":["http://127.0.0.1:7003"],"name":"node3","clientURLs":["http://127.0.0.1:4003"]},{"id":"42134f434382925","peerURLs":["http://127.0.0.1:2380","http://127.0.0.1:7001"],"name":"node1","clientURLs":["http://127.0.0.1:2379","http://127.0.0.1:4001"]},{"id":"94088180e21eb87b","peerURLs":["http://127.0.0.1:7002"],"name":"node2","clientURLs":["http://127.0.0.1:4002"]}]}`),
			want: memberCollection(
				[]Member{
					{
						ID:   "2745e2525fce8fe",
						Name: "node3",
						PeerURLs: []string{
							"http://127.0.0.1:7003",
						},
						ClientURLs: []string{
							"http://127.0.0.1:4003",
						},
					},
					{
						ID:   "42134f434382925",
						Name: "node1",
						PeerURLs: []string{
							"http://127.0.0.1:2380",
							"http://127.0.0.1:7001",
						},
						ClientURLs: []string{
							"http://127.0.0.1:2379",
							"http://127.0.0.1:4001",
						},
					},
					{
						ID:   "94088180e21eb87b",
						Name: "node2",
						PeerURLs: []string{
							"http://127.0.0.1:7002",
						},
						ClientURLs: []string{
							"http://127.0.0.1:4002",
						},
					},
				},
			),
		},
	}

	for i, tt := range tests {
		var got memberCollection
		err := json.Unmarshal(tt.body, &got)
		if err != nil {
			t.Errorf("#%d: unexpected error: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("#%d: incorrect output: want=%#v, got=%#v", i, tt.want, got)
		}
	}
}

func TestMemberCreateRequestMarshal(t *testing.T) {
	req := memberCreateRequest{
		PeerURLs: types.URLs([]url.URL{
			url.URL{Scheme: "http", Host: "127.0.0.1:8081"},
			url.URL{Scheme: "https", Host: "127.0.0.1:8080"},
		}),
	}
	want := []byte(`{"peerURLs":["http://127.0.0.1:8081","https://127.0.0.1:8080"]}`)

	got, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("Marshal returned unexpected err=%v", err)
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("Failed to marshal memberCreateRequest: want=%s, got=%s", want, got)
	}
}
