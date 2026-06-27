// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package multilinediff

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Get the diff between two strings.
func Diff(a, b, lineSep string) (string, int) {
	reporter := Reporter{LineSep: lineSep}
	cmp.Diff(
		a, b,
		cmpopts.AcyclicTransformer("multiline", func(s string) []string {
			return strings.Split(s, lineSep)
		}),
		cmp.Reporter(&reporter),
	)
	return reporter.String(), reporter.DiffCount
}

type diffType int

const (
	diffTypeEqual diffType = iota
	diffTypeChange
	diffTypeAdd
)

type diffLine struct {
	diff diffType
	old  string
	new  string
}

func (l diffLine) toLine(length int) string {
	line := ""

	switch l.diff {
	case diffTypeChange:
		line += "- "
	case diffTypeAdd:
		line += "+ "
	default:
		line += "  "
	}

	line += l.old

	line += strings.Repeat(" ", length-len(l.old))
	line += "  "

	line += l.new

	return line
}

// A pretty reporter to pass into cmp.Diff using the cmd.Reporter function.
type Reporter struct {
	LineSep   string
	DiffCount int

	path  cmp.Path
	lines []diffLine
}

func (r *Reporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

func (r *Reporter) Report(rs cmp.Result) {
	line := diffLine{}
	vOld, vNew := r.path.Last().Values()
	if !rs.Equal() {
		r.DiffCount++
		if vOld.IsValid() {
			line.diff = diffTypeChange
			line.old = fmt.Sprintf("%+v", vOld)
		}
		if vNew.IsValid() {
			if line.diff == diffTypeEqual {
				line.diff = diffTypeAdd
			}
			line.new = fmt.Sprintf("%+v", vNew)
		}
	} else {
		line.old = fmt.Sprintf("%+v", vOld)
		line.new = fmt.Sprintf("%+v", vOld)
	}
	r.lines = append(r.lines, line)
}

func (r *Reporter) PopStep() {
	r.path = r.path[:len(r.path)-1]
}

func (r *Reporter) String() string {
	maxLen := 0
	for _, l := range r.lines {
		if len(l.old) > maxLen {
			maxLen = len(l.old)
		}
	}

	diffLines := []string{}
	for _, l := range r.lines {
		diffLines = append(diffLines, l.toLine(maxLen))
	}

	return strings.Join(diffLines, r.LineSep)
}
