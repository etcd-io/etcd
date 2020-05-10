package testdata

import (
	"reflect"
	"testing"
)

func TestFooFilter(t *testing.T) {
	type args struct {
		strs []string
	}
	tests := []struct {
		name    string
		args    args
		want    []*Bar
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		got, err := FooFilter(tt.args.strs)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. FooFilter() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. FooFilter() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func Test_bazFilter(t *testing.T) {
	type args struct {
		f *float64
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if got := bazFilter(tt.args.f); got != tt.want {
			t.Errorf("%q. bazFilter() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
