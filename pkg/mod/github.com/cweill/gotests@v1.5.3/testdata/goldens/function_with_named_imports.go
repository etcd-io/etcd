package testdata

import (
	ht "html/template"
	"reflect"
	"testing"
)

func TestFoo22(t *testing.T) {
	type args struct {
		t *ht.Template
	}
	tests := []struct {
		name string
		args args
		want *ht.Template
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := Foo22(tt.args.t); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Foo22() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
