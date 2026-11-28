// Copyright 2026 The etcd Authors
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

package command

import (
	"testing"
)

func TestPermRange(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		prefix       bool
		fromKey      bool
		wantKey      string
		wantRangeEnd string
		wantErr      bool
	}{
		{name: "single key", args: []string{"a"}, wantKey: "a"},
		{name: "key and endkey", args: []string{"a", "c"}, wantKey: "a", wantRangeEnd: "c"},
		{name: "prefix", args: []string{"a"}, prefix: true, wantKey: "a", wantRangeEnd: "b"},
		{name: "from key", args: []string{"a"}, fromKey: true, wantKey: "a", wantRangeEnd: "\x00"},
		{name: "empty key", args: []string{""}, wantKey: "\x00"},
		{name: "empty key with prefix", args: []string{""}, prefix: true, wantKey: "\x00", wantRangeEnd: "\x00"},
		{name: "empty key with from key", args: []string{""}, fromKey: true, wantKey: "\x00", wantRangeEnd: "\x00"},
		{name: "empty key and endkey", args: []string{"", "c"}, wantKey: "\x00", wantRangeEnd: "c"},
		{name: "empty key and endkey with prefix", args: []string{"", "c"}, prefix: true, wantErr: true},
		{name: "empty key and endkey with from key", args: []string{"", "c"}, fromKey: true, wantErr: true},
		{name: "empty key with prefix and from key", args: []string{""}, prefix: true, fromKey: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rolePermPrefix, rolePermFromKey = tt.prefix, tt.fromKey
			defer func() { rolePermPrefix, rolePermFromKey = false, false }()

			key, rangeEnd, err := permRange(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got key %q range end %q", key, rangeEnd)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tt.wantKey || rangeEnd != tt.wantRangeEnd {
				t.Errorf("got key %q range end %q, want key %q range end %q", key, rangeEnd, tt.wantKey, tt.wantRangeEnd)
			}
		})
	}
}
