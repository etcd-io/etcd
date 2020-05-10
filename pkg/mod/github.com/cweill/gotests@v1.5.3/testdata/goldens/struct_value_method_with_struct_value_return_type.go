package testdata

import (
	"reflect"
	"testing"
)

func TestBar_Foo9(t *testing.T) {
	tests := []struct {
		name string
		b    Bar
		want Bar
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		b := Bar{}
		if got := b.Foo9(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Bar.Foo9() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
