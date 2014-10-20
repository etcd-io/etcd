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

package proxy

import (
	"net/url"
	"reflect"
	"testing"
)

func TestNewDirectorScheme(t *testing.T) {
	tests := []struct {
		scheme string
		addrs  []string
		want   []string
	}{
		{
			scheme: "http",
			addrs:  []string{"192.0.2.8:4002", "example.com:8080"},
			want:   []string{"http://192.0.2.8:4002", "http://example.com:8080"},
		},
		{
			scheme: "https",
			addrs:  []string{"192.0.2.8:4002", "example.com:8080"},
			want:   []string{"https://192.0.2.8:4002", "https://example.com:8080"},
		},

		// accept addrs without a port
		{
			scheme: "http",
			addrs:  []string{"192.0.2.8"},
			want:   []string{"http://192.0.2.8"},
		},

		// accept addrs even if they are garbage
		{
			scheme: "http",
			addrs:  []string{"."},
			want:   []string{"http://."},
		},
	}

	for i, tt := range tests {
		got, err := newDirector(tt.scheme, tt.addrs)
		if err != nil {
			t.Errorf("#%d: newDirectory returned unexpected error: %v", i, err)
		}

		for ii, wep := range tt.want {
			gep := got.ep[ii].URL.String()
			if !reflect.DeepEqual(wep, gep) {
				t.Errorf("#%d: want endpoints[%d] = %#v, got = %#v", i, ii, wep, gep)
			}
		}
	}
}

func TestDirectorEndpointsFiltering(t *testing.T) {
	d := director{
		ep: []*endpoint{
			&endpoint{
				URL:       url.URL{Scheme: "http", Host: "192.0.2.5:5050"},
				Available: false,
			},
			&endpoint{
				URL:       url.URL{Scheme: "http", Host: "192.0.2.4:4000"},
				Available: true,
			},
		},
	}

	got := d.endpoints()
	want := []*endpoint{
		&endpoint{
			URL:       url.URL{Scheme: "http", Host: "192.0.2.4:4000"},
			Available: true,
		},
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("directed to incorrect endpoint: want = %#v, got = %#v", want, got)
	}
}
