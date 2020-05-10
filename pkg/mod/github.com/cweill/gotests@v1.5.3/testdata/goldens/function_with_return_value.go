package testdata

import "testing"

func TestFoo4(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := Foo4(); got != tt.want {
			t.Errorf("%q. Foo4() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
