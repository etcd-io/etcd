// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"go/token"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/vuln/internal/govulncheck"
	"golang.org/x/vuln/internal/osv"
	"golang.org/x/vuln/internal/traces"
)

type findingSummary struct {
	*govulncheck.Finding
	Compact string
	OSV     *osv.Entry
}

type summaryCounters struct {
	VulnerabilitiesCalled   int
	ModulesCalled           int
	VulnerabilitiesImported int
	VulnerabilitiesRequired int
	StdlibCalled            bool
}

func fixupFindings(osvs []*osv.Entry, findings []*findingSummary) {
	for _, f := range findings {
		f.OSV = getOSV(osvs, f.Finding.OSV)
	}
}

func groupByVuln(findings []*findingSummary) [][]*findingSummary {
	return groupBy(findings, func(left, right *findingSummary) int {
		return -strings.Compare(left.OSV.ID, right.OSV.ID)
	})
}

func groupByModule(findings []*findingSummary) [][]*findingSummary {
	return groupBy(findings, func(left, right *findingSummary) int {
		return strings.Compare(left.Trace[0].Module, right.Trace[0].Module)
	})
}

func groupBy(findings []*findingSummary, compare func(left, right *findingSummary) int) [][]*findingSummary {
	switch len(findings) {
	case 0:
		return nil
	case 1:
		return [][]*findingSummary{findings}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return compare(findings[i], findings[j]) < 0
	})
	result := [][]*findingSummary{}
	first := 0
	for i, next := range findings {
		if i == first {
			continue
		}
		if compare(findings[first], next) != 0 {
			result = append(result, findings[first:i])
			first = i
		}
	}
	result = append(result, findings[first:])
	return result
}

func isRequired(findings []*findingSummary) bool {
	for _, f := range findings {
		if f.Trace[0].Module != "" {
			return true
		}
	}
	return false
}

func isImported(findings []*findingSummary) bool {
	for _, f := range findings {
		if f.Trace[0].Package != "" {
			return true
		}
	}
	return false
}

func isCalled(findings []*findingSummary) bool {
	for _, f := range findings {
		if f.Trace[0].Function != "" {
			return true
		}
	}
	return false
}

func getOSV(osvs []*osv.Entry, id string) *osv.Entry {
	for _, entry := range osvs {
		if entry.ID == id {
			return entry
		}
	}
	return &osv.Entry{
		ID:               id,
		DatabaseSpecific: &osv.DatabaseSpecific{},
	}
}

func newFindingSummary(f *govulncheck.Finding) *findingSummary {
	return &findingSummary{
		Finding: f,
		Compact: compactTrace(f),
	}
}

// platforms returns a string describing the GOOS, GOARCH,
// or GOOS/GOARCH pairs that the vuln affects for a particular
// module mod. If it affects all of them, it returns the empty
// string.
//
// When mod is an empty string, returns platform information for
// all modules of e.
func platforms(mod string, e *osv.Entry) []string {
	if e == nil {
		return nil
	}
	platforms := map[string]bool{}
	for _, a := range e.Affected {
		if mod != "" && a.Module.Path != mod {
			continue
		}
		for _, p := range a.EcosystemSpecific.Packages {
			for _, os := range p.GOOS {
				// In case there are no specific architectures,
				// just list the os entries.
				if len(p.GOARCH) == 0 {
					platforms[os] = true
					continue
				}
				// Otherwise, list all the os+arch combinations.
				for _, arch := range p.GOARCH {
					platforms[os+"/"+arch] = true
				}
			}
			// Cover the case where there are no specific
			// operating systems listed.
			if len(p.GOOS) == 0 {
				for _, arch := range p.GOARCH {
					platforms[arch] = true
				}
			}
		}
	}
	var keys []string
	for k := range platforms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func posToString(p *govulncheck.Position) string {
	if p == nil || p.Line <= 0 {
		return ""
	}
	return token.Position{
		Filename: AbsRelShorter(p.Filename),
		Offset:   p.Offset,
		Line:     p.Line,
		Column:   p.Column,
	}.String()
}

func symbol(frame *govulncheck.Frame, short bool) string {
	buf := &strings.Builder{}
	addSymbol(buf, frame, short)
	return buf.String()
}

func symbolName(frame *govulncheck.Frame) string {
	buf := &strings.Builder{}
	addSymbolName(buf, frame)
	return buf.String()
}

// compactTrace returns a short description of the call stack.
// It prefers to show you the edge from the top module to other code, along with
// the vulnerable symbol.
// Where the vulnerable symbol directly called by the users code, it will only
// show those two points.
// If the vulnerable symbol is in the users code, it will show the entry point
// and the vulnerable symbol.
func compactTrace(finding *govulncheck.Finding) string {
	compact := traces.Compact(finding)
	if len(compact) == 0 {
		return ""
	}

	l := len(compact)
	iTop := l - 1
	buf := &strings.Builder{}
	topPos := posToString(compact[iTop].Position)
	if topPos != "" {
		buf.WriteString(topPos)
		buf.WriteString(": ")
	}

	if l > 1 {
		// print the root of the compact trace
		addSymbol(buf, compact[iTop], true)
		buf.WriteString(" calls ")
	}
	if l > 2 {
		// print next element of the trace, if any
		addSymbol(buf, compact[iTop-1], true)
		buf.WriteString(", which")
		if l > 3 {
			// don't print the third element, just acknowledge it
			buf.WriteString(" eventually")
		}
		buf.WriteString(" calls ")
	}
	addSymbol(buf, compact[0], true) // print the vulnerable symbol
	return buf.String()
}

// notIdentifier reports whether ch is an invalid identifier character.
func notIdentifier(ch rune) bool {
	return !('a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' ||
		'0' <= ch && ch <= '9' ||
		ch == '_' ||
		ch >= utf8.RuneSelf && (unicode.IsLetter(ch) || unicode.IsDigit(ch)))
}

// importPathToAssumedName is taken from goimports, it works out the natural imported name
// for a package.
// This is used to get a shorter identifier in the compact stack trace
func importPathToAssumedName(importPath string) string {
	base := path.Base(importPath)
	if strings.HasPrefix(base, "v") {
		if _, err := strconv.Atoi(base[1:]); err == nil {
			dir := path.Dir(importPath)
			if dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, notIdentifier); i >= 0 {
		base = base[:i]
	}
	return base
}

func addSymbol(w io.Writer, frame *govulncheck.Frame, short bool) {
	if frame.Function == "" {
		return
	}
	if frame.Package != "" {
		pkg := frame.Package
		if short {
			pkg = importPathToAssumedName(frame.Package)
		}
		io.WriteString(w, pkg)
		io.WriteString(w, ".")
	}
	addSymbolName(w, frame)
}

func addSymbolName(w io.Writer, frame *govulncheck.Frame) {
	if frame.Receiver != "" {
		if frame.Receiver[0] == '*' {
			io.WriteString(w, frame.Receiver[1:])
		} else {
			io.WriteString(w, frame.Receiver)
		}
		io.WriteString(w, ".")
	}
	funcname := strings.Split(frame.Function, "$")[0]
	io.WriteString(w, funcname)
}
