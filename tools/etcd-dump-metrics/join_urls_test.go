// Copyright 2024 The etcd Authors
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

package main

import (
	"net/url"
	"testing"
)

func parseURL(s string) url.URL {
	x, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return *x
}

func TestJoinURLs(t *testing.T) {
	tests := []struct {
		urls []url.URL
		want string
	}{
		{
			urls: []url.URL{},
			want: "",
		},
		{
			urls: []url.URL{
				parseURL("http://example.com/#x"),
			},
			want: "0=http://example.com/#x",
		},
		{
			urls: []url.URL{
				parseURL("http://example.com/#x"),
				parseURL("http://example.com/#y"),
			},
			want: "0=http://example.com/#x,1=http://example.com/#y",
		},
		{
			urls: []url.URL{
				parseURL("http://example.com/#x"),
				parseURL("http://example.com/#y"),
				parseURL("http://example.com/#z"),
			},
			want: "0=http://example.com/#x,1=http://example.com/#y,2=http://example.com/#z",
		},
	}
	for i, tt := range tests {
		if got := joinURLs(tt.urls); got != tt.want {
			t.Errorf("#%d want: %s, got: %s", i, tt.want, got)
		}
	}
}
