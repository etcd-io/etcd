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
	"reflect"
	"testing"
)

func TestArgify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "simple args",
			input:    "foo bar baz",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "single-quoted string",
			input:    "'hello world'",
			expected: []string{"hello world"},
		},
		{
			name:     "double-quoted string",
			input:    `"hello world"`,
			expected: []string{"hello world"},
		},
		{
			name:     "mixed args",
			input:    "put 'my key' 'my value'",
			expected: []string{"put", "my key", "my value"},
		},
		{
			name:     "empty single-quoted string",
			input:    "''",
			expected: []string{""},
		},
		{
			name:     "empty double-quoted string",
			input:    `""`,
			expected: []string{""},
		},
		{
			name:     "double-quoted with escape",
			input:    `"hello\"world"`,
			expected: []string{`hello"world`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Argify(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Argify(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
