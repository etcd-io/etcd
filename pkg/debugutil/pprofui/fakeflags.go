// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package pprofui

import (
	"github.com/google/pprof/driver"
	"github.com/spf13/pflag"
)

// pprofFlags is a wrapper to satisfy pprof's client flag interface.
// That interface is satisfied by what the standard flag package
// offers, with some tweaks. In this package, we just want to specify
// the command line args directly; pprofFlags lets us do that
// essentially by mocking out `os.Args`. `pprof` will register all of
// its flags via this struct, and then they get populated from `args`
// below.
type pprofFlags struct {
	args []string // passed to Parse()
	*pflag.FlagSet
}

var _ driver.FlagSet = &pprofFlags{}

// ExtraUsage is part of the driver.FlagSet interface.
func (pprofFlags) ExtraUsage() string {
	return ""
}

// AddExtraUsage is part of the driver.FlagSet interface.
func (pprofFlags) AddExtraUsage(eu string) {
}

func (f pprofFlags) StringList(o, d, c string) *[]*string {
	return &[]*string{f.String(o, d, c)}
}

func (f pprofFlags) Parse(usage func()) []string {
	f.FlagSet.Usage = usage
	if err := f.FlagSet.Parse(f.args); err != nil {
		panic(err)
	}
	return f.FlagSet.Args()
}
