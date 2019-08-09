// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package datadriven

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var (
	rewriteTestFiles = flag.Bool(
		"rewrite", false,
		"ignore the expected results and rewrite the test files with the actual results from this "+
			"run. Used to update tests when a change affects many cases; please verify the testfile "+
			"diffs carefully!",
	)
)

// RunTest invokes a data-driven test. The test cases are contained in a
// separate test file and are dynamically loaded, parsed, and executed by this
// testing framework. By convention, test files are typically located in a
// sub-directory called "testdata". Each test file has the following format:
//
//   <command>[,<command>...] [arg | arg=val | arg=(val1, val2, ...)]...
//   <input to the command>
//   ----
//   <expected results>
//
// The command input can contain blank lines. However, by default, the expected
// results cannot contain blank lines. This alternate syntax allows the use of
// blank lines:
//
//   <command>[,<command>...] [arg | arg=val | arg=(val1, val2, ...)]...
//   <input to the command>
//   ----
//   ----
//   <expected results>
//
//   <more expected results>
//   ----
//   ----
//
// To execute data-driven tests, pass the path of the test file as well as a
// function which can interpret and execute whatever commands are present in
// the test file. The framework invokes the function, passing it information
// about the test case in a TestData struct. The function then returns the
// actual results of the case, which this function compares with the expected
// results, and either succeeds or fails the test.
func RunTest(t *testing.T, path string, f func(d *TestData) string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR, 0644 /* irrelevant */)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = file.Close()
	}()

	runTestInternal(t, path, file, f, *rewriteTestFiles)
}

// RunTestFromString is a version of RunTest which takes the contents of a test
// directly.
func RunTestFromString(t *testing.T, input string, f func(d *TestData) string) {
	t.Helper()
	runTestInternal(t, "<string>" /* optionalPath */, strings.NewReader(input), f, *rewriteTestFiles)
}

func runTestInternal(
	t *testing.T, sourceName string, reader io.Reader, f func(d *TestData) string, rewrite bool,
) {
	t.Helper()

	r := newTestDataReader(t, sourceName, reader, rewrite)
	for r.Next(t) {
		t.Run("", func(t *testing.T) {
			d := &r.data
			actual := func() string {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("\npanic during %s:\n%s\n", d.Pos, d.Input)
						panic(r)
					}
				}()
				actual := f(d)
				if !strings.HasSuffix(actual, "\n") {
					actual += "\n"
				}
				return actual
			}()

			if r.rewrite != nil {
				r.emit("----")
				if hasBlankLine(actual) {
					r.emit("----")
					r.rewrite.WriteString(actual)
					r.emit("----")
					r.emit("----")
				} else {
					r.emit(actual)
				}
			} else if d.Expected != actual {
				t.Fatalf("\n%s: %s\nexpected:\n%s\nfound:\n%s", d.Pos, d.Input, d.Expected, actual)
			} else if testing.Verbose() {
				input := d.Input
				if input == "" {
					input = "<no input to command>"
				}
				// TODO(tbg): it's awkward to reproduce the args, but it would be helpful.
				fmt.Printf("\n%s:\n%s [%d args]\n%s\n----\n%s", d.Pos, d.Cmd, len(d.CmdArgs), input, actual)
			}
		})
		if t.Failed() {
			t.FailNow()
		}
	}

	if r.rewrite != nil {
		data := r.rewrite.Bytes()
		if l := len(data); l > 2 && data[l-1] == '\n' && data[l-2] == '\n' {
			data = data[:l-1]
		}
		if dest, ok := reader.(*os.File); ok {
			if _, err := dest.WriteAt(data, 0); err != nil {
				t.Fatal(err)
			}
			if err := dest.Truncate(int64(len(data))); err != nil {
				t.Fatal(err)
			}
			if err := dest.Sync(); err != nil {
				t.Fatal(err)
			}
		} else {
			t.Logf("input is not a file; rewritten output is:\n%s", data)
		}
	}
}

// Walk goes through all the files in a subdirectory, creating subtests to match
// the file hierarchy; for each "leaf" file, the given function is called.
//
// This can be used in conjunction with RunTest. For example:
//
//    datadriven.Walk(t, path, func (t *testing.T, path string) {
//      // initialize per-test state
//      datadriven.RunTest(t, path, func (d *datadriven.TestData) {
//       // ...
//      }
//    }
//
//   Files:
//     testdata/typing
//     testdata/logprops/scan
//     testdata/logprops/select
//
//   If path is "testdata/typing", the function is called once and no subtests
//   care created.
//
//   If path is "testdata/logprops", the function is called two times, in
//   separate subtests /scan, /select.
//
//   If path is "testdata", the function is called three times, in subtest
//   hierarchy /typing, /logprops/scan, /logprops/select.
//
func Walk(t *testing.T, path string, f func(t *testing.T, path string)) {
	finfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !finfo.IsDir() {
		f(t, path)
		return
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			Walk(t, filepath.Join(path, file.Name()), f)
		})
	}
}

// TestData contains information about one data-driven test case that was
// parsed from the test file.
type TestData struct {
	Pos string // reader and line number

	// Cmd is the first string on the directive line (up to the first whitespace).
	Cmd string

	CmdArgs []CmdArg

	Input    string
	Expected string
}

// ScanArgs looks up the first CmdArg matching the given key and scans it into
// the given destinations in order. If the arg does not exist, the number of
// destinations does not match that of the arguments, or a destination can not
// be populated from its matching value, a fatal error results.
//
// For example, for a TestData originating from
//
// cmd arg1=50 arg2=yoruba arg3=(50, 50, 50)
//
// the following would be valid:
//
// var i1, i2, i3, i4 int
// var s string
// td.ScanArgs(t, "arg1", &i1)
// td.ScanArgs(t, "arg2", &s)
// td.ScanArgs(t, "arg3", &i2, &i3, &i4)
func (td *TestData) ScanArgs(t *testing.T, key string, dests ...interface{}) {
	t.Helper()
	var arg CmdArg
	for i := range td.CmdArgs {
		if td.CmdArgs[i].Key == key {
			arg = td.CmdArgs[i]
			break
		}
	}
	if arg.Key == "" {
		t.Fatalf("missing argument: %s", key)
	}
	if len(dests) != len(arg.Vals) {
		t.Fatalf("%s: got %d destinations, but %d values", arg.Key, len(dests), len(arg.Vals))
	}

	for i := range dests {
		arg.Scan(t, i, dests[i])

	}
}

// CmdArg contains information about an argument on the directive line. An
// argument is specified in one of the following forms:
//  - argument
//  - argument=value
//  - argument=(values, ...)
type CmdArg struct {
	Key  string
	Vals []string
}

func (arg CmdArg) String() string {
	switch len(arg.Vals) {
	case 0:
		return arg.Key

	case 1:
		return fmt.Sprintf("%s=%s", arg.Key, arg.Vals[0])

	default:
		return fmt.Sprintf("%s=(%s)", arg.Key, strings.Join(arg.Vals, ", "))
	}
}

// Scan attempts to parse the value at index i into the dest.
func (arg CmdArg) Scan(t *testing.T, i int, dest interface{}) {
	if i < 0 || i >= len(arg.Vals) {
		t.Fatalf("cannot scan index %d of key %s", i, arg.Key)
	}
	val := arg.Vals[i]
	switch dest := dest.(type) {
	case *string:
		*dest = val
	case *int:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		*dest = int(n) // assume 64bit ints
	case *uint64:
		n, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		*dest = n
	case *bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			t.Fatal(err)
		}
		*dest = b
	default:
		t.Fatalf("unsupported type %T for destination #%d (might be easy to add it)", dest, i+1)
	}
}

// Fatalf wraps a fatal testing error with test file position information, so
// that it's easy to locate the source of the error.
func (td TestData) Fatalf(tb testing.TB, format string, args ...interface{}) {
	tb.Helper()
	tb.Fatalf("%s: %s", td.Pos, fmt.Sprintf(format, args...))
}

func hasBlankLine(s string) bool {
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			return true
		}
	}
	return false
}
