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

package etcdmain

import (
	"net/url"
	"testing"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/pkg/types"
)

func mustClsFromString(t *testing.T, s string) *etcdserver.Cluster {
	cls, err := etcdserver.NewClusterFromString("", s)
	if err != nil {
		t.Fatalf("error creating cluster from %q: %v", s, err)
	}
	return cls
}

func mustNewURLs(t *testing.T, urls []string) []url.URL {
	u, err := types.NewURLs(urls)
	if err != nil {
		t.Fatalf("unexpected new urls error: %v", err)
	}
	return u
}

func TestClusterPeerURLsMatch(t *testing.T) {
	tests := []struct {
		name string
		cls  *etcdserver.Cluster
		urls []url.URL

		w bool
	}{
		{
			name: "default",
			cls:  mustClsFromString(t, "default=http://localhost:12345"),
			urls: mustNewURLs(t, []string{"http://localhost:12345"}),

			w: true,
		},
		{
			name: "default",
			cls:  mustClsFromString(t, "default=http://localhost:7001,other=http://192.168.0.1:7002,default=http://localhost:12345"),
			urls: mustNewURLs(t, []string{"http://localhost:7001", "http://localhost:12345"}),

			w: true,
		},
		{
			name: "infra1",
			cls:  mustClsFromString(t, "infra1=http://localhost:7001"),
			urls: mustNewURLs(t, []string{"http://localhost:12345"}),

			w: false,
		},
		{
			name: "infra1",
			cls:  mustClsFromString(t, "infra1=http://localhost:7001,infra2=http://localhost:12345"),
			urls: mustNewURLs(t, []string{"http://localhost:12345"}),

			w: false,
		},
	}
	for i, tt := range tests {
		if g := clusterPeerURLsMatch(tt.name, tt.cls, tt.urls); g != tt.w {
			t.Errorf("#%d: clusterPeerURLsMatch=%t, want %t", i, g, tt.w)
		}
	}
}

func TestGenClusterString(t *testing.T) {
	tests := []struct {
		token string
		urls  []string
		wstr  string
	}{
		{
			"default", []string{"http://127.0.0.1:4001"},
			"default=http://127.0.0.1:4001",
		},
		{
			"node1", []string{"http://0.0.0.0:2379", "http://1.1.1.1:2379"},
			"node1=http://0.0.0.0:2379,node1=http://1.1.1.1:2379",
		},
	}
	for i, tt := range tests {
		urls := mustNewURLs(t, tt.urls)
		str := genClusterString(tt.token, urls)
		if str != tt.wstr {
			t.Errorf("#%d: cluster = %s, want %s", i, str, tt.wstr)
		}
	}
}
