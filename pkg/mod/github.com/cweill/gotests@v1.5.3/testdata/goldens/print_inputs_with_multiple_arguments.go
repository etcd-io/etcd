package testdata

import "testing"

func TestFoo6(t *testing.T) {
	type args struct {
		i int
		b bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		got, err := Foo6(tt.args.i, tt.args.b)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. Foo6(%v, %v) error = %v, wantErr %v", tt.name, tt.args.i, tt.args.b, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("%q. Foo6(%v, %v) = %v, want %v", tt.name, tt.args.i, tt.args.b, got, tt.want)
		}
	}
}
