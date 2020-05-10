// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text_test

import (
	"go/parser"
	"go/token"
	"reflect"

	"github.com/godoctor/godoctor/text"

	"testing"
)

const file1 = `package main`

const file2 = `package main //1
import "fmt"                //2
func main() {               //3
  fmt.Println("")           //4
}                           //5`

func setup(t *testing.T) *token.FileSet {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "main_test.go", file1, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Put the "interesting" stuff into a second file, so Pos ≠ offset
	_, err = parser.ParseFile(fset, "main.go", file2, 0)
	if err != nil {
		t.Fatal(err)
	}
	return fset
}

func TestConvert(t *testing.T) {
	fset := setup(t)
	lcsel := text.LineColSelection{
		Filename:  "main.go",
		StartLine: 3,
		StartCol:  6,
		EndLine:   4,
		EndCol:    5,
	}
	start, end, err := lcsel.Convert(fset)
	if err != nil {
		t.Fatal(err)
	}
	startOff := fset.Position(start).Offset
	endOff := fset.Position(end).Offset
	selText := file2[startOff:endOff]
	expected := "main() {               //3\n  fmt"
	if selText != expected {
		t.Fatalf("Wrong selection: %s", selText)
	}
	olsel := text.OffsetLengthSelection{
		Filename: "main.go",
		Offset:   startOff,
		Length:   len(expected),
	}
	start2, end2, err := olsel.Convert(fset)
	if err != nil {
		t.Fatal(err)
	}
	if start != start2 || end != end2 {
		t.Fatalf("Convert mismatch for Line/Col vs. Offset/Length %v %v %v %v",
			start, start2, end, end2)
	}
}

func TestConvertErrors(t *testing.T) {
	fset := setup(t)
	lcsel := text.LineColSelection{
		Filename:  "this_does_not_exist.go",
		StartLine: 1,
		StartCol:  1,
		EndLine:   1,
		EndCol:    1,
	}
	if _, _, err := lcsel.Convert(fset); err == nil {
		t.Fatal("Erroneously found file for invalid filename")
	}
	lcsel.Filename = ""
	if _, _, err := lcsel.Convert(fset); err == nil {
		t.Fatal("Erroneously found file for invalid filename")
	}
	lcsel.Filename = "main.go"
	lcsel.StartLine = -1
	lcsel.StartCol = 1
	lcsel.EndLine = 1
	lcsel.EndCol = 1
	if _, _, err := lcsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted invalid start line")
	}
	lcsel.StartLine = 2
	lcsel.StartCol = -1
	lcsel.EndLine = 2
	lcsel.EndCol = 1
	if _, _, err := lcsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted invalid start line")
	}
	lcsel.StartLine = 1
	lcsel.StartCol = 1
	lcsel.EndLine = 10
	lcsel.EndCol = 1
	if _, _, err := lcsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted invalid end line")
	}
	lcsel.StartLine = 1
	lcsel.StartCol = 1
	lcsel.EndLine = 3
	lcsel.EndCol = 80
	if _, _, err := lcsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted invalid end line")
	}
	lcsel.StartLine = 2
	lcsel.StartCol = 1
	lcsel.EndLine = 1
	lcsel.EndCol = 1
	if _, _, err := lcsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted end < start")
	}

	olsel := text.OffsetLengthSelection{
		Filename: "this_does_not_exist.go",
		Offset:   0,
		Length:   1,
	}
	if _, _, err := olsel.Convert(fset); err == nil {
		t.Fatal("Erroneously found file for invalid filename")
	}
	olsel.Filename = ""
	if _, _, err := olsel.Convert(fset); err == nil {
		t.Fatal("Erroneously found file for invalid filename")
	}
	olsel.Filename = "main.go"
	olsel.Offset = -1
	olsel.Length = 1
	if _, _, err := olsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted invalid offset")
	}
	olsel.Offset = 0
	olsel.Length = -1
	if _, _, err := olsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted invalid length")
	}
	olsel.Offset = 0
	olsel.Length = 100000
	if _, _, err := olsel.Convert(fset); err == nil {
		t.Fatal("Erroneously converted invalid length")
	}
}

func TestGetFilename(t *testing.T) {
	lcsel := text.LineColSelection{
		Filename:  "main.go",
		StartLine: 1,
		StartCol:  1,
		EndLine:   1,
		EndCol:    1,
	}
	if lcsel.GetFilename() != "main.go" {
		t.Fatal("Incorrect filename returned")
	}
	olsel := text.OffsetLengthSelection{
		Filename: "main.go",
		Offset:   0,
		Length:   0,
	}
	if olsel.GetFilename() != "main.go" {
		t.Fatal("Incorrect filename returned")
	}
}

func TestSelectionString(t *testing.T) {
	lc := text.LineColSelection{
		Filename:  "filename",
		StartLine: 1,
		StartCol:  2,
		EndLine:   3,
		EndCol:    4}
	if lc.String() != "filename: 1,2:3,4" {
		t.Fatalf("Unexpected result of String(): %s", lc.String())
	}

	ol := text.OffsetLengthSelection{
		Filename: "filename",
		Offset:   6,
		Length:   18}
	if ol.String() != "filename: 6,18" {
		t.Fatalf("Unexpected result of String(): %s", ol.String())
	}
}

func TestNewSelection(t *testing.T) {
	olsel, err := text.NewSelection("main.go", "3,16")
	if err != nil {
		t.Fatal(err)
	}
	ol, ok := olsel.(*text.OffsetLengthSelection)
	if !ok {
		t.Fatalf("Unexpected type %s", reflect.TypeOf(ol))
	}
	if ol.Offset != 3 || ol.Length != 16 {
		t.Fatalf("Wrong offset/length")
	}

	lcsel, err := text.NewSelection("main.go", "1,2:3,4")
	if err != nil {
		t.Fatal(err)
	}
	lc, ok := lcsel.(*text.LineColSelection)
	if !ok {
		t.Fatalf("Unexpected type %s", reflect.TypeOf(ol))
	}
	if lc.StartLine != 1 || lc.StartCol != 2 ||
		lc.EndLine != 3 || lc.EndCol != 4 {
		t.Fatalf("Wrong line/col position(s)")
	}
}

func TestNewSelectionErrors(t *testing.T) {
	invalid := []string{
		"-3,16",
		"3 , 16",
		"3,",
		",3",
		"1,2,3",
		"1,2:3,4:5,6",
		"1:3,4",
		"1,2:3",
		",",
		":",
		",:,",
		"0,1:2,1",
		"1,1:2,0",
		"99999999999999999999,1",
		"1,99999999999999999999",
		"1,1:99999999999999999999,1",
		"1,1:1,99999999999999999999",
	}
	for _, s := range invalid {
		if _, err := text.NewSelection("main.go", s); err == nil {
			t.Fatalf("NewSelection should have failed for %s", s)
		}
	}
}
