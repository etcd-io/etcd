// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const diffTestDir = "testdata/diff/"

func TestDiffs(t *testing.T) {
	strings := []string{"", "ABCABBA", "CBABAC"}
	for _, a := range strings {
		for _, b := range strings {
			testDiffs(a, b, t)
		}
	}
	for _, b := range []string{"a\nbcd", "abcfg", "defg", "abcd", "ag",
		"bcd", "abd", "efg", "axy", "xcg", "xcdghy", "xabcdefgy"} {
		testDiffs("abcdefg", b, t)
	}
}

func testDiffs(a, b string, t *testing.T) {
	diff := Diff(strings.Split(a, ""), strings.Split(b, ""))
	result, err := ApplyToString(diff, a)
	if err != nil {
		t.Fatal(err)
	}
	assertEquals(b, result, t)
}

func TestLineRdr(t *testing.T) {
	// Line2 starts at offset 10, Line3 at 20, etc.
	s := "Line1....\nLine2....\nLine3....\nLine4....\nLine5"
	r := newLineRdr(strings.NewReader(s))

	r.readLine()
	assertEquals("Line1....\n", r.line, t)
	assertTrue(r.lineOffset == 0, t)
	assertTrue(r.lineNum == 1, t)
	assertTrue(len(r.leadingCtxLines) == 0, t)

	r.readLine()
	assertEquals("Line2....\n", r.line, t)
	assertTrue(r.lineOffset == 10, t)
	assertTrue(r.lineNum == 2, t)
	assertTrue(len(r.leadingCtxLines) == 1, t)

	r.readLine()
	r.readLine()
	r.readLine()
	assertEquals("Line5", r.line, t)
	assertTrue(r.lineOffset == 40, t)
	assertTrue(r.lineNum == 5, t)
	assertTrue(len(r.leadingCtxLines) == numCtxLines, t)
	assertEquals("Line2....\n", r.leadingCtxLines[0], t)
	assertEquals("Line3....\n", r.leadingCtxLines[1], t)
	assertEquals("Line4....\n", r.leadingCtxLines[2], t)
}

func TestCreatePatch(t *testing.T) {
	// Line2 starts at offset 10, Line3 at 20, etc.
	s := "Line1....\nLine2....\nLine3....\nLine4"

	es := NewEditSet()
	p, err := es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 0, t)

	es = NewEditSet()
	es.Add(&Extent{0, 0}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(&Extent{0, 2}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(&Extent{2, 5}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(&Extent{2, 15}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	// Line n starts at offset (n-1)*5
	s2 := "1...\n2...\n3...\n4...\n5...\n6...\n7...\n8...\n9...\n0...\n"
	es = NewEditSet()
	es.Add(&Extent{20, 2}, "5555\n5!")
	es.Add(&Extent{40, 0}, "CCC")
	p, err = es.CreatePatch(strings.NewReader(s2))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 5, t)
	assertTrue(p.hunks[0].startLine == 2, t)
	assertTrue(p.hunks[0].numLines == 10, t)
	assertEquals("2...\n3...\n4...\n5...\n6...\n7...\n8...\n9...\n0...\n", p.hunks[0].hunk.String(), t)
	assertTrue(len(p.hunks[0].edits) == 2, t)

	es = NewEditSet()
	es.Add(&Extent{0, 0}, "A")
	es.Add(&Extent{36, 0}, "B")
	p, err = es.CreatePatch(strings.NewReader(s2))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 2, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertTrue(p.hunks[0].numLines >= 4, t) // Actually 7
	assertTrue(len(p.hunks[0].edits) == 1, t)
	assertTrue(p.hunks[1].startOffset == 20, t)
	assertTrue(p.hunks[1].startLine == 5, t)
	assertTrue(p.hunks[1].numLines == 7, t)
	assertTrue(len(p.hunks[1].edits) == 1, t)
}

func TestUnifiedDiff(t *testing.T) {
	testDirs, err := ioutil.ReadDir(diffTestDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, testDirInfo := range testDirs {
		if testDirInfo.IsDir() {
			fmt.Printf("Diff Test %s\n", testDirInfo.Name())
			dir := filepath.Join(diffTestDir, testDirInfo.Name())
			from := readFile(filepath.Join(dir, "from.txt"), t)
			to := readFile(filepath.Join(dir, "to.txt"), t)
			diff := readFile(filepath.Join(dir, "diff.txt"), t)
			testUnifiedDiff(from, to, diff, testDirInfo.Name(), t)
		}
	}
}

func readFile(path string, t *testing.T) string {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

func testUnifiedDiff(a, b, expected, name string, t *testing.T) {
	edits := Diff(strings.SplitAfter(a, "\n"), strings.SplitAfter(b, "\n"))
	s, _ := ApplyToString(edits, a)
	assertEquals(b, s, t)

	patch, _ := edits.CreatePatch(strings.NewReader(a))
	var result bytes.Buffer
	patch.Write("filename", "filename", time.Time{}, time.Time{}, &result)
	diff := strings.Replace(result.String(), "\r\n", "\n", -1)
	expected = strings.Replace(expected, "\r\n", "\n", -1)

	if diff != expected {
		t.Fatalf("Diff test %s failed.  Expected:\n[%s]\nActual:\n[%s]\n",
			name, expected, diff)
	}
}

func TestRandomDiffs(t *testing.T) {
	seed := time.Now().Unix()
	r := rand.New(rand.NewSource(seed))
	for i := 0; i < 100; i++ {
		s1 := makeLines(100, r)
		s1s := strings.Join(s1, "")
		s2 := makeLines(100, r)
		s2s := strings.Join(s2, "")
		edits := Diff(s1, s2)
		result, err := ApplyToString(edits, s1s)
		if err != nil || result != s2s {
			t.Errorf("Random diff failed - seed %d, iteration %d",
				seed, i)
		}
	}
}

func BenchmarkDiff(b *testing.B) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	s1 := makeLines(5000, r)
	s1s := strings.Join(s1, "\n")
	s2 := makeLines(5000, r)
	for i := 0; i < b.N; i++ {
		Diff(s1, s2).CreatePatch(strings.NewReader(s1s))
	}
}

func makeLines(count int, r *rand.Rand) []string {
	possibilities := []string{
		"Lorem ipsum dolor sit amet, consectetur adipisicing elit,\n",
		"sed do eiusmod tempor incididunt ut labore et dolore magna\n",
		"aliqua. Ut enim ad minim veniam, quis nostrud exercitation\n",
		"ullamco laboris nisi ut aliquip ex ea commodo consequat.\n",
		"Duis aute irure dolor in reprehenderit in voluptate velit\n",
		"esse cillum dolore eu fugiat nulla pariatur. Excepteur sint\n",
		"occaecat cupidatat non proident, sunt in culpa qui officia\n",
		"deserunt mollit anim id est laborum.\n"}
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		line := possibilities[r.Intn(len(possibilities))]
		result = append(result, line)
	}
	return result
}

// -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// These are utility methods used by other tests as well.  They need to be in
// a file named something_test.go so that command line arguments used for
// testing do not get compiled into the main driver (TODO maybe there's another
// way around that?), and this seemed like a reasonable place for them...

func fatalf(t *testing.T, format string, args ...interface{}) {
	_, file, line, ok := runtime.Caller(2)
	if ok {
		var msg string
		if len(args) == 0 {
			msg = format
		} else {
			msg = fmt.Sprintf(format, args...)
		}
		t.Fatalf("from %s:%d: %s", filepath.Base(file), line, msg)
	}
}

// assertEquals is a utility method for unit tests that marks a function as
// having failed if expected != actual
func assertEquals(expected string, actual string, t *testing.T) {
	if expected != actual {
		fatalf(t, "Expected: %s Actual: %s", expected, actual)
	}
}

// assertError is a utility method for unit tests that marks a function as
// having failed if the given string does not begin with "ERROR: "
func assertError(result string, t *testing.T) {
	if !strings.HasPrefix(result, "ERROR: ") {
		fatalf(t, "Expected error; actual: \"%s\"", result)
	}
}

// assertTrue is a utility method for unit tests that marks a function as
// having succeeded iff the supplied value is true
func assertTrue(value bool, t *testing.T) {
	if value != true {
		fatalf(t, "assertTrue failed")
	}
}

// assertFalse is a utility method for unit tests that marks a function as
// having succeeded iff the supplied value is true
func assertFalse(value bool, t *testing.T) {
	if value != false {
		fatalf(t, "assertFalse failed")
	}
}
