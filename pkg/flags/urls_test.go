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

package flags

import (
	"flag"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateURLsValueBad(t *testing.T) {
	tests := []string{
		// bad IP specification
		":2379",
		"127.0:8080",
		"123:456",
		// bad port specification
		"127.0.0.1:foo",
		"127.0.0.1:",
		// unix sockets not supported
		"unix://",
		"unix://tmp/etcd.sock",
		// bad strings
		"somewhere",
		"234#$",
		"file://foo/bar",
		"http://hello/asdf",
		"http://10.1.1.1",
	}
	for i, in := range tests {
		u := URLsValue{}
		if err := u.Set(in); err == nil {
			t.Errorf(`#%d: unexpected nil error for in=%q`, i, in)
		}
	}
}

func TestNewURLsValue(t *testing.T) {
	tests := []struct {
		s   string
		exp []url.URL
	}{
		{s: "https://1.2.3.4:8080", exp: []url.URL{{Scheme: "https", Host: "1.2.3.4:8080"}}},
		{s: "http://10.1.1.1:80", exp: []url.URL{{Scheme: "http", Host: "10.1.1.1:80"}}},
		{s: "http://localhost:80", exp: []url.URL{{Scheme: "http", Host: "localhost:80"}}},
		{s: "http://:80", exp: []url.URL{{Scheme: "http", Host: ":80"}}},
		{
			s: "http://localhost:1,https://localhost:2",
			exp: []url.URL{
				{Scheme: "http", Host: "localhost:1"},
				{Scheme: "https", Host: "localhost:2"},
			},
		},
	}
	for i := range tests {
		uu := []url.URL(*NewURLsValue(tests[i].s))
		require.Equal(t, tests[i].exp, uu)
	}
}

func TestUniqueURLsFromFlag(t *testing.T) {
	const name = "test"
	urls := []string{
		"https://1.2.3.4:1",
		"https://1.2.3.4:2",
		"https://1.2.3.4:3",
		"https://1.2.3.4:1",
	}
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	u := NewUniqueURLsWithExceptions(strings.Join(urls, ","))
	fs.Var(u, name, "usage")
	uss := UniqueURLsFromFlag(fs, name)

	require.Equal(t, len(u.Values), len(uss))

	um := make(map[string]struct{})
	for _, x := range uss {
		um[x.String()] = struct{}{}
	}
	require.Equal(t, u.Values, um)
}
