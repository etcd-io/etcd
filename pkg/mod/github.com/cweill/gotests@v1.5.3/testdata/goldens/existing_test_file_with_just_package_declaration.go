package testdata

import "testing"

func TestFoo101(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := Foo101(tt.args.s); got != tt.want {
			t.Errorf("%q. Foo101() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
