// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil_test

import (
	"testing"

	"github.com/godoctor/godoctor/refactoring/testutil"
)

func TestTestRefactorings(t *testing.T) {
	// This should pass silently since this directory contains no testdata
	testutil.TestRefactorings(".", t)
}
