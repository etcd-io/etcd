// Copyright (c) 2017 David R. Jenni. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestFillByOffset(t *testing.T) {
	tests := [...]struct {
		folder string
		offset int
	}{
		{folder: "typeswitch_1", offset: 75},
		{folder: "typeswitch_2", offset: 59},
		{folder: "typeswitch_3", offset: 69},
		{folder: "typeswitch_4", offset: 67},
		{folder: "typeswitch_5", offset: 160},
		{folder: "broken_typeswitch", offset: 146},
		{folder: "switch_1", offset: 78},
		{folder: "empty_switch", offset: 51},
		{folder: "multipkgs", offset: 75},
	}

	for _, test := range tests {
		path, err := absPath(filepath.Join("./testdata", test.folder, "input.go"))
		if err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}
		lprog, err := load(path, false)
		if err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}

		var buf bytes.Buffer
		if err = byOffset(lprog, path, test.offset, &buf); err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}

		var outs []output
		if err = json.NewDecoder(&buf).Decode(&outs); err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}
		if len(outs) != 1 {
			t.Fatalf("%s: expected len(outs) == 1\n", test.folder)
		}
		got := []byte(outs[0].Code)

		want, err := ioutil.ReadFile(filepath.Join("./testdata", test.folder, "output.golden"))
		if err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("%s:\ngot:\n%s\n\nwant:\n%s\n\n", test.folder, got, want)
		}
	}
}

func TestFillByLine(t *testing.T) {
	tests := [...]struct {
		folder string
		line   int
	}{
		{folder: "typeswitch_1", line: 6},
		{folder: "typeswitch_2", line: 6},
		{folder: "typeswitch_3", line: 9},
		{folder: "typeswitch_4", line: 9},
		{folder: "typeswitch_5", line: 10},
		{folder: "broken_typeswitch", line: 7},
		{folder: "switch_1", line: 7},
		{folder: "empty_switch", line: 6},
	}

	for _, test := range tests {
		path, err := absPath(filepath.Join("./testdata", test.folder, "input.go"))
		if err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}
		lprog, err := load(path, false)
		if err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}

		var buf bytes.Buffer
		if err = byLine(lprog, path, test.line, &buf); err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}

		var outs []output
		if err = json.NewDecoder(&buf).Decode(&outs); err != nil {
			t.Fatal(err)
		}
		if len(outs) != 1 {
			t.Fatalf("%s: expected len(outs) == 1\n", test.folder)
		}
		got := []byte(outs[0].Code)

		want, err := ioutil.ReadFile(filepath.Join("./testdata", test.folder, "output.golden"))
		if err != nil {
			t.Fatalf("%s: %v\n", test.folder, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("%s:\ngot:\n%s\n\nwant:\n%s\n\n", test.folder, got, want)
		}
	}
}
