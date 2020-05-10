package bar

import "testing"

func TestBar_Bar(t *testing.T) {
	type fields struct {
		Foo string
	}
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		b := &Bar{
			Foo: tt.fields.Foo,
		}
		if err := b.Bar(tt.args.s); (err != nil) != tt.wantErr {
			t.Errorf("%q. Bar.Bar() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
