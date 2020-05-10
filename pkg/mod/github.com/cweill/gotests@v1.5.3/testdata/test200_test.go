// testdata package holds example functions.
package testdata

import (
	// ast gives a representation of the Go abstract syntax tree.
	"go/ast"
	// types wraps ast and holds the Go type checker algorithm.
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
