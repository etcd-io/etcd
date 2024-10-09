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
	"testing"
)

func TestSelectiveStringValue(t *testing.T) {
	tests := []struct {
		vals []string

		val  string
		pass bool
	}{
		// known values
		{[]string{"abc", "def"}, "abc", true},
		{[]string{"on", "off", "false"}, "on", true},

		// unrecognized values
		{[]string{"abc", "def"}, "ghi", false},
		{[]string{"on", "off"}, "", false},
	}
	for i, tt := range tests {
		sf := NewSelectiveStringValue(tt.vals...)
		if sf.v != tt.vals[0] {
			t.Errorf("#%d: want default val=%v,but got %v", i, tt.vals[0], sf.v)
		}
		err := sf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}
	}
}

func TestSelectiveStringsValue(t *testing.T) {
	tests := []struct {
		vals []string

		val  string
		pass bool
	}{
		{[]string{"abc", "def"}, "abc", true},
		{[]string{"abc", "def"}, "abc,def", true},
		{[]string{"abc", "def"}, "abc, def", false},
		{[]string{"on", "off", "false"}, "on,false", true},
		{[]string{"abc", "def"}, "ghi", false},
		{[]string{"on", "off"}, "", false},
		{[]string{"a", "b", "c", "d", "e"}, "a,c,e", true},
	}
	for i, tt := range tests {
		sf := NewSelectiveStringsValue(tt.vals...)
		err := sf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}
	}
}
