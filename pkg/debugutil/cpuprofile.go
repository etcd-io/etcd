// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package debugutil

import (
	"net/http"
	"net/http/pprof"
	"strconv"
)

// CPUProfileType tracks whether a CPU profile is in progress.
type CPUProfileType int32

var GlobalCPUProfiling CPUProfileType

const (
	// CPUProfileNone means that no CPU profile is currently taken.
	CPUProfileNone CPUProfileType = iota
	// CPUProfileDefault means that a CPU profile is currently taken, but
	// pprof labels are not enabled.
	CPUProfileDefault
	// CPUProfileWithLabels means that a CPU profile is currently taken and
	// pprof labels are enabled.
	CPUProfileWithLabels
)

// CPUProfileOptions contains options for generating a CPU profile.
type CPUProfileOptions struct {
	// Number of seconds to profile for.
	Seconds int32
	// Whether to enable pprof labels while the profile is taken.
	WithLabels bool
}

// Type returns the CPUProfileType corresponding to the options.
func (opts CPUProfileOptions) Type() CPUProfileType {
	typ := CPUProfileDefault
	if opts.WithLabels {
		typ = CPUProfileWithLabels
	}
	return typ
}

// CPUProfileOptionsFromRequest parses the `seconds` and `labels` fragments
// from the URL and populates CPUProfileOptions from it.
//
// For convenience, `labels` defaults to true, that is, `?labels=false`
// must be specified to disable them. `seconds` defaults to the pprof
// default of 30s.
func CPUProfileOptionsFromRequest(r *http.Request) CPUProfileOptions {
	seconds, err := strconv.ParseInt(r.FormValue("seconds"), 10, 32)
	if err != nil || seconds <= 0 {
		seconds = 30
	}
	// NB: default to using labels unless it's specifically set to false.
	withLabels := r.FormValue("labels") != "false"
	return CPUProfileOptions{
		Seconds:    int32(seconds),
		WithLabels: withLabels,
	}
}

// CPUProfileDo invokes the closure while enabling (and disabling) the supplied
// CPUProfileMode. Errors if the profiling mode could not be set or if do()
// returns an error.
func CPUProfileDo(typ CPUProfileType, do func() error) error {
	GlobalCPUProfiling = typ
	defer func() { GlobalCPUProfiling = CPUProfileNone }()
	return do()
}

// CPUProfileHandler is replacement for `pprof.Profile` that supports additional
// options.
func CPUProfileHandler(w http.ResponseWriter, r *http.Request) {
	opts := CPUProfileOptionsFromRequest(r)
	if err := CPUProfileDo(opts.Type(), func() error {
		pprof.Profile(w, r)
		return nil
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
