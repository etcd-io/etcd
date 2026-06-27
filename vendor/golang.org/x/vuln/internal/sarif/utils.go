// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sarif

import (
	"strings"

	"golang.org/x/vuln/internal/govulncheck"
)

func choose(s1, s2 string, cond bool) string {
	if cond {
		return s1
	}
	return s2
}

func list(elems []string) string {
	l := len(elems)
	if l == 0 {
		return ""
	}
	if l == 1 {
		return elems[0]
	}

	cList := strings.Join(elems[:l-1], ", ")
	return cList + choose("", ",", l == 2) + " and " + elems[l-1]
}

// symbol is simplified adaptation of internal/scan/symbol.
func symbol(fr *govulncheck.Frame) string {
	if fr.Function == "" {
		return ""
	}
	sym := strings.Split(fr.Function, "$")[0]
	if fr.Receiver != "" {
		sym = fr.Receiver + "." + sym
	}
	if fr.Package != "" {
		sym = fr.Package + "." + sym
	}
	return sym
}
