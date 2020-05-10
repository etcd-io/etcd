package testdata

import "testing"

func TestReserved_DontFail(t *testing.T) {
	type fields struct {
		Name        string
		Break       string
		Default     string
		Func        string
		Interface   string
		Select      string
		Case        string
		Defer       string
		Go          string
		Map         string
		Struct      string
		Chan        string
		Else        string
		Goto        string
		Package     string
		Switch      string
		Const       string
		Fallthrough string
		If          string
		Range       string
		Type        string
		Continue    string
		For         string
		Import      string
		Return      string
		Var         string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		r := &Reserved{
			Name:        tt.fields.Name,
			Break:       tt.fields.Break,
			Default:     tt.fields.Default,
			Func:        tt.fields.Func,
			Interface:   tt.fields.Interface,
			Select:      tt.fields.Select,
			Case:        tt.fields.Case,
			Defer:       tt.fields.Defer,
			Go:          tt.fields.Go,
			Map:         tt.fields.Map,
			Struct:      tt.fields.Struct,
			Chan:        tt.fields.Chan,
			Else:        tt.fields.Else,
			Goto:        tt.fields.Goto,
			Package:     tt.fields.Package,
			Switch:      tt.fields.Switch,
			Const:       tt.fields.Const,
			Fallthrough: tt.fields.Fallthrough,
			If:          tt.fields.If,
			Range:       tt.fields.Range,
			Type:        tt.fields.Type,
			Continue:    tt.fields.Continue,
			For:         tt.fields.For,
			Import:      tt.fields.Import,
			Return:      tt.fields.Return,
			Var:         tt.fields.Var,
		}
		if got := r.DontFail(); got != tt.want {
			t.Errorf("%q. Reserved.DontFail() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
