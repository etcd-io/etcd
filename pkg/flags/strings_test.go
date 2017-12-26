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
	"reflect"
	"testing"
)

func TestStringsSet(t *testing.T) {
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
		sf := NewStringsFlag(tt.vals...)
		if sf.val != tt.vals[0] {
			t.Errorf("#%d: want default val=%v,but got %v", i, tt.vals[0], sf.val)
		}

		err := sf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}
	}
}

func TestStringSliceFlag(t *testing.T) {
	tests := []struct {
		vals []string

		val  string
		res  []string
		pass bool
	}{
		// known values
		{[]string{"abc", "def"}, "abc,def", []string{"abc", "def"}, true},
		{[]string{"on", "off", "false"}, "on", []string{"on"}, true},

		// unrecognized values
		{[]string{"abc", "def"}, "ghi", nil, false},
		{[]string{"abc", "def"}, "abc,ghi", nil, false},
		{[]string{"on", "off"}, "", nil, false},
	}

	for i, tt := range tests {
		sf := NewStringSliceFlag(tt.vals...)

		err := sf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}

		res := sf.Slice()
		if tt.pass && !reflect.DeepEqual(res, tt.res) {
			t.Errorf("#%d: want slice=%v, but got slice=%v", i, tt.res, res)
		}
	}
}
