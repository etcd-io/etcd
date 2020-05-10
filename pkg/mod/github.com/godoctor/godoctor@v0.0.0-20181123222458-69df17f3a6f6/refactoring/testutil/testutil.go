// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file provides tests for all refactorings.  The testdata directory is
// structured as such:
//
//     testdata/
//         refactoring-name/
//             001-test-name/
//             002-test-name/
//
// To filter which refactorings are tested, run
//     go test -filter=something
// Then, only tests in directories containing "something" are run.  E.g.::
//     go test -filter=rename              # Only run rename tests
//     go test -filter=shortassign/003     # Only run shortassign test #3
//
// Refactorings are run on the files in each test directory; special comments
// in the .go files indicate what refactoring(s) to run and whether the
// refactorings are expected to succeed or fail.  Specifically, test files are
// annotated with markers of the form
//     //<<<<<name,startline,startcol,endline,endcol,arg1,arg2,...,argn,pass
// The name indicates the refactoring to run.  The next four fields specify a
// text selection on which to invoke the refactoring.  The arguments
// arg1,arg2,...,argn are passed as arguments to the refactoring (see
// Config.Args).  The last field is either "pass" or "fail", indicating whether
// the refactoring is expected to complete successfully or raise an error.  If
// the refactoring is expected to succeed, the resulting file is compared
// against a .golden file with the same name in the same directory.
//
// Each test directory (001-test-name, 002-test-name, etc.) is treated as the
// root of a Go workspace when its tests are run; i.e., the GOPATH is set to
// the test directory.  This allows it to define its own packages.  In such
// cases, the test directory is usually structured as follows:
//
//     testdata/
//         refactoring-name/
//             001-test-name/
//                 src/
//                     main.go
//                     main.golden
//                     package-name/
//                         package-file.go
//                         package-file.golden
//
// If filename.go.fixWhitespace exists, all leading and trailing whitespace will
// be removed from the .golden file and the actual output, and \n\n\n will be
// replaced by \n\n.  This is used by the formatter tests, since go/printer's
// behavior changed between versions.
//
// To test the Debug refactoring, include a file named filename.go.debugOutput
// containing the output that is expected to be written to Result.DebugOutput.
// The actual debug output usually includes absolute paths; to be testable,
// all occurrences of the current working directory are replaced with "." when
// comparing against this file.

package testutil

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/filesystem"
	"github.com/godoctor/godoctor/refactoring"
	"github.com/godoctor/godoctor/text"
)

const MARKER = "<<<<<"
const PASS = "pass"
const FAIL = "fail"

const MAIN_DOT_GO = "main.go"

var filterFlag = flag.String("filter", "",
	"Only tests from directories containing this substring will be run")

func TestRefactorings(directory string, t *testing.T) {
	testDirs, err := ioutil.ReadDir(directory)
	failIfError(err, t)
	for _, testDirInfo := range testDirs {
		if testDirInfo.IsDir() {
			runAllTestsInSubdirectories(directory, testDirInfo, t)
		}
	}
}

func runAllTestsInSubdirectories(directory string, testDirInfo os.FileInfo, t *testing.T) {
	testDirPath := filepath.Join(directory, testDirInfo.Name())
	subDirs, err := ioutil.ReadDir(testDirPath)
	failIfError(err, t)
	for _, subDirInfo := range subDirs {
		if subDirInfo.IsDir() {
			subDirPath := filepath.Join(testDirPath, subDirInfo.Name())
			if strings.Contains(subDirPath, *filterFlag) {
				runAllTestsInDirectory(subDirPath, t)
			}
		}
	}
}

// RunAllTests is a utility method that runs a set of refactoring tests
// based on markers in all of the files in subdirectories of a given directory
func runAllTestsInDirectory(directory string, t *testing.T) {
	files, err := recursiveReadDir(directory)
	failIfError(err, t)

	runTestsInFiles(directory, files, t)
}

// Assumes no duplication or circularity due to symbolic links
func recursiveReadDir(path string) ([]string, error) {
	result := []string{}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		return []string{}, err
	}

	for _, fi := range fileInfos {
		if fi.IsDir() {
			current := result
			rest, err := recursiveReadDir(filepath.Join(path, fi.Name()))
			if err != nil {
				return []string{}, err
			}

			newLen := len(current) + len(rest)
			result = make([]string, newLen, newLen)
			copy(result, current)
			copy(result[len(current):], rest)
		} else {
			result = append(result, filepath.Join(path, fi.Name()))
		}
	}
	return result, err
}

func failIfError(err error, t *testing.T) {
	if err != nil {
		t.Fatal(err)
	}
}

func runTestsInFiles(directory string, files []string, t *testing.T) {
	markers := make(map[string][]string)
	for _, path := range files {
		if strings.HasSuffix(path, ".go") {
			markers[path] = extractMarkers(path, t)
		}
	}

	if len(markers) == 0 {
		pwd, _ := os.Getwd()
		pwd = filepath.Base(pwd)
		t.Fatalf("No <<<<< markers found in any files in %s", pwd)
	}

	for path, markersInFile := range markers {
		for _, marker := range markersInFile {
			runRefactoring(directory, path, marker, t)
		}
	}
}

func runRefactoring(directory string, filename string, marker string, t *testing.T) {
	refac, selection, remainder, passFail := splitMarker(filename, marker, t)

	r := engine.GetRefactoring(refac)
	if r == nil {
		t.Fatalf("There is no refactoring named %s (from marker %s)", refac, marker)
	}

	shouldPass := (passFail == PASS)
	name := r.Description().Name

	cwd, _ := os.Getwd()
	absPath, _ := filepath.Abs(filename)
	relativePath, _ := filepath.Rel(cwd, absPath)
	fmt.Println(name, relativePath)

	mainFile := filepath.Join(directory, MAIN_DOT_GO)
	if !exists(mainFile, t) {
		mainFile = filepath.Join(filepath.Join(directory, "src"), MAIN_DOT_GO)
		if !exists(mainFile, t) {
			mainFile = filename
		}
	}

	mainFile, err := filepath.Abs(mainFile)
	if err != nil {
		t.Fatal(err)
	}

	args := refactoring.InterpretArgs(remainder, r)

	gopath, _ := filepath.Abs(directory)

	fileSystem := &filesystem.LocalFileSystem{}
	config := &refactoring.Config{
		FileSystem: fileSystem,
		Scope:      []string{mainFile},
		Selection:  selection,
		Args:       args,
		GoPath:     gopath,
	}
	result := r.Run(config)
	if shouldPass && result.Log.ContainsErrors() {
		t.Log(result.Log)
		t.Fatalf("Refactoring produced unexpected errors")
	} else if !shouldPass && !result.Log.ContainsErrors() {
		t.Fatalf("Refactoring should have produced errors but didn't")
	}

	debugOutput := result.DebugOutput.String()
	if len(debugOutput) > 0 {
		debugOutputFilename := filename + ".debugOutput"
		bytes, err := ioutil.ReadFile(debugOutputFilename)
		if err != nil {
			t.Fatal(err)
		}
		expectedOutput := sanitize(string(bytes), false)
		actualOutput := sanitize(debugOutput, true)
		if expectedOutput != actualOutput {
			fmt.Printf(">>>>> Debug output does not match contents of %s\n", debugOutputFilename)
			fmt.Printf(">>>>> NOTE: All occurrences of the working directory name are replaced by \".\"\n")
			showExpectedAndActual(expectedOutput, actualOutput)
			t.Fatalf("Refactoring test failed - %s", filename)
		}
	}

	err = filepath.Walk(directory,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".go") {
				path, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				edits, ok := result.Edits[path]
				if !ok {
					edits = text.NewEditSet()
				}
				output, err := filesystem.ApplyEdits(edits, fileSystem, path)
				if err != nil {
					t.Fatal(err)
				}
				if shouldPass {
					checkResult(path, string(output), t)
				}
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

func exists(filename string, t *testing.T) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	} else {
		if os.IsNotExist(err) {
			return false
		} else {
			t.Fatal(err)
			return false
		}
	}
}

func sanitize(debugOutput string, replaceCwd bool) string {
	cwd, _ := os.Getwd()
	debugOutput = strings.Replace(debugOutput, "\r\n", "\n", -1)
	if replaceCwd {
		debugOutput = strings.Replace(debugOutput, cwd, ".", -1)
	}
	return debugOutput
}

func checkResult(filename string, actualOutput string, t *testing.T) {
	bytes, err := ioutil.ReadFile(filename + "lden")
	if err != nil {
		t.Fatal(err)
	}
	expectedOutput := strings.Replace(string(bytes), "\r\n", "\n", -1)
	actualOutput = strings.Replace(actualOutput, "\r\n", "\n", -1)
	if exists(filename+".fixWhitespace", t) {
		expectedOutput = strings.TrimSpace(expectedOutput)
		actualOutput = strings.TrimSpace(actualOutput)
		expectedOutput = strings.Replace(expectedOutput, "\n\n\n", "\n\n", -1)
		actualOutput = strings.Replace(actualOutput, "\n\n\n", "\n\n", -1)
	}
	if expectedOutput != actualOutput {
		fmt.Printf(">>>>> Output does not match %slden\n", filename)
		showExpectedAndActual(expectedOutput, actualOutput)
		t.Fatalf("Refactoring test failed - %s", filename)
	}
}

func showExpectedAndActual(expectedOutput, actualOutput string) {
	fmt.Println("EXPECTED OUTPUT")
	fmt.Println("vvvvvvvvvvvvvvv")
	fmt.Print(expectedOutput)
	fmt.Println("^^^^^^^^^^^^^^^")
	fmt.Println("ACTUAL OUTPUT")
	fmt.Println("vvvvvvvvvvvvv")
	fmt.Print(actualOutput)
	fmt.Println("^^^^^^^^^^^^^")
	lenExpected, lenActual := len(expectedOutput), len(actualOutput)
	if lenExpected != lenActual {
		fmt.Printf("Length of expected output is %d; length of actual output is %d\n",
			lenExpected, lenActual)
		minLen := lenExpected
		if lenActual < minLen {
			minLen = lenActual
		}
		for i := 0; i < minLen; i++ {
			if expectedOutput[i] != actualOutput[i] {
				fmt.Printf("Strings differ at index %d\n", i)
				fmt.Printf("Substrings starting at that index are:\n")
				fmt.Printf("Expected: [%s]\n", describe(expectedOutput[i:]))
				fmt.Printf("Actual: [%s]\n", describe(actualOutput[i:]))
				break
			}
		}
	}
}

//TODO Define after getting the value of Gopath
/*func checkRenamedDir(result.RenameDir []string,filename string) {

 if result.RenameDir != nil {

    bytes, err := ioutil.ReadFile(filename)
       if err != nil {
		t.Fatal(err)
	}
   expectedoutput :=  string(bytes)

}

}
*/
func describe(s string) string {
	// FIXME: Jeff: Handle other non-printing characters
	if len(s) > 10 {
		s = s[:10]
	}
	s = strings.Replace(s, "\n", "\\n", -1)
	s = strings.Replace(s, "\r", "\\r", -1)
	s = strings.Replace(s, "\t", "\\t", -1)
	return s
}

func splitMarker(filename string, marker string, t *testing.T) (refac string, selection text.Selection, remainder []string, result string) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(marker, ",")
	if len(fields) < 6 {
		t.Fatalf("Marker is invalid (must contain >= 5 fields): %s", marker)
	}
	refac = fields[0]
	startLine := parseInt(fields[1], t)
	startCol := parseInt(fields[2], t)
	endLine := parseInt(fields[3], t)
	endCol := parseInt(fields[4], t)
	selection = &text.LineColSelection{filename,
		startLine, startCol, endLine, endCol}
	remainder = fields[5 : len(fields)-1]
	result = fields[len(fields)-1]
	if result != PASS && result != FAIL {
		t.Fatalf("Marker is invalid: last field must be %s or %s",
			PASS, FAIL)
	}
	return
}

func parseInt(s string, t *testing.T) int {
	result, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		t.Fatalf("Marker is invalid: expecting integer, found %s", s)
	}
	return int(result)
}

// extractMarkers extracts comments of the form //<<<<<a,b,c,d,e,f,g removing
// the leading <<<<< and trimming any spaces from the left and right ends
func extractMarkers(filename string, t *testing.T) []string {
	result := []string{}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Cannot extract markers from %s -- unable to parse",
			filename)
		wd, _ := os.Getwd()
		t.Logf("Working directory is %s", wd)
		t.Fatal(err)
	}
	for _, commentGroup := range f.Comments {
		for _, comment := range commentGroup.List {
			txt := comment.Text
			if strings.Contains(txt, MARKER) {
				idx := strings.Index(txt, MARKER) + len(MARKER)
				txt = strings.TrimSpace(txt[idx:])
				result = append(result, txt)
			}
		}
	}
	return result
}
