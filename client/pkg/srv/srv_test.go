// Copyright 2015 The etcd Authors
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

package srv

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
)

func notFoundErr(service, proto, domain string) error {
	name := fmt.Sprintf("_%s._%s.%s", service, proto, domain)
	return &net.DNSError{Err: "no such host", Name: name, Server: "10.0.0.53:53", IsTimeout: false, IsTemporary: false, IsNotFound: true}
}

func TestSRVGetCluster(t *testing.T) {
	defer func() {
		lookupSRV = net.LookupSRV
		resolveTCPAddr = net.ResolveTCPAddr
	}()

	hasErr := func(err error) bool {
		return err != nil
	}

	name := "dnsClusterTest"
	dns := map[string]string{
		"1.example.com.:2480": "10.0.0.1:2480",
		"2.example.com.:2480": "10.0.0.2:2480",
		"3.example.com.:2480": "10.0.0.3:2480",
		"4.example.com.:2380": "10.0.0.3:2380",
	}
	srvAll := []*net.SRV{
		{Target: "1.example.com.", Port: 2480},
		{Target: "2.example.com.", Port: 2480},
		{Target: "3.example.com.", Port: 2480},
	}
	srvNone := []*net.SRV{}

	tests := []struct {
		service    string
		scheme     string
		withSSL    []*net.SRV
		withoutSSL []*net.SRV
		urls       []string
		expected   string
		werr       bool
	}{
		{
			"etcd-server-ssl",
			"https",
			srvNone,
			srvNone,
			nil,
			"",
			true,
		},
		{
			"etcd-server-ssl",
			"https",
			srvAll,
			srvNone,
			nil,
			"0=https://1.example.com:2480,1=https://2.example.com:2480,2=https://3.example.com:2480",
			false,
		},
		{
			"etcd-server",
			"http",
			srvNone,
			srvAll,
			nil,
			"0=http://1.example.com:2480,1=http://2.example.com:2480,2=http://3.example.com:2480",
			false,
		},
		{
			"etcd-server-ssl",
			"https",
			srvAll,
			srvNone,
			[]string{"https://10.0.0.1:2480"},
			"dnsClusterTest=https://1.example.com:2480,0=https://2.example.com:2480,1=https://3.example.com:2480",
			false,
		},
		// matching local member with resolved addr and return unresolved hostnames
		{
			"etcd-server-ssl",
			"https",
			srvAll,
			srvNone,
			[]string{"https://10.0.0.1:2480"},
			"dnsClusterTest=https://1.example.com:2480,0=https://2.example.com:2480,1=https://3.example.com:2480",
			false,
		},
		// reject if apurls are TLS but SRV is only http
		{
			"etcd-server",
			"http",
			srvNone,
			srvAll,
			[]string{"https://10.0.0.1:2480"},
			"0=http://2.example.com:2480,1=http://3.example.com:2480",
			false,
		},
	}

	resolveTCPAddr = func(network, addr string) (*net.TCPAddr, error) {
		if strings.Contains(addr, "10.0.0.") {
			// accept IP addresses when resolving apurls
			return net.ResolveTCPAddr(network, addr)
		}
		if dns[addr] == "" {
			return nil, errors.New("missing dns record")
		}
		return net.ResolveTCPAddr(network, dns[addr])
	}

	for i, tt := range tests {
		lookupSRV = func(service string, proto string, domain string) (string, []*net.SRV, error) {
			if service == "etcd-server-ssl" {
				if len(tt.withSSL) > 0 {
					return "", tt.withSSL, nil
				}
				return "", nil, notFoundErr(service, proto, domain)
			}
			if service == "etcd-server" {
				if len(tt.withoutSSL) > 0 {
					return "", tt.withoutSSL, nil
				}
				return "", nil, notFoundErr(service, proto, domain)
			}
			return "", nil, errors.New("unknown service in mock")
		}

		urls := testutil.MustNewURLs(t, tt.urls)
		str, err := GetCluster(tt.scheme, tt.service, name, "example.com", urls)

		if hasErr(err) != tt.werr {
			t.Fatalf("%d: err = %#v, want = %#v", i, err, tt.werr)
		}
		if strings.Join(str, ",") != tt.expected {
			t.Errorf("#%d: cluster = %s, want %s", i, str, tt.expected)
		}
	}
}

func TestSRVDiscover(t *testing.T) {
	defer func() { lookupSRV = net.LookupSRV }()

	hasErr := func(err error) bool {
		return err != nil
	}

	tests := []struct {
		withSSL    []*net.SRV
		withoutSSL []*net.SRV
		expected   []string
		werr       bool
	}{
		{
			[]*net.SRV{},
			[]*net.SRV{},
			[]string{},
			true,
		},
		{
			[]*net.SRV{},
			[]*net.SRV{
				{Target: "10.0.0.1", Port: 2480},
				{Target: "10.0.0.2", Port: 2480},
				{Target: "10.0.0.3", Port: 2480},
			},
			[]string{"http://10.0.0.1:2480", "http://10.0.0.2:2480", "http://10.0.0.3:2480"},
			false,
		},
		{
			[]*net.SRV{
				{Target: "10.0.0.1", Port: 2480},
				{Target: "10.0.0.2", Port: 2480},
				{Target: "10.0.0.3", Port: 2480},
			},
			[]*net.SRV{},
			[]string{"https://10.0.0.1:2480", "https://10.0.0.2:2480", "https://10.0.0.3:2480"},
			false,
		},
		{
			[]*net.SRV{
				{Target: "10.0.0.1", Port: 2480},
				{Target: "10.0.0.2", Port: 2480},
				{Target: "10.0.0.3", Port: 2480},
			},
			[]*net.SRV{
				{Target: "10.0.0.1", Port: 7001},
			},
			[]string{"https://10.0.0.1:2480", "https://10.0.0.2:2480", "https://10.0.0.3:2480", "http://10.0.0.1:7001"},
			false,
		},
		{
			[]*net.SRV{
				{Target: "10.0.0.1", Port: 2480},
				{Target: "10.0.0.2", Port: 2480},
				{Target: "10.0.0.3", Port: 2480},
			},
			[]*net.SRV{
				{Target: "10.0.0.1", Port: 7001},
			},
			[]string{"https://10.0.0.1:2480", "https://10.0.0.2:2480", "https://10.0.0.3:2480", "http://10.0.0.1:7001"},
			false,
		},
		{
			[]*net.SRV{
				{Target: "a.example.com", Port: 2480},
				{Target: "b.example.com", Port: 2480},
				{Target: "c.example.com", Port: 2480},
			},
			[]*net.SRV{},
			[]string{"https://a.example.com:2480", "https://b.example.com:2480", "https://c.example.com:2480"},
			false,
		},
	}

	for i, tt := range tests {
		lookupSRV = func(service string, proto string, domain string) (string, []*net.SRV, error) {
			if service == "etcd-client-ssl" {
				if len(tt.withSSL) > 0 {
					return "", tt.withSSL, nil
				}
				return "", nil, notFoundErr(service, proto, domain)
			}
			if service == "etcd-client" {
				if len(tt.withoutSSL) > 0 {
					return "", tt.withoutSSL, nil
				}
				return "", nil, notFoundErr(service, proto, domain)
			}
			return "", nil, errors.New("unknown service in mock")
		}

		srvs, err := GetClient("etcd-client", "example.com", "")

		if hasErr(err) != tt.werr {
			t.Fatalf("%d: err = %#v, want = %#v", i, err, tt.werr)
		}
		if srvs == nil {
			if len(tt.expected) > 0 {
				t.Errorf("#%d: srvs = nil, want non-nil", i)
			}
		} else {
			if !reflect.DeepEqual(srvs.Endpoints, tt.expected) {
				t.Errorf("#%d: endpoints = %v, want = %v", i, srvs.Endpoints, tt.expected)
			}
		}
	}
}

func TestGetSRVService(t *testing.T) {
	tests := []struct {
		scheme      string
		serviceName string

		expected string
	}{
		{
			"https",
			"",
			"etcd-client-ssl",
		},
		{
			"http",
			"",
			"etcd-client",
		},
		{
			"https",
			"foo",
			"etcd-client-ssl-foo",
		},
		{
			"http",
			"bar",
			"etcd-client-bar",
		},
	}

	for i, tt := range tests {
		service := GetSRVService("etcd-client", tt.serviceName, tt.scheme)
		if strings.Compare(service, tt.expected) != 0 {
			t.Errorf("#%d: service = %s, want %s", i, service, tt.expected)
		}
	}
}
