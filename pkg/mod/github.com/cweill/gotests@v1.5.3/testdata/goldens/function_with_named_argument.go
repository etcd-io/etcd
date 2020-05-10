package testdata

import "testing"

func TestFoo3(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		Foo3(tt.args.s)
	}
}
