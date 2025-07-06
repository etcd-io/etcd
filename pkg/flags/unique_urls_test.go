// Copyright 2018 The etcd Authors
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
	"reflect"
	"testing"
)

func TestNewUniqueURLsWithExceptions(t *testing.T) {
	tests := []struct {
		s         string
		exp       map[string]struct{}
		rs        string
		exception string
	}{
		{ // non-URL but allowed by exception
			s:         "*",
			exp:       map[string]struct{}{"*": {}},
			rs:        "*",
			exception: "*",
		},
		{
			s:         "",
			exp:       map[string]struct{}{},
			rs:        "",
			exception: "*",
		},
		{
			s:         "https://1.2.3.4:8080",
			exp:       map[string]struct{}{"https://1.2.3.4:8080": {}},
			rs:        "https://1.2.3.4:8080",
			exception: "*",
		},
		{
			s:         "https://1.2.3.4:8080,https://1.2.3.4:8080",
			exp:       map[string]struct{}{"https://1.2.3.4:8080": {}},
			rs:        "https://1.2.3.4:8080",
			exception: "*",
		},
		{
			s:         "http://10.1.1.1:80",
			exp:       map[string]struct{}{"http://10.1.1.1:80": {}},
			rs:        "http://10.1.1.1:80",
			exception: "*",
		},
		{
			s:         "http://localhost:80",
			exp:       map[string]struct{}{"http://localhost:80": {}},
			rs:        "http://localhost:80",
			exception: "*",
		},
		{
			s:         "http://:80",
			exp:       map[string]struct{}{"http://:80": {}},
			rs:        "http://:80",
			exception: "*",
		},
		{
			s:         "https://localhost:5,https://localhost:3",
			exp:       map[string]struct{}{"https://localhost:3": {}, "https://localhost:5": {}},
			rs:        "https://localhost:3,https://localhost:5",
			exception: "*",
		},
		{
			s:         "http://localhost:5,https://localhost:3",
			exp:       map[string]struct{}{"https://localhost:3": {}, "http://localhost:5": {}},
			rs:        "http://localhost:5,https://localhost:3",
			exception: "*",
		},
	}
	for i := range tests {
		uv := NewUniqueURLsWithExceptions(tests[i].s, tests[i].exception)
		if !reflect.DeepEqual(tests[i].exp, uv.Values) {
			t.Fatalf("#%d: expected %+v, got %+v", i, tests[i].exp, uv.Values)
		}
		if uv.String() != tests[i].rs {
			t.Fatalf("#%d: expected %q, got %q", i, tests[i].rs, uv.String())
		}
	}
}
