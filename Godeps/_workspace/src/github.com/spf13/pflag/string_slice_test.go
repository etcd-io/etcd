// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pflag

import (
	"fmt"
	"strings"
	"testing"
)

func setUpSSFlagSet(ssp *[]string) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.StringSliceVar(ssp, "ss", []string{}, "Command seperated list!")
	return f
}

func TestSS(t *testing.T) {
	var ss []string
	f := setUpSSFlagSet(&ss)

	vals := []string{"one", "two", "4", "3"}
	arg := fmt.Sprintf("--ss=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ss {
		if vals[i] != v {
			t.Fatal("expected ss[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	getSS, err := f.GetStringSlice("ss")
	if err != nil {
		t.Fatal("got an error from GetStringSlice(): %v", err)
	}
	for i, v := range getSS {
		if vals[i] != v {
			t.Fatal("expected ss[%d] to be %s from GetStringSlice but got: %s", i, vals[i], v)
		}
	}
}
