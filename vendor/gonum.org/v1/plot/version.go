// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.12
// +build go1.12

package plot

import (
	"fmt"
	"runtime/debug"
)

const root = "gonum.org/v1/plot"

// Version returns the version of Gonum/plot and its checksum. The returned
// values are only valid in binaries built with module support.
//
// If a replace directive exists in the Gonum/plot go.mod, the replace will
// be reported in the version in the following format:
//
//	"version=>[replace-path] [replace-version]"
//
// and the replace sum will be returned in place of the original sum.
//
// The exact version format returned by Version may change in future.
func Version() (version, sum string) {
	b, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	for _, m := range b.Deps {
		if m.Path == root {
			if m.Replace != nil {
				switch {
				case m.Replace.Version != "" && m.Replace.Path != "":
					return fmt.Sprintf("%s=>%s %s", m.Version, m.Replace.Path, m.Replace.Version), m.Replace.Sum
				case m.Replace.Version != "":
					return fmt.Sprintf("%s=>%s", m.Version, m.Replace.Version), m.Replace.Sum
				case m.Replace.Path != "":
					return fmt.Sprintf("%s=>%s", m.Version, m.Replace.Path), m.Replace.Sum
				default:
					return m.Version + "*", m.Sum + "*"
				}
			}
			return m.Version, m.Sum
		}
	}
	return "", ""
}
