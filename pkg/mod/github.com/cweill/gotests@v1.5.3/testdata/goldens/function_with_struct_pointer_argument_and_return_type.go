package testdata

import (
	"reflect"
	"testing"
)

func TestFoo8(t *testing.T) {
	type args struct {
		b *Bar
	}
	tests := []struct {
		name    string
		args    args
		want    *Bar
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		got, err := Foo8(tt.args.b)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. Foo8() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Foo8() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
