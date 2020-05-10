// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xerrors_test

import (
	"fmt"

	"golang.org/x/xerrors"
)

type MyError2 struct {
	Message string
	frame   xerrors.Frame
}

func (m *MyError2) Error() string {
	return m.Message
}

func (m *MyError2) Format(f fmt.State, c rune) { // implements fmt.Formatter
	xerrors.FormatError(m, f, c)
}

func (m *MyError2) FormatError(p xerrors.Printer) error { // implements xerrors.Formatter
	p.Print(m.Message)
	if p.Detail() {
		m.frame.Format(p)
	}
	return nil
}

func ExampleFormatError() {
	err := &MyError2{Message: "oops", frame: xerrors.Caller(1)}
	fmt.Printf("%v\n", err)
	fmt.Println()
	fmt.Printf("%+v\n", err)
}
