package testdata

import "testing"

func TestFoo12(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if err := Foo12(tt.args.str); (err != nil) != tt.wantErr {
			t.Errorf("%q. Foo12() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
