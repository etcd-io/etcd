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
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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
			i, err := strconv.Atoi(port)
			if err != nil {
				return nil, err
			}
			if ip := net.ParseIP(host); ip != nil {
				return &net.TCPAddr{IP: ip, Port: i, Zone: ""}, nil
			}
			if tt.hostMap[host] == "" {
				return nil, errors.New("cannot resolve host")
			}
			return &net.TCPAddr{IP: net.ParseIP(tt.hostMap[host]), Port: i, Zone: ""}, nil
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		urls, err := resolveTCPAddrs(ctx, zaptest.NewLogger(t), tt.urls)
		cancel()
		if tt.hasError {
			require.Errorf(t, err, "expected error")
			continue
		}
		assert.Truef(t, reflect.DeepEqual(urls, tt.expected), "expected: %v, got %v", tt.expected, urls)
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
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		i, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			return &net.TCPAddr{IP: ip, Port: i, Zone: ""}, nil
		}
		if hostm[host] == "" {
			return nil, errors.New("cannot resolve host")
		}
		return &net.TCPAddr{IP: net.ParseIP(hostm[host]), Port: i, Zone: ""}, nil
	}

	tests := []struct {
		n      int
		a      []url.URL
		b      []url.URL
		expect bool
		err    error
	}{
		{
			n:      0,
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			expect: true,
		},
		{
			n:      1,
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}},
			expect: true,
		},
		{
			n:      2,
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "https", Host: "10.0.10.1:2379"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://10.0.10.1:2379" != "https://10.0.10.1:2379"`),
		},
		{
			n:      3,
			a:      []url.URL{{Scheme: "https", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}},
			expect: false,
			err:    errors.New(`resolved urls: "https://10.0.10.1:2379" != "http://10.0.10.1:2379"`),
		},
		{
			n:      4,
			a:      []url.URL{{Scheme: "unix", Host: "abc:2379"}},
			b:      []url.URL{{Scheme: "unix", Host: "abc:2379"}},
			expect: true,
		},
		{
			n:      5,
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: true,
		},
		{
			n:      6,
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: true,
		},
		{
			n:      7,
			a:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: true,
		},
		{
			n:      8,
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://127.0.0.1:2379" != "http://127.0.0.1:2380"`),
		},
		{
			n:      9,
			a:      []url.URL{{Scheme: "http", Host: "example.com:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.10.1:2379"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://10.0.10.1:2380" != "http://10.0.10.1:2379"`),
		},
		{
			n:      10,
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://127.0.0.1:2379" != "http://10.0.0.1:2379"`),
		},
		{
			n:      11,
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://10.0.10.1:2379" != "http://10.0.0.1:2379"`),
		},
		{
			n:      12,
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2380"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://127.0.0.1:2379" != "http://127.0.0.1:2380"`),
		},
		{
			n:      13,
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2380"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://10.0.10.1:2379" != "http://127.0.0.1:2380"`),
		},
		{
			n:      14,
			a:      []url.URL{{Scheme: "http", Host: "127.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://127.0.0.1:2379" != "http://10.0.0.1:2379"`),
		},
		{
			n:      15,
			a:      []url.URL{{Scheme: "http", Host: "example.com:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`resolved urls: "http://10.0.10.1:2379" != "http://10.0.0.1:2379"`),
		},
		{
			n:      16,
			a:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}, {Scheme: "http", Host: "127.0.0.1:2380"}},
			expect: false,
			err:    errors.New(`len(["http://10.0.0.1:2379"]) != len(["http://10.0.0.1:2379" "http://127.0.0.1:2380"])`),
		},
		{
			n:      17,
			a:      []url.URL{{Scheme: "http", Host: "first.com:2379"}, {Scheme: "http", Host: "second.com:2380"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.11.1:2379"}, {Scheme: "http", Host: "10.0.11.2:2380"}},
			expect: true,
		},
		{
			n:      18,
			a:      []url.URL{{Scheme: "http", Host: "second.com:2380"}, {Scheme: "http", Host: "first.com:2379"}},
			b:      []url.URL{{Scheme: "http", Host: "10.0.11.1:2379"}, {Scheme: "http", Host: "10.0.11.2:2380"}},
			expect: true,
		},
	}

	for i, test := range tests {
		result, err := urlsEqual(t.Context(), zaptest.NewLogger(t), test.a, test.b)
		assert.Equalf(t, result, test.expect, "idx=%d #%d: a:%v b:%v, expected %v but %v", i, test.n, test.a, test.b, test.expect, result)
		if test.err != nil {
			if err.Error() != test.err.Error() {
				t.Errorf("idx=%d #%d: err expected %v but %v", i, test.n, test.err, err)
			}
		}
	}
}

func TestURLStringsEqual(t *testing.T) {
	defer func() { resolveTCPAddr = resolveTCPAddrDefault }()
	errOnResolve := func(ctx context.Context, addr string) (*net.TCPAddr, error) {
		return nil, fmt.Errorf("unexpected attempt to resolve: %q", addr)
	}
	cases := []struct {
		urlsA    []string
		urlsB    []string
		resolver func(ctx context.Context, addr string) (*net.TCPAddr, error)
	}{
		{[]string{"http://127.0.0.1:8080"}, []string{"http://127.0.0.1:8080"}, resolveTCPAddrDefault},
		{[]string{
			"http://host1:8080",
			"http://host2:8080",
		}, []string{
			"http://host1:8080",
			"http://host2:8080",
		}, errOnResolve},
		{
			urlsA:    []string{"https://[c262:266f:fa53:0ee6:966e:e3f0:d68f:b046]:2380"},
			urlsB:    []string{"https://[c262:266f:fa53:ee6:966e:e3f0:d68f:b046]:2380"},
			resolver: resolveTCPAddrDefault,
		},
	}
	for idx, c := range cases {
		t.Logf("TestURLStringsEqual, case #%d", idx)
		resolveTCPAddr = c.resolver
		result, err := URLStringsEqual(t.Context(), zaptest.NewLogger(t), c.urlsA, c.urlsB)
		assert.Truef(t, result, "unexpected result %v", result)
		assert.NoErrorf(t, err, "unexpected error %v", err)
	}
}
