/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package strutil

import (
	"testing"
)

func TestIDAsHex(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{
			input: uint64(12),
			want:  "c",
		},
		{
			input: uint64(4918257920282737594),
			want:  "444129853c343bba",
		},
	}

	for i, tt := range tests {
		got := IDAsHex(tt.input)
		if tt.want != got {
			t.Errorf("#%d: IDAsHex failure: want=%v, got=%v", i, tt.want, got)
		}
	}
}

func TestIDFromHex(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{
			input: "17",
			want:  uint64(23),
		},
		{
			input: "612840dae127353",
			want:  uint64(437557308098245459),
		},
	}

	for i, tt := range tests {
		got, err := IDFromHex(tt.input)
		if err != nil {
			t.Errorf("#%d: IDFromHex failure: err=%v", i, err)
			continue
		}
		if tt.want != got {
			t.Errorf("#%d: IDFromHex failure: want=%v, got=%v", i, tt.want, got)
		}
	}
}

func TestIDFromHexFail(t *testing.T) {
	tests := []string{
		"",
		"XXX",
		"612840dae127353612840dae127353",
	}

	for i, tt := range tests {
		_, err := IDFromHex(tt)
		if err == nil {
			t.Fatalf("#%d: IDFromHex expected error, but err=nil", i)
		}
	}
}
