// Copyright 2016 The CMux Authors. All rights reserved.
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

package cmux

import (
	"strings"
	"testing"
)

func testPTree(t *testing.T, strs ...string) {
	pt := newPatriciaTreeString(strs...)
	for _, s := range strs {
		if !pt.match(strings.NewReader(s)) {
			t.Errorf("%s is not matched by %s", s, s)
		}

		if !pt.matchPrefix(strings.NewReader(s + s)) {
			t.Errorf("%s is not matched as a prefix by %s", s+s, s)
		}

		if pt.match(strings.NewReader(s + s)) {
			t.Errorf("%s matches %s", s+s, s)
		}

		// The following tests are just to catch index out of
		// range and off-by-one errors and not the functionality.
		pt.matchPrefix(strings.NewReader(s[:len(s)-1]))
		pt.match(strings.NewReader(s[:len(s)-1]))
		pt.matchPrefix(strings.NewReader(s + "$"))
		pt.match(strings.NewReader(s + "$"))
	}
}

func TestPatriciaOnePrefix(t *testing.T) {
	testPTree(t, "prefix")
}

func TestPatriciaNonOverlapping(t *testing.T) {
	testPTree(t, "foo", "bar", "dummy")
}

func TestPatriciaOverlapping(t *testing.T) {
	testPTree(t, "foo", "far", "farther", "boo", "ba", "bar")
}
