// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo_test

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	gc "gopkg.in/check.v1"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/juju/loggo"
)

func Test(t *testing.T) {
	gc.TestingT(t)
}

func assertLocation(c *gc.C, msg loggo.TestLogValues, tag string) {
	loc := location(tag)
	c.Assert(msg.Filename, gc.Equals, loc.file)
	c.Assert(msg.Line, gc.Equals, loc.line)
}

// All this location stuff is to avoid having hard coded line numbers
// in the tests.  Any line where as a test writer you want to capture the
// file and line number, add a comment that has `//tag name` as the end of
// the line.  The name must be unique across all the tests, and the test
// will panic if it is not.  This name is then used to read the actual
// file and line numbers.

func location(tag string) Location {
	loc, ok := tagToLocation[tag]
	if !ok {
		panic(fmt.Errorf("tag %q not found", tag))
	}
	return loc
}

type Location struct {
	file string
	line int
}

func (loc Location) String() string {
	return fmt.Sprintf("%s:%d", loc.file, loc.line)
}

var tagToLocation = make(map[string]Location)

func setLocationsForTags(filename string) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if j := strings.Index(line, "//tag "); j >= 0 {
			tag := line[j+len("//tag "):]
			if _, found := tagToLocation[tag]; found {
				panic(fmt.Errorf("tag %q already processed previously"))
			}
			tagToLocation[tag] = Location{file: filename, line: i + 1}
		}
	}
}

func init() {
	setLocationsForTags("logger_test.go")
	setLocationsForTags("writer_test.go")
}
