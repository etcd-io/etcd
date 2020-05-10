// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xerrors_test

import (
	"fmt"
	"regexp"
	"testing"

	"golang.org/x/xerrors"
)

func TestNewEqual(t *testing.T) {
	// Different allocations should not be equal.
	if xerrors.New("abc") == xerrors.New("abc") {
		t.Errorf(`New("abc") == New("abc")`)
	}
	if xerrors.New("abc") == xerrors.New("xyz") {
		t.Errorf(`New("abc") == New("xyz")`)
	}

	// Same allocation should be equal to itself (not crash).
	err := xerrors.New("jkl")
	if err != err {
		t.Errorf(`err != err`)
	}
}

func TestErrorMethod(t *testing.T) {
	err := xerrors.New("abc")
	if err.Error() != "abc" {
		t.Errorf(`New("abc").Error() = %q, want %q`, err.Error(), "abc")
	}
}

func TestNewDetail(t *testing.T) {
	got := fmt.Sprintf("%+v", xerrors.New("error"))
	want := `(?s)error:.+errors_test.go:\d+`
	ok, err := regexp.MatchString(want, got)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf(`fmt.Sprintf("%%+v", New("error")) = %q, want %q"`, got, want)
	}
}

func ExampleNew() {
	err := xerrors.New("emit macho dwarf: elf header corrupted")
	if err != nil {
		fmt.Print(err)
	}
	// Output: emit macho dwarf: elf header corrupted
}

// The fmt package's Errorf function lets us use the package's formatting
// features to create descriptive error messages.
func ExampleNew_errorf() {
	const name, id = "bimmler", 17
	err := fmt.Errorf("user %q (id %d) not found", name, id)
	if err != nil {
		fmt.Print(err)
	}
	// Output: user "bimmler" (id 17) not found
}
