package testdata

import "testing"

func TestBar_BarFilter(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name    string
		b       *Bar
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		b := &Bar{}
		if err := b.BarFilter(tt.args.i); (err != nil) != tt.wantErr {
			t.Errorf("%q. Bar.BarFilter() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
