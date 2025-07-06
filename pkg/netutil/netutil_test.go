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

package netutil

import (
	"context"
	"errors"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestResolveTCPAddrs(t *testing.T) {
	defer func() { resolveTCPAddr = resolveTCPAddrDefault }()
	tests := []struct {
		urls     [][]url.URL
		expected [][]url.URL
		hostMap  map[string]string
		hasError bool
	}{
		{
			urls: [][]url.URL{
				{
					{Scheme: "http", Host: "127.0.0.1:4001"},
					{Scheme: "http", Host: "127.0.0.1:2379"},
				},
				{
					{Scheme: "http", Host: "127.0.0.1:7001"},
					{Scheme: "http", Host: "127.0.0.1:2380"},
				},
			},
			expected: [][]url.URL{
				{
					{Scheme: "http", Host: "127.0.0.1:4001"},
					{Scheme: "http", Host: "127.0.0.1:2379"},
				},
				{
					{Scheme: "http", Host: "127.0.0.1:7001"},
					{Scheme: "http", Host: "127.0.0.1:2380"},
				},
			},
		},
		{
			urls: [][]url.URL{
				{
					{Scheme: "http", Host: "infra0.example.com:4001"},
					{Scheme: "http", Host: "infra0.example.com:2379"},
				},
				{
					{Scheme: "http", Host: "infra0.example.com:7001"},
					{Scheme: "http", Host: "infra0.example.com:2380"},
				},
			},
			expected: [][]url.URL{
				{
					{Scheme: "http", Host: "10.0.1.10:4001"},
					{Scheme: "http", Host: "10.0.1.10:2379"},
				},
				{
					{Scheme: "http", Host: "10.0.1.10:7001"},
					{Scheme: "http", Host: "10.0.1.10:2380"},
				},
			},
			hostMap: map[string]string{
				"infra0.example.com": "10.0.1.10",
			},
			hasError: false,
		},
		{
			urls: [][]url.URL{
				{
					{Scheme: "http", Host: "infra0.example.com:4001"},
					{Scheme: "http", Host: "infra0.example.com:2379"},
				},
				{
					{Scheme: "http", Host: "infra0.example.com:7001"},
					{Scheme: "http", Host: "infra0.example.com:2380"},
				},
			},
			hostMap: map[string]string{
				"infra0.example.com": "",
			},
			hasError: true,
		},
		{
			urls: [][]url.URL{
				{
					{Scheme: "http", Host: "ssh://infra0.example.com:4001"},
					{Scheme: "http", Host: "ssh://infra0.example.com:2379"},
				},
				{
					{Scheme: "http", Host: "ssh://infra0.example.com:7001"},
					{Scheme: "http", Host: "ssh://infra0.example.com:2380"},
				},
			},
			hasError: true,
		},
	}
	for _, tt := range tests {
		resolveTCPAddr = func(ctx context.Context, addr string) (*net.TCPAddr, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if tt.hostMap[host] == "" {
				return nil, errors.New("cannot resolve host")
			}
			i, err := strconv.Atoi(port)
			if err != nil {
				return nil, err
			}
			return &net.TCPAddr{IP: net.ParseIP(tt.hostMap[host]), Port: i, Zone: ""}, nil
		}
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
		urls, err := resolveTCPAddrs(ctx, zap.NewExample(), tt.urls)
		cancel()
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error")
			}
			continue
		}
		if !reflect.DeepEqual(urls, tt.expected) {
			t.Errorf("expected: %v, got %v", tt.expected, urls)
		}
	}
}

func TestURLsEqual(t *testing.T) {
	defer func() { resolveTCPAddr = resolveTCPAddrDefault }()
	hostm := map[string]string{
		"example.com": "10.0.10.1",
		"first.com":   "10.0.11.1",
		"second.com":  "10.0.11.2",
	}
	resolveTCPAddr = func(ctx context.Context, addr string) (*net.TCPAddr, error) {
		host, port, herr := net.SplitHostPort(addr)
		if herr != nil {
			return nil, herr
		}
		if _, ok := hostm[host]; !ok {
			return nil, errors.New("cannot resolve host.")
		}
		i, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		return &net.TCPAddr{IP: net.ParseIP(hostm[host]), Port: i, Zone: ""}, nil
	}

	tests := []struct {
		a      []url.URL
		b      []url.URL
		expect bool
		err    error
	}{
		{
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			expect: true,
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}},
			expect: true,
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "https", Host: "10.0.10.1:2379"}},
			expect: false,
			err:    errors.New(`"http://10.0.10.1:2379"(resolved from "http://example.com:2379") != "https://10.0.10.1:2379"(resolved from "https://10.0.10.1:2379")`),
		},
		{
			a:      []url.URL{{Scheme: "https", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}},
			expect: false,
			err:    errors.New(`"https://10.0.10.1:2379"(resolved from "https://example.com:2379") != "http://10.0.10.1:2379"(resolved from "http://10.0.10.1:2379")`),
		},
		{
			a:      []url.URL{{Scheme: "unix", Host: "abc:2379"}},
			b:      []url.URL{{Scheme: "unix", Host: "abc:2379"}},
			expect: true,
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: true,
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: true,
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: true,
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`"http://127.0.0.1:2379"(resolved from "http://127.0.0.1:2379") != "http://127.0.0.1:2380"(resolved from "http://127.0.0.1:2380")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "example.com:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}},
			expect: false,
			err:    errors.New(`"http://10.0.10.1:2380"(resolved from "http://example.com:2380") != "http://10.0.10.1:2379"(resolved from "http://10.0.10.1:2379")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}},
			expect: false,
			err:    errors.New(`"http://127.0.0.1:2379"(resolved from "http://127.0.0.1:2379") != "http://10.0.0.1:2379"(resolved from "http://10.0.0.1:2379")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}},
			expect: false,
			err:    errors.New(`"http://10.0.10.1:2379"(resolved from "http://example.com:2379") != "http://10.0.0.1:2379"(resolved from "http://10.0.0.1:2379")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2380"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`"http://127.0.0.1:2379"(resolved from "http://127.0.0.1:2379") != "http://127.0.0.1:2380"(resolved from "http://127.0.0.1:2380")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2380"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`"http://10.0.10.1:2379"(resolved from "http://example.com:2379") != "http://127.0.0.1:2380"(resolved from "http://127.0.0.1:2380")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`"http://127.0.0.1:2379"(resolved from "http://127.0.0.1:2379") != "http://10.0.0.1:2379"(resolved from "http://10.0.0.1:2379")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`"http://10.0.10.1:2379"(resolved from "http://example.com:2379") != "http://10.0.0.1:2379"(resolved from "http://10.0.0.1:2379")`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`len(["http://10.0.0.1:2379"]) != len(["http://10.0.0.1:2379" "http://127.0.0.1:2380"])`),
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "first.com:2379"}, {Scheme: "http", Host: "second.com:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.11.1:2379"}, {Scheme: "http", Host: "10.0.11.2:2380"}},
			expect: true,
		},
		{
			a:      []url.URL{{Scheme: "http", Host: "second.com:2380"}, {Scheme: "http", Host: "first.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.11.1:2379"}, {Scheme: "http", Host: "10.0.11.2:2380"}},
			expect: true,
		},
	}

	for i, test := range tests {
		result, err := urlsEqual(context.TODO(), zap.NewExample(), test.a, test.b)
		if result != test.expect {
			t.Errorf("#%d: a:%v b:%v, expected %v but %v", i, test.a, test.b, test.expect, result)
		}
		if test.err != nil {
			if err.Error() != test.err.Error() {
				t.Errorf("#%d: err expected %v but %v", i, test.err, err)
			}
		}
	}
}
func TestURLStringsEqual(t *testing.T) {
	result, err := URLStringsEqual(context.TODO(), zap.NewExample(), []string{"http://127.0.0.1:8080"}, []string{"http://127.0.0.1:8080"})
	if !result {
		t.Errorf("unexpected result %v", result)
	}
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}
