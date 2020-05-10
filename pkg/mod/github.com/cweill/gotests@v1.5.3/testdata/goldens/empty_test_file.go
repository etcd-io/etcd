package blanktest

import (
	"os"
	"testing"
)

func TestNot(t *testing.T) {
	type args struct {
		this *os.File
	}
	tests := []struct {
		name string
		args args
		want string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := Not(tt.args.this); got != tt.want {
			t.Errorf("%q. Not() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
