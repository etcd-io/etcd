package testdata

import (
	"go/types"
	"reflect"
	"testing"
)

func TestImporter_Foo35(t *testing.T) {
	type fields struct {
		Importer types.Importer
		Field    *types.Var
	}
	type args struct {
		t types.Type
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *types.Var
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		i := &Importer{
			Importer: tt.fields.Importer,
			Field:    tt.fields.Field,
		}
		if got := i.Foo35(tt.args.t); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. Importer.Foo35() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
