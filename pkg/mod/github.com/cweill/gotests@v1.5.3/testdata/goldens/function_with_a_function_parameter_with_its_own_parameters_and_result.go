package testdata

import "testing"

func TestFoo14(t *testing.T) {
	type args struct {
		f func(string, int) string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		if err := Foo14(tt.args.f); (err != nil) != tt.wantErr {
			t.Errorf("%q. Foo14() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
