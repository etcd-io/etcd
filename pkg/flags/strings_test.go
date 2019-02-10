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

func TestStringsValue(t *testing.T) {
	tests := []struct {
		s   string
		exp []string
	}{
		{s: "a,b,c", exp: []string{"a", "b", "c"}},
		{s: "a, b,c", exp: []string{"a", " b", "c"}},
		{s: "", exp: []string{}},
	}
	for i := range tests {
		ss := []string(*NewStringsValue(tests[i].s))
		if !reflect.DeepEqual(tests[i].exp, ss) {
			t.Fatalf("#%d: expected %q, got %q", i, tests[i].exp, ss)
		}
	}
}
