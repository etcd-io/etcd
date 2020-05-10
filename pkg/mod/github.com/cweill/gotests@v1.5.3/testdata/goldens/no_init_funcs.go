package testdata

import "testing"

func Test_initFuncStruct_init(t *testing.T) {
	type fields struct {
		field int
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		i := initFuncStruct{
			field: tt.fields.field,
		}
		if got := i.init(); got != tt.want {
			t.Errorf("%q. initFuncStruct.init() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func Test_initFieldStruct_getInit(t *testing.T) {
	type fields struct {
		init int
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		i := initFieldStruct{
			init: tt.fields.init,
		}
		if got := i.getInit(); got != tt.want {
			t.Errorf("%q. initFieldStruct.getInit() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
