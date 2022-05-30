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

func TestInvalidUint32(t *testing.T) {
	tests := []string{
		// string
		"invalid",
		// negative number
		"-1",
		// float number
		"0.1",
		"-0.2",
		// larger than math.MaxUint32
		"4294967296",
	}
	for i, in := range tests {
		var u uint32Value
		if err := u.Set(in); err == nil {
			t.Errorf(`#%d: unexpected nil error for in=%q`, i, in)
		}
	}
}

func TestUint32Value(t *testing.T) {
	tests := []struct {
		s   string
		exp uint32
	}{
		{s: "0", exp: 0},
		{s: "1", exp: 1},
		{s: "", exp: 0},
	}
	for i := range tests {
		ss := uint32(*NewUint32Value(tests[i].s))
		if !reflect.DeepEqual(tests[i].exp, ss) {
			t.Fatalf("#%d: expected %q, got %q", i, tests[i].exp, ss)
		}
	}
}
