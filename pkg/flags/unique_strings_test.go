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

func TestNewUniqueStrings(t *testing.T) {
	tests := []struct {
		s   string
		exp map[string]struct{}
		rs  string
	}{
		{ // non-URL but allowed by exception
			s:   "*",
			exp: map[string]struct{}{"*": {}},
			rs:  "*",
		},
		{
			s:   "",
			exp: map[string]struct{}{},
			rs:  "",
		},
		{
			s:   "example.com",
			exp: map[string]struct{}{"example.com": {}},
			rs:  "example.com",
		},
		{
			s:   "localhost,localhost",
			exp: map[string]struct{}{"localhost": {}},
			rs:  "localhost",
		},
		{
			s:   "b.com,a.com",
			exp: map[string]struct{}{"a.com": {}, "b.com": {}},
			rs:  "a.com,b.com",
		},
		{
			s:   "c.com,b.com",
			exp: map[string]struct{}{"b.com": {}, "c.com": {}},
			rs:  "b.com,c.com",
		},
	}
	for i := range tests {
		uv := NewUniqueStringsValue(tests[i].s)
		if !reflect.DeepEqual(tests[i].exp, uv.Values) {
			t.Fatalf("#%d: expected %+v, got %+v", i, tests[i].exp, uv.Values)
		}
		if uv.String() != tests[i].rs {
			t.Fatalf("#%d: expected %q, got %q", i, tests[i].rs, uv.String())
		}
	}
}
