// testdata package holds example functions.
package testdata

import (
	"go/ast"
	"go/types"
	"testing"
)

func TestFoo200(t *testing.T) {
	tests := []struct {
		name string
		x    ast.Expr
		t    types.Type
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := Foo200(tt.x, tt.t); got != tt.want {
			t.Errorf("%q. Foo200() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBar200(t *testing.T) {
	type args struct {
		t types.Type
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := Bar200(tt.args.t); got != tt.want {
			t.Errorf("%q. Bar200() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
