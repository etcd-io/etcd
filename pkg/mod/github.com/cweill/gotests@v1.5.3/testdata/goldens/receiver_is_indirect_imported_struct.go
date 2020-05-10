package testdata

import "testing"

func Test_someIndirectImportedStruct_Foo037(t *testing.T) {
	tests := []struct {
		name string
		smtg *someIndirectImportedStruct
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		smtg := &someIndirectImportedStruct{}
		smtg.Foo037()
	}
}
