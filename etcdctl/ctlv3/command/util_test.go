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
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"go.etcd.io/etcd/pkg/v3/cobrautl"
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

func TestIsSerializable(t *testing.T) {
	if IsSerializable("l") {
		t.Errorf(`IsSerializable("l") = true, want false`)
	}
	if !IsSerializable("s") {
		t.Errorf(`IsSerializable("s") = false, want true`)
	}
}

// TestIsSerializableUnknownOption runs IsSerializable in a subprocess,
// because it exits the process on an unknown option.
func TestIsSerializableUnknownOption(t *testing.T) {
	if option := os.Getenv("ETCDCTL_TEST_CONSISTENCY"); option != "" {
		IsSerializable(option)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestIsSerializableUnknownOption$")
	cmd.Env = append(os.Environ(), "ETCDCTL_TEST_CONSISTENCY=x")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != cobrautl.ExitBadFeature {
		t.Fatalf("expected exit code %d, got %v (stderr: %q)", cobrautl.ExitBadFeature, err, stderr.String())
	}
	if want := `unknown consistency flag "x"`; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
	}
}
