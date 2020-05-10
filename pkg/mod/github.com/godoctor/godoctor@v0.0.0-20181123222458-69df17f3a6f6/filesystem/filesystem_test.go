// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filesystem

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/loader"

	"go/build"
	"go/parser"

	"github.com/godoctor/godoctor/text"
)

// Use zz_* for test file and directory names, since those paths are in
// .gitignore.

const testFile = "zz_test.txt"
const testFile2 = "zz_test2.txt"
const testDir = "zz_test"

func TestCreateFile1(t *testing.T) {
	contents := "This is a file\nwith two lines"
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(testFile, contents); err != nil {
		t.Fatal(err)
	}
	bytes, err := ioutil.ReadFile(testFile)
	if err != nil {
		os.Remove(testFile)
		t.Fatal(err)
	}
	if string(bytes) != contents {
		os.Remove(testFile)
		t.Fatal("Incorrect file contents:\n", string(bytes))
	}
	os.Remove(testFile)
}

func TestCreateFile2Remove(t *testing.T) {
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(testFile, ""); err != nil {
		t.Fatal(err)
	}
	if err := fs.CreateFile(testFile, "x"); err == nil {
		os.Remove(testFile)
		t.Fatal("Create over existing file should have failed")
	}
	if err := fs.Remove(testFile); err != nil {
		t.Fatal(err)
	}
	if err := fs.CreateFile(testFile, ""); err != nil {
		t.Fatal(err)
	}
	os.Remove(testFile)
}

func TestCreateFile3(t *testing.T) {
	os.RemoveAll(testDir)
	if err := os.Mkdir(testDir, os.ModeDir|0775); err != nil {
		t.Fatal(err)
	}
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(testDir, "x"); err == nil {
		os.Remove(testDir)
		t.Fatal("Create over directory should have failed")
	}
	os.Remove(testDir)
}

func TestRenameFile(t *testing.T) {
	os.RemoveAll(testDir)
	if err := os.Mkdir(testDir, os.ModeDir|0775); err != nil {
		os.RemoveAll(testDir)
		t.Fatal(err)
	}
	path := fmt.Sprintf("%s/%s", testDir, testFile)
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(path, ""); err != nil {
		t.Fatal(err)
	}
	fs.Rename(path, testFile2)
	newPath := fmt.Sprintf("%s/%s", testDir, testFile2)
	if err := fs.CreateFile(newPath, "x"); err == nil {
		os.RemoveAll(testDir)
		t.Fatal("Create over renamed file should have failed")
	}
	if err := os.RemoveAll(testDir); err != nil {
		t.Fatal(err)
	}
}

func TestEditedFileSystem(t *testing.T) {
	contents := "123456789\nABCDEFGHIJ"
	lfs := NewLocalFileSystem()
	if err := lfs.CreateFile(testFile, contents); err != nil {
		t.Fatal(err)
	}
	es := text.NewEditSet()
	es.Add(&text.Extent{3, 5}, "xyz")
	expected := "123xyz9\nABCDEFGHIJ"
	fs := NewEditedFileSystem(NewLocalFileSystem(),
		map[string]*text.EditSet{testFile: es})
	editedFile, err := fs.OpenFile(testFile)
	if err != nil {
		os.Remove(testFile)
		t.Fatal(err)
	}
	bytes, err := ioutil.ReadAll(editedFile)
	editedFile.Close()
	if err != nil {
		os.Remove(testFile)
		t.Fatal(err)
	}
	if string(bytes) != expected {
		os.Remove(testFile)
		t.Fatal("Incorrect file contents:\n", string(bytes))
	}
	if err := os.Remove(testFile); err != nil {
		t.Fatal(err)
	}
}

func TestPatchOnFile(t *testing.T) {
	// Insert "Before line 1" at the top of testdata/diff/lines.txt
	testfile := "testdata/lines.txt"

	fs := &LocalFileSystem{}
	es := text.NewEditSet()
	es.Add(&text.Extent{0, 0}, "Before line 1\n")
	patch, err := CreatePatch(es, fs, testfile)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ApplyEdits(es, fs, testfile)
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Replace(`Before line 1
Line 1
Line 2
Line 3
`, "\r\n", "\n", -1)
	actual := strings.Replace(string(result), "\r\n", "\n", -1)
	if expected != actual {
		t.Fatalf("ApplyToFile failed:\n%s", actual)
	}

	var b bytes.Buffer
	time1 := time.Date(2013, time.June, 4, 0, 0, 0, 0, time.UTC)
	time2 := time.Date(2014, time.July, 8, 13, 28, 43, 0, time.UTC)
	err = patch.Write("from", "to", time1, time2, &b)
	if err != nil {
		t.Fatal(err)
	}
	expected = strings.Replace(`--- from  2013-06-04 00:00:00 +0000
+++ to  2014-07-08 13:28:43 +0000
@@ -1,3 +1,4 @@
+Before line 1
 Line 1
 Line 2
 Line 3
`, "\r\n", "\n", -1)
	actual = strings.Replace(b.String(), "\r\n", "\n", -1)
	if expected != actual {
		t.Fatalf("patch.Write failed:\n%s", actual)
	}
}

func TestPatchOnMissingFile(t *testing.T) {
	fileDNE := "this_file_does_not_exist_ZzZzZz.txt"

	fs := &LocalFileSystem{}
	es := text.NewEditSet()
	_, err := CreatePatch(es, fs, fileDNE)
	if err == nil {
		t.Fatalf("Should have failed attempting to patch %s", fileDNE)
	}

	_, err = ApplyEdits(es, fs, fileDNE)
	if err == nil {
		t.Fatalf("Should have failed attempting to patch %s", fileDNE)
	}
}

func TestLoader(t *testing.T) {
	local := NewLocalFileSystem()
	var lconfig loader.Config
	build := build.Default
	build.GOPATH = "testdata"
	build.OpenFile = local.OpenFile
	build.ReadDir = local.ReadDir
	build.IsDir = nil     // FIXME
	build.HasSubdir = nil // FIXME
	lconfig.Build = &build
	lconfig.ParserMode = parser.ParseComments | parser.DeclarationErrors
	lconfig.AllowErrors = false
	//lconfig.SourceImports = true
	lconfig.TypeChecker.Error = func(err error) {
		t.Fatal(err)
	}
	lconfig.FromArgs([]string{"testdata/src/main.go"}, true)
	_, err := lconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
}
