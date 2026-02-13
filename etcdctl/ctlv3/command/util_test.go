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
