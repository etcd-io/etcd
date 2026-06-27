// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openvex

import (
	"net/url"
	"strings"

	"golang.org/x/vuln/internal/govulncheck"
)

// The PURL is printed as: pkg:golang/MODULE_PATH@VERSION
// Conceptually there is no namespace and the name is entirely defined by
// the module path. See https://github.com/package-url/purl-spec/issues/63
// for further disucssion.

const suffix = "pkg:golang/"

type purl struct {
	name    string
	version string
}

func (p *purl) String() string {
	var b strings.Builder
	b.WriteString(suffix)
	b.WriteString(url.PathEscape(p.name))
	if p.version != "" {
		b.WriteString("@")
		b.WriteString(p.version)
	}
	return b.String()
}

// purlFromFinding takes a govulncheck finding and generates a purl to the
// vulnerable dependency.
func purlFromFinding(f *govulncheck.Finding) string {
	purl := purl{
		name:    f.Trace[0].Module,
		version: f.Trace[0].Version,
	}

	return purl.String()
}
