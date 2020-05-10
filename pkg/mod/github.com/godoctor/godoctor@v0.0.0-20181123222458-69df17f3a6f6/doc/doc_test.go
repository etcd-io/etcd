// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doc_test

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/godoctor/godoctor/doc"
)

func TestDoc(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.PanicOnError)
	var b bytes.Buffer

	doc.PrintManPage("about", fs, &b)
	if !strings.Contains(b.String(), ".TH") {
		t.Fatal("PrintManPage output does not contain .TH")
	}

	doc.PrintVimdoc("about", fs, &b)
	if !strings.Contains(b.String(), ":GoRefactor") {
		t.Fatal("PrintVimdoc output does not contain :GoRefactor")
	}

	doc.PrintUserGuide("about", fs, &b)
	if !strings.Contains(b.String(), "<html") {
		t.Fatal("PrintUserGuide output does not contain <html")
	}
}
