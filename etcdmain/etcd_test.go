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
	"errors"
	"net"
	"net/url"
	"testing"

	"github.com/coreos/etcd/pkg/types"
)

func mustNewURLs(t *testing.T, urls []string) []url.URL {
	u, err := types.NewURLs(urls)
	if err != nil {
		t.Fatalf("unexpected new urls error: %v", err)
	}
	return u
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

func TestGenDNSClusterString(t *testing.T) {
	tests := []struct {
		withSSL    []*net.SRV
		withoutSSL []*net.SRV
		expected   string
	}{
		{
			[]*net.SRV{},
			[]*net.SRV{},
			"",
		},
		{
			[]*net.SRV{
				&net.SRV{Target: "10.0.0.1", Port: 2480},
				&net.SRV{Target: "10.0.0.2", Port: 2480},
				&net.SRV{Target: "10.0.0.3", Port: 2480},
			},
			[]*net.SRV{},
			"0=https://10.0.0.1:2480,1=https://10.0.0.2:2480,2=https://10.0.0.3:2480",
		},
		{
			[]*net.SRV{
				&net.SRV{Target: "10.0.0.1", Port: 2480},
				&net.SRV{Target: "10.0.0.2", Port: 2480},
				&net.SRV{Target: "10.0.0.3", Port: 2480},
			},
			[]*net.SRV{
				&net.SRV{Target: "10.0.0.1", Port: 7001},
			},
			"0=https://10.0.0.1:2480,1=https://10.0.0.2:2480,2=https://10.0.0.3:2480,0=http://10.0.0.1:7001",
		},
	}

	for i, tt := range tests {
		lookupSRV = func(service string, proto string, domain string) (string, []*net.SRV, error) {
			if service == "etcd-server-ssl" {
				return "", tt.withSSL, nil
			}
			if service == "etcd-server" {
				return "", tt.withoutSSL, nil
			}
			return "", nil, errors.New("Unkown service in mock")
		}
		str, token, err := genDNSClusterString("token")
		if err != nil {
			t.Fatalf("%d: err: %#v", i, err)
		}
		if token != "token" {
			t.Error("Token doesn't match default token")
		}
		if str != tt.expected {
			t.Errorf("#%d: cluster = %s, want %s", i, str, tt.expected)
		}
	}
}
