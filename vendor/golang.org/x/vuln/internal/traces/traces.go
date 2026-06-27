// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package traces

import (
	"golang.org/x/vuln/internal/govulncheck"
)

// Compact returns a summarization of finding.Trace. The first
// returned element is the vulnerable symbol and the last element
// is the exit point of the user module. There can also be two
// elements in between, if applicable, which are the two elements
// preceding the user module exit point.
func Compact(finding *govulncheck.Finding) []*govulncheck.Frame {
	if len(finding.Trace) < 1 {
		return nil
	}
	iTop := len(finding.Trace) - 1
	topModule := finding.Trace[iTop].Module
	// search for the exit point of the top module
	for i, frame := range finding.Trace {
		if frame.Module == topModule {
			iTop = i
			break
		}
	}

	if iTop == 0 {
		// all in one module, reset to the end
		iTop = len(finding.Trace) - 1
	}

	compact := []*govulncheck.Frame{finding.Trace[0]}
	if iTop > 1 {
		if iTop > 2 {
			compact = append(compact, finding.Trace[iTop-2])
		}
		compact = append(compact, finding.Trace[iTop-1])
	}
	if iTop > 0 {
		compact = append(compact, finding.Trace[iTop])
	}
	return compact
}
