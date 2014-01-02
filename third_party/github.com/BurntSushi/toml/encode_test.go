package toml

import (
	"bytes"
	"testing"
)

// XXX(burntsushi)
// I think these tests probably should be removed. They are good, but they
// ought to be obsolete by toml-test.
func TestEncode(t *testing.T) {
	tests := map[string]struct {
		input      interface{}
		wantOutput string
		wantError  error
	}{
		"bool field": {
			input: struct {
				BoolTrue  bool
				BoolFalse bool
			}{true, false},
			wantOutput: "BoolTrue = true\nBoolFalse = false",
		},
		"int fields": {
			input: struct {
				Int   int
				Int8  int8
				Int16 int16
				Int32 int32
				Int64 int64
			}{1, 2, 3, 4, 5},
			wantOutput: "Int = 1\nInt8 = 2\nInt16 = 3\nInt32 = 4\nInt64 = 5",
		},
		"uint fields": {
			input: struct {
				Uint   uint
				Uint8  uint8
				Uint16 uint16
				Uint32 uint32
				Uint64 uint64
			}{1, 2, 3, 4, 5},
			wantOutput: "Uint = 1\nUint8 = 2\nUint16 = 3\nUint32 = 4" +
				"\nUint64 = 5",
		},
		"float fields": {
			input: struct {
				Float32 float32
				Float64 float64
			}{1.5, 2.5},
			wantOutput: "Float32 = 1.5\nFloat64 = 2.5",
		},
		"string field": {
			input:      struct{ String string }{"foo"},
			wantOutput: `String = "foo"`,
		},
		"array fields": {
			input: struct {
				IntArray0 [0]int
				IntArray3 [3]int
			}{[0]int{}, [3]int{1, 2, 3}},
			wantOutput: "IntArray0 = []\nIntArray3 = [1, 2, 3]",
		},
		"slice fields": {
			input: struct{ IntSliceNil, IntSlice0, IntSlice3 []int }{
				nil, []int{}, []int{1, 2, 3},
			},
			wantOutput: "IntSlice0 = []\nIntSlice3 = [1, 2, 3]",
		},
		"nested arrays and slices": {
			input: struct {
				SliceOfArrays         [][2]int
				ArrayOfSlices         [2][]int
				SliceOfArraysOfSlices [][2][]int
				ArrayOfSlicesOfArrays [2][][2]int
				SliceOfMixedArrays    [][2]interface{}
				ArrayOfMixedSlices    [2][]interface{}
			}{
				[][2]int{[2]int{1, 2}, [2]int{3, 4}},
				[2][]int{[]int{1, 2}, []int{3, 4}},
				[][2][]int{
					[2][]int{
						[]int{1, 2}, []int{3, 4},
					},
					[2][]int{
						[]int{5, 6}, []int{7, 8},
					},
				},
				[2][][2]int{
					[][2]int{
						[2]int{1, 2}, [2]int{3, 4},
					},
					[][2]int{
						[2]int{5, 6}, [2]int{7, 8},
					},
				},
				[][2]interface{}{
					[2]interface{}{1, 2}, [2]interface{}{"a", "b"},
				},
				[2][]interface{}{
					[]interface{}{1, 2}, []interface{}{"a", "b"},
				},
			},
			wantOutput: `SliceOfArrays = [[1, 2], [3, 4]]
ArrayOfSlices = [[1, 2], [3, 4]]
SliceOfArraysOfSlices = [[[1, 2], [3, 4]], [[5, 6], [7, 8]]]
ArrayOfSlicesOfArrays = [[[1, 2], [3, 4]], [[5, 6], [7, 8]]]
SliceOfMixedArrays = [[1, 2], ["a", "b"]]
ArrayOfMixedSlices = [[1, 2], ["a", "b"]]`,
		},
		"(error) slice with element type mismatch (string and integer)": {
			input:     struct{ Mixed []interface{} }{[]interface{}{1, "a"}},
			wantError: ErrArrayMixedElementTypes,
		},
		"(error) slice with element type mismatch (integer and float)": {
			input:     struct{ Mixed []interface{} }{[]interface{}{1, 2.5}},
			wantError: ErrArrayMixedElementTypes,
		},
		"slice with elems of differing Go types, same TOML types": {
			input: struct {
				MixedInts   []interface{}
				MixedFloats []interface{}
			}{
				[]interface{}{
					int(1), int8(2), int16(3), int32(4), int64(5),
					uint(1), uint8(2), uint16(3), uint32(4), uint64(5),
				},
				[]interface{}{float32(1.5), float64(2.5)},
			},
			wantOutput: "MixedInts = [1, 2, 3, 4, 5, 1, 2, 3, 4, 5]\n" +
				"MixedFloats = [1.5, 2.5]",
		},
		"(error) slice w/ element type mismatch (one is nested array)": {
			input: struct{ Mixed []interface{} }{
				[]interface{}{1, []interface{}{2}},
			},
			wantError: ErrArrayMixedElementTypes,
		},
		"(error) slice with 1 nil element": {
			input:     struct{ NilElement1 []interface{} }{[]interface{}{nil}},
			wantError: ErrArrayNilElement,
		},
		"(error) slice with 1 nil element (and other non-nil elements)": {
			input: struct{ NilElement []interface{} }{
				[]interface{}{1, nil},
			},
			wantError: ErrArrayNilElement,
		},
		"simple map": {
			input:      map[string]int{"a": 1, "b": 2},
			wantOutput: "a = 1\nb = 2",
		},
		"map with interface{} value type": {
			input:      map[string]interface{}{"a": 1, "b": "c"},
			wantOutput: "a = 1\nb = \"c\"",
		},
		"map with interface{} value type, some of which are structs": {
			input: map[string]interface{}{
				"a": struct{ Int int }{2},
				"b": 1,
			},
			wantOutput: "b = 1\n[a]\n  Int = 2",
		},
		"nested map": {
			input: map[string]map[string]int{
				"a": map[string]int{"b": 1},
				"c": map[string]int{"d": 2},
			},
			wantOutput: "[a]\n  b = 1\n\n[c]\n  d = 2",
		},
		"nested struct": {
			input: struct{ Struct struct{ Int int } }{
				struct{ Int int }{1},
			},
			wantOutput: "[Struct]\n  Int = 1",
		},
		"nested struct and non-struct field": {
			input: struct {
				Struct struct{ Int int }
				Bool   bool
			}{struct{ Int int }{1}, true},
			wantOutput: "Bool = true\n\n[Struct]\n  Int = 1",
		},
		"2 nested structs": {
			input: struct{ Struct1, Struct2 struct{ Int int } }{
				struct{ Int int }{1}, struct{ Int int }{2},
			},
			wantOutput: "[Struct1]\n  Int = 1\n\n[Struct2]\n  Int = 2",
		},
		"deeply nested structs": {
			input: struct {
				Struct1, Struct2 struct{ Struct3 *struct{ Int int } }
			}{
				struct{ Struct3 *struct{ Int int } }{&struct{ Int int }{1}},
				struct{ Struct3 *struct{ Int int } }{nil},
			},
			wantOutput: "[Struct1]\n  [Struct1.Struct3]\n    Int = 1" +
				"\n\n[Struct2]\n",
		},
		"nested struct with nil struct elem": {
			input: struct {
				Struct struct{ Inner *struct{ Int int } }
			}{
				struct{ Inner *struct{ Int int } }{nil},
			},
			wantOutput: "[Struct]\n",
		},
		"nested struct with no fields": {
			input: struct {
				Struct struct{ Inner struct{} }
			}{
				struct{ Inner struct{} }{struct{}{}},
			},
			wantOutput: "[Struct]\n  [Struct.Inner]\n",
		},
		"struct with tags": {
			input: struct {
				Struct struct {
					Int int `toml:"_int"`
				} `toml:"_struct"`
				Bool bool `toml:"_bool"`
			}{
				struct {
					Int int `toml:"_int"`
				}{1}, true,
			},
			wantOutput: "_bool = true\n\n[_struct]\n  _int = 1",
		},
		"embedded struct": {
			input:      struct{ Embedded }{Embedded{1}},
			wantOutput: "_int = 1",
		},
		"embedded *struct": {
			input:      struct{ *Embedded }{&Embedded{1}},
			wantOutput: "_int = 1",
		},
		"nested embedded struct": {
			input: struct {
				Struct struct{ Embedded } `toml:"_struct"`
			}{struct{ Embedded }{Embedded{1}}},
			wantOutput: "[_struct]\n  _int = 1",
		},
		"nested embedded *struct": {
			input: struct {
				Struct struct{ *Embedded } `toml:"_struct"`
			}{struct{ *Embedded }{&Embedded{1}}},
			wantOutput: "[_struct]\n  _int = 1",
		},
		"array of tables": {
			input: struct {
				Structs []*struct{ Int int } `toml:"struct"`
			}{
				[]*struct{ Int int }{
					{1}, nil, {3},
				},
			},
			wantOutput: "[[struct]]\n  Int = 1\n\n[[struct]]\n  Int = 3",
		},
	}
	for label, test := range tests {
		var buf bytes.Buffer
		e := NewEncoder(&buf)
		err := e.Encode(test.input)
		if err != test.wantError {
			if test.wantError != nil {
				t.Errorf("%s: want Encode error %v, got %v",
					label, test.wantError, err)
			} else {
				t.Errorf("%s: Encode failed: %s", label, err)
			}
		}
		if err != nil {
			continue
		}
		if got := buf.String(); test.wantOutput != got {
			t.Errorf("%s: want %q, got %q", label, test.wantOutput, got)
		}
	}
}

type Embedded struct {
	Int int `toml:"_int"`
}
