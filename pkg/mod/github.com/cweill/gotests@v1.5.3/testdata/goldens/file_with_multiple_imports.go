package testdata

import (
	"go/ast"
	"go/types"
	"io"
	"testing"
)

func TestFoo24(t *testing.T) {
	type args struct {
		r io.Reader
		x ast.Expr
		t types.Type
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if err := Foo24(tt.args.r, tt.args.x, tt.args.t); (err != nil) != tt.wantErr {
			t.Errorf("%q. Foo24() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
