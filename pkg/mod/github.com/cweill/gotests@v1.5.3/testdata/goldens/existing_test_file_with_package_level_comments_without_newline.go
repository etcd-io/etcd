// Lorem ipsum dolor sit amet, consectetur adipiscing
// elit, sed do eiusmod tempor incididunt ut labore et
// dolore magna aliqua. Ut enim ad minim veniam, quis
// nostrud exercitation ullamco laboris nisi ut aliquip
// ex ea commodo consequat. Duis aute irure dolor in
// reprehenderit in voluptate velit esse cillum dolore
// eu fugiat nulla pariatur. Excepteur sint occaecat
// cupidatat non proident, sunt in culpa qui officia
// deserunt mollit anim id est laborum.

// Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
// tempor incididunt ut labore et dolore magna aliqua.
package testdata

import "testing"

func TestBeforeCommentNewLine(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := BeforeCommentNewLine(); got != tt.want {
			t.Errorf("%q. BeforeCommentNewLine() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
