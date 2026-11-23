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
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSerializable(t *testing.T) {
	// IsSerializable exits the process on an unknown value, so run that case in a child process.
	if os.Getenv("ETCDCTL_TEST_IS_SERIALIZABLE") == "1" {
		getConsistency = "l" // the default of "get --consistency"
		IsSerializable("x")
		return
	}

	require.True(t, IsSerializable("s"))
	require.False(t, IsSerializable("l"))

	cmd := exec.Command(os.Args[0], "-test.run=^TestIsSerializable$")
	cmd.Env = append(os.Environ(), "ETCDCTL_TEST_IS_SERIALIZABLE=1")
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(out), `unknown consistency flag "x"`)
}

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
